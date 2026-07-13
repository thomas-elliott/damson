package imports

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/extract"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

func OpenOriginal(ctx context.Context, s storage.Service, v models.AssetVersion) (io.ReadCloser, error) {
	if isArchiveKind(v.PackageKind) {
		return s.GetOriginal(ctx, v.OriginalPackageObjectKey)
	}
	return s.GetBlob(ctx, v.OriginalPackageObjectKey)
}

func isArchiveKind(kind string) bool {
	return kind == "unitypackage" || kind == "zip"
}

// jobContext holds everything the pipeline needs about a job, hydrated from
// the database at the start of Run. Centralizing the load makes Run a
// straightforward state machine and lets resume-on-startup feed in just
// the import id with the pipeline figuring out what's left to do.
type jobContext struct {
	importID  uint
	asset     models.Asset
	version   models.AssetVersion
	libraryID uint
	isUnity   bool
}

// Run executes the import pipeline for one job. The asset, version, and
// import rows already exist (created by the upload handler); this function
// fills in asset_files, the thumbnail, and the asset's current_version_id
// pointer, transitioning asset_imports.stage as it goes. Each transition is
// broadcast on the events channel keyed by library id.
func Run(ctx context.Context, deps Deps, job Job) error {
	if deps.DB == nil || deps.Storage == nil {
		return errors.New("imports: pipeline missing db or storage")
	}

	jc, err := loadJobContext(deps.DB.WithContext(ctx), job.ImportID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	if job.LibraryID != 0 && jc.libraryID != job.LibraryID {
		return fmt.Errorf("library mismatch: job=%d row=%d", job.LibraryID, jc.libraryID)
	}

	publish := func(stage, errMsg string) {
		if deps.Broadcaster == nil {
			return
		}
		deps.Broadcaster.Publish(jc.libraryID, Event{
			ID:          jc.importID,
			AssetID:     jc.asset.ID,
			AssetName:   jc.asset.Name,
			Stage:       stage,
			Error:       errMsg,
			UpdatedAt:   time.Now().UTC(),
			CreatedAt:   jc.asset.CreatedAt,
			NeedsReview: jc.asset.NeedsReview,
		})
	}

	transition := func(stage string) error {
		if err := updateImportStage(deps.DB.WithContext(ctx), jc.importID, stage, ""); err != nil {
			return fmt.Errorf("transition %s: %w", stage, err)
		}
		publish(stage, "")
		return nil
	}

	fail := func(reason string) error {
		if err := updateImportStage(deps.DB.WithContext(ctx), jc.importID, StageFailed, reason); err != nil {
			slog.Warn("imports: persist failure", "import_id", jc.importID, "err", err)
		}
		publish(StageFailed, reason)
		return errors.New(reason)
	}

	if jc.version.OriginalPackageObjectKey == "" {
		return fail("source blob missing — original_package_object_key not set on version")
	}

	if err := transition(StageExtracting); err != nil {
		return err
	}

	var extracted *extract.UnityResult
	if jc.isUnity {
		blob, gerr := OpenOriginal(ctx, deps.Storage, jc.version)
		if gerr != nil {
			return fail("fetch blob: " + gerr.Error())
		}
		ex := &extract.UnityExtractor{
			Storage: deps.Storage,
			EntryMeta: storage.ObjectMeta{
				LibraryID: jc.libraryID,
				AssetID:   jc.asset.ID,
				VersionID: jc.version.ID,
				Kind:      "unitypackage-asset",
			},
		}
		res, xerr := ex.Extract(ctx, blob)
		blob.Close()
		if xerr != nil {
			return fail("extract: " + xerr.Error())
		}
		extracted = res
	}

	if err := transition(StageDetecting); err != nil {
		return err
	}

	if err := transition(StageThumbnailing); err != nil {
		return err
	}

	thumbKey, thumbType := "", ""
	if extracted != nil {
		thumbKey = extracted.ThumbObjectKey
		thumbType = extracted.ThumbContentType
	}
	if thumbKey == "" {
		settings, sErr := loadOrCreateLibrarySettings(deps.DB.WithContext(ctx), jc.libraryID)
		if sErr != nil {
			slog.Warn("imports: load settings", "err", sErr, "library_id", jc.libraryID)
		} else if !jc.isUnity || settings.ThumbnailsUnityPreview {
			toggles := thumbnail.Toggles{
				Raster: settings.ThumbnailsRasterEnabled,
				Audio:  settings.ThumbnailsAudioEnabled,
				Font:   settings.ThumbnailsFontEnabled,
			}
			filename, contentType := looseSourceMetadata(deps.DB.WithContext(ctx), jc.version.ID, jc.asset.Name)
			blob, bErr := OpenOriginal(ctx, deps.Storage, jc.version)
			if bErr != nil {
				slog.Warn("imports: fetch blob for thumbnail", "err", bErr, "object_key", jc.version.OriginalPackageObjectKey)
			} else {
				gen := &thumbnail.Generator{Storage: deps.Storage}
				tres, tErr := gen.Generate(ctx, thumbnail.Input{
					Reader:      blob,
					Filename:    filename,
					ContentType: contentType,
				}, toggles)
				blob.Close()
				if tErr != nil {
					slog.Warn("imports: generate thumbnail", "err", tErr, "filename", filename)
				} else if tres.ObjectKey != "" {
					thumbKey = tres.ObjectKey
					thumbType = tres.ContentType
				}
			}
		}
	}

	err = deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if jc.isUnity {
			// Idempotency: clear any existing files (e.g. a resumed run)
			// before re-writing so the (asset_version_id, relative_path)
			// unique index doesn't trip.
			if err := tx.Where("asset_version_id = ?", jc.version.ID).Delete(&models.AssetFile{}).Error; err != nil {
				return err
			}
			if _, _, err := WriteAssetFiles(tx, jc.version.ID, "", 0, "", "", extracted); err != nil {
				return err
			}
		}
		// Loose case: the upload handler already wrote the single
		// asset_files row (so the pipeline could read filename/content
		// type for thumbnail detection); nothing to do here.

		var asset models.Asset
		if err := tx.First(&asset, jc.asset.ID).Error; err != nil {
			return err
		}
		vid := jc.version.ID
		asset.CurrentVersionID = &vid
		if thumbKey != "" {
			asset.ThumbnailObjectKey = thumbKey
			asset.ThumbnailContentType = thumbType
		}
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fail("finalize: " + err.Error())
	}

	// Auto-enrichment: best-effort storefront metadata fill. Gated on the
	// per-library toggle and a conservative title match; never fails the import.
	if err := transition(StageEnriching); err != nil {
		return err
	}
	enrichAsset(ctx, deps, jc)

	// Previewing is decoupled: once asset_files are committed, hand the version
	// off to the preview processing queue. The import finishes immediately and
	// previews fill in afterward, tracked on their own queue.
	enqueuePreviews(ctx, deps, jc)

	if err := updateImportStage(deps.DB.WithContext(ctx), jc.importID, StageDone, ""); err != nil {
		return fail("finalize done: " + err.Error())
	}
	publish(StageDone, "")

	// Purge only after the import is marked done: a crash before Done would let
	// ResumeInFlight re-run extraction against an archive we already deleted.
	if jc.isUnity {
		maybePurgeOriginal(ctx, deps, jc)
	}
	return nil
}

