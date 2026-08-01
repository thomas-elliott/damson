# AGENTS.md

`CLAUDE.md` is a symlink to this file — edit `AGENTS.md`.

## Project documents

Before making non-trivial changes, read the docs in `docs/`. They describe the code as it stands today.

- `docs/architecture.md` — current architecture, data model, API surface, shipped decisions
- `docs/CONVENTIONS.md` — stack, formatting rules, handler patterns, test helpers
- `docs/design-system.md` — frontend design system spec
- `docs/progress.md` — what's actually in the tree: the MVP ladder in summary, then post-MVP work slice by slice. Read this before starting a new slice so you know what foundations do (and don't) already exist
- `docs/specs/` — signed-off specs, `YYYY-MM-DD-slug.md`. Post-MVP work is spec-driven: each file is the agreed scope for one slice, written before implementation and left in place afterwards as the record of *why* it was built that way
- `docs/observability.md` — logging shape, request correlation, LogsQL queries
- `docs/e2e-flows.md` — Playwright coverage: what's tested, what's deliberately not
- `docs/archive/` — historical inputs, kept verbatim: the original product / technical / UI design docs (Draft v1/v2) that drove M0–M7, plus point-in-time research artefacts (preview feasibility, dependency audit). Section numbers and API shapes have drifted — prefer `architecture.md` for the live shape and consult the archive only for the *why*

## Development commands

All workflows go through Task (`Taskfile.yml`) — do not invoke `go`, `npm`, `docker compose` directly for build/test/lint operations. Run `task --list` to discover targets. Naming follows a canonical vocabulary (`build`, `test`, `lint`, `format`, `check`, `clean`, `run`/`dev`, `install`) with `:` for subtasks, and subdomain tasks split by language (`lint:go` / `lint:web`, …) — use the umbrella when in doubt.

Run one Go test (Go tests need Postgres — set `DAMSON_TEST_POSTGRES_DSN`, or run under `task test:unit:go`, which defaults to the shared local Postgres on 5432 via the `damson_test` database):

```
DAMSON_TEST_POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/damson_test?sslmode=disable \
  go test ./internal/handlers/api -run '^TestName$'
go test ./internal/handlers/api -run '^TestName$/SubtestName$'
```

Build the CLI (`dmsn`) and install its Claude Code skill:

```
task build:cli
./dmsn setup
```

Vitest single-file:

```
cd web && pnpm run test -- src/path/to/file.test.tsx --run
```

Local dev sign-in (`task dev:spa`, local auth mode) uses a throwaway account — email `dev@example.com`, password `password1`. Not a real credential; it exists so the SPA can be driven end-to-end against the dev server.

## Conventions

The full rules live in `docs/CONVENTIONS.md`. Load-bearing ones worth keeping in mind while editing:

