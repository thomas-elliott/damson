package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/extract"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/services/tags"
)

type assetGridRow struct {
	ID                 uint       `json:"id"`
	Name               string     `json:"name"`
	FileCount          int        `json:"file_count"`
	SizeBytes          int64      `json:"size_bytes"`
	NeedsReview        bool       `json:"needs_review"`
	CreatedAt          time.Time  `json:"created_at"`
	ThumbnailObjectKey string     `json:"thumbnail_object_key,omitempty"`
	LicenceSPDX        string     `json:"licence_spdx,omitempty"`
	LicenceStatus      string     `json:"licence_status,omitempty"`
	PackageKind        string     `json:"package_kind,omitempty"`
	Publisher          string     `json:"publisher,omitempty"`
	ReleasedAt         *time.Time `json:"released_at,omitempty"`
	UpdateAvailable    bool       `json:"update_available,omitempty"`
	IsReference        bool       `json:"is_reference,omitempty"`
	Store              string     `json:"store,omitempty"`
	SourceURL          string     `json:"source_url,omitempty"`
	StoreDeprecated    bool       `json:"store_deprecated,omitempty"`
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
	packageKindZip          = "zip"

	// MaxBytesReader caps the whole multipart body, not just the file, so the
	// limit needs headroom for the boundary and part-header framing.
	multipartOverheadBytes = 1 << 20
)

// assetListFilters carries the decoded query params for getAssets. Empty
// zero values mean "no filter for this dimension".
type assetListFilters struct {
	Query           string // full-text / LIKE fallback
	TagID           *uint  // matches asset via asset_tags OR descendants by path
	TagPath         string // alternative to TagID — match by `path LIKE '<p>%'`
	LicenceStatus   string // ok|warn|err|unk
	LicenceSPDX     string // exact SPDX id
	ProjectID       *uint  // asset is pinned in this project
	CollectionID    *uint  // asset is a member of this collection
	SourceHost      string // exact-match host portion of source_url
	NeedsReview     *bool  // unreviewed-only when true
	StoreDeprecated *bool  // store_deprecated matches this when set
	PackageKind     string
	Publisher       string     // exact publisher match
	Store           string     // exact store match
	ReleasedAfter   *time.Time // released_at >= this instant
	UpdatesOnly     bool       // latest_store_version is newer than every version held
	NotDownloaded   bool       // Purchased, not downloaded: a reference asset with no path
	Sort            string     // created_desc (default) | released_desc | released_asc | name_asc
}

// storeVersionArraySQL renders a dotted-numeric store version as an int[] for
// semantic ordering, or NULL when it isn't orderable. The CASE guards the cast:
// a non-numeric segment would otherwise abort the whole query, and WHERE-clause
// conjuncts have no guaranteed evaluation order. Trailing .0 segments are
// trimmed first so 1.0 and 1 compare equal, matching
// imports.CompareStoreVersions.
func storeVersionArraySQL(column string) string {
	return `CASE WHEN ` + column + ` ~ '^[0-9]+(\.[0-9]+)*$'
		THEN string_to_array(regexp_replace(` + column + `, '(\.0)+$', ''), '.')::int[] END`
}

// updatesAvailableSQL matches assets whose latest known store version is newer
// than the highest version they hold. A NULL on either side — nothing fetched,
// or no orderable version held — yields NULL, so the asset is not matched.
// A deprecated listing is left out: the store won't publish a newer version.
// Mirrors storeUpdateAvailable, which drives the badge.
func updatesAvailableSQL() string {
	return `NOT a.store_deprecated AND ` + storeVersionArraySQL("a.latest_store_version") + ` > (
		SELECT MAX(` + storeVersionArraySQL("v.store_version") + `)
		FROM asset_versions v WHERE v.asset_id = a.id)`
}

// assetSortColumns maps the sort query values to their ORDER BY clause. Only
// these values are accepted, so the clause is never built from user input.
// Release-date sorts put assets with no release date last either way. Each
// clause ends on a.id so tied rows keep one order across LIMIT/OFFSET pages.
var assetSortColumns = map[string]string{
	"created_desc":  "a.created_at DESC, a.id DESC",
	"created_asc":   "a.created_at ASC, a.id ASC",
	"released_desc": "a.released_at DESC NULLS LAST, a.id DESC",
	"released_asc":  "a.released_at ASC NULLS LAST, a.id ASC",
	"name_asc":      "a.name ASC, a.id ASC",
}

const defaultAssetSort = "created_desc"

