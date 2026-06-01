# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project documents

Before making non-trivial changes, read the docs in `docs/`. They describe the code as it stands today.

- `docs/architecture.md` — current architecture, data model, API surface, shipped decisions
- `docs/CONVENTIONS.md` — stack, formatting rules, handler patterns, test helpers
- `docs/design-system.md` — frontend design system spec
- `docs/progress.md` — milestone-by-milestone status of what's actually in the tree; read this before starting a new slice so you know what foundations do (and don't) already exist
- `docs/archive/` — original product / technical / UI design docs (Draft v1/v2). They drove the M0–M7 build and are kept verbatim for historical reference; section numbers and API shapes have drifted, so prefer `architecture.md` for the live shape and only consult the archive for the *why*

## Development commands

All workflows go through Task (`Taskfile.yml`) — do not invoke `go`, `npm`, `docker compose` directly for build/test/lint operations. Run `task --list` to discover targets. Naming follows a canonical vocabulary (`build`, `test`, `lint`, `format`, `check`, `clean`, `run`/`dev`, `install`) with `:` for subtasks. The high-frequency ones:

```
task                  # alias for `task --list`
task install          # Go modules + npm + Playwright browsers (fresh clone)
task dev              # Go API only (air hot-reload)
task dev:spa          # backend + Vite SPA in parallel
task build            # SPA + Go production binary
task check            # read-only: lint + format:check + mod:tidy:check
task lint             # go vet + eslint
task format           # go fmt + prettier --write
task test:unit        # Go unit tests + Vitest
task test:e2e         # Playwright (boots e2e-server + vite internally)
task test:integration # Postgres-backed Go integration tests
task ci               # full preflight: check + test:unit + test:e2e
task clean            # remove build artifacts
```

Subdomain tasks split by language: `lint:go` / `lint:web`, `format:go` / `format:web`, `format:check:go` / `format:check:web`, `test:unit:go` / `test:unit:web`. Use the umbrella when in doubt.

Run one Go test:

```
go test ./internal/handlers/api -run '^TestName$'
go test ./internal/handlers/api -run '^TestName$/SubtestName$'
```

Vitest single-file:

```
cd web && npm test -- src/path/to/file.test.tsx --run
```

## Conventions

The full rules live in `docs/CONVENTIONS.md`. Load-bearing ones worth keeping in mind while editing:

