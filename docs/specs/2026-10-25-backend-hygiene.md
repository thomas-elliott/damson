# Backend hygiene: startup, shutdown, logging, lint

Covers DAM-59

## Why are we doing this

Contributors get a server that starts, fails and stops cleanly, logs only JSON without parameter
values, and a linter that catches error-handling and context mistakes before review.

## Steering

Session, 2026-10-25, operator: "Split it into however many specs they need for each phase", then
chose "Fewer, larger specs": one spec per phase.

## Design

none. Direct decision: Phase 1 of the backend architecture review, as scoped on DAM-59.

## Decisions

### Shutdown order
Answer: already right. HTTP stops before the queue today, so it drops out of scope. SSE
cancellation through `BaseContext`, waiting on background goroutines and closing the GORM pool
remain.
By: settled by `cmd/server/main.go` (2026-10-25)

### What happens to an invalid config value?
Answer: startup fails with a non-zero exit and an slog line naming the variable, for example
`QUEUE_WORKERS=abc`. Today such values fall back to defaults silently.
By: recommended, operator said "go" (2026-10-25)

### How is golangci-lint installed?
Answer: pinned as a Go tool in `go.mod`, so `just lint backend` needs no separate install. Every
finding it raises is fixed in this change.
By: recommended, operator said "go" (2026-10-25)

### Are DB pool limits configurable?
Answer: no. They are constants in `db.Open`, with no new environment variables.
By: recommended, operator said "go" (2026-10-25)

## Approach

- `cmd/server`: `main` becomes `run() error`; `config.Load` returns an error and rejects invalid
  values; SSE is cancelled through `BaseContext`; background goroutines are waited on (errgroup);
  the GORM pool is closed on shutdown.
- `db.Open`: an slog adapter for GORM that never logs parameter values, and pool limits.
- Test hooks out of production code: `auth.SetUserForTest`, `imports.NewSync`, `preview.NewSync`.
- Delete dead exported code: `auth.Mode`, `config.LogLevel`, `storage.Service.PresignGet`;
  unexport the `metadata` surface nothing outside the package uses.
- golangci-lint with errorlint, contextcheck and unused in `just lint backend`.
- Gather the handler helpers scattered across feature files into `helpers.go`; drop the stale
  "SQLite" comments and milestone tags.

## Acceptance

- `just check` passes with golangci-lint (errorlint, contextcheck, unused) in it.
- A startup config error, missing or invalid, exits non-zero with an slog line and no panic.
- GORM output reaches the log as JSON lines, and a slow-query line carries no parameter values.
- Stopping the server with an SSE stream open finishes without waiting out the shutdown timeout.
- `docs/CONVENTIONS.md` names the linter.

## Out of scope

- `t.Parallel` and removing `time.Sleep` from tests.
- Dead routes and SPA props (DAM-54).

## Start here

- `docs/architecture.md`
- `docs/CONVENTIONS.md`
- `internal/config/config.go`

## Deferred follow-ups

None

## Implementation report

Mode: committing — feature branch `dam-59-backend-hygiene`, one commit per stage (repo AGENTS.md).

### Stages

1. Config errors: `config.Load` returns an error and rejects invalid values; `main` becomes
   `run() error`, so a config error exits non-zero with an slog line and no panic.
2. Shutdown: SSE cancelled through `BaseContext`; background goroutines waited on with errgroup;
   the GORM pool closed on shutdown.
3. `db.Open`: an slog adapter for GORM that never logs parameter values, and pool limits.
4. Test hooks out of production code: `auth.SetUserForTest`, `imports.NewSync`, `preview.NewSync`.
5. Dead exported code: `auth.Mode`, `config.LogLevel`, `storage.Service.PresignGet`; unexport the
   `metadata` surface nothing outside the package uses.
6. Handler helpers gathered into `helpers.go`; stale "SQLite" comments and milestone tags dropped.
7. golangci-lint (errorlint, contextcheck, unused) pinned as a Go tool in `just lint backend`, every
   finding fixed; `docs/CONVENTIONS.md` names the linter.

### Choices made

- `internal/config/config.go`: a non-positive worker count or duration is rejected like an
  unparseable one; empty still means the default. `DAMSON_S3_FORCE_PATH_STYLE` parses with
  `strconv.ParseBool`, so `1`/`0` are accepted and anything else is an error.
- `internal/config/config.go`: every config error is collected and reported together
  (`errors.Join`), rather than stopping at the first.
- `cmd/server/main.go`: `DAMSON_LOG_LEVEL` and `DAMSON_LOG_FORMAT` are rejected when unknown, the
  same as values `config.Load` reads.
- `internal/handlers/api/imports.go`: the server's `BaseContext` carries the stopping signal as a
  value (`api.WithServerStopping`) rather than being cancelled, so shutdown ends the SSE streams
  and lets uploads and other in-flight requests finish; `Broadcaster.Close`, which did this job
  before, is deleted.
- `cmd/server/main.go`: background goroutines run on a plain `errgroup.Group`, not
  `WithContext`, so one failing does not cancel the others; the wait is bounded by the 10s
  shutdown timeout.
