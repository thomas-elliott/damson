# Migrate the self-rolled queues to River

**Status:** signed off (2026-07-19) — ready for `implement`.
**Alignment artifact:** `.lavish/river-migration-plan.html`.
**Supersedes the queue mechanism in:** `docs/specs/2026-06-18-preview-processing-queue.md`
(the per-file `preview_status` state model is *kept*; only the transport changes).

## Problem

damson runs two background job queues — imports and previews — both hand-rolled as a bounded
pool of goroutines draining an in-memory buffered channel, with the database as the durable
source of truth:

- Import pool: `internal/services/imports/worker.go` (`Pool`, `New`, `worker`, `Enqueue`, `Shutdown`).
- Preview pool: `internal/services/preview/pool.go` (same shape + `ResumeInFlight`).

The design is durable (DB-as-truth + startup resume sweep + idempotent pipelines) but has
three gaps that matter now that imports can be multi-GB and processing may need to move to
separate containers:

1. **No per-job timeout.** Workers run on `context.Background()` (`worker.go:93`,
   `pool.go:58`); a wedged job holds one of only `GOMAXPROCS` slots forever and can starve
   the whole pool.
2. **No automatic retry/backoff.** A failure sets `stage=failed` / `preview_status=failed`
   and stops; recovery is a manual requeue by a human.
3. **Single process.** The pools live inside the API process, so workers scale with — and
   share the fate of — the web tier. They can't be moved to dedicated worker containers.

The goal is to move both queues onto [River](https://riverqueue.com) (the Postgres-backed Go
job queue) to get per-job timeouts, retries with backoff, durable job rows, and a clean path
to worker containers — **while preserving** the current durability guarantees (DB as source
of truth, idempotent pipelines, crash resume).

## Chosen approach

**River as the transport behind a thin `Enqueuer` interface seam; the pipeline bodies do not
change.** `imports.Run(ctx, deps, job)` and `preview.ProcessVersion(ctx, deps, ...)` already
take only IDs and do all the work by loading state from the DB. River replaces only the
*transport*: the buffered channel becomes a `river_job` row, and the `for job := range p.jobs`
loop becomes a River `Worker.Work` method that calls the same function.

A one-line interface seam (`ImportEnqueuer` / `PreviewEnqueuer`) lets production inject a
River-backed enqueuer while handler tests inject an inline enqueuer that runs the pipeline
synchronously — keeping test assertions deterministic without standing up a River client. The
River path itself is covered by dedicated Postgres integration tests.

**Driver: `riverpgxv5` on a dedicated `pgxpool`** (Approach A), separate from GORM's pool.
This is River's first-class driver (LISTEN/NOTIFY → sub-second job pickup), keeps River's
connection usage from starving GORM's and vice-versa, and is the cleanest footing for the
future worker-container split. `pgx/v5` is already an indirect dependency (via GORM's Postgres
driver); this promotes it to a direct one.

Why this beat the alternatives:
- **`riverdatabasesql` sharing GORM's `*sql.DB`** — simpler (one pool) but River's
  second-class driver: polling only (no LISTEN/NOTIFY), and the queue contends with web
  traffic for the same connections. Rejected for weaker multi-container footing.
- **Patch the self-rolled pools** (add `context.WithTimeout` + an `attempts` column) — tiny
  diff and no new dependency, but delivers neither durable job rows nor multi-container
  workers, and amounts to re-implementing a worse River. Rejected as not meeting the goal.

**SQLite is dropped.** SQLite is used only as the default test database (River is
Postgres-only). Rather than contort the design to keep a SQLite-compatible path, all tests
move onto the existing `internal/testdb` schema-per-test Postgres harness. This is a
prerequisite stage that also lets River be tested for real.

## Task breakdown

Ordered stages, roughly one commit each. Stage 0 is a prerequisite; stages 1–4 are the
migration proper.

### Stage 0 — Drop SQLite; standardise tests on the Postgres harness

- Migrate the SQLite-based test files onto `internal/testdb` (schema-per-test): the helpers in
  `internal/handlers/api/api_test.go` (`newTestDB`), `internal/auth/bootstrap_test.go`,
  `internal/services/imports/enrich_test.go`, `internal/services/preview/process_test.go`,
  `internal/services/audit/purge_test.go`, `internal/services/storage/cleanup/cleanup_test.go`,
  and any other `gorm.io/driver/sqlite` importer.
- All tests **require** `DAMSON_TEST_POSTGRES_DSN` (reuse the existing harness / E2E setup).
  No testcontainers. `go test ./...` now needs a Postgres; CI and local dev must provide one.
