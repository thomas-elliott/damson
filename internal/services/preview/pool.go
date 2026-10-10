package preview

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imaging"
)

// Job is the unit of work for the preview pool: a version to (re)process, and
// optionally a single file within it (file-level requeue). FileID == 0 means
// every queued file in the version.
type Job struct {
	LibraryID uint
	VersionID uint
	FileID    uint
}

// Pool is a synchronous enqueuer that runs preview processing inline on the
// calling goroutine. Production uses the River-backed queue; this remains for
// tests (via NewSync) that need deterministic status transitions without a
// running River client. It satisfies the api PreviewEnqueuer seam and the
// import pipeline's PreviewEnqueuer.
type Pool struct {
	deps Deps
}

// NewSync returns a pool whose Enqueue runs processing on the calling
// goroutine. Used by tests (and the synchronous import test pool) to keep
// status transitions deterministic.
func NewSync(deps Deps) *Pool {
	return &Pool{deps: deps}
}

// Enqueue runs processing inline.
func (p *Pool) Enqueue(job Job) {
	if err := ProcessVersion(context.Background(), p.deps, job.LibraryID, job.VersionID, job.FileID); err != nil {
		slog.Warn("sync preview pipeline failed", "version_id", job.VersionID, "err", err)
	}
}

// EnqueueVersion enqueues every queued file in a version. It satisfies the
// enqueuer interface the import pipeline depends on, so imports can hand off
// preview work without importing the concrete pool type.
func (p *Pool) EnqueueVersion(libraryID, versionID uint) {
	p.Enqueue(Job{LibraryID: libraryID, VersionID: versionID})
}

// ResumeInFlight recovers preview work interrupted by a crash or deploy. Files
// stuck in 'processing' are reset to 'queued' (the worker died mid-render), then
// every version with queued files is re-enqueued through enq. ProcessVersion is
// idempotent — content-addressed derivatives mean a re-run is cheap and safe;
// the River enqueuer additionally dedupes so this is safe to run every boot.
func ResumeInFlight(ctx context.Context, db *gorm.DB, enq interface{ Enqueue(Job) }) (int, error) {
	if err := db.WithContext(ctx).Model(&models.AssetFile{}).
		Where("preview_status = ?", StatusProcessing).
		Update("preview_status", StatusQueued).Error; err != nil {
		return 0, err
	}

	type ref struct {
		VersionID uint
		LibraryID uint
	}
	var refs []ref
	err := db.WithContext(ctx).
		Table("asset_files AS f").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("f.preview_status = ?", StatusQueued).
		Group("v.id, a.library_id").
		Select("v.id AS version_id, a.library_id AS library_id").
		Scan(&refs).Error
	if err != nil {
		return 0, err
	}
	for _, r := range refs {
		enq.Enqueue(Job{LibraryID: r.LibraryID, VersionID: r.VersionID})
	}
	return len(refs), nil
}

// QueueNewlySupported queues the files an earlier build marked unsupported
// that a renderer now exists for, such as the tier-2 image formats, in
// libraries with previews on. Run it before ResumeInFlight, which enqueues
// them. Once queued a file is no longer unsupported, so a second run finds
// nothing.
func QueueNewlySupported(ctx context.Context, db *gorm.DB) (int, error) {
	// The extensions classify renders, as a Postgres regex: \.(svg|psd|...)$.
	// They are letters and digits, so none needs escaping.
	var exts []string
	for _, ext := range append([]string{".svg"}, imaging.ConvertedExtensions()...) {
		exts = append(exts, strings.TrimPrefix(ext, "."))
	}
	pattern := `\.(` + strings.Join(exts, "|") + `)$`

	previewsOn := db.Table("asset_versions AS v").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Joins("LEFT JOIN library_settings s ON s.library_id = a.library_id").
		Where("COALESCE(s.previews_enabled, true)").
		Select("v.id")
	res := db.WithContext(ctx).Model(&models.AssetFile{}).
		Where("preview_status = ? AND lower(relative_path) ~ ?", StatusUnsupported, pattern).
		Where("asset_version_id IN (?)", previewsOn).
		Updates(map[string]any{"preview_status": StatusQueued, "preview_updated_at": time.Now().UTC()})
	return int(res.RowsAffected), res.Error
}