- `internal/db/db.go`: GORM's own `logger.NewSlogLogger` with `ParameterizedQueries`, plus
  `logger.RecorderParamsFilter` overridden, because `Scan` logs through GORM's trace recorder and
  bypasses the setting. Slow is 200ms (GORM's default). A record-not-found is no longer logged.
  Pool: 20 open, 10 idle, 30m lifetime, 5m idle time.
- `internal/services/storage/localfs.go`: `NewLocalFS` loses its `baseURL` parameter, which only
  `PresignGet` read.
- `internal/services/metadata`: the unexported client type is `httpClient`, not `client`, so it
  does not shadow the many `client` parameters; the unused `Registry.Client()` accessor is
  deleted with it.
- `internal/handlers/api/helpers.go`: gathered `parseIDParam`, `parsePositiveID`,
  `parsePagination` (with its page-size constants), `requireSysadmin`, `isUniqueViolation`,
  `respond502` and `respond503`. `respondLookupFailed` stays in `metadata.go` (store-specific),
  `getLibrary`/`getMembership` beside the middleware that sets them, and the upload responders in
  `upload_errors.go`.
- The SQLite comments on the hand-written child deletes now say the cascades make them redundant;
  the deletes themselves stay for the library and asset work in DAM-60.
- `go.mod`: golangci-lint v2.14.0 (v2.15.0 was under two weeks old). Tool dependencies share the
  module graph, so adding it raised `golang.org/x/crypto` 0.54→0.57, `golang.org/x/net`
  0.57→0.59 and `golang.org/x/text` 0.40→0.42 for the server too.
- `.golangci.yml`: only the three named linters (`default: none`), with the per-linter and
  same-issue caps off so the gate shows every finding.

### Results

| Item | Status | Notes |
| --- | --- | --- |
| `just check` passes with golangci-lint (errorlint, contextcheck, unused) in it | passed | `sg docker -c "just check"`, exit 0: `0 issues.` from golangci-lint, every Go package ok, Vitest 339/339, stories 94/94, e2e 20 passed. A throwaway file with a `context.Background()` call and an unused func got one contextcheck and one unused finding, so both linters are running. |
| A startup config error, missing or invalid, exits non-zero with an slog line and no panic | passed | `TestMain_ConfigErrorExitsWithAnSlogLine` re-runs the test binary through the real `main` (invalid `DAMSON_IMPORT_QUEUE_WORKERS`, missing DSN, bad `DAMSON_LOG_LEVEL`). It fails if `config.Load` goes back to `log.Panicf` (exit 2 and a stack) or if `main` stops logging the error. `TestLoad_RejectsInvalidValues` fails if a parser falls back to its default again. |
| GORM output reaches the log as JSON lines, and a slow-query line carries no parameter values | passed | `TestOpen_LogsSQLAsJSONWithoutParameterValues` runs against real Postgres: a 250ms `Scan` and a failing `Exec`, each with a bound secret. It failed on its first run, because `Scan` bypasses `ParameterizedQueries`, and passes with `RecorderParamsFilter` overridden. It fails if either override is removed. |
| Stopping the server with an SSE stream open finishes without waiting out the shutdown timeout | passed | `TestShutdown_EndsSSEStreamsThenStopsTheQueue` uses the real import and preview SSE handlers on real Postgres with `newHTTPServer`. When `stopStreams` was replaced with a no-op it failed (`shutdown took 2.000829135s`). Real binary: SIGTERM with `/api/imports/events` open exited 0 in 0.0086s; the background workers logged stopping and there were no warn or error lines. The throwaway database `damson_proof` was dropped afterwards. |
| `docs/CONVENTIONS.md` names the linter | passed | Formatting & linting names golangci-lint, its three linters, `.golangci.yml`, and the Go-tool pin. |
| stage 3 pool limits | passed | No test: the limits are constants passed to `database/sql`, and a test would only repeat them. Exercised by every Postgres-backed test through `db.Open`. |
| stages 4–6 (test hooks, dead code, helpers) | passed | No new tests: these are moves and deletions. The full Go suite under `just check` exercises the moved pools and the `setupRouter` change, which goes through `auth.Middleware` for 247 call sites. |
| review: stages 1…7 | passed | `### Review — stages 1…7`, no blockers, no findings to fix. |

### Review — stages 1…7

#### Verdict
No blockers

#### Checked
repo rules (AGENTS.md, docs/CONVENTIONS.md, docs/architecture.md), documented hazards (500 generic message, no tokens or raw values in logs, library_id scoping, `go mod tidy` drift, doc kept true with the change), mechanical: shape conformance, completeness honesty, coverage, cross-boundary contract, logic, error handling, silent failure, input validation, idempotency & resilience, credentials & secrets, security, concurrency, resource management, logging & observability, smell baseline · issues in: none · n/a: shared-library precedence (no UI added), external contracts (no new calls), performance (no hot path touched) · validation: in-context, 0 candidates, 0 confirmed, 0 refuted · ran: `go test ./cmd/server ./internal/config ./internal/db ./internal/broadcast` pass, `go tool golangci-lint run ./...` 0 issues · read from code: `run()` defer order closes the pool after the queue stop and the bounded background wait; the `BaseContext` stopping value ends both SSE handlers (imports, previews) without cancelling other requests, and a stream opened after shutdown starts sees an already-closed channel; GORM v1.31.2 `traceRecorder.ParamsFilter` reads `logger.RecorderParamsFilter` and is only used for the log line, so dropping params does not change query results; `pgconn.PgError.Error()` carries no DETAIL, so SQL error lines name no key values · `Broadcaster.Close` removal has no remaining callers · README and CONVENTIONS updated for the panic→error and helper moves · needs a human approver: new tool dependency (golangci-lint v2.14.0) and the `golang.org/x/{crypto,net,text}` bumps it pulls into the server's module graph (recorded in Choices made) · not examined (budget): full `just check`, the stage 5–6 metadata renames beyond compile and lint