- Remove `gorm.io/driver/sqlite` and `github.com/mattn/go-sqlite3` from `go.mod`, and the
  SQLite branch in `cmd/e2e-server/main.go`.
- Remove the SQLite-only `aggTime` scanner hack in `internal/services/preview/process.go`
  (it exists because SQLite loses column affinity through `MAX(timestamp)` aggregates;
  Postgres returns `time.Time` directly). Simplify the affected rollup queries.
- Update `internal/testdb/postgres.go`'s package doc (no longer "instead of the in-memory
  SQLite default") and drop the `//go:build postgres_integration` gating / env-var-skip now
  that Postgres is the only backend. Verify the full suite is green against Postgres.

### Stage 1 — Interface seam (pure refactor, no River yet)

- Introduce `ImportEnqueuer` (`Enqueue(imports.Job)`) and `PreviewEnqueuer`
  (`Enqueue(preview.Job)` + `EnqueueVersion(libraryID, versionID uint)`) interfaces. The
  preview `PreviewEnqueuer` in `imports/worker.go` already models part of this — reconcile
  the naming.
- Change `internal/handlers/api/handler.go` `Deps`/`Handler` fields from `*imports.Pool` /
  `*preview.Pool` to the interface types. The existing concrete pools already satisfy them.
- Build stays green; tests unchanged. This isolates the transport swap that follows.

### Stage 2 — Add River + `internal/queue` package

- Add `github.com/riverqueue/river`, `.../riverdriver/riverpgxv5`, and promote
  `github.com/jackc/pgx/v5` to a direct dependency.
- New `internal/queue` package that:
  - Builds a dedicated `pgxpool.Pool` from the Postgres DSN.
  - Defines job args: `ImportArgs{ImportID, LibraryID uint}` (Kind `"import"`) and
    `PreviewArgs{LibraryID, VersionID, FileID uint}` (Kind `"preview"`).
  - Defines `Worker` structs whose `Work` calls `imports.Run` / `preview.ProcessVersion`
    (deps injected into the worker, mirroring today's pool deps).
  - Registers workers, constructs the `river.Client` with two queues (`import`, `preview`),
    each with its own `MaxWorkers`, and a per-queue/`Config.JobTimeout` default of **30 min**.
  - Runs River's schema migrations at startup via `rivermigrate` (programmatically), under the
    existing advisory-lock discipline in `internal/db` — alongside the custom SQL migration
    runner, not replacing it.
  - Exposes River-backed `ImportEnqueuer` / `PreviewEnqueuer` implementations
    (`Insert` / `InsertTx`).
- New config in `internal/config/config.go`: `DAMSON_IMPORT_QUEUE_WORKERS` and
  `DAMSON_PREVIEW_QUEUE_WORKERS` (with sane defaults), plus optional per-queue timeout
  overrides. **Remove `DAMSON_IMPORT_WORKERS` outright** — roll forward, no backwards-compat.
- Not yet wired into `main.go`.

### Stage 3 — Wire River into `cmd/server` (and `cmd/e2e-server`)

- Construct the queue client, `Start` it, inject the River enqueuers into `api.Deps`, and
  `Stop` it during graceful shutdown (replacing `importPool.Shutdown` / `previewPool.Shutdown`).
- Move enqueue to **`InsertTx` at seed time**: insert the River job in the same transaction
  that creates the domain row (`SeedImport` in `imports/resume.go`; the `MarkVersionQueued`
  handoff in `imports/pipeline.go:enqueuePreviews` and the preview requeue endpoints), so a
  committed domain row always has a matching job and vice-versa.
- Replace both channel-based `ResumeInFlight` sweeps with an **idempotent River reconcile**:
  on startup, re-enqueue any domain row in a non-terminal state that has no live River job,
  deduped via `river.UniqueOpts` (by args + non-terminal states) so it is safe to run every
  boot and safe for the one-time cutover of rows that were in flight before River existed.
  For previews, keep the `processing → queued` reset before reconcile (a worker died
  mid-render). Thereafter River's own stuck-job rescue covers crashes.
- Delete the production path of the channel pools (`New`/`worker`/channel `Enqueue`); keep an
  inline/sync enqueuer for tests (see Resolved questions).

### Stage 4 — Failure / retry / timeout semantics + tests

- Map errors: return an error from `Work` → River retries with exponential backoff. Terminal,
  non-retryable errors (source blob missing, library mismatch) use `river.JobCancel(err)` to
  skip retries. Mark the domain row `failed` only when River cancels or exhausts attempts
  (detect last attempt via `job.Attempt`/`MaxAttempts`, or an `ErrorHandler`).
- Preserve preview's per-file best-effort behaviour (a single file's render error marks that
  file `failed` and the batch continues; the version job still returns nil).
