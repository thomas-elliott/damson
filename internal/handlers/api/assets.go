package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/extract"
)

type assetGridRow struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	FileCount   int       `json:"file_count"`
	SizeBytes   int64     `json:"size_bytes"`
	NeedsReview bool      `json:"needs_review"`
	CreatedAt   time.Time `json:"created_at"`
}

type assetListResponse struct {
	Assets []assetGridRow `json:"assets"`
	Total  int64          `json:"total"`
}

const (
	defaultLimit = 50
	maxLimit     = 200

	packageKindLoose        = "loose"
	packageKindUnityPackage = "unitypackage"
)

// getAssets lists assets in the current library, newest first, paginated.
// Deliberately minimal for the first-pass end-to-end slice — no filter, no
// search, no licence pill. Those join in at M3/M5 per docs/technical-design.md §21.
func (h *handler) getAssets(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	limit, offset := parsePagination(c)

	var total int64
	if err := h.db.WithContext(c.Request.Context()).
		Model(&models.Asset{}).
		Where("library_id = ?", lib.ID).
		Count(&total).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// One query, joined to asset_files via the asset's current version, so
	// file_count / size_bytes are computed in SQL rather than N+1 lookups.
	type row struct {
		ID          uint
		Name        string
		NeedsReview bool
		CreatedAt   time.Time
		FileCount   int
		SizeBytes   int64
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("assets AS a").
		Select(`a.id, a.name, a.needs_review, a.created_at,
			COALESCE(COUNT(f.id), 0) AS file_count,
			COALESCE(SUM(f.size_bytes), 0) AS size_bytes`).
		Joins("LEFT JOIN asset_files f ON f.asset_version_id = a.current_version_id").
		Where("a.library_id = ?", lib.ID).
		Group("a.id").
		Order("a.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	out := make([]assetGridRow, len(rows))
	for i, r := range rows {
		out[i] = assetGridRow{
			ID:          r.ID,
			Name:        r.Name,
			FileCount:   r.FileCount,
			SizeBytes:   r.SizeBytes,
			NeedsReview: r.NeedsReview,
			CreatedAt:   r.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, assetListResponse{Assets: out, Total: total})
}

type importResponse struct {
	AssetID        uint   `json:"asset_id"`
	AssetVersionID uint   `json:"asset_version_id"`
	ImportID       uint   `json:"import_id"`
	FileCount      int    `json:"file_count"`
	SizeBytes      int64  `json:"size_bytes"`
	PackageKind    string `json:"package_kind"`
}

// postImport accepts a single-file upload via multipart/form-data (`file`
// field) and creates an asset + first version. `.unitypackage` uploads are
// extracted in-process: every inner file becomes an asset_files row, the
// original archive is retained, and an embedded preview.png (if any) is
// harvested as the asset's thumbnail.
func (h *handler) postImport(c *gin.Context) {
	if h.storage == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "storage not configured"})
		return
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing 'file' form field"})
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	objectKey, size, err := h.storage.PutBlob(c.Request.Context(), src)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "storage put failed"})
		return
	}

	filename := filepath.Base(fh.Filename)
	if filename == "" || filename == "." || filename == "/" {
		filename = "upload"
	}

	isUnity := strings.EqualFold(filepath.Ext(filename), ".unitypackage")

	// Run Unity extraction outside the DB transaction — it streams inner
	// blobs to storage, which may take noticeable time. If extraction fails,
	// the uploaded archive blob is still in storage (content-addressed; GC
	// picks it up later) but no DB rows exist.
	var extracted *extract.UnityResult
	if isUnity {
		blob, gerr := h.storage.GetBlob(c.Request.Context(), objectKey)
		if gerr != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		ex := &extract.UnityExtractor{Storage: h.storage}
		res, xerr := ex.Extract(c.Request.Context(), blob)
		_ = blob.Close()
		if xerr != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "extract: " + xerr.Error()})
			return
		}
		extracted = res
	}

	var resp importResponse
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		asset := models.Asset{
			LibraryID:       lib.ID,
			Name:            assetNameFromFilename(filename),
			NeedsReview:     true,
			CreatedByUserID: &u.ID,
		}
		if extracted != nil && extracted.ThumbObjectKey != "" {
			asset.ThumbnailObjectKey = extracted.ThumbObjectKey
			asset.ThumbnailContentType = extracted.ThumbContentType
		}
		if err := tx.Create(&asset).Error; err != nil {
			return err
		}

		version := models.AssetVersion{
			AssetID:         asset.ID,
			VersionLabel:    "v1",
			ImportDate:      time.Now().UTC(),
			CreatedByUserID: &u.ID,
		}
		if isUnity {
			version.PackageKind = packageKindUnityPackage
			version.OriginalPackageObjectKey = objectKey
		} else {
			version.PackageKind = packageKindLoose
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}

		fileCount, totalBytes, err := writeAssetFiles(tx, version.ID, objectKey, size, filename, fh.Header.Get("Content-Type"), extracted)
		if err != nil {
			return err
		}

		asset.CurrentVersionID = &version.ID
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}

		imp := models.AssetImport{
			AssetID:   asset.ID,
			Stage:     "done",
			UpdatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&imp).Error; err != nil {
			return err
		}

		resp = importResponse{
			AssetID:        asset.ID,
			AssetVersionID: version.ID,
			ImportID:       imp.ID,
			FileCount:      fileCount,
			SizeBytes:      totalBytes,
			PackageKind:    version.PackageKind,
		}
		return nil
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// postReExtract creates a new AssetVersion by re-reading the original
// .unitypackage archive that was retained when the source version was
// imported. The previous version's rows are untouched; the asset's
// current_version_id switches to the new version.
func (h *handler) postReExtract(c *gin.Context) {
	if h.storage == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "storage not configured"})
		return
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	assetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	versionID, ok := parseIDParam(c, "vid")
	if !ok {
		return
	}

	var asset models.Asset
	err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", assetID, lib.ID).
		First(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var source models.AssetVersion
	err = h.db.WithContext(c.Request.Context()).
		Where("id = ? AND asset_id = ?", versionID, asset.ID).
		First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if source.OriginalPackageObjectKey == "" || source.PackageKind != packageKindUnityPackage {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "source version has no retained package"})
		return
	}

	blob, err := h.storage.GetBlob(c.Request.Context(), source.OriginalPackageObjectKey)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	ex := &extract.UnityExtractor{Storage: h.storage}
	result, err := ex.Extract(c.Request.Context(), blob)
	_ = blob.Close()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "extract: " + err.Error()})
		return
	}

	nextLabel, err := nextVersionLabel(h.db.WithContext(c.Request.Context()), asset.ID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var resp importResponse
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		version := models.AssetVersion{
			AssetID:                  asset.ID,
			VersionLabel:             nextLabel,
			ImportDate:               time.Now().UTC(),
			PackageKind:              packageKindUnityPackage,
			OriginalPackageObjectKey: source.OriginalPackageObjectKey,
			CreatedByUserID:          &u.ID,
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}

		fileCount, totalBytes, err := writeAssetFiles(tx, version.ID, "", 0, "", "", result)
		if err != nil {
			return err
		}

		asset.CurrentVersionID = &version.ID
		if result.ThumbObjectKey != "" {
			asset.ThumbnailObjectKey = result.ThumbObjectKey
			asset.ThumbnailContentType = result.ThumbContentType
		}
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}

		resp = importResponse{
			AssetID:        asset.ID,
			AssetVersionID: version.ID,
			FileCount:      fileCount,
			SizeBytes:      totalBytes,
			PackageKind:    version.PackageKind,
		}
		return nil
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// writeAssetFiles creates asset_files rows for a version. If extracted is
// non-nil, each extracted file becomes a row; otherwise a single row is
// written for the loose upload. Returns (count, totalBytes).
func writeAssetFiles(tx *gorm.DB, versionID uint, looseObjectKey string, looseSize int64, looseFilename, looseContentType string, extracted *extract.UnityResult) (int, int64, error) {
	if extracted == nil {
		file := models.AssetFile{
			AssetVersionID: versionID,
			RelativePath:   looseFilename,
			SizeBytes:      looseSize,
			ContentHash:    looseObjectKey,
			ObjectKey:      looseObjectKey,
			DetectedType:   looseContentType,
		}
		if err := tx.Create(&file).Error; err != nil {
			return 0, 0, err
		}
		return 1, looseSize, nil
	}

	var total int64
	rows := make([]models.AssetFile, 0, len(extracted.Files))
	for _, f := range extracted.Files {
		rows = append(rows, models.AssetFile{
			AssetVersionID: versionID,
			RelativePath:   f.RelativePath,
			SizeBytes:      f.SizeBytes,
			ContentHash:    f.ContentHash,
			ObjectKey:      f.ObjectKey,
			UnityGUID:      f.GUID,
		})
		total += f.SizeBytes
	}
	if len(rows) == 0 {
		return 0, 0, nil
	}
	if err := tx.Create(&rows).Error; err != nil {
		return 0, 0, err
	}
	return len(rows), total, nil
}

// nextVersionLabel returns the next `v<N>` label for an asset by counting
// existing versions. The scheme is simple and stable for v1; when users
// supply upstream-derived labels, this becomes a fallback.
func nextVersionLabel(db *gorm.DB, assetID uint) (string, error) {
	var n int64
	if err := db.Model(&models.AssetVersion{}).Where("asset_id = ?", assetID).Count(&n).Error; err != nil {
		return "", err
	}
	return "v" + strconv.FormatInt(n+1, 10), nil
}

func parseIDParam(c *gin.Context, key string) (uint, bool) {
	s := c.Param(key)
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid " + key})
		return 0, false
	}
	return uint(n), true
}

func parsePagination(c *gin.Context) (limit, offset int) {
	limit = defaultLimit
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= maxLimit {
			limit = n
		}
	}
	if s := c.Query("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

// assetNameFromFilename strips the extension so the default asset name is a
// user-friendly label. Users can rename after import.
func assetNameFromFilename(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" {
		base = filename
	}
	return base
}
