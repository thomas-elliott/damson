package ingest

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
	if models.IsArchiveKind(v.PackageKind) {
		return s.Get(ctx, storage.Originals, v.OriginalPackageObjectKey)
	}
	return s.Get(ctx, storage.Blobs, v.OriginalPackageObjectKey)
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

// jobContext is one import as it moves through the stages: the rows Run
// loaded, the library's settings, and what each stage leaves for the next.
type jobContext struct {
	importID  uint
	asset     models.Asset
	version   models.AssetVersion
	libraryID uint
	isUnity   bool
	isArchive bool
	// settings is nil when the library's settings failed to load. Each stage
	// then keeps its default.
	settings *models.LibrarySettings

	extracted *extract.UnityResult
	thumbKey  string
	thumbType string
	fileCount int
	sizeBytes int64
	// inCommit runs inside the transaction that writes the files.
	inCommit func(tx *gorm.DB) error
}

// stage is one step of the pipeline. Run records and announces announce, when
// set, before the step runs. A step's error fails the import, unless it is a
// retryError, which leaves the import in flight for the queue to retry.
type stage struct {
	announce  string
	run       func(context.Context, Deps, *jobContext) error
	reextract bool
}

// stages is the import pipeline in order. Reextract runs the steps marked
// reextract, so a re-extract writes files the way an import does.
var stages = []stage{
	{announce: StageExtracting, run: extractFiles, reextract: true},
	{announce: StageDetecting},
	{announce: StageThumbnailing, run: chooseCover, reextract: true},
	{run: writeFiles, reextract: true},
	{announce: StageEnriching, run: enrichVersion},
}

// retryError is a stage failure that a later attempt may not hit, such as a
// store that could not answer.
type retryError struct {
	op  string
	err error
}

func (e *retryError) Error() string { return e.op + ": " + e.err.Error() }
func (e *retryError) Unwrap() error { return e.err }

// errOriginalMissing fails an import whose original is gone from storage.
var errOriginalMissing = errors.New("source blob missing from storage")

// Run executes the import pipeline for one job. The asset, version, and
// import rows already exist (created by the upload handler); the stages fill
// in asset_files, the thumbnail, and the asset's current_version_id pointer,
// transitioning asset_imports.stage as they go. Each transition is broadcast
// on the events channel keyed by library id.
func Run(ctx context.Context, deps Deps, job Job) error {
	if deps.DB == nil || deps.Storage == nil {
		return errors.New("imports: pipeline missing db or storage")
	}

	jc, err := loadJobContext(deps.DB.WithContext(ctx), job.ImportID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}

	fail := func(reason string) error {
		if err := failJob(ctx, deps, jc, reason); err != nil {
			slog.Warn("imports: persist failure", "import_id", jc.importID, "err", err)
		}
		return errors.New(reason)
	}

	// A library mismatch is a terminal input error: mark failed so the queue
	// cancels rather than retries.
	if job.LibraryID != 0 && jc.libraryID != job.LibraryID {
		return fail(fmt.Sprintf("library mismatch: job=%d row=%d", job.LibraryID, jc.libraryID))
	}

	if jc.version.OriginalPackageObjectKey == "" {
		return fail("source blob missing — original_package_object_key not set on version")
	}

	jc.settings = loadSettings(ctx, deps, jc.libraryID)

	for _, s := range stages {
		if s.announce != "" {
			if err := transition(ctx, deps, jc, s.announce); err != nil {
				return err
			}
		}
		if s.run == nil {
			continue
		}
		err := s.run(ctx, deps, &jc)
		var re *retryError
		if errors.As(err, &re) {
			slog.Warn("imports: pipeline will retry",
				"import_id", jc.importID,
				"asset_id", jc.asset.ID,
				"library_id", jc.libraryID,
				"object_key", jc.version.OriginalPackageObjectKey,
				"op", re.op,
				"err", re.err)
			return re
		}
		if err != nil {
			return fail(err.Error())
		}
	}

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

func transition(ctx context.Context, deps Deps, jc jobContext, stage string) error {
	if err := updateImportStage(deps.DB.WithContext(ctx), jc.importID, stage, ""); err != nil {
		return fmt.Errorf("transition %s: %w", stage, err)
	}
	publishStage(deps, jc, stage, "")
	return nil
}

func loadSettings(ctx context.Context, deps Deps, libraryID uint) *models.LibrarySettings {
	settings, err := library.Settings(deps.DB.WithContext(ctx), libraryID)
	if err != nil {
		slog.Warn("imports: load settings", "err", err, "library_id", libraryID)
		return nil
	}
	return settings
}

// extractFiles unpacks an archive's original into blobs. A loose upload has
// nothing to unpack.
func extractFiles(ctx context.Context, deps Deps, jc *jobContext) error {
	if !jc.isArchive {
		return nil
	}
	blob, err := OpenOriginal(ctx, deps.Storage, jc.version)
	if err != nil {
		// A missing original never comes back, so only a store that could
		// not answer is worth retrying.
		if _, ok, herr := deps.Storage.Stat(ctx, storage.Originals, jc.version.OriginalPackageObjectKey); herr == nil && !ok {
			return errOriginalMissing
		}
		return &retryError{op: "fetch blob", err: err}
	}
	defer blob.Close()
	meta := storage.ObjectMeta{
		LibraryID: jc.libraryID,
		AssetID:   jc.asset.ID,
		VersionID: jc.version.ID,
	}
	var res *extract.UnityResult
	switch jc.version.PackageKind {
	case models.PackageKindZip:
		meta.Kind = "zip-asset"
		res, err = (&extract.ZipExtractor{Storage: deps.Storage, EntryMeta: meta}).Extract(ctx, storageReader{blob})
	default:
		meta.Kind = "unitypackage-asset"
		res, err = (&extract.UnityExtractor{Storage: deps.Storage, EntryMeta: meta}).Extract(ctx, storageReader{blob})
	}
	if errors.Is(err, extract.ErrStorage) {
		return &retryError{op: "extract", err: err}
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadPackage, err)
	}
	jc.extracted = res
	return nil
}

