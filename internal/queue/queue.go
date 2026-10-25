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
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/services/enrich"
	"github.com/thomas-elliott/damson/internal/services/ingest"
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

// StoreLookupRunner runs queued store lookups. *enrich.Lookups is the
// production one.
type StoreLookupRunner interface {
	Run(ctx context.Context, lookupID uint) error
	// Fail records a lookup failed once River has given up on it.
	Fail(ctx context.Context, lookupID uint, reason string) error
	// Resume re-enqueues every lookup left queued.
	Resume(ctx context.Context, enq enrich.LookupEnqueuer) (int, error)
}

// kind is one job kind: the queue it runs on, how it runs, what marks its row
// failed once River gives up, and the boot sweep that re-enqueues the rows
// left unfinished. Every kind has all of them, so no row is left in flight.
type kind[A river.JobArgs] struct {
	queue   string
	workers int
	timeout time.Duration
	run     func(context.Context, A) error
	// terminal reports that run already marked the row failed, so River
	// cancels the job instead of retrying it. Optional.
	terminal func(context.Context, A) bool
	fail     func(context.Context, A, string) error
	resume   func(context.Context) (int, error)
}

// worker runs one kind's jobs for River.
type worker[A river.JobArgs] struct {
	river.WorkerDefaults[A]
	kind kind[A]
}

func (w *worker[A]) Work(ctx context.Context, job *river.Job[A]) error {
	err := w.kind.run(ctx, job.Args)
	if err == nil {
		return nil
	}
	// The job's own context may have timed out, so the checks below run
	// without it.
	ctx = context.WithoutCancel(ctx)
	if w.kind.terminal != nil && w.kind.terminal(ctx, job.Args) {
		return river.JobCancel(err)
	}
	// On the last attempt, mark the row failed so it doesn't stay in flight
	// until the next start.
	if job.Attempt >= job.MaxAttempts {
		if merr := w.kind.fail(ctx, job.Args, err.Error()); merr != nil {
			slog.Warn("queue: mark job failed on last attempt", "kind", job.Kind, "job_id", job.ID, "err", merr)
		}
	}
	return err
}

func (w *worker[A]) Timeout(*river.Job[A]) time.Duration { return w.kind.timeout }

// sweep is one kind's boot-time resume.
type sweep struct {
	queue  string
	resume func(context.Context) (int, error)
}

// register adds a kind's worker, queue and sweep.
func register[A river.JobArgs](q *Queue, workers *river.Workers, queues map[string]river.QueueConfig, k kind[A]) {
	river.AddWorker(workers, &worker[A]{kind: k})
	queues[k.queue] = river.QueueConfig{MaxWorkers: k.workers}
	q.sweeps = append(q.sweeps, sweep{queue: k.queue, resume: k.resume})
}

func importKind(q *Queue, deps ingest.Deps, workers int, timeout time.Duration) kind[ImportArgs] {
	return kind[ImportArgs]{
		queue: QueueImport, workers: workers, timeout: timeout,
		run: func(ctx context.Context, a ImportArgs) error {
			return ingest.Run(ctx, deps, ingest.Job{ImportID: a.ImportID, LibraryID: a.LibraryID})
		},
		// The pipeline marks the row failed for terminal input errors
		// (missing source blob, library mismatch), which no retry fixes.
		terminal: func(ctx context.Context, a ImportArgs) bool { return ingest.IsFailed(ctx, deps.DB, a.ImportID) },
		fail: func(ctx context.Context, a ImportArgs, reason string) error {
			return ingest.FailImport(ctx, deps, a.ImportID, reason)
		},
		resume: func(ctx context.Context) (int, error) { return ingest.ResumeInFlight(ctx, deps.DB, q.Imports()) },
	}
}

func previewKind(q *Queue, deps preview.Deps, workers int, timeout time.Duration) kind[PreviewArgs] {
	return kind[PreviewArgs]{
		queue: QueuePreview, workers: workers, timeout: timeout,
		run: func(ctx context.Context, a PreviewArgs) error {
			return preview.ProcessVersion(ctx, deps, a.LibraryID, a.VersionID, a.FileID)
		},
		fail: func(ctx context.Context, a PreviewArgs, _ string) error {
			return preview.FailJob(ctx, deps.DB, preview.Job{LibraryID: a.LibraryID, VersionID: a.VersionID, FileID: a.FileID})
		},
		// Files an earlier build could not render are queued first, so the
		// resume enqueues them with the rest.
		resume: func(ctx context.Context) (int, error) {
			if _, err := preview.QueueNewlySupported(ctx, deps.DB); err != nil {
				return 0, err
			}
			return preview.ResumeInFlight(ctx, deps.DB, q.Previews())
		},
	}
}