func maybePurgeOriginal(ctx context.Context, deps Deps, jc jobContext) {
	if jc.version.OriginalPackageObjectKey == "" {
		return
	}
	settings, err := loadOrCreateLibrarySettings(deps.DB.WithContext(ctx), jc.libraryID)
	if err != nil {
		slog.Warn("imports: load settings for purge", "err", err, "library_id", jc.libraryID)
		return
	}
	if settings.RetainOriginalPackages {
		return
	}

	var liveReferrers int64
	if err := deps.DB.WithContext(ctx).Model(&models.AssetVersion{}).
		Where("original_package_object_key = ? AND id <> ? AND original_package_purged = ?",
			jc.version.OriginalPackageObjectKey, jc.version.ID, false).
		Count(&liveReferrers).Error; err != nil {
		slog.Warn("imports: count original referrers", "err", err, "version_id", jc.version.ID)
		return
	}
	// Originals are content-addressed, so an identical archive uploaded to
	// another asset shares one object; only delete it when no other live
	// version still references it. The per-version flag is set regardless.
	if liveReferrers == 0 {
		if err := deps.Storage.DeleteOriginal(ctx, jc.version.OriginalPackageObjectKey); err != nil {
			slog.Warn("imports: purge original blob", "err", err, "object_key", jc.version.OriginalPackageObjectKey)
			return
		}
	}
	if err := deps.DB.WithContext(ctx).Model(&models.AssetVersion{}).
		Where("id = ?", jc.version.ID).
		Update("original_package_purged", true).Error; err != nil {
		slog.Warn("imports: mark original purged", "err", err, "version_id", jc.version.ID)
	}
}