// assetOrderClause resolves a sort value, falling back to the default for
// anything unrecognised.
func assetOrderClause(sort string) string {
	if clause, ok := assetSortColumns[sort]; ok {
		return clause
	}
	return assetSortColumns[defaultAssetSort]
}

func parseAssetFilters(c *gin.Context) assetListFilters {
	f := assetListFilters{
		Query:         strings.TrimSpace(c.Query("q")),
		TagPath:       strings.TrimSpace(c.Query("tag_path")),
		LicenceStatus: strings.TrimSpace(c.Query("licence_status")),
		LicenceSPDX:   strings.TrimSpace(c.Query("licence_spdx")),
		SourceHost:    strings.TrimSpace(c.Query("source_host")),
		PackageKind:   strings.TrimSpace(c.Query("package_kind")),
		Publisher:     strings.TrimSpace(c.Query("publisher")),
		Store:         strings.TrimSpace(c.Query("store")),
		Sort:          strings.TrimSpace(c.Query("sort")),
	}
	if s := c.Query("updates_available"); s == "true" || s == "1" {
		f.UpdatesOnly = true
	}
	if s := c.Query("not_downloaded"); s == "true" || s == "1" {
		f.NotDownloaded = true
	}
	if s := strings.TrimSpace(c.Query("released_after")); s != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, s); err == nil {
				utc := t.UTC()
				f.ReleasedAfter = &utc
				break
			}
		}
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
	f.NeedsReview = optionalBool(c.Query("needs_review"))
	f.StoreDeprecated = optionalBool(c.Query("store_deprecated"))
	return f
}

// optionalBool reads a true/false query value; anything else reads as unset.
func optionalBool(s string) *bool {
	switch s {
	case "true", "1":
		return new(true)
	case "false", "0":
		return new(false)
	}
	return nil
}

// storeUpdateAvailable is the badge's half of updatesAvailableSQL: a newer
// store version than any held, unless the listing is deprecated.
func storeUpdateAvailable(deprecated bool, latestStoreVersion string, heldVersions []string) bool {
	if deprecated {
		return false
	}
	available, _ := imports.StoreUpdateAvailable(latestStoreVersion, heldVersions)
	return available
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
	if f.StoreDeprecated != nil {
		q = q.Where("a.store_deprecated = ?", *f.StoreDeprecated)
	}
	if f.PackageKind != "" {
		q = q.Joins("JOIN asset_versions cv ON cv.id = a.current_version_id").
			Where("cv.package_kind = ?", f.PackageKind)
	}
	if f.SourceHost != "" {
		// LIKE host match; source_url may not be parsed to host yet, so
		// match substring. Good enough for v1; URL parsing is a future
		// refinement. Escape `%` and `_` so user input doesn't become
		// an unintended wildcard.
		q = q.Where(`a.source_url LIKE ? ESCAPE '\'`, "%"+escapeLikePattern(f.SourceHost)+"%")
	}
	if f.Publisher != "" {
		q = q.Where("a.publisher = ?", f.Publisher)
	}
	if f.Store != "" {
		q = q.Where("a.store = ?", f.Store)
	}
	if f.ReleasedAfter != nil {
		q = q.Where("a.released_at >= ?", *f.ReleasedAfter)
	}
	if f.UpdatesOnly {
		q = q.Where(updatesAvailableSQL())
	}
	if f.NotDownloaded {
		q = q.Where("a.is_reference AND a.location_path IS NULL")
	}
	if f.LicenceSPDX != "" || f.LicenceStatus != "" {
		q = q.Joins("LEFT JOIN licences l ON l.id = a.licence_id")
		if f.LicenceSPDX != "" {
			q = q.Where("l.detected_type = ?", f.LicenceSPDX)
		}
		if f.LicenceStatus != "" {
			q = q.Where(licenceStatusSQL("l")+" = ?", permissiveLicences, f.LicenceStatus)
		}
	}
	if f.TagID != nil {
		q = q.Joins("JOIN asset_tags atf ON atf.asset_id = a.id").
			Where("atf.tag_id = ?", *f.TagID)
	} else if f.TagPath != "" {
		// Match tag + descendants.
		path := tags.NormalisePath(f.TagPath)
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
		// Prefix-aware to_tsquery, so "pack" matches "package".
		// The LIKE branch is unreachable: Postgres is the only database, in
		// tests too.
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
		LicenceStatus      string
		Publisher          *string
		ReleasedAt         *time.Time
		LatestStoreVersion *string
		StoreVersions      string
		IsReference        bool
		Store              *string
		SourceURL          string
		StoreDeprecated    bool
	}
	var rows []row
	err := base.Session(&gorm.Session{}).
		// File count and size come from correlated subqueries, not a join +
		// SUM/COUNT: the base query may fan out (e.g. a tag-path filter joins
		// asset_tags many-to-many), and the previous SUM(DISTINCT size_bytes)
		// dedup-by-value silently undercounted whenever two files shared a size.
		// Subqueries are immune to both.
		Select(`a.id, a.name, a.needs_review, a.created_at, a.thumbnail_object_key,
			a.publisher, a.released_at, a.latest_store_version, a.is_reference,
			a.store, a.source_url, a.store_deprecated,
			COALESCE((SELECT string_agg(v3.store_version, ',') FROM asset_versions v3
				WHERE v3.asset_id = a.id AND v3.store_version <> ''), '') AS store_versions,
			COALESCE((SELECT COUNT(*) FROM asset_files f WHERE f.asset_version_id = a.current_version_id), 0) AS file_count,
			COALESCE((SELECT SUM(f.size_bytes) FROM asset_files f WHERE f.asset_version_id = a.current_version_id), 0) AS size_bytes,
			COALESCE(cv2.package_kind, '') AS package_kind,
			COALESCE(l2.detected_type, '') AS licence_spdx,
			`+licenceStatusSQL("l2")+` AS licence_status`, permissiveLicences).
		Joins("LEFT JOIN asset_versions cv2 ON cv2.id = a.current_version_id").
		Joins("LEFT JOIN licences l2 ON l2.id = a.licence_id").
		Group("a.id, cv2.package_kind, l2.detected_type, l2.attributes_json").
		Order(assetOrderClause(f.Sort)).
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
		return
	}

	out := make([]assetGridRow, 0, len(rows))
	for _, r := range rows {
		var held []string
		if r.StoreVersions != "" {
			held = strings.Split(r.StoreVersions, ",")
		}
		updateAvailable := storeUpdateAvailable(r.StoreDeprecated, derefString(r.LatestStoreVersion), held)
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
			LicenceStatus:      r.LicenceStatus,
			Publisher:          derefString(r.Publisher),
			ReleasedAt:         r.ReleasedAt,
			UpdateAvailable:    updateAvailable,
			IsReference:        r.IsReference,
			Store:              derefString(r.Store),
			SourceURL:          r.SourceURL,
			StoreDeprecated:    r.StoreDeprecated,
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
	AssetID        uint             `json:"asset_id"`
	ImportID       uint             `json:"import_id"`
	SuggestedMatch *matchSuggestion `json:"suggested_match,omitempty"`
}

