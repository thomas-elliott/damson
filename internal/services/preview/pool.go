package preview

import (
	"context"
	"log/slog"
	"runtime"
	"sync"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// Job is the unit of work for the preview pool: a version to (re)process, and
// optionally a single file within it (file-level requeue). FileID == 0 means
// every queued file in the version.
type Job struct {
	LibraryID uint
	VersionID uint
	FileID    uint
}

// Pool is a bounded worker pool that drains preview jobs, mirroring
// imports.Pool. Construct with New(deps, size) for production or NewSync(deps)
// for tests that need processing to run inline.
type Pool struct {
	deps     Deps
	jobs     chan Job
	wg       sync.WaitGroup
	sync     bool
	shutdown sync.Once
}

// New starts a worker pool with `size` goroutines. If size <= 0 the pool uses
// runtime.GOMAXPROCS(0).
func New(deps Deps, size int) *Pool {
	if size <= 0 {
		size = runtime.GOMAXPROCS(0)
	}
	p := &Pool{deps: deps, jobs: make(chan Job, size*4)}
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// NewSync returns a pool whose Enqueue runs processing on the calling
// goroutine. Used by tests (and the synchronous import test pool) to keep
// status transitions deterministic.
func NewSync(deps Deps) *Pool {
	return &Pool{deps: deps, sync: true}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for job := range p.jobs {
		if err := ProcessVersion(context.Background(), p.deps, job.LibraryID, job.VersionID, job.FileID); err != nil {
			slog.Warn("preview pipeline failed", "version_id", job.VersionID, "err", err)
		}
	}
}

// Enqueue submits a job. In sync mode it runs inline; otherwise it blocks only
// when the queue buffer is full.
func (p *Pool) Enqueue(job Job) {
	if p.sync {
		if err := ProcessVersion(context.Background(), p.deps, job.LibraryID, job.VersionID, job.FileID); err != nil {
			slog.Warn("sync preview pipeline failed", "version_id", job.VersionID, "err", err)
		}
		return
	}
	p.jobs <- job
}

// EnqueueVersion enqueues every queued file in a version. It satisfies the
// enqueuer interface the import pipeline depends on, so imports can hand off
// preview work without importing the concrete pool type.
func (p *Pool) EnqueueVersion(libraryID, versionID uint) {
	p.Enqueue(Job{LibraryID: libraryID, VersionID: versionID})
}

// Shutdown closes the queue and waits for in-flight workers, or for ctx to
// expire. Idempotent.
func (p *Pool) Shutdown(ctx context.Context) {
	if p.sync {
		return
	}
	p.shutdown.Do(func() {
		close(p.jobs)
		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			slog.Warn("preview pool shutdown timed out — workers may still be running", "err", ctx.Err())
		}
	})
}

// ResumeInFlight recovers preview work interrupted by a crash or deploy. Files
// stuck in 'processing' are reset to 'queued' (the worker died mid-render), then
// every version with queued files is re-enqueued. ProcessVersion is idempotent —
// content-addressed derivatives mean a re-run is cheap and safe.
func ResumeInFlight(ctx context.Context, db *gorm.DB, pool *Pool) (int, error) {
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
		pool.Enqueue(Job{LibraryID: r.LibraryID, VersionID: r.VersionID})
	}
	return len(refs), nil
}