// chooseCover picks the version's cover. An archive's cover is the one its
// extractor harvested. For a Unity package that is a preview.png, which the
// library can switch off; the store cover or the placeholder then applies.
// Settings that fail to load keep the default, on. A loose file thumbnails
// from its own bytes; an archive's raw bytes are not a thumbnailable image.
func chooseCover(ctx context.Context, deps Deps, jc *jobContext) error {
	if jc.extracted != nil && (!jc.isUnity || jc.settings == nil || jc.settings.ThumbnailsUnityPreview) {
		jc.thumbKey = jc.extracted.ThumbObjectKey
		jc.thumbType = jc.extracted.ThumbContentType
	}
	if jc.thumbKey != "" || jc.settings == nil || jc.isArchive {
		return nil
	}
	toggles := thumbnail.Toggles{
		Raster: jc.settings.ThumbnailsRasterEnabled,
		Audio:  jc.settings.ThumbnailsAudioEnabled,
		Font:   jc.settings.ThumbnailsFontEnabled,
	}
	filename, contentType := looseSourceMetadata(deps.DB.WithContext(ctx), jc.version.ID, jc.asset.Name)
	blob, err := OpenOriginal(ctx, deps.Storage, jc.version)
	if err != nil {
		slog.Warn("imports: fetch blob for thumbnail", "err", err, "object_key", jc.version.OriginalPackageObjectKey)
		return nil
	}
	defer blob.Close()
	gen := &thumbnail.Generator{Storage: deps.Storage}
	res, err := gen.Generate(ctx, thumbnail.Input{
		Reader:      blob,
		Filename:    filename,
		ContentType: contentType,
	}, toggles)
	if err != nil {
		slog.Warn("imports: generate thumbnail", "err", err, "filename", filename)
		return nil
	}
	if res.ObjectKey != "" {
		jc.thumbKey = res.ObjectKey
		jc.thumbType = res.ContentType
	}
	return nil
}

// writeFiles writes the extracted files and commits them in one transaction.
// A loose upload's single asset_files row was written by the upload handler,
// so the pipeline could read its filename and content type.
func writeFiles(ctx context.Context, deps Deps, jc *jobContext) error {
	err := deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if jc.extracted != nil {
			// Clear any files from an earlier run first, so the
			// (asset_version_id, relative_path) unique index does not trip.
			if err := tx.Where("asset_version_id = ?", jc.version.ID).Delete(&models.AssetFile{}).Error; err != nil {
				return err
			}
			var err error
			if jc.fileCount, jc.sizeBytes, err = WriteAssetFiles(tx, jc.version.ID, "", 0, "", "", jc.extracted); err != nil {
				return err
			}
		}
		// An import of an older version finishes without taking the asset
		// back to it, or its cover.
		if err := commitFiles(tx, jc.asset.ID, jc.version.ID, jc.thumbKey, jc.thumbType); err != nil {
			return err
		}
		if jc.inCommit != nil {
			return jc.inCommit(tx)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("finalize: %w", err)
	}
	return nil
}

// enrichVersion fills what the version's source path and storefront listing
// say, then hands the files to the preview queue. Both are best-effort and
// never fail the import. The enriching stage is announced even when
// enrichment is off, so the SSE stage sequence stays stable.
func enrichVersion(ctx context.Context, deps Deps, jc *jobContext) error {
	ApplySourceHints(ctx, deps, *jc)
	enrichAsset(ctx, deps, *jc)
	enqueuePreviews(ctx, deps, *jc)
	return nil
}

func maybePurgeOriginal(ctx context.Context, deps Deps, jc jobContext) {
	if jc.version.OriginalPackageObjectKey == "" {
		return
	}
	// Settings that fail to load keep the original.
	if jc.settings == nil || jc.settings.RetainOriginalPackages {
		return
	}

	var liveReferrers int64
	if err := deps.DB.WithContext(ctx).Model(&models.AssetVersion{}).
		Where("original_package_object_key = ? AND id <> ?", jc.version.OriginalPackageObjectKey, jc.version.ID).
		Where(models.DownloadableOriginalSQL("asset_versions")).
		Count(&liveReferrers).Error; err != nil {
		slog.Warn("imports: count original referrers", "err", err, "version_id", jc.version.ID)
		return
	}
	// Originals are content-addressed, so an identical archive uploaded to
	// another asset shares one object; only delete it when no other live
	// version still references it. The per-version flag is set regardless.
	if liveReferrers == 0 {
		if err := deps.Storage.Delete(ctx, storage.Originals, jc.version.OriginalPackageObjectKey); err != nil {
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
// with previews off skips it. preview.Queue reads the previews setting itself,
// as it does for every caller.
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
	jc := newJobContext(asset, version)
	jc.importID = imp.ID
	return jc, nil
}

func newJobContext(asset models.Asset, version models.AssetVersion) jobContext {
	return jobContext{
		asset:     asset,
		version:   version,
		libraryID: asset.LibraryID,
		isUnity:   version.PackageKind == models.PackageKindUnityPackage,
		isArchive: models.IsArchiveKind(version.PackageKind),
	}
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