type matchSuggestion struct {
	AssetID uint   `json:"asset_id"`
	Name    string `json:"name"`
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

	settings, err := loadOrCreateServerSettings(h.db.WithContext(c.Request.Context()))
	if err != nil {
		respond500(c, err)
		return
	}
	if settings.MaxUploadBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, settings.MaxUploadBytes+multipartOverheadBytes)
	}

	form, src, err := readUploadForm(c.Request)
	if err != nil {
		respondUploadError(c, err, "library_id", lib.ID, "user_id", u.ID)
		return
	}
	defer src.Close()

	spec := newImportSpec(lib.ID, u.ID, src.FileName(), src.Header.Get("Content-Type"))

	if s := strings.TrimSpace(form.get("asset_id")); s != "" {
		n, perr := parsePositiveID(s)
		if perr != nil {
			respond400Msg(c, fmt.Errorf("invalid asset_id %q: %w", s, perr), "invalid asset_id")
			return
		}
		spec.targetAssetID = n
	}
	spec.reqStoreVersion = strings.TrimLeft(strings.TrimSpace(form.get("store_version")), "vV")
	spec.sourceKind = strings.TrimSpace(form.get("source_kind"))

	sourceRelativePath, err := imports.ValidateSource(spec.sourceKind, form.get("source_relative_path"))
	if err != nil {
		slog.Warn("import: invalid source hint",
			"err", err, "library_id", lib.ID, "source_kind", spec.sourceKind)
		respond400(c, err)
		return
	}
	spec.sourceRelativePath = sourceRelativePath

	// Create the rows before the upload so the object metadata can carry the
	// asset/version IDs. The version's object key is set after the upload and
	// the import is enqueued only once it is, so a crash in between leaves a
	// keyless version that ResumeInFlight fails rather than a live import.
	rows, err := h.createImportRows(c.Request.Context(), spec)
	if err != nil {
		respondGORM(c, err, "target asset not found")
		return
	}

	meta := spec.objectMeta(rows.assetID, rows.versionID)
	var (
		objectKey string
		size      int64
	)
	if spec.isArchive {
		objectKey, size, err = h.storage.PutOriginal(c.Request.Context(), src, meta)
	} else {
		objectKey, size, err = h.storage.PutBlob(c.Request.Context(), src, meta)
	}
	// The rows exist by now, so any failure past this point has to undo them.
	// Detached from the request context, which a client abort has cancelled.
	abandon := func() {
		h.abandonImport(context.WithoutCancel(c.Request.Context()), spec, rows)
	}
	if err != nil {
		slog.Error("import: storage put failed",
			"library_id", lib.ID,
			"asset_id", rows.assetID,
			"version_id", rows.versionID,
			"import_id", rows.importID,
			"filename", spec.filename,
			"package_kind", spec.packageKind,
			"err", err)
		abandon()
		respondUploadOrServerError(c, err, fmt.Errorf("storage put (library %d): %w", lib.ID, err),
			"library_id", lib.ID, "asset_id", rows.assetID, "import_id", rows.importID, "filename", spec.filename)
		return
	}
	if err := form.checkNoTrailingFields(); err != nil {
		slog.Warn("import: form field sent after the file part",
			"err", err, "library_id", lib.ID, "asset_id", rows.assetID, "import_id", rows.importID)
		abandon()
		respond400(c, err)
		return
	}

	if err := h.finishImport(c.Request.Context(), spec, rows, objectKey, size); err != nil {
		respond500(c, err)
		return
	}

	c.JSON(http.StatusAccepted, importEnqueueResponse{
		AssetID:        rows.assetID,
		ImportID:       rows.importID,
		SuggestedMatch: h.suggestMatch(c.Request.Context(), spec, rows.assetID),
	})
}