func storeLookupKind(q *Queue, r StoreLookupRunner) kind[StoreLookupArgs] {
	return kind[StoreLookupArgs]{
		// One lookup at a time, so the store sees one caller however many
		// lookups are queued.
		queue: QueueStoreLookup, workers: 1, timeout: storeLookupTimeout,
		run: func(ctx context.Context, a StoreLookupArgs) error { return r.Run(ctx, a.LookupID) },
		fail: func(ctx context.Context, a StoreLookupArgs, reason string) error {
			return r.Fail(ctx, a.LookupID, reason)
		},
		resume: func(ctx context.Context) (int, error) { return r.Resume(ctx, q.StoreLookups()) },
	}
}

// Config configures the queue. ImportDeps.Previews is overwritten with the
// River-backed preview enqueuer so the import→preview handoff also flows through
// River.
type Config struct {
	DSN string
	// DB is the GORM database whose transactions jobs are inserted in.
	DB *gorm.DB

	ImportWorkers  int
	PreviewWorkers int

	ImportJobTimeout  time.Duration
	PreviewJobTimeout time.Duration

	ImportDeps  ingest.Deps
	PreviewDeps preview.Deps
	// StoreLookups runs store lookups. Without it the queue has no
	// store_lookup kind.
	StoreLookups StoreLookupRunner

	Logger *slog.Logger

	// Schema targets a specific Postgres schema for River's tables. Empty means
	// use the connection's search_path (production). Tests set it to isolate
	// per-test schemas.
	Schema string
}

// Queue owns the River clients. The worker client runs on its own pgx pool;
// the insert-only client runs on GORM's connections, so a job is inserted in
// the transaction that creates its row.
type Queue struct {
	pool     *pgxpool.Pool
	client   *river.Client[pgx.Tx]
	inserter *river.Client[*sql.Tx]
	sweeps   []sweep
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

	q := &Queue{pool: pool}

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
	queues := map[string]river.QueueConfig{}
	register(q, workers, queues, importKind(q, importDeps, importWorkers, importTimeout))
	register(q, workers, queues, previewKind(q, cfg.PreviewDeps, previewWorkers, previewTimeout))
	if cfg.StoreLookups != nil {
		register(q, workers, queues, storeLookupKind(q, cfg.StoreLookups))
	}

	client, err := river.NewClient(driver, &river.Config{
		Logger:     cfg.Logger,
		JobTimeout: DefaultJobTimeout,
		Schema:     cfg.Schema,
		Queues:     queues,
		Workers:    workers,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("queue: new client: %w", err)
	}
	sqlDB, err := cfg.DB.DB()
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("queue: gorm sql.DB: %w", err)
	}
	inserter, err := river.NewClient(riverdatabasesql.New(sqlDB), &river.Config{
		Logger: cfg.Logger,
		Schema: cfg.Schema,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("queue: new insert client: %w", err)
	}
	q.inserter = inserter
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

// Resume runs every kind's boot sweep, re-enqueueing the rows left
// unfinished by a crash or deploy, and logs what each found. River dedupes by
// args, so it is safe on every boot.
func (q *Queue) Resume(ctx context.Context) {
	for _, s := range q.sweeps {
		if n, err := s.resume(ctx); err != nil {
			slog.Warn("queue: resume failed", "queue", s.queue, "err", err)
		} else if n > 0 {
			slog.Info("queue: resumed", "queue", s.queue, "enqueued", n)
		}
	}
}

// insert adds a job in db's transaction, so it commits or rolls back with the
// row it is for. Outside a transaction it is inserted on its own.
func (q *Queue) insert(db *gorm.DB, args river.JobArgs, queue string) error {
	opts := &river.InsertOpts{
		Queue:       queue,
		MaxAttempts: maxAttempts,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: activeStates},
	}
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	if tx, ok := db.Statement.ConnPool.(*sql.Tx); ok {
		_, err = q.inserter.InsertTx(ctx, tx, args, opts)
	} else {
		_, err = q.inserter.Insert(ctx, args, opts)
	}
	if err != nil {
		return fmt.Errorf("queue: insert %s job: %w", args.Kind(), err)
	}
	return nil
}

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
