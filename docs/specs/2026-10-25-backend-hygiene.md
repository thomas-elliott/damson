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
