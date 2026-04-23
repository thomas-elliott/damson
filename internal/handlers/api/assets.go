package api

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
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
	AssetID        uint  `json:"asset_id"`
	AssetVersionID uint  `json:"asset_version_id"`
	FileID         uint  `json:"file_id"`
	SizeBytes      int64 `json:"size_bytes"`
}

// postImport accepts a single-file upload via multipart/form-data (`file`
// field), streams it to content-addressed storage, and creates the Asset +
// AssetVersion + AssetFile rows in one transaction. No Unity extraction, no
// licence detection, no thumbnails — those land in later milestones.
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

	var resp importResponse
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		asset := models.Asset{
			LibraryID:       lib.ID,
			Name:            assetNameFromFilename(filename),
			NeedsReview:     true,
			CreatedByUserID: &u.ID,
		}
		if err := tx.Create(&asset).Error; err != nil {
			return err
		}

		version := models.AssetVersion{
			AssetID:         asset.ID,
			VersionLabel:    "v1",
			ImportDate:      time.Now().UTC(),
			PackageKind:     "loose",
			CreatedByUserID: &u.ID,
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}

		file := models.AssetFile{
			AssetVersionID: version.ID,
			RelativePath:   filename,
			SizeBytes:      size,
			ContentHash:    objectKey,
			ObjectKey:      objectKey,
			DetectedType:   fh.Header.Get("Content-Type"),
		}
		if err := tx.Create(&file).Error; err != nil {
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
			FileID:         file.ID,
			SizeBytes:      size,
		}
		return nil
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, resp)
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
