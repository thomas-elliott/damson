package api

import (
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/extract"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

// importRow is the wire shape returned by GET /api/imports — the
// queue/admin viewers care about stage + error + a human-readable filename.
// We build the filename from the asset's first file row when available; the
// asset name otherwise.
type importRow struct {
	ID          uint      `json:"id"`
	AssetID     uint      `json:"asset_id"`
	AssetName   string    `json:"asset_name"`
	Stage       string    `json:"stage"`
	Error       string    `json:"error,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
	NeedsReview bool      `json:"needs_review"`
}

type importListResponse struct {
	Imports []importRow `json:"imports"`
	Total   int64       `json:"total"`
}

// getImports returns imports for the current library. Default filter is
// "in flight or failed" — `done` rows are kept for debugging but the
// queue UI doesn't surface them. Set `?stage=done` to get the historical
// view; `?stage=all` returns everything.
func (h *Handler) getImports(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c)
		return
	}
	limit, offset := parsePagination(c)
	stageFilter := strings.TrimSpace(c.Query("stage"))

	q := h.db.WithContext(c.Request.Context()).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("a.library_id = ?", lib.ID)

	switch stageFilter {
	case "":
		q = q.Where("i.stage <> ?", "done")
	case "all":
		// no stage filter
	default:
		q = q.Where("i.stage = ?", stageFilter)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		respond500(c)
		return
	}

	type row struct {
		ID          uint
		AssetID     uint
		AssetName   string
		Stage       string
		Error       string
		UpdatedAt   time.Time
		CreatedAt   time.Time
		NeedsReview bool
	}
	var rows []row
	err := q.Session(&gorm.Session{}).
		Select("i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at, a.created_at, a.needs_review").
		Order("i.updated_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		respond500(c)
		return
	}

	out := make([]importRow, len(rows))
	for i, r := range rows {
		out[i] = importRow{
			ID:          r.ID,
			AssetID:     r.AssetID,
			AssetName:   r.AssetName,
			Stage:       r.Stage,
			Error:       r.Error,
			UpdatedAt:   r.UpdatedAt,
			CreatedAt:   r.CreatedAt,
			NeedsReview: r.NeedsReview,
		}
	}
	c.JSON(http.StatusOK, importListResponse{Imports: out, Total: total})
}

// getImport returns one import row by id, library-gated via the linked asset.
func (h *Handler) getImport(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	type row struct {
		ID          uint
		AssetID     uint
		AssetName   string
		Stage       string
		Error       string
		UpdatedAt   time.Time
		CreatedAt   time.Time
		NeedsReview bool
	}
	var r row
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("i.id = ? AND a.library_id = ?", id, lib.ID).
		Select("i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at, a.created_at, a.needs_review").
		Scan(&r).Error
	if err != nil {
		respond500(c)
		return
	}
	if r.ID == 0 {
		respond404(c, "import not found")
		return
	}
	c.JSON(http.StatusOK, importRow{
		ID:          r.ID,
		AssetID:     r.AssetID,
		AssetName:   r.AssetName,
		Stage:       r.Stage,
		Error:       r.Error,
		UpdatedAt:   r.UpdatedAt,
		CreatedAt:   r.CreatedAt,
		NeedsReview: r.NeedsReview,
	})
}

// postImportRetry re-runs the import pipeline against the original blob of
// a failed import. Only `failed` rows can be retried; `done` rows are
// rejected with 409. Idempotency: each retry creates a fresh asset_import
// row rather than mutating the existing one — easier to reason about and
// keeps the failure history visible.
func (h *Handler) postImportRetry(c *gin.Context) {
	if h.storage == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "storage not configured"})
		return
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var imp models.AssetImport
	err := h.db.WithContext(c.Request.Context()).First(&imp, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "import not found")
		return
	}
	if err != nil {
		respond500(c)
		return
	}
	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).First(&asset, imp.AssetID).Error; err != nil {
		respond500(c)
		return
	}
	if asset.LibraryID != lib.ID {
		respond404(c, "import not found")
		return
	}
	if imp.Stage != "failed" {
		respond409(c, "import is not in failed state")
		return
	}

	// Find a source blob to retry against. Prefer the original-package blob
	// from the most recent version (Unity case); otherwise fall back to a
	// single asset file. If neither exists, the upload bytes are gone and
	// we can't recover.
	var version models.AssetVersion
	verr := h.db.WithContext(c.Request.Context()).
		Where("asset_id = ?", asset.ID).
		Order("import_date DESC").
		First(&version).Error
	if verr != nil && !errors.Is(verr, gorm.ErrRecordNotFound) {
		respond500(c)
		return
	}

	// Pick a source blob to retry against. The Unity case retains the
	// original archive on the version row; the non-Unity case requires an
	// existing asset_files row — without one the upload bytes are gone (the
	// failed import never wrote a manifest) and re-running the pipeline
	// would create a corrupt empty version.
	var (
		sourceKey      string
		sourceFilename string
		sourceCT       string
		isUnity        bool
	)
	if verr == nil && version.PackageKind == packageKindUnityPackage && version.OriginalPackageObjectKey != "" {
		sourceKey = version.OriginalPackageObjectKey
		sourceFilename = asset.Name + ".unitypackage"
		isUnity = true
	} else if verr == nil {
		var f models.AssetFile
		if err := h.db.WithContext(c.Request.Context()).
			Where("asset_version_id = ?", version.ID).
			First(&f).Error; err == nil {
			sourceKey = f.ObjectKey
			sourceFilename = f.RelativePath
			sourceCT = f.DetectedType
			isUnity = strings.EqualFold(filepath.Ext(sourceFilename), ".unitypackage")
		}
	}
	if sourceKey == "" {
		respond409(c, "no source blob to retry")
		return
	}

	// Re-run the pipeline against the source blob. Treat it as a fresh
	// import: existing asset stays, but we get a new version + thumbnail
	// generation if applicable.
	var extracted *extract.UnityResult
	if isUnity {
		blob, gerr := h.storage.GetBlob(c.Request.Context(), sourceKey)
		if gerr != nil {
			h.markImportFailed(c, asset.ID, "fetch retained blob")
			respond500(c)
			return
		}
		ex := &extract.UnityExtractor{Storage: h.storage}
		res, xerr := ex.Extract(c.Request.Context(), blob)
		blob.Close()
		if xerr != nil {
			h.markImportFailed(c, asset.ID, "extract: "+xerr.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "extract: " + xerr.Error()})
			return
		}
		extracted = res
	}

	// Thumbnail pipeline mirrors postImport.
	thumbKey, thumbType := "", ""
	if extracted != nil {
		thumbKey = extracted.ThumbObjectKey
		thumbType = extracted.ThumbContentType
	}
	if thumbKey == "" && !isUnity {
		settings, sErr := loadOrCreateLibrarySettings(h.db.WithContext(c.Request.Context()), lib.ID)
		if sErr == nil {
			toggles := thumbnail.Toggles{
				Raster: settings.ThumbnailsRasterEnabled,
				Audio:  settings.ThumbnailsAudioEnabled,
				Font:   settings.ThumbnailsFontEnabled,
			}
			blob, bErr := h.storage.GetBlob(c.Request.Context(), sourceKey)
			if bErr == nil {
				gen := &thumbnail.Generator{Storage: h.storage}
				tres, tErr := gen.Generate(c.Request.Context(), thumbnail.Input{
					Reader:      blob,
					Filename:    sourceFilename,
					ContentType: sourceCT,
				}, toggles)
				blob.Close()
				if tErr == nil && tres.ObjectKey != "" {
					thumbKey = tres.ObjectKey
					thumbType = tres.ContentType
				}
			}
		}
	}

	nextLabel, nlErr := nextVersionLabel(h.db.WithContext(c.Request.Context()), asset.ID)
	if nlErr != nil {
		respond500(c)
		return
	}

	var newImp models.AssetImport
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		v := models.AssetVersion{
			AssetID:         asset.ID,
			VersionLabel:    nextLabel,
			ImportDate:      time.Now().UTC(),
			CreatedByUserID: &u.ID,
		}
		if isUnity {
			v.PackageKind = packageKindUnityPackage
			v.OriginalPackageObjectKey = sourceKey
		} else {
			v.PackageKind = packageKindLoose
		}
		if err := tx.Create(&v).Error; err != nil {
			return err
		}
		if _, _, err := writeAssetFiles(tx, v.ID, sourceKey, 0, filepath.Base(sourceFilename), sourceCT, extracted); err != nil {
			return err
		}
		asset.CurrentVersionID = &v.ID
		if thumbKey != "" {
			asset.ThumbnailObjectKey = thumbKey
			asset.ThumbnailContentType = thumbType
		}
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		newImp = models.AssetImport{
			AssetID:   asset.ID,
			Stage:     "done",
			UpdatedAt: time.Now().UTC(),
		}
		return tx.Create(&newImp).Error
	})
	if err != nil {
		respond500(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"asset_id":  asset.ID,
		"import_id": newImp.ID,
	})
}

// markImportFailed writes a `failed` import row in its own transaction. The
// failure is itself an event worth recording; we don't want to roll it back
// just because a downstream call also failed. Errors here are logged and
// swallowed — failing to record a failure shouldn't escalate the failure.
func (h *Handler) markImportFailed(c *gin.Context, assetID uint, reason string) {
	imp := models.AssetImport{
		AssetID:   assetID,
		Stage:     "failed",
		Error:     reason,
		UpdatedAt: time.Now().UTC(),
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&imp).Error; err != nil {
		slog.Warn("failed to record failed import", "err", err, "asset_id", assetID)
	}
}
