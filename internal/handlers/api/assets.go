package api

import (
	"errors"
	"log/slog"
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
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

type assetGridRow struct {
	ID                 uint      `json:"id"`
	Name               string    `json:"name"`
	FileCount          int       `json:"file_count"`
	SizeBytes          int64     `json:"size_bytes"`
	NeedsReview        bool      `json:"needs_review"`
	CreatedAt          time.Time `json:"created_at"`
	ThumbnailObjectKey string    `json:"thumbnail_object_key,omitempty"`
	LicenceSPDX        string    `json:"licence_spdx,omitempty"`
	LicenceStatus      string    `json:"licence_status,omitempty"`
	PackageKind        string    `json:"package_kind,omitempty"`
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

// assetListFilters carries the decoded query params for getAssets. Empty
// zero values mean "no filter for this dimension".
type assetListFilters struct {
	Query         string // full-text / LIKE fallback
	TagID         *uint  // matches asset via asset_tags OR descendants by path
	TagPath       string // alternative to TagID — match by `path LIKE '<p>%'`
	LicenceStatus string // ok|warn|err|unk
	LicenceSPDX   string // exact SPDX id
	ProjectID     *uint  // asset is pinned in this project
	CollectionID  *uint  // asset is a member of this collection
	SourceHost    string // exact-match host portion of source_url
	NeedsReview   *bool  // unreviewed-only when true
	PackageKind   string
}

func parseAssetFilters(c *gin.Context) assetListFilters {
	f := assetListFilters{
		Query:         strings.TrimSpace(c.Query("q")),
		TagPath:       strings.TrimSpace(c.Query("tag_path")),
		LicenceStatus: strings.TrimSpace(c.Query("licence_status")),
		LicenceSPDX:   strings.TrimSpace(c.Query("licence_spdx")),
		SourceHost:    strings.TrimSpace(c.Query("source_host")),
		PackageKind:   strings.TrimSpace(c.Query("package_kind")),
	}
	if s := c.Query("tag_id"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil && n > 0 {
			v := uint(n)
			f.TagID = &v
		}
	}
	if s := c.Query("project_id"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil && n > 0 {
			v := uint(n)
			f.ProjectID = &v
		}
	}
	if s := c.Query("collection_id"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil && n > 0 {
			v := uint(n)
			f.CollectionID = &v
		}
	}
	if s := c.Query("needs_review"); s != "" {
		if s == "true" || s == "1" {
			t := true
			f.NeedsReview = &t
		} else if s == "false" || s == "0" {
			fl := false
			f.NeedsReview = &fl
		}
	}
	return f
}

// applyAssetFilters builds the shared WHERE/JOIN shape used by both the
// list endpoint and the search endpoint. Callers get a GORM query they can
// extend (pagination, ordering). Returns the `bool` `usedTsvector` so the
// caller knows whether a tsvector-rank ORDER BY is valid.
func (h *Handler) applyAssetFilters(db *gorm.DB, libID uint, f assetListFilters) *gorm.DB {
	q := db.Table("assets AS a").Where("a.library_id = ?", libID)

	if f.NeedsReview != nil {
		q = q.Where("a.needs_review = ?", *f.NeedsReview)
	}
	if f.PackageKind != "" {
		q = q.Joins("JOIN asset_versions cv ON cv.id = a.current_version_id").
			Where("cv.package_kind = ?", f.PackageKind)
	}
	if f.SourceHost != "" {
		// LIKE host match; source_url may not be parsed to host yet, so
		// match substring. Good enough for v1; URL parsing is a future
		// refinement. Escape `%` and `_` so user input doesn't become
		// an unintended wildcard. ESCAPE clause works on Postgres + SQLite.
		q = q.Where(`a.source_url LIKE ? ESCAPE '\'`, "%"+escapeLikePattern(f.SourceHost)+"%")
	}
	if f.LicenceSPDX != "" || f.LicenceStatus != "" {
		q = q.Joins("LEFT JOIN licences l ON l.id = a.licence_id")
		if f.LicenceSPDX != "" {
			q = q.Where("l.detected_type = ?", f.LicenceSPDX)
		}
		if f.LicenceStatus != "" {
			// Translate the requested status into the matching SPDX set so
			// the filter runs in SQL. Keeps Total honest and stops pages
			// shrinking after pagination has been applied. quickLicenceStatus
			// only depends on detected_type, so the mapping is deterministic
			// without JSONB inspection.
			ok, errSet, unkExtras := licenceStatusSPDXBuckets()
			switch f.LicenceStatus {
			case "ok":
				q = q.Where("l.detected_type IN ?", ok)
			case "err":
				q = q.Where("l.detected_type IN ?", errSet)
			case "unk":
				// no licence row OR detected_type is unknown/empty
				q = q.Where("l.detected_type IS NULL OR l.detected_type = '' OR l.detected_type IN ?", unkExtras)
			case "warn":
				// licence row exists, type is set, and not in ok/err/unk lists.
				q = q.Where("l.detected_type IS NOT NULL AND l.detected_type <> '' AND l.detected_type NOT IN ? AND l.detected_type NOT IN ? AND l.detected_type NOT IN ?", ok, errSet, unkExtras)
			}
		}
	}
	if f.TagID != nil {
		q = q.Joins("JOIN asset_tags atf ON atf.asset_id = a.id").
			Where("atf.tag_id = ?", *f.TagID)
	} else if f.TagPath != "" {
		// Match tag + descendants.
		path := normaliseTagPath(f.TagPath)
		q = q.Joins("JOIN asset_tags atf ON atf.asset_id = a.id").
			Joins("JOIN tags tf ON tf.id = atf.tag_id").
			Where("tf.library_id = ? AND (tf.path = ? OR tf.path LIKE ?)", libID, path, path+"/%")
	}
	if f.ProjectID != nil {
		q = q.Joins("JOIN project_asset_pins pf ON pf.asset_id = a.id").
			Where("pf.project_id = ?", *f.ProjectID)
	}
	if f.CollectionID != nil {
		q = q.Joins("JOIN collection_assets caf ON caf.asset_id = a.id").
			Where("caf.collection_id = ?", *f.CollectionID)
	}
	if f.Query != "" {
		// SQLite fallback: LIKE over name/description. Postgres: use
		// plainto_tsquery against the search_vector column. DB driver name
		// is the switch.
		if db.Dialector.Name() == "postgres" {
			q = q.Where("a.search_vector @@ plainto_tsquery('simple', ?)", f.Query)
		} else {
			like := "%" + f.Query + "%"
			q = q.Where("(a.name LIKE ? OR a.description LIKE ?)", like, like)
		}
	}
	return q
}

// getAssets lists assets in the current library with optional filter and
// search. Composable: every filter param ANDs with the others.
func (h *Handler) getAssets(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c)
		return
	}
	limit, offset := parsePagination(c)
	f := parseAssetFilters(c)

	base := h.applyAssetFilters(h.db.WithContext(c.Request.Context()), lib.ID, f)

	var total int64
	// Clone before adding SELECT/GROUP so Count runs cleanly.
	if err := base.Session(&gorm.Session{}).Distinct("a.id").Count(&total).Error; err != nil {
		respond500(c)
		return
	}

	type row struct {
		ID                 uint
		Name               string
		NeedsReview        bool
		CreatedAt          time.Time
		ThumbnailObjectKey string
		FileCount          int
		SizeBytes          int64
		PackageKind        string
		LicenceSPDX        string
	}
	var rows []row
	err := base.Session(&gorm.Session{}).
		Select(`a.id, a.name, a.needs_review, a.created_at, a.thumbnail_object_key,
			COALESCE(COUNT(DISTINCT ffc.id), 0) AS file_count,
			COALESCE(SUM(DISTINCT ffc.size_bytes), 0) AS size_bytes,
			COALESCE(cv2.package_kind, '') AS package_kind,
			COALESCE(l2.detected_type, '') AS licence_spdx`).
		Joins("LEFT JOIN asset_files ffc ON ffc.asset_version_id = a.current_version_id").
		Joins("LEFT JOIN asset_versions cv2 ON cv2.id = a.current_version_id").
		Joins("LEFT JOIN licences l2 ON l2.id = a.licence_id").
		Group("a.id, cv2.package_kind, l2.detected_type").
		Order("a.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		respond500(c)
		return
	}

	out := make([]assetGridRow, 0, len(rows))
	for _, r := range rows {
		// Status is derived for display; the SQL filter (above) has already
		// picked the matching rows. Empty SPDX → "unk".
		status := quickLicenceStatus(r.LicenceSPDX)
		out = append(out, assetGridRow{
			ID:                 r.ID,
			Name:               r.Name,
			FileCount:          r.FileCount,
			SizeBytes:          r.SizeBytes,
			NeedsReview:        r.NeedsReview,
			CreatedAt:          r.CreatedAt,
			ThumbnailObjectKey: r.ThumbnailObjectKey,
			PackageKind:        r.PackageKind,
			LicenceSPDX:        r.LicenceSPDX,
			LicenceStatus:      status,
		})
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
func (h *Handler) postImport(c *gin.Context) {
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
		// No asset_id yet — log only; the user gets a 500 and will retry.
		slog.Warn("storage put failed", "err", err, "library_id", lib.ID)
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
		defer blob.Close()
		ex := &extract.UnityExtractor{Storage: h.storage}
		res, xerr := ex.Extract(c.Request.Context(), blob)
		if xerr != nil {
			slog.Warn("unity extract failed", "err", xerr, "filename", filename, "size", size, "library_id", lib.ID)
			// Record the failure as a stub asset + failed import so the
			// queue surface can show + retry it. Without an asset_id we'd
			// have nowhere to attach the row, so create a minimal asset
			// pointing at the retained archive blob.
			h.recordFailedUploadImport(c, lib.ID, &u.ID, filename, objectKey, true, "extract: "+xerr.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "extract: " + xerr.Error()})
			return
		}
		extracted = res
	}

	// M6 thumbnail pipeline. Unity packages already harvest preview.png during
	// extraction (services/extract); only run the generator if no preview was
	// found. Loose uploads always go through the generator with the file's
	// reported content type and filename. Failures are logged and ignored —
	// thumbnails are never mandatory; the SPA falls back to a file-type icon.
	thumbKey, thumbType := "", ""
	if extracted != nil {
		thumbKey = extracted.ThumbObjectKey
		thumbType = extracted.ThumbContentType
	}
	if thumbKey == "" {
		settings, sErr := loadOrCreateLibrarySettings(h.db.WithContext(c.Request.Context()), lib.ID)
		if sErr != nil {
			slog.Warn("thumbnail: load settings", "err", sErr, "library_id", lib.ID)
		} else if !isUnity || settings.ThumbnailsUnityPreview {
			toggles := thumbnail.Toggles{
				Raster: settings.ThumbnailsRasterEnabled,
				Audio:  settings.ThumbnailsAudioEnabled,
				Font:   settings.ThumbnailsFontEnabled,
			}
			blob, bErr := h.storage.GetBlob(c.Request.Context(), objectKey)
			if bErr != nil {
				slog.Warn("thumbnail: get blob", "err", bErr, "object_key", objectKey)
			} else {
				gen := &thumbnail.Generator{Storage: h.storage}
				tres, tErr := gen.Generate(c.Request.Context(), thumbnail.Input{
					Reader:      blob,
					Filename:    filename,
					ContentType: fh.Header.Get("Content-Type"),
				}, toggles)
				blob.Close()
				if tErr != nil {
					slog.Warn("thumbnail: generate", "err", tErr, "filename", filename)
				} else if tres.ObjectKey != "" {
					thumbKey = tres.ObjectKey
					thumbType = tres.ContentType
				}
			}
		}
	}

	var resp importResponse
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		asset := models.Asset{
			LibraryID:       lib.ID,
			Name:            assetNameFromFilename(filename),
			NeedsReview:     true,
			CreatedByUserID: &u.ID,
		}
		if thumbKey != "" {
			asset.ThumbnailObjectKey = thumbKey
			asset.ThumbnailContentType = thumbType
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
func (h *Handler) postReExtract(c *gin.Context) {
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
	defer blob.Close()
	ex := &extract.UnityExtractor{Storage: h.storage}
	result, err := ex.Extract(c.Request.Context(), blob)
	if err != nil {
		slog.Warn("unity re-extract failed", "err", err, "asset_id", asset.ID, "version_id", source.ID)
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

// recordFailedUploadImport persists a stub asset + a failed import row when
// an upload makes it to storage but the post-upload pipeline (Unity extract,
// validation, etc.) bombs out before any asset_files exist. The row gives
// the queue UI something to show and a handle for retry. Failures here are
// best-effort logged: the user is already getting an error response, so we
// don't escalate further.
func (h *Handler) recordFailedUploadImport(c *gin.Context, libID uint, userID *uint, filename, objectKey string, isUnity bool, reason string) {
	tx := h.db.WithContext(c.Request.Context())
	asset := models.Asset{
		LibraryID:       libID,
		Name:            assetNameFromFilename(filename),
		NeedsReview:     true,
		CreatedByUserID: userID,
	}
	if err := tx.Create(&asset).Error; err != nil {
		slog.Warn("record failed import: create asset", "err", err)
		return
	}
	v := models.AssetVersion{
		AssetID:         asset.ID,
		VersionLabel:    "v1",
		ImportDate:      time.Now().UTC(),
		CreatedByUserID: userID,
	}
	if isUnity {
		v.PackageKind = packageKindUnityPackage
		v.OriginalPackageObjectKey = objectKey
	} else {
		v.PackageKind = packageKindLoose
	}
	if err := tx.Create(&v).Error; err != nil {
		slog.Warn("record failed import: create version", "err", err)
	}
	imp := models.AssetImport{
		AssetID:   asset.ID,
		Stage:     "failed",
		Error:     reason,
		UpdatedAt: time.Now().UTC(),
	}
	if err := tx.Create(&imp).Error; err != nil {
		slog.Warn("record failed import: create import row", "err", err)
	}
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

// escapeLikePattern doubles up `%` and `_` characters and the escape `\` so
// raw user input passed into a `LIKE ? ESCAPE '\'` clause matches literally.
// Both Postgres and SQLite recognise the explicit ESCAPE clause.
func escapeLikePattern(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' || c == '_' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

// licenceStatusSPDXBuckets returns the SPDX-id sets that map to "ok", "err",
// and "unk" status respectively. Mirrors quickLicenceStatus / projects.go's
// status switch — keep these in sync. "warn" is the complement of the
// other three; the SQL filter expresses it as "not in any of these".
func licenceStatusSPDXBuckets() (ok, errSet, unkExtras []string) {
	ok = []string{
		"MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "Zlib", "ISC",
		"CC-BY-4.0", "CC-BY-3.0", "CC0-1.0", "OFL-1.1", "Unlicense",
	}
	errSet = []string{"CC-BY-NC-4.0", "CC-BY-NC-3.0", "CC-BY-NC-SA-4.0"}
	unkExtras = []string{"unknown"}
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