- **Error responses.** `400` includes `err.Error()`; `404` / `409` used where they fit; `500` always returns the generic `"internal error"` string — don't leak internals. Shape: `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`. Inside a `Transaction(func)` use the sentinel constructors (`errBadRequestf`, `errConflictf`) and let the outer handler call `respondGORM(c, err, "<entity> not found")` once — single source of truth for the status mapping.
- **Migration shape.** `NNN_descriptive_name.sql` (zero-padded 3-digit sequence), wrapped in `BEGIN; … COMMIT;`, `IF NOT EXISTS` throughout. Timestamps are `timestamptz NOT NULL DEFAULT now()`. Any schema change must land in BOTH a new SQL migration AND the model struct tags (see "Migrations vs tests" below for why).
- **Frontend API discipline.** Never call `fetch` directly from components — go through `fetchJson` / `fetchMaybeJson` in `src/api.ts` so bearer-token attachment, 401 renewal, and `ApiError` shape stay uniform. Query keys live in `lib/queryKeys.ts` only. Data-fetching hooks wrap TanStack Query in `hooks/useDataQueries.ts` and return the normalized `{ data, loading, error, reload }` shape.
- **Never log tokens or raw session values.** Applies to `slog` debug output, error wraps, anything that might end up in a log collector.
- **Mirror existing patterns; don't refactor unrelated code.** Focused changes only. Read `docs/` before architectural changes.
- **Taskfile shell.** `Taskfile.yml` commands run under mvdan/sh (Task's embedded interpreter), not bash. Bash-isms like `trap INT TERM`, process substitution, and `set -o pipefail` fail silently or with confusing errors — wrap in `bash -c '...'` when you need them.
- **`go mod tidy` is enforced in CI.** CI is Woodpecker (`.woodpecker.yml`, not GitHub Actions). It runs `go mod tidy` then `git diff --exit-code -- go.mod go.sum` and fails the build on any drift. Whenever you add an import — especially when a previously-indirect dep becomes a direct require because new code uses it — run `task mod:tidy:check` (or `go mod tidy` + diff) before committing. `task check` (and `task ci`) include this step; running either locally before pushing avoids CI-only failures.

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

- **Local** (default; `local.go`) — bcrypt password hashes in `users.password_hash`, issues HS256 JWTs (1h TTL). Users are matched by `lower(email)` + a non-null `password_hash`; local users have no `user_identities` row. `users.tokens_valid_after` acts as a revocation watermark: JWTs with `iat` before that instant are rejected.

  `users.subject` no longer exists — migration 008 dropped it and moved external identities to `user_identities`. Don't reintroduce a synthetic-subject pattern.
- **OIDC** (`oidc.go`) — validates IdP bearer tokens and resolves them through `user_identities` (`unique(issuer, subject)`), not through `users.subject`. A damson user can therefore carry several IdP identities. Resolution order: match an existing identity row → else match a **verified** email claim against `users.email` (case-insensitive) and link a new identity to that user → else create a user and its first identity. Email is the cross-IdP join key, so it is unique among non-blank values (partial unique index on `lower(email)`); unverified email claims are dropped. A user with no email is flagged `needs_profile` so the SPA collects one on first login, after which email/name are app-owned via `PATCH /api/me`.

Both modes share one bootstrap path (`BootstrapFirstAdmin`): the first user on an empty system (no other users, no libraries) gets sysadmin + "Personal" library + admin membership + `current_library_id`, all in one transaction. No env-var allowlist — identity is whoever signs in first.

Local-mode bootstrap has two entry points: the env-seeded startup path (`SeedLocalAdmin`, active when both `DAMSON_LOCAL_ADMIN_EMAIL` and `_PASSWORD` are set) and the HTTP path (`POST /api/auth/bootstrap`, reachable when no users exist). The latter is what `GET /api/auth/mode`'s `needs_bootstrap` flag signals to the SPA.

### Pluggable storage

`internal/services/storage.Service` has two impls, picked by `DAMSON_STORAGE_MODE`:

- `LocalFS` (default) — content-addressed under `<root>/blobs/<hh>/<hash>` and `<root>/thumbs/<hh>/<hash>`. Atomic rename, SHA-256 dedup.
- `S3` — aws-sdk-go-v2 with endpoint override + path-style for MinIO/Garage. Blob puts go through `manager.Uploader` (streaming multipart) so a multi-GB upload never lands in memory; only the small, bounded `PutThumb` path buffers. HEAD-before-PUT dedup.

Layout matches between backends so a deployment can migrate with a plain copy. Per-file preview derivatives share the `thumbs/` prefix.

Blobs are never deleted on the write path — deleting an asset leaves its objects behind. Reclaiming them is a deliberate, sysadmin-driven two-step in `internal/services/storage/cleanup`: `FindOrphans` lists objects referenced by no live row and older than `DAMSON_ORPHAN_MIN_AGE` (default `168h`), then `DeleteOrphans` re-verifies each key against a freshly-read reference set immediately before deleting and audits every deletion. Exposed as `POST /api/admin/storage/cleanup/{scan,delete}`.

### Background jobs (River)

Imports and previews run as [River](https://riverqueue.com) jobs (`internal/queue`), Postgres-backed — not goroutine pools. Two queues, `import` and `preview`, sized independently. River owns its own pgxpool, separate from GORM's, so LISTEN/NOTIFY pickup and queue connections stay isolated from web traffic. River's own migrations run from `queue.New`, not from `database/migrations/`.

- Enqueue writes a `river_job` row; job args carry ids only, and the pipeline reloads everything from the DB.
- `asset_imports.stage` (and `asset_files.preview_status`) remain the durable source of truth for user-visible state — the SSE broadcasters are notification channels only, so a dropped event is recoverable by refetching.
- Import stages: `uploaded → extracting → detecting → thumbnailing → enriching → done`, or `failed`. Imports hand off preview work by enqueuing onto the preview queue.
- On boot, `imports.ResumeInFlight` / `preview.ResumeInFlight` re-enqueue rows left mid-flight. River dedupes by args, so this is idempotent and safe every start.

### Request lifecycle

1. `auth.Middleware(authenticator)` — verifies bearer token, upserts-or-loads the user, attaches to Gin context via `auth.GetUser(c)`.
2. For library-scoped routes, add `h.requireLibrary()` middleware — resolves `current_library_id`, verifies membership, attaches `(*Library, *LibraryMembership)` via `getLibrary(c)` / `getMembership(c)`. 403 on any gap (never 404, so membership existence isn't leaked).
3. Handler runs its DB work scoped to `library_id`.

Route wiring in `internal/handlers/api/handler.go`: public routes, then an `authed` group with the auth middleware, then a `scoped` sub-group that layers `requireLibrary()`.

### Migrations vs tests

**Production:** SQL-only migrations in `database/migrations/NNN_*.sql`. Runner in `internal/db/migrate.go` reads them via `damson.MigrationsFS` (embedded at project root — see "The damson.go quirk" below), applies in lexicographic order inside a Postgres advisory lock, tracks applied files in `schema_migrations`.

**Tests:** the same SQL migrations, run against a real Postgres. `newTestHandler(t)` (and the other test-DB helpers) go through `internal/testdb`, which creates a throwaway per-test schema, applies the migrations, and drops it on cleanup. `DAMSON_TEST_POSTGRES_DSN` is **required** — with it unset the suite fails fast rather than silently skipping (there is no SQLite fallback). A schema change must still land in BOTH an SQL migration AND the corresponding GORM struct tags: the migration is now the test schema too, but GORM maps columns from the tags, so a migration/tag mismatch surfaces in tests. Postgres-specific behaviour (tsvector, JSONB operators, `pg_advisory_lock`) is now exercised directly.

### Handlers (per CONVENTIONS.md)

- One feature per file (`assets.go`, `library.go`, …); all handlers are receiver methods on the shared unexported `handler` struct.
- Construction: `api.New(api.Deps{...})` in `main.go`; tests call `newTestHandler(t)` which wires a per-test Postgres schema (via `internal/testdb`) + a `LocalFS` storage rooted in `t.TempDir()`.
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

Loaded in `internal/config/config.go`; see `.env.example` for the full set and the per-mode requirements. A `.env` in the repo root is auto-loaded at startup (real env vars win on conflict). Secrets (`DAMSON_POSTGRES_DSN`, `DAMSON_JWT_SIGNING_KEY`, `DAMSON_LOCAL_ADMIN_PASSWORD`, `DAMSON_S3_ACCESS_KEY`, `DAMSON_S3_SECRET_KEY`) also accept a `<NAME>_FILE` variant that reads the value from that path — for Docker Secrets. Setting both the direct var and the `_FILE` variant panics at startup. The only always-required var is `DAMSON_POSTGRES_DSN` (or `DAMSON_POSTGRES_DSN_FILE`). `DAMSON_JWT_SIGNING_KEY` is optional in local mode — if unset, `internal/auth.LoadOrCreateJWTKey` generates 32 random bytes on first boot and persists them to `./data/jwt.key` (overridable via `DAMSON_JWT_KEY_FILE`).

Missing required values panic at startup (deliberate — see CONVENTIONS.md "Return early on error").
