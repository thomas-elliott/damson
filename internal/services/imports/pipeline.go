package imports

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/extract"
	"github.com/thomas-elliott/damson/internal/services/library"
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

// storageReader tags a failed read of an original as a storage error, so a
// connection lost mid-extract is retried like one lost while opening it.
type storageReader struct{ r io.Reader }

func (s storageReader) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		err = fmt.Errorf("%w: %w", extract.ErrStorage, err)
	}
	return n, err
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
	isArchive bool
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

	transition := func(stage string) error {
		if err := updateImportStage(deps.DB.WithContext(ctx), jc.importID, stage, ""); err != nil {
			return fmt.Errorf("transition %s: %w", stage, err)
		}
		publishStage(deps, jc, stage, "")
		return nil
	}

	fail := func(reason string) error {
		if err := failJob(ctx, deps, jc, reason); err != nil {
			slog.Warn("imports: persist failure", "import_id", jc.importID, "err", err)
		}
		return errors.New(reason)
	}

	// retry leaves the row in flight and returns the error, so the queue
	// retries a transient failure and marks the row failed only once the
	// attempts run out.
	retry := func(op string, err error) error {
		slog.Warn("imports: pipeline will retry",
			"import_id", jc.importID,
			"asset_id", jc.asset.ID,
			"library_id", jc.libraryID,
			"object_key", jc.version.OriginalPackageObjectKey,
			"op", op,
			"err", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	// A library mismatch is a terminal input error: mark failed so the queue
	// cancels rather than retries.
	if job.LibraryID != 0 && jc.libraryID != job.LibraryID {
		return fail(fmt.Sprintf("library mismatch: job=%d row=%d", job.LibraryID, jc.libraryID))
	}

	if jc.version.OriginalPackageObjectKey == "" {
		return fail("source blob missing — original_package_object_key not set on version")
	}

	if err := transition(StageExtracting); err != nil {
		return err
	}

	var extracted *extract.UnityResult
	if jc.isArchive {
		blob, gerr := OpenOriginal(ctx, deps.Storage, jc.version)
		if gerr != nil {
			// A missing original never comes back, so only a store that
			// could not answer is worth retrying.
			if _, ok, herr := deps.Storage.HeadOriginal(ctx, jc.version.OriginalPackageObjectKey); herr == nil && !ok {
				return fail("source blob missing from storage")
			}
			return retry("fetch blob", gerr)
		}
		var (
			res  *extract.UnityResult
			xerr error
		)
		switch jc.version.PackageKind {
		case "zip":
			ex := &extract.ZipExtractor{
				Storage: deps.Storage,
				EntryMeta: storage.ObjectMeta{
					LibraryID: jc.libraryID,
					AssetID:   jc.asset.ID,
					VersionID: jc.version.ID,
					Kind:      "zip-asset",
				},
			}
			res, xerr = ex.Extract(ctx, storageReader{blob})
		default:
			ex := &extract.UnityExtractor{
				Storage: deps.Storage,
				EntryMeta: storage.ObjectMeta{
					LibraryID: jc.libraryID,
					AssetID:   jc.asset.ID,
					VersionID: jc.version.ID,
					Kind:      "unitypackage-asset",
				},
			}
			res, xerr = ex.Extract(ctx, storageReader{blob})
		}
		blob.Close()
		if errors.Is(xerr, extract.ErrStorage) {
			return retry("extract", xerr)
		}
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

	settings, sErr := library.Settings(deps.DB.WithContext(ctx), jc.libraryID)
	if sErr != nil {
		slog.Warn("imports: load settings", "err", sErr, "library_id", jc.libraryID)
	}

	// An archive's cover is the one its extractor harvested. For a Unity
	// package that is a preview.png, which the library can switch off; the
	// store cover or the placeholder then applies. Settings that fail to load
	// keep the default, on.
	thumbKey, thumbType := "", ""
	if extracted != nil && (!jc.isUnity || sErr != nil || settings.ThumbnailsUnityPreview) {
		thumbKey = extracted.ThumbObjectKey
		thumbType = extracted.ThumbContentType
	}
	// A loose file thumbnails from its own bytes. An archive's raw bytes are
	// not a thumbnailable image, so it relies on its extractor's cover alone.
	if thumbKey == "" && sErr == nil && !jc.isArchive {
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

	err = deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if extracted != nil {
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

		// An import of an older version finishes without taking the asset
		// back to it, or its cover.
		return chooseCurrentAndCover(tx, jc.asset.ID, jc.version.ID, jc.version.ID, thumbKey, thumbType)
	})
	if err != nil {
		return fail("finalize: " + err.Error())
	}

	// Auto-enrichment: best-effort storefront metadata fill. Gated on the
	// per-library toggle and a conservative title match; never fails the import.
	// The stage transition fires even when enrichment is disabled so the SSE
	// stage sequence stays stable; enrichAsset itself is the gated no-op.
	if err := transition(StageEnriching); err != nil {
		return err
	}
	ApplySourceHints(ctx, deps, jc)
	enrichAsset(ctx, deps, jc)

	// Previewing is decoupled: once asset_files are committed, hand the version
	// off to the preview processing queue. The import finishes immediately and
	// previews fill in afterward, tracked on their own queue.
	enqueuePreviews(ctx, deps, jc)

	if err := updateImportStage(deps.DB.WithContext(ctx), jc.importID, StageDone, ""); err != nil {
		return fail("finalize done: " + err.Error())
	}
	publishStage(deps, jc, StageDone, "")

	// Purge only after the import is marked done: a crash before Done would let
	// ResumeInFlight re-run extraction against an archive we already deleted.
	if jc.isArchive {
		maybePurgeOriginal(ctx, deps, jc)
	}
	return nil
}

func maybePurgeOriginal(ctx context.Context, deps Deps, jc jobContext) {
	if jc.version.OriginalPackageObjectKey == "" {
		return
	}
	settings, err := library.Settings(deps.DB.WithContext(ctx), jc.libraryID)
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

// enqueuePreviews hands a committed version to the preview queue. A library
// with previews off skips it.
func enqueuePreviews(ctx context.Context, deps Deps, jc jobContext) {
	_, err := preview.Queue(ctx, deps.DB, deps.Previews, preview.Job{LibraryID: jc.libraryID, VersionID: jc.version.ID})
	if err != nil && !errors.Is(err, preview.ErrDisabled) {
		slog.Warn("imports: queue previews", "err", err, "version_id", jc.version.ID)
	}
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
	if imp.AssetVersionID == nil {
		return jobContext{}, errors.New("load version: the import's version no longer exists")
	}
	var version models.AssetVersion
	if err := db.Where("id = ? AND asset_id = ?", *imp.AssetVersionID, asset.ID).First(&version).Error; err != nil {
		return jobContext{}, fmt.Errorf("load version: %w", err)
	}
	return jobContext{
		importID:  imp.ID,
		asset:     asset,
		version:   version,
		libraryID: asset.LibraryID,
		isUnity:   version.PackageKind == "unitypackage",
		isArchive: isArchiveKind(version.PackageKind),
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

func publishStage(deps Deps, jc jobContext, stage, errMsg string) {
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

// failJob logs the failure, marks the row failed and publishes it.
func failJob(ctx context.Context, deps Deps, jc jobContext, reason string) error {
	slog.Error("imports: pipeline failed",
		"import_id", jc.importID,
		"asset_id", jc.asset.ID,
		"library_id", jc.libraryID,
		"package_kind", jc.version.PackageKind,
		"object_key", jc.version.OriginalPackageObjectKey,
		"reason", reason)
	if err := updateImportStage(deps.DB.WithContext(ctx), jc.importID, StageFailed, reason); err != nil {
		return err
	}
	publishStage(deps, jc, StageFailed, reason)
	return nil
}

// FailImport marks an import failed once the queue has given up retrying it.
// It logs and streams the failure the same way the pipeline does.
func FailImport(ctx context.Context, deps Deps, importID uint, reason string) error {
	jc, err := loadJobContext(deps.DB.WithContext(ctx), importID)
	if err != nil {
		return MarkImportFailed(deps.DB.WithContext(ctx), importID, reason)
	}
	return failJob(ctx, deps, jc, reason)
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

// IsFailed reports whether an import row is in the terminal failed stage. The
// queue uses it to tell a pipeline-marked terminal failure (don't retry) from a
// transient error (retry). A missing row or query error reports false.
func IsFailed(ctx context.Context, db *gorm.DB, importID uint) bool {
	var imp models.AssetImport
	if err := db.WithContext(ctx).Select("stage").First(&imp, importID).Error; err != nil {
		return false
	}
	return imp.Stage == StageFailed
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

	// Postgres' extended protocol caps one statement at 65535 bind parameters,
	// and each asset_files row binds a dozen columns, so a single multi-row
	// INSERT fails past roughly five thousand files. Batch well under that; the
	// batches share the caller's transaction, so the write stays atomic.
	const assetFileInsertBatch = 1000

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
	if err := tx.CreateInBatches(&rows, assetFileInsertBatch).Error; err != nil {
		return 0, 0, err
	}
	return len(rows), total, nil
}