// cleanupFailedUpload removes the asset/version/import rows created for an
// upload that then failed to store, so no keyless orphan rows are left behind.
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

// cleanupFailedVersionUpload removes only the version + import rows from a
// failed add-version upload; the target asset pre-existed and must survive.
func (h *Handler) cleanupFailedVersionUpload(ctx context.Context, versionID, importID uint) {
	db := h.db.WithContext(ctx)
	if importID != 0 {
		if err := db.Delete(&models.AssetImport{}, importID).Error; err != nil {
			slog.Warn("import: cleanup failed version upload (import)", "err", err, "import_id", importID)
		}
	}
	if versionID != 0 {
		if err := db.Delete(&models.AssetVersion{}, versionID).Error; err != nil {
			slog.Warn("import: cleanup failed version upload (version)", "err", err, "version_id", versionID)
		}
	}
}

// postReExtract re-reads the .unitypackage archive retained when the version
// was imported and rewrites that version's files in place; it creates no new
// version. The asset's current_version_id points at the re-extracted version.
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
		respond404(c, "asset not found")
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
		respond404(c, "version not found")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}
	if source.OriginalPackageObjectKey == "" || source.PackageKind != packageKindUnityPackage || source.OriginalPackagePurged {
		respond409(c, "source version has no retained package")
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
		respond400Msg(c, err, "extract: "+err.Error())
		return
	}

	var resp importResponse
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("asset_version_id = ?", source.ID).Delete(&models.AssetFile{}).Error; err != nil {
			return err
		}

		fileCount, totalBytes, err := imports.WriteAssetFiles(tx, source.ID, "", 0, "", "", result)
		if err != nil {
			return err
		}

		asset.CurrentVersionID = &source.ID
		if !asset.ThumbnailFromStore && result.ThumbObjectKey != "" {
			asset.ThumbnailObjectKey = result.ThumbObjectKey
			asset.ThumbnailContentType = result.ThumbContentType
		}
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		if err := audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.version.reextract",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Created(map[string]any{"version_id": source.ID, "file_count": fileCount}),
		}); err != nil {
			return err
		}

		resp = importResponse{
			AssetID:        asset.ID,
			AssetVersionID: source.ID,
			FileCount:      fileCount,
			SizeBytes:      totalBytes,
			PackageKind:    source.PackageKind,
		}
		return nil
	})
	if err != nil {
		respond500(c, err)
		return
	}
	// Rewritten files start with no preview, so queue them as an import does.
	h.rescanPreviews(c, lib.ID, source.ID)

	c.JSON(http.StatusCreated, resp)
}

func parseIDParam(c *gin.Context, key string) (uint, bool) {
	s := c.Param(key)
	n, err := parsePositiveID(s)
	if err != nil {
		respond400Msg(c, fmt.Errorf("invalid %s %q: %w", key, s, err), "invalid "+key)
		return 0, false
	}
	return n, true
}

func parsePositiveID(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, errors.New("must be a positive integer")
	}
	return uint(n), nil
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