- Set the 30 min per-job timeout and confirm long (multi-GB) imports run to completion while a
  wedged job is killed and retried.
- Add Postgres integration tests for the River path (enqueue → work → domain state; retry;
  cancel; reconcile/cutover). Keep handler tests on the inline enqueuer for determinism.

### Deferred (out of scope) — `cmd/worker`

A worker-only binary (same River client, no HTTP) that proves the multi-container split is
**not** built in this change. It needs further decisions about which processing moves off-box.
The `internal/queue` package should be structured so adding it later is additive, not a
rewrite.

## Reuse

- **`internal/testdb/postgres.go`** — the schema-per-test Postgres harness (env-var DSN,
  per-test schema, auto-migrate, cleanup) already exists and is used by `*_postgres_test.go`
  files; Stage 0 extends it to all tests rather than building anything new. River's own tables
  slot in via `rivermigrate`'s `Schema` config for isolated per-test schemas.
- **`imports.Run` / `preview.ProcessVersion`** — the entire job logic is reused unchanged;
  River only wraps them.
- **`internal/db` advisory-lock migration discipline** — reused to gate `rivermigrate` at
  startup.
- **River** (`riverqueue/river`, `riverpgxv5`, `rivermigrate`) — the queue, timeouts,
  retries/backoff, unique-job dedupe, and stuck-job rescue are all off-the-shelf; we write no
  queue mechanism of our own.
- **Existing `preview_status` / `asset_imports.stage` state model** — kept as the domain
  source of truth; River is layered under it, not a replacement for it.

## Resolved questions

- **Driver:** Approach A — `riverpgxv5` on a dedicated `pgxpool` (not `riverdatabasesql`
  sharing GORM's pool).
- **Test-DB provisioning:** require `DAMSON_TEST_POSTGRES_DSN`, reusing the existing
  E2E/`internal/testdb` harness. No testcontainers. All `go test` runs now need Postgres.
- **`cmd/worker`:** deferred to a future change; needs decisions about what processing moves
  off-box. Structure the code so it's later additive.
- **Config compatibility:** roll forward — replace `DAMSON_IMPORT_WORKERS` with per-queue
  `DAMSON_{IMPORT,PREVIEW}_QUEUE_WORKERS`. No deprecation path, no backwards-compat.
- **Timeout default:** 30 min per job (imports can be multi-GB), env-tunable per queue.
- **SSE broadcaster (known limitation, not addressed here):** the live stage/preview SSE
  broadcaster stays in-memory. It keeps working single-process; when workers eventually move
  to a separate container, cross-process SSE will need Postgres LISTEN/NOTIFY or polling. Out
  of scope for this migration; flagged as a follow-up tied to the deferred `cmd/worker`.

## Known risks

- **Cutover of in-flight rows** — rows non-terminal at the deploy that introduces River have
  no River job. The idempotent reconcile (Stage 3) handles this; it needs an explicit test.
- **Two connection pools** — the River `pgxpool` plus GORM's pool means total Postgres
  connection count must be tuned (both bounded, sized against the server's `max_connections`).
- **All tests need Postgres** — dropping SQLite means `go test ./...` no longer runs
  standalone; CI and local dev must provide a Postgres instance.

## Deferred comments

- `internal/testdb/postgres.go` (OpenSchema) — why MaxOpenConns(4)/MaxIdleConns(2)+Close on cleanup: one GORM pool is opened per test; unbounded, the many concurrent per-test schemas exhaust Postgres `max_connections` (default 100).
- `internal/services/preview/process.go` (rollupRow.UpdatedAt) — why `sql.NullTime`: `MAX(preview_updated_at)` is NULL when a version has files none of which have been touched yet; NullTime handles that without a scan error (this replaced the SQLite-only `aggTime` string-parsing hack).
- `internal/testdb/postgres.go` (NewPostgres/RequireDSN) — why hard-fail instead of `t.Skip`: a silent skip lets a run with no database look like a pass; Postgres is now mandatory.
- `internal/queue/queue.go` (migrateLocked) — why reuse `db.AdvisoryLockKey`: rivermigrate does not self-lock, so two server boots could race on River's CREATE TABLE; taking the same advisory lock db.Migrate uses serialises them.
- `internal/queue/queue.go` (New, importDeps.Previews override) — why: the import→preview handoff must enqueue River jobs, and the enqueuer references the Queue whose client is populated after workers are built; safe because Work runs only after Start.
- `internal/queue/enqueuer.go` — why context.Background(): enqueue is fire-and-forget (matching the old channel pool); a request's cancellation must not abort the insert, and the startup reconcile covers any gap.
