package api

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/storage"
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
		// to_tsquery with a prefix-aware query so "pack" matches "package".
		if db.Dialector.Name() == "postgres" {
			q = q.Where("a.search_vector @@ to_tsquery('simple', ?)", buildPrefixTsQuery(f.Query))
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
		respond500(c, errRequestContextIncomplete)
		return
	}
	limit, offset := parsePagination(c)
	f := parseAssetFilters(c)

	base := h.applyAssetFilters(h.db.WithContext(c.Request.Context()), lib.ID, f)

	var total int64
	// Clone before adding SELECT/GROUP so Count runs cleanly.
	if err := base.Session(&gorm.Session{}).Distinct("a.id").Count(&total).Error; err != nil {
		respond500(c, err)
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
		// File count and size come from correlated subqueries, not a join +
		// SUM/COUNT: the base query may fan out (e.g. a tag-path filter joins
		// asset_tags many-to-many), and the previous SUM(DISTINCT size_bytes)
		// dedup-by-value silently undercounted whenever two files shared a size.
		// Subqueries are immune to both.
		Select(`a.id, a.name, a.needs_review, a.created_at, a.thumbnail_object_key,
			COALESCE((SELECT COUNT(*) FROM asset_files f WHERE f.asset_version_id = a.current_version_id), 0) AS file_count,
			COALESCE((SELECT SUM(f.size_bytes) FROM asset_files f WHERE f.asset_version_id = a.current_version_id), 0) AS size_bytes,
			COALESCE(cv2.package_kind, '') AS package_kind,
			COALESCE(l2.detected_type, '') AS licence_spdx`).
		Joins("LEFT JOIN asset_versions cv2 ON cv2.id = a.current_version_id").
		Joins("LEFT JOIN licences l2 ON l2.id = a.licence_id").
		Group("a.id, cv2.package_kind, l2.detected_type").
		Order("a.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
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

// importEnqueueResponse is the slim 202 payload returned by POST
// /api/imports and POST /api/imports/:id/retry. The rich asset metadata
// (file count, size, package kind) materializes asynchronously — the SPA
// learns about it via SSE on /api/imports/events or by re-fetching the
// asset.
type importEnqueueResponse struct {
	AssetID  uint `json:"asset_id"`
	ImportID uint `json:"import_id"`
}

// importResponse is the wire shape returned by /reextract — that flow stays
// synchronous for now (re-processing already-stored bytes is fast and the
// caller wants the new version id immediately).
type importResponse struct {
	AssetID        uint   `json:"asset_id"`
	AssetVersionID uint   `json:"asset_version_id"`
	FileCount      int    `json:"file_count"`
	SizeBytes      int64  `json:"size_bytes"`
	PackageKind    string `json:"package_kind"`
}

// postImport accepts a single-file upload via multipart/form-data (`file`
// field), persists the upload blob and a stub asset/version/import row,
// then enqueues the heavy lifting (extract, detect, thumbnail) onto the
// background pool. Returns 202 with the import id so the SPA can subscribe
// to /api/imports/events for stage transitions.
func (h *Handler) postImport(c *gin.Context) {
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
		return
	}
	if h.importPool == nil {
		respond500(c, errImportPoolNotConfigured)
		return
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
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

	filename := filepath.Base(fh.Filename)
	if filename == "" || filename == "." || filename == "/" {
		filename = "upload"
	}
	isUnity := strings.EqualFold(filepath.Ext(filename), ".unitypackage")
	contentType := fh.Header.Get("Content-Type")
	packageKind := packageKindLoose
	if isUnity {
		packageKind = packageKindUnityPackage
	}

	var (
		assetID   uint
		versionID uint
		importID  uint
		libID     = lib.ID
	)
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		asset := models.Asset{
			LibraryID:       libID,
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
			CreatedByUserID: &u.ID,
			PackageKind:     packageKind,
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		imp := models.AssetImport{
			AssetID:   asset.ID,
			Stage:     imports.StageUploaded,
			UpdatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&imp).Error; err != nil {
			return err
		}
		assetID = asset.ID
		versionID = version.ID
		importID = imp.ID
		return nil
	})
	if err != nil {
		respond500(c, err)
		return
	}

	meta := storage.ObjectMeta{
		Filename:  filename,
		LibraryID: libID,
		AssetID:   assetID,
		VersionID: versionID,
		Kind:      packageKind,
	}
	var (
		objectKey string
		size      int64
	)
	if isUnity {
		objectKey, size, err = h.storage.PutOriginal(c.Request.Context(), src, meta)
	} else {
		objectKey, size, err = h.storage.PutBlob(c.Request.Context(), src, meta)
	}
	if err != nil {
		h.cleanupFailedUpload(c.Request.Context(), assetID)
		respond500(c, fmt.Errorf("storage put (library %d): %w", lib.ID, err))
		return
	}

	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AssetVersion{}).
			Where("id = ?", versionID).
			Update("original_package_object_key", objectKey).Error; err != nil {
			return err
		}
		if !isUnity {
			f := models.AssetFile{
				AssetVersionID: versionID,
				RelativePath:   filename,
				SizeBytes:      size,
				ContentHash:    objectKey,
				ObjectKey:      objectKey,
				DetectedType:   contentType,
			}
			if err := tx.Create(&f).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		respond500(c, err)
		return
	}

	h.importPool.Enqueue(imports.Job{ImportID: importID, LibraryID: libID})

	c.JSON(http.StatusAccepted, importEnqueueResponse{AssetID: assetID, ImportID: importID})
}