- **Error responses.** `400` includes `err.Error()`; `404` / `409` used where they fit; `500` always returns the generic `"internal error"` string — don't leak internals. Shape: `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`. Inside a `Transaction(func)` use the sentinel constructors (`errBadRequestf`, `errConflictf`) and let the outer handler call `respondGORM(c, err, "<entity> not found")` once — single source of truth for the status mapping.
- **Migration shape.** `NNN_descriptive_name.sql` (zero-padded 3-digit sequence), wrapped in `BEGIN; … COMMIT;`, `IF NOT EXISTS` throughout. Timestamps are `timestamptz NOT NULL DEFAULT now()`. Any schema change must land in BOTH a new SQL migration AND the model struct tags (see "Migrations vs tests" below for why).
- **Frontend API discipline.** Never call `fetch` directly from components — go through `fetchJson` / `fetchMaybeJson` in `src/api.ts` so bearer-token attachment, 401 renewal, and `ApiError` shape stay uniform. Query keys live in `lib/queryKeys.ts` only. Data-fetching hooks wrap TanStack Query in `hooks/useDataQueries.ts` and return the normalized `{ data, loading, error, reload }` shape.
- **Never log tokens or raw session values.** Applies to `slog` debug output, error wraps, anything that might end up in a log collector.
- **Mirror existing patterns; don't refactor unrelated code.** Focused changes only. Read `docs/` before architectural changes.
- **Taskfile shell.** `Taskfile.yml` commands run under mvdan/sh (Task's embedded interpreter), not bash. Bash-isms like `trap INT TERM`, process substitution, and `set -o pipefail` fail silently or with confusing errors — wrap in `bash -c '...'` when you need them.
- **`go mod tidy` is enforced in CI.** `.github/workflows/ci.yml` runs `go mod tidy` then `git diff --exit-code -- go.mod go.sum` and fails the build on any drift. Whenever you add an import — especially when a previously-indirect dep becomes a direct require because new code uses it — run `task mod:tidy:check` (or `go mod tidy` + diff) before committing. `task check` (and `task ci`) include this step; running either locally before pushing avoids CI-only failures.

## Architecture

### Ownership model: library-based tenancy

**Not per-user. Not shared.** A `libraries` row is the tenancy boundary; users get access via `library_memberships` (role `admin|member`). Within a library, every member sees the same data — there is no per-user partitioning below that. Users may belong to multiple libraries and switch via `users.current_library_id`.

Implications:
- Every library-scoped entity (`assets`, `projects`, `collections`, `tags`, …) carries `library_id` directly. Queries filter at the top level — never infer via parent FKs.
- `created_by_user_id` / `pinned_by_user_id` columns are **attribution, not ownership** — they do not affect access rules.
- Server-level `users.role = 'user' | 'sysadmin'` is orthogonal to per-library roles.
- Tags are library-scoped: `unique(library_id, path)`. A shared cross-library vocabulary is a deliberate future add.

### Pluggable auth (exclusive modes)

`internal/auth` defines `Authenticator` with two concrete impls, picked at startup by `DAMSON_AUTH_MODE`:

- **Local** (default; `local.go`) — bcrypt password hashes in `users.password_hash`, issues HS256 JWTs (1h TTL, `jwtIssuer = "damson-local"`). `users.tokens_valid_after` acts as a revocation watermark: JWTs with `iat` before that instant are rejected. Synthetic subject `"local:<email>"` so the `users.subject` unique index still works.
- **OIDC** (`oidc.go`) — validates IdP bearer tokens, upserts the user by `sub` only (email/name are optional claims, seeded once at creation and thereafter app-owned via `PATCH /api/me`). A user with no email is flagged `needs_profile` so the SPA collects one on first login.

Both modes share one bootstrap path (`BootstrapFirstAdmin`): the first user on an empty system (no other users, no libraries) gets sysadmin + "Personal" library + admin membership + `current_library_id`, all in one transaction. No env-var allowlist — identity is whoever signs in first.

Local-mode bootstrap has two entry points: the env-seeded startup path (`SeedLocalAdmin`, active when both `DAMSON_LOCAL_ADMIN_EMAIL` and `_PASSWORD` are set) and the HTTP path (`POST /api/auth/bootstrap`, reachable when no users exist). The latter is what `GET /api/auth/mode`'s `needs_bootstrap` flag signals to the SPA.

### Pluggable storage

`internal/services/storage.Service` has two impls, picked by `DAMSON_STORAGE_MODE`:

- `LocalFS` (default) — content-addressed under `<root>/blobs/<hh>/<hash>` and `<root>/thumbs/<hh>/<hash>`. Atomic rename, SHA-256 dedup.
- `S3` — aws-sdk-go-v2 with endpoint override + path-style for MinIO. HEAD-before-PUT dedup.

Layout matches between backends so a deployment can migrate with a plain copy.

### Request lifecycle

1. `auth.Middleware(authenticator)` — verifies bearer token, upserts-or-loads the user, attaches to Gin context via `auth.GetUser(c)`.
2. For library-scoped routes, add `h.requireLibrary()` middleware — resolves `current_library_id`, verifies membership, attaches `(*Library, *LibraryMembership)` via `getLibrary(c)` / `getMembership(c)`. 403 on any gap (never 404, so membership existence isn't leaked).
3. Handler runs its DB work scoped to `library_id`.

Route wiring in `internal/handlers/api/handler.go`: public routes, then an `authed` group with the auth middleware, then a `scoped` sub-group that layers `requireLibrary()`.

### Migrations vs tests

**Production:** SQL-only migrations in `database/migrations/NNN_*.sql`. Runner in `internal/db/migrate.go` reads them via `damson.MigrationsFS` (embedded at project root — see "The damson.go quirk" below), applies in lexicographic order inside a Postgres advisory lock, tracks applied files in `schema_migrations`.

**Tests:** in-memory SQLite via `newTestHandler(t)` + GORM `AutoMigrate` on the model structs. **The two paths are independent** — a schema change must land in BOTH an SQL migration AND the corresponding struct tags. Postgres-specific behaviour (tsvector, JSONB operators, `pg_advisory_lock`) is not exercised by SQLite; when that starts to bite, add a real-Postgres integration tier (see `docs/architecture.md` §17 decision 6).

### Handlers (per CONVENTIONS.md)

- One feature per file (`assets.go`, `library.go`, …); all handlers are receiver methods on the shared unexported `handler` struct.
- Construction: `api.New(api.Deps{...})` in `main.go`; tests call `newTestHandler(t)` which wires an in-memory SQLite + a `LocalFS` storage rooted in `t.TempDir()`.
- Errors: `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`. Generic `"internal error"` for 500 — don't leak internals.
- Log server-side at failure sites. `main.go` installs a custom `requestLogger()` (not `gin.Logger()`), so every response gets a status/path line. But 4xx handlers that carry diagnostic detail (extractor errors, validation beyond the binding layer) should `slog.Warn` the underlying error *before* `AbortWithStatusJSON` — the JSON body alone won't appear in logs.

### The damson.go quirk

`//go:embed` patterns can only reach **into** a directory, not **up** from the embedding file. Since `database/migrations/*.sql` and `web/static/*` are at project root but the Go packages that need them live under `internal/`, there is a single file at project root — `damson.go` — that holds both `//go:embed` directives and exports `damson.MigrationsFS` and `damson.WebStaticFS`. Consumers import `github.com/thomas-elliott/damson` and use those vars.

`web/static/.gitkeep` exists because `//go:embed all:web/static` requires the directory to match at least one file at compile time, even before the SPA has been built.

### SPA auth flow

1. App boots → `useAuthMode()` hits `/api/auth/mode` (public) and the result is cached in TanStack Query (single source of truth).
2. In **OIDC mode**: `AuthGuard` uses `oidc-client-ts` UserManager; missing/expired user → `signinRedirect` (browser leaves the SPA).
3. In **local mode**: `AuthGuard` reads the stored JWT from localStorage (`damson.local.token`); missing/expired → `<Navigate to="/login">`.
4. `lib/token.ts:getAccessToken()` provides the bearer to `api.ts:fetchJson` regardless of mode. In local mode it calls `maybeRefreshLocalSession` first — when the stored token is within five minutes of expiry it swaps in a fresh JWT via `POST /api/auth/refresh` (concurrent fetches share one in-flight refresh). OIDC silent-renew runs the equivalent path through `oidc-client-ts`.
5. On 401, `api.ts` tries silent renew (OIDC) or throws `AuthRedirectError` (local). The proactive local refresh keeps typical sessions from ever hitting the 401 path.
6. **Auth-mode cache is TanStack-only.** `lib/authMode.ts:getCachedAuthMode()` reads directly out of TanStack's query cache for the synchronous-access sites in `api.ts` and `lib/token.ts`; the previous module-level `cached` singleton (and `invalidateAuthMode()`) was removed. Post-bootstrap code seeds the new value via `queryClient.setQueryData` (synchronous) and fires a no-await invalidate — see PER-68 for the React 19 + RQ5 hang that drove the cleanup.

## Environment variables

Loaded in `internal/config/config.go`. A `.env` in the repo root is auto-loaded at startup (real env vars win on conflict). See `.env.example` for the full set. Secrets (`DAMSON_POSTGRES_DSN`, `DAMSON_JWT_SIGNING_KEY`, `DAMSON_LOCAL_ADMIN_PASSWORD`, `DAMSON_S3_ACCESS_KEY`, `DAMSON_S3_SECRET_KEY`) also accept a `<NAME>_FILE` variant that reads the value from that path — for Docker Secrets. Setting both the direct var and the `_FILE` variant panics at startup. The only always-required var is `DAMSON_POSTGRES_DSN` (or `DAMSON_POSTGRES_DSN_FILE`). `DAMSON_JWT_SIGNING_KEY` is optional in local mode — if unset, `internal/auth.LoadOrCreateJWTKey` generates 32 random bytes on first boot and persists them to `./data/jwt.key` (overridable via `DAMSON_JWT_KEY_FILE`). Defaults worth knowing: `DAMSON_LISTEN_ADDR=:8080`, `DAMSON_BASE_URL=http://localhost:8080`, `DAMSON_LOG_LEVEL=info`, `DAMSON_AUTH_MODE=local`, `DAMSON_STORAGE_MODE=local`, `DAMSON_STORAGE_LOCAL_PATH=./data/blobs`.

Mode-dependent:
- `DAMSON_AUTH_MODE=oidc` → `DAMSON_OIDC_ISSUER_URL`, `DAMSON_OIDC_CLIENT_ID`, `DAMSON_OIDC_AUDIENCE`.
- `DAMSON_AUTH_MODE=local` → `DAMSON_JWT_SIGNING_KEY` (≥32 bytes), `DAMSON_LOCAL_ADMIN_EMAIL`, `DAMSON_LOCAL_ADMIN_PASSWORD`.
- `DAMSON_STORAGE_MODE=s3` → `DAMSON_S3_ENDPOINT`, `DAMSON_S3_REGION`, `DAMSON_S3_BUCKET`, `DAMSON_S3_ACCESS_KEY`, `DAMSON_S3_SECRET_KEY`, `DAMSON_S3_FORCE_PATH_STYLE` (default true).
- `DAMSON_STORAGE_MODE=local` → `DAMSON_STORAGE_LOCAL_PATH` (default `./data/blobs`).

Missing required values panic at startup (deliberate — see CONVENTIONS.md "Return early on error").
