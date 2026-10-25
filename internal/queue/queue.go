// Package queue backs damson's background jobs with River (riverqueue.com), a
// Postgres-backed job queue. It replaces the hand-rolled goroutine pools: the
// import and preview pipelines are unchanged and are simply wrapped by River
// workers, while enqueue becomes a river_job row instead of a buffered channel.
//
// River runs on its own dedicated pgxpool (separate from GORM's connection
// pool) so the queue's LISTEN/NOTIFY-driven, low-latency pickup and its
// connection usage stay isolated from web traffic.
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/preview"
)

// Queue names. Imports and previews run on separate queues so each can be sized
// independently and, later, moved to a dedicated worker container.
const (
	QueueImport      = "import"
	QueuePreview     = "preview"
	QueueStoreLookup = "store_lookup"
)

// storeLookupTimeout bounds one store lookup. A lookup may sit out up to three
// store pauses of at most 10 minutes each, then store a gallery.
const storeLookupTimeout = 45 * time.Minute

// DefaultJobTimeout is the per-job ceiling when a queue timeout is unset.
// Generous because imports can be multi-GB.
const DefaultJobTimeout = 30 * time.Minute

// ImportArgs is the River payload for an import job. It carries only ids; the
// pipeline loads everything else from the DB.
type ImportArgs struct {
	ImportID  uint `json:"import_id"`
	LibraryID uint `json:"library_id"`
}

// Kind identifies the job type in the river_job table.
func (ImportArgs) Kind() string { return "import" }

// PreviewArgs is the River payload for a preview job. FileID == 0 means every
// queued file in the version.
type PreviewArgs struct {
	LibraryID uint `json:"library_id"`
	VersionID uint `json:"version_id"`
	FileID    uint `json:"file_id"`
}

// Kind identifies the job type in the river_job table.
func (PreviewArgs) Kind() string { return "preview" }

// StoreLookupArgs is the River payload for one background store lookup. The
// store_lookups row carries everything else.
type StoreLookupArgs struct {
	LookupID uint `json:"lookup_id"`
}

// Kind identifies the job type in the river_job table.
func (StoreLookupArgs) Kind() string { return "store_lookup" }

// StoreLookupRunner runs one queued store lookup. The API handler implements
// it, since applying a lookup to an asset is the same work Fetch metadata does.
type StoreLookupRunner interface {
	RunStoreLookup(ctx context.Context, lookupID uint) error
	// FailStoreLookup records a lookup failed once River has given up on it.
	FailStoreLookup(ctx context.Context, lookupID uint, reason string) error
}

// storeLookupWorker runs store lookups. run is set by SetStoreLookupRunner
// before the queue starts, because the handler that implements it is built
// after the queue it enqueues on.
type storeLookupWorker struct {
	river.WorkerDefaults[StoreLookupArgs]
	run StoreLookupRunner
}

func (w *storeLookupWorker) Work(ctx context.Context, job *river.Job[StoreLookupArgs]) error {
	if w.run == nil {
		return fmt.Errorf("queue: no store lookup runner set")
	}
	err := w.run.RunStoreLookup(ctx, job.Args.LookupID)
	// On the last attempt, mark the row failed so it doesn't stay queued, and
	// dmsn doesn't wait on it, until the next start. The job's own context may
	// have timed out, so the mark runs without it.
	if err != nil && job.Attempt >= job.MaxAttempts {
		if merr := w.run.FailStoreLookup(context.WithoutCancel(ctx), job.Args.LookupID, err.Error()); merr != nil {
			slog.Warn("queue: mark store lookup failed on last attempt", "lookup_id", job.Args.LookupID, "err", merr)
		}
	}
	return err
}

func (w *storeLookupWorker) Timeout(*river.Job[StoreLookupArgs]) time.Duration {
	return storeLookupTimeout
}

// importWorker runs the import pipeline for one job. timeout is the per-job
// ceiling for this queue.
type importWorker struct {
	river.WorkerDefaults[ImportArgs]
	deps    imports.Deps
	timeout time.Duration
}

func (w *importWorker) Work(ctx context.Context, job *river.Job[ImportArgs]) error {
	err := imports.Run(ctx, w.deps, imports.Job{ImportID: job.Args.ImportID, LibraryID: job.Args.LibraryID})
	if err == nil {
		return nil
	}
	// The job's own context may have timed out, so the checks below run
	// without it.
	ctx = context.WithoutCancel(ctx)
	// The pipeline marks the row failed for terminal input errors (missing
	// source blob, library mismatch). Those won't succeed on retry — cancel so
	// River stops instead of burning attempts.
	if imports.IsFailed(ctx, w.deps.DB, job.Args.ImportID) {
		return river.JobCancel(err)
	}
	// Transient error: let River retry with backoff. On the final attempt, mark
	// the row failed so it doesn't sit non-terminal forever.
	if job.Attempt >= job.MaxAttempts {
		if merr := imports.FailImport(ctx, w.deps, job.Args.ImportID, err.Error()); merr != nil {
			slog.Warn("queue: mark import failed on last attempt", "import_id", job.Args.ImportID, "err", merr)
		}
	}
	return err
}

