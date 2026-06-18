package imports

import (
	"context"
	"log/slog"
	"runtime"
	"sync"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/services/storage"
)

// Job is the unit of work for the import pipeline. It carries only the
// import row's id (and the library id for broadcaster routing); everything
// else the pipeline needs is loaded from the DB. This keeps Enqueue cheap
// and makes resume-on-startup trivial — feed in the IDs of stuck rows and
// the pipeline figures out what's left to do.
type Job struct {
	ImportID  uint
	LibraryID uint
}

// PreviewEnqueuer hands a committed version off to the preview processing
// queue. Defined here (not as a concrete *preview.Pool) so the import package
// depends only on the enqueue behaviour, keeping the two pipelines decoupled.
type PreviewEnqueuer interface {
	EnqueueVersion(libraryID, versionID uint)
}

// Deps are the long-lived collaborators every pipeline run needs. They live
// on the Pool and are passed into Run.
type Deps struct {
	DB          *gorm.DB
	Storage     storage.Service
	Broadcaster *Broadcaster
	// Previews receives each version after its asset_files are committed, so
	// preview generation runs on its own queue instead of blocking the import.
	// Optional — a nil enqueuer simply skips the handoff.
	Previews PreviewEnqueuer
}

// Pool is a bounded worker pool that drains a job queue. Construct via
// New(deps, size) for production or NewSync(deps) for tests that need the
// pipeline to run inline.
type Pool struct {
	deps     Deps
	jobs     chan Job
	wg       sync.WaitGroup
	sync     bool
	shutdown sync.Once
}

// New starts a worker pool with `size` goroutines. If size <= 0 the pool
// uses runtime.GOMAXPROCS(0).
func New(deps Deps, size int) *Pool {
	if size <= 0 {
		size = runtime.GOMAXPROCS(0)
	}
	p := &Pool{
		deps: deps,
		jobs: make(chan Job, size*4),
	}
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// NewSync returns a pool whose Enqueue runs the pipeline on the calling
// goroutine. Used by tests to keep stage transitions deterministic without
// the real channel/goroutine machinery.
func NewSync(deps Deps) *Pool {
	return &Pool{deps: deps, sync: true}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for job := range p.jobs {
		ctx := context.Background()
		if err := Run(ctx, p.deps, job); err != nil {
			slog.Warn("import pipeline failed", "import_id", job.ImportID, "err", err)
		}
	}
}

// Enqueue submits a job. In sync mode the job runs inline. In async mode
// the call blocks only if the queue buffer is full.
func (p *Pool) Enqueue(job Job) {
	if p.sync {
		if err := Run(context.Background(), p.deps, job); err != nil {
			slog.Warn("sync import pipeline failed", "import_id", job.ImportID, "err", err)
		}
		return
	}
	p.jobs <- job
}

// Shutdown closes the queue and waits for in-flight workers to finish, or
// for ctx to expire. Idempotent — multiple calls are no-ops after the
// first.
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
			slog.Warn("import pool shutdown timed out — workers may still be running", "err", ctx.Err())
		}
	})
}