// enqueuePreviews hands a committed version off to the preview processing queue.
// It marks the version's supported files 'queued' (so they show in the queue
// immediately) and enqueues the version for the preview worker pool. Gated by
// the per-library previews_enabled toggle and a nil-safe enqueuer, so a library
// with previews off — or a server without a preview pool wired — simply skips.
func enqueuePreviews(ctx context.Context, deps Deps, jc jobContext) {
	if deps.Previews == nil {
		return
	}
	settings, err := loadOrCreateLibrarySettings(deps.DB.WithContext(ctx), jc.libraryID)
	if err != nil {
		slog.Warn("imports: load settings for previews", "err", err, "library_id", jc.libraryID)
		return
	}
	if !settings.PreviewsEnabled {
		return
	}
	if _, err := preview.MarkVersionQueued(ctx, deps.DB, jc.version.ID, 0); err != nil {
		slog.Warn("imports: mark previews queued", "err", err, "version_id", jc.version.ID)
		return
	}
	deps.Previews.EnqueueVersion(jc.libraryID, jc.version.ID)
}

// loadJobContext hydrates the asset + version + library bits the pipeline
// needs from a single import id.
func loadJobContext(db *gorm.DB, importID uint) (jobContext, error) {
	var imp models.AssetImport
	if err := db.First(&imp, importID).Error; err != nil {
		return jobContext{}, fmt.Errorf("load import row: %w", err)
	}
	var asset models.Asset
	if err := db.First(&asset, imp.AssetID).Error; err != nil {
		return jobContext{}, fmt.Errorf("load asset: %w", err)
	}
	var version models.AssetVersion
	if err := db.Where("asset_id = ?", asset.ID).Order("import_date DESC").First(&version).Error; err != nil {
		return jobContext{}, fmt.Errorf("load version: %w", err)
	}
	return jobContext{
		importID:  imp.ID,
		asset:     asset,
		version:   version,
		libraryID: asset.LibraryID,
		isUnity:   version.PackageKind == "unitypackage",
	}, nil
}

// looseSourceMetadata best-effort recovers a filename + content type for
// thumbnail generation against a loose upload's retained blob. If the
// asset_files row already exists (e.g. resumed run) we use it; otherwise
// fall back to the asset name + empty content type.
func looseSourceMetadata(db *gorm.DB, versionID uint, fallbackName string) (string, string) {
	var f models.AssetFile
	err := db.Where("asset_version_id = ?", versionID).First(&f).Error
	if err == nil {
		return f.RelativePath, f.DetectedType
	}
	return fallbackName, ""
}

// updateImportStage writes a stage transition on the asset_imports row.
// The DB row is the source of truth for stage state; the broadcaster only
// forwards what was already persisted.
func updateImportStage(db *gorm.DB, importID uint, stage, errMsg string) error {
	return db.Model(&models.AssetImport{}).
		Where("id = ?", importID).
		Updates(map[string]any{
			"stage":      stage,
			"error":      errMsg,
			"updated_at": time.Now().UTC(),
		}).Error
}

// MarkImportFailed updates an existing import row to the failed state. The
// row must already exist; callers that don't yet have an import row should
// create one with stage=failed directly.
func MarkImportFailed(db *gorm.DB, importID uint, reason string) error {
	return updateImportStage(db, importID, StageFailed, reason)
}

// WriteAssetFiles persists asset_files rows for an asset version. If
// extracted is non-nil, every extracted file becomes a row; otherwise a
// single row is written for the loose upload. Returns (count, totalBytes).
func WriteAssetFiles(tx *gorm.DB, versionID uint, looseObjectKey string, looseSize int64, looseFilename, looseContentType string, extracted *extract.UnityResult) (int, int64, error) {
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

// NextVersionLabel returns the next `v<N>` label for an asset by counting
// existing versions. Pulled out of the api package so the retry handler
// and the pipeline can both call it.
func NextVersionLabel(db *gorm.DB, assetID uint) (string, error) {
	var n int64
	if err := db.Model(&models.AssetVersion{}).Where("asset_id = ?", assetID).Count(&n).Error; err != nil {
		return "", err
	}
	return "v" + strconv.FormatInt(n+1, 10), nil
}

// loadOrCreateLibrarySettings is the pipeline-local mirror of the api
// helper of the same name; duplicated to avoid an import cycle (the api
// package depends on imports, not vice versa). Kept in sync with
// settings.go's defaults.
func loadOrCreateLibrarySettings(tx *gorm.DB, libID uint) (*models.LibrarySettings, error) {
	var s models.LibrarySettings
	err := tx.Where("library_id = ?", libID).First(&s).Error
	if err == nil {
		return &s, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	s = models.LibrarySettings{
		LibraryID:               libID,
		ThemeDefault:            "dark",
		GridDensityDefault:      "medium",
		RetainOriginalPackages:  true,
		DedupeEnabled:           true,
		ThumbnailsRasterEnabled: true,
		ThumbnailsAudioEnabled:  true,
		ThumbnailsFontEnabled:   true,
		ThumbnailsUnityPreview:  true,
		PreviewsEnabled:         true,
		AutoEnrichEnabled:       true,
	}
	if err := tx.Create(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}