func (w *importWorker) Timeout(*river.Job[ImportArgs]) time.Duration { return w.timeout }

// previewWorker runs preview processing for one job.
type previewWorker struct {
	river.WorkerDefaults[PreviewArgs]
	deps    preview.Deps
	timeout time.Duration
}

func (w *previewWorker) Work(ctx context.Context, job *river.Job[PreviewArgs]) error {
	return preview.ProcessVersion(ctx, w.deps, job.Args.LibraryID, job.Args.VersionID, job.Args.FileID)
}

func (w *previewWorker) Timeout(*river.Job[PreviewArgs]) time.Duration { return w.timeout }

// Config configures the queue. ImportDeps.Previews is overwritten with the
// River-backed preview enqueuer so the import→preview handoff also flows through
// River.
type Config struct {
	DSN string

	ImportWorkers  int
	PreviewWorkers int

	ImportJobTimeout  time.Duration
	PreviewJobTimeout time.Duration

	ImportDeps  imports.Deps
	PreviewDeps preview.Deps

	Logger *slog.Logger

	// Schema targets a specific Postgres schema for River's tables. Empty means
	// use the connection's search_path (production). Tests set it to isolate
	// per-test schemas.
	Schema string
}

// Queue owns the River client and its dedicated connection pool.
type Queue struct {
	pool         *pgxpool.Pool
	client       *river.Client[pgx.Tx]
	storeLookups *storeLookupWorker
}

// New builds the dedicated pgxpool, applies River's schema migrations, and
// constructs the River client with the import and preview queues wired to their
// workers. The client is not started; call Start.
func New(ctx context.Context, cfg Config) (*Queue, error) {
	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("queue: open pgx pool: %w", err)
	}

	driver := riverpgxv5.New(pool)

	migrator, err := rivermigrate.New(driver, &rivermigrate.Config{Schema: cfg.Schema})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("queue: new migrator: %w", err)
	}
	if err := migrateLocked(ctx, pool, migrator); err != nil {
		pool.Close()
		return nil, fmt.Errorf("queue: migrate: %w", err)
	}

	q := &Queue{pool: pool, storeLookups: &storeLookupWorker{}}

	importTimeout := cfg.ImportJobTimeout
	if importTimeout <= 0 {
		importTimeout = DefaultJobTimeout
	}
	previewTimeout := cfg.PreviewJobTimeout
	if previewTimeout <= 0 {
		previewTimeout = DefaultJobTimeout
	}

	importWorkers := cfg.ImportWorkers
	if importWorkers <= 0 {
		importWorkers = runtime.GOMAXPROCS(0)
	}
	previewWorkers := cfg.PreviewWorkers
	if previewWorkers <= 0 {
		previewWorkers = runtime.GOMAXPROCS(0)
	}

	importDeps := cfg.ImportDeps
	importDeps.Previews = PreviewEnqueuer{q: q}

	workers := river.NewWorkers()
	river.AddWorker(workers, &importWorker{deps: importDeps, timeout: importTimeout})
	river.AddWorker(workers, &previewWorker{deps: cfg.PreviewDeps, timeout: previewTimeout})
	river.AddWorker(workers, q.storeLookups)

	client, err := river.NewClient(driver, &river.Config{
		Logger:     cfg.Logger,
		JobTimeout: DefaultJobTimeout,
		Schema:     cfg.Schema,
		Queues: map[string]river.QueueConfig{
			QueueImport:  {MaxWorkers: importWorkers},
			QueuePreview: {MaxWorkers: previewWorkers},
			// One lookup at a time, so the store sees one caller however many
			// lookups are queued.
			QueueStoreLookup: {MaxWorkers: 1},
		},
		Workers: workers,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("queue: new client: %w", err)
	}
	q.client = client
	return q, nil
}

// migrateLocked runs River's up-migration while holding the same Postgres
// advisory lock db.Migrate uses, so concurrent server startups serialise their
// schema changes (rivermigrate does not lock on its own).
func migrateLocked(ctx context.Context, pool *pgxpool.Pool, migrator *rivermigrate.Migrator[pgx.Tx]) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for lock: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, db.AdvisoryLockKey); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, db.AdvisoryLockKey) }()

	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	return err
}

// SetStoreLookupRunner sets what runs store lookup jobs. Call it before Start.
func (q *Queue) SetStoreLookupRunner(r StoreLookupRunner) { q.storeLookups.run = r }

// Start begins fetching and working jobs.
func (q *Queue) Start(ctx context.Context) error { return q.client.Start(ctx) }

// Stop drains in-flight work, or returns when ctx expires. It also closes the
// dedicated pool.
func (q *Queue) Stop(ctx context.Context) error {
	err := q.client.Stop(ctx)
	q.pool.Close()
	return err
}

// Imports returns the River-backed import enqueuer.
func (q *Queue) Imports() ImportEnqueuer { return ImportEnqueuer{q: q} }

// Previews returns the River-backed preview enqueuer.
func (q *Queue) Previews() PreviewEnqueuer { return PreviewEnqueuer{q: q} }

// StoreLookups returns the River-backed store lookup enqueuer.
func (q *Queue) StoreLookups() StoreLookupEnqueuer { return StoreLookupEnqueuer{q: q} }