func (h *Handler) cleanupFailedUpload(ctx context.Context, assetID uint) {
	db := h.db.WithContext(ctx)
	if err := db.Where("asset_id = ?", assetID).Delete(&models.AssetImport{}).Error; err != nil {
		slog.Warn("import: cleanup failed upload (imports)", "err", err, "asset_id", assetID)
	}
	if err := db.Where("asset_id = ?", assetID).Delete(&models.AssetVersion{}).Error; err != nil {
		slog.Warn("import: cleanup failed upload (versions)", "err", err, "asset_id", assetID)
	}
	if err := db.Delete(&models.Asset{}, assetID).Error; err != nil {
		slog.Warn("import: cleanup failed upload (asset)", "err", err, "asset_id", assetID)
	}
}

// postReExtract creates a new AssetVersion by re-reading the original
// .unitypackage archive that was retained when the source version was
// imported. The previous version's rows are untouched; the asset's
// current_version_id switches to the new version.
func (h *Handler) postReExtract(c *gin.Context) {
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
		return
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
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
		respond500(c, err)
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
		respond500(c, err)
		return
	}
	if source.OriginalPackageObjectKey == "" || source.PackageKind != packageKindUnityPackage || source.OriginalPackagePurged {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "source version has no retained package"})
		return
	}

	blob, err := imports.OpenOriginal(c.Request.Context(), h.storage, source)
	if err != nil {
		respond500(c, err)
		return
	}
	defer blob.Close()
	ex := &extract.UnityExtractor{
		Storage: h.storage,
		EntryMeta: storage.ObjectMeta{
			LibraryID: lib.ID,
			AssetID:   asset.ID,
			Kind:      "unitypackage-asset",
		},
	}
	result, err := ex.Extract(c.Request.Context(), blob)
	if err != nil {
		slog.Warn("unity re-extract failed", "err", err, "asset_id", asset.ID, "version_id", source.ID)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "extract: " + err.Error()})
		return
	}

	nextLabel, err := imports.NextVersionLabel(h.db.WithContext(c.Request.Context()), asset.ID)
	if err != nil {
		respond500(c, err)
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

		fileCount, totalBytes, err := imports.WriteAssetFiles(tx, version.ID, "", 0, "", "", result)
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
		respond500(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
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

// buildPrefixTsQuery converts a raw search string into a to_tsquery expression
// where each word is a required (&) term and the last word gets the :* prefix
// operator so partial input like "pack" matches "package". Special tsquery
// characters are stripped to avoid syntax errors.
func buildPrefixTsQuery(raw string) string {
	words := strings.Fields(raw)
	terms := make([]string, 0, len(words))
	for i, w := range words {
		// Strip characters that have special meaning in tsquery syntax.
		w = strings.Map(func(r rune) rune {
			switch r {
			case '\'', '\\', '&', '|', '!', '(', ')', ':', '<', '>':
				return -1
			}
			return r
		}, w)
		if w == "" {
			continue
		}
		if i == len(words)-1 {
			terms = append(terms, w+":*")
		} else {
			terms = append(terms, w)
		}
	}
	return strings.Join(terms, " & ")
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
