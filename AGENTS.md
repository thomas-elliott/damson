# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project documents

Before making non-trivial changes, read the design docs in `docs/`. They are the source of truth for scope and shape — the code implements them.

- `docs/product-design-doc.md` — product scope, MVP, domain model
- `docs/ui-design.md` — frontend design brief
- `docs/technical-design.md` — architecture, schema, API surface, confirmed decisions (§22)
- `docs/CONVENTIONS.md` — stack, formatting rules, handler patterns, test helpers

## Development commands

All workflows go through Task (`Taskfile.yml`). Common:

```
task dev              # Go server with air hot-reload (API only)
task dev:spa          # Backend + Vite SPA in parallel
task build            # Build SPA then CGO_ENABLED=0 go build
task fmt / vet        # gofmt / go vet
task test             # go test ./...
task web:test         # Vitest run
task pre-release-checks  # fmt + vet + Go tests + web tests
```

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

- **Error responses.** `400` includes `err.Error()`; `404` / `409` used where they fit; `500` always returns the generic `"internal error"` string — don't leak internals. Shape: `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`.
- **Migration shape.** `NNN_descriptive_name.sql` (zero-padded 3-digit sequence), wrapped in `BEGIN; … COMMIT;`, `IF NOT EXISTS` throughout. Timestamps are `timestamptz NOT NULL DEFAULT now()`. Any schema change must land in BOTH a new SQL migration AND the model struct tags (see "Migrations vs tests" below for why).
- **Frontend API discipline.** Never call `fetch` directly from components — go through `fetchJson` / `fetchMaybeJson` in `src/api.ts` so bearer-token attachment, 401 renewal, and `ApiError` shape stay uniform. Query keys live in `lib/queryKeys.ts` only. Data-fetching hooks wrap TanStack Query in `hooks/useDataQueries.ts` and return the normalized `{ data, loading, error, reload }` shape.
- **Never log tokens or raw session values.** Applies to `slog` debug output, error wraps, anything that might end up in a log collector.
- **Mirror existing patterns; don't refactor unrelated code.** Focused changes only. Read `docs/` before architectural changes.

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

- **OIDC** (`oidc.go`) — validates IdP bearer tokens, upserts the user from claims. `BootstrapFirstAdmin` runs on the first user whose email matches `DAMSON_BOOTSTRAP_ADMIN_EMAILS`.
- **Local** (`local.go`) — bcrypt password hashes in `users.password_hash`, issues HS256 JWTs (1h TTL, `jwtIssuer = "damson-local"`). `users.tokens_valid_after` acts as a revocation watermark: JWTs with `iat` before that instant are rejected. Synthetic subject `"local:<email>"` so the `users.subject` unique index still works.

Both modes share one bootstrap path (`BootstrapFirstAdmin`): first user + "Personal" library + admin membership + `current_library_id`, all in one transaction.

### Pluggable storage

`internal/services/storage.Service` has two impls, picked by `DAMSON_STORAGE_MODE`:

- `LocalFS` — content-addressed under `<root>/blobs/<hh>/<hash>` and `<root>/thumbs/<hh>/<hash>`. Atomic rename, SHA-256 dedup.
- `S3` — aws-sdk-go-v2 with endpoint override + path-style for MinIO. HEAD-before-PUT dedup.

Layout matches between backends so a deployment can migrate with a plain copy.

### Request lifecycle

1. `auth.Middleware(authenticator)` — verifies bearer token, upserts-or-loads the user, attaches to Gin context via `auth.GetUser(c)`.
2. For library-scoped routes, add `h.requireLibrary()` middleware — resolves `current_library_id`, verifies membership, attaches `(*Library, *LibraryMembership)` via `getLibrary(c)` / `getMembership(c)`. 403 on any gap (never 404, so membership existence isn't leaked).
3. Handler runs its DB work scoped to `library_id`.

Route wiring in `internal/handlers/api/handler.go`: public routes, then an `authed` group with the auth middleware, then a `scoped` sub-group that layers `requireLibrary()`.

### Migrations vs tests

**Production:** SQL-only migrations in `database/migrations/NNN_*.sql`. Runner in `internal/db/migrate.go` reads them via `damson.MigrationsFS` (embedded at project root — see "The damson.go quirk" below), applies in lexicographic order inside a Postgres advisory lock, tracks applied files in `schema_migrations`.

**Tests:** in-memory SQLite via `newTestHandler(t)` + GORM `AutoMigrate` on the model structs. **The two paths are independent** — a schema change must land in BOTH an SQL migration AND the corresponding struct tags. Postgres-specific behaviour (tsvector, JSONB operators, `pg_advisory_lock`) is not exercised by SQLite; when that starts to bite, add a real-Postgres integration tier (see `docs/technical-design.md` §22 decision 5).

### Handlers (per CONVENTIONS.md)

- One feature per file (`assets.go`, `library.go`, …); all handlers are receiver methods on the shared unexported `handler` struct.
- Construction: `api.New(api.Deps{...})` in `main.go`; tests call `newTestHandler(t)` which wires an in-memory SQLite + a `LocalFS` storage rooted in `t.TempDir()`.
- Errors: `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`. Generic `"internal error"` for 500 — don't leak internals.

### The damson.go quirk

`//go:embed` patterns can only reach **into** a directory, not **up** from the embedding file. Since `database/migrations/*.sql` and `web/static/*` are at project root but the Go packages that need them live under `internal/`, there is a single file at project root — `damson.go` — that holds both `//go:embed` directives and exports `damson.MigrationsFS` and `damson.WebStaticFS`. Consumers import `github.com/thomas-elliott/damson` and use those vars.

`web/static/.gitkeep` exists because `//go:embed all:web/static` requires the directory to match at least one file at compile time, even before the SPA has been built.

### SPA auth flow

1. App boots → `useAuthMode()` hits `/api/auth/mode` (public) and caches the result.
2. In **OIDC mode**: `AuthGuard` uses `oidc-client-ts` UserManager; missing/expired user → `signinRedirect` (browser leaves the SPA).
3. In **local mode**: `AuthGuard` reads the stored JWT from localStorage (`damson.local.token`); missing/expired → `<Navigate to="/login">`.
4. `lib/token.ts:getAccessToken()` provides the bearer to `api.ts:fetchJson` regardless of mode.
5. On 401, `api.ts` tries silent renew (OIDC only) or throws `AuthRedirectError` so the app can bounce to login. Local mode has no refresh endpoint yet.

## Environment variables

Loaded in `internal/config/config.go`. Required regardless of mode:
`DAMSON_BASE_URL`, `DAMSON_POSTGRES_DSN`.

Mode-dependent:
- `DAMSON_AUTH_MODE=oidc` → `DAMSON_OIDC_ISSUER_URL`, `DAMSON_OIDC_CLIENT_ID`, `DAMSON_OIDC_AUDIENCE`, optional `DAMSON_BOOTSTRAP_ADMIN_EMAILS` (CSV).
- `DAMSON_AUTH_MODE=local` → `DAMSON_JWT_SIGNING_KEY` (≥32 bytes), `DAMSON_LOCAL_ADMIN_EMAIL`, `DAMSON_LOCAL_ADMIN_PASSWORD`.
- `DAMSON_STORAGE_MODE=s3` → `DAMSON_S3_ENDPOINT`, `DAMSON_S3_REGION`, `DAMSON_S3_BUCKET`, `DAMSON_S3_ACCESS_KEY`, `DAMSON_S3_SECRET_KEY`, `DAMSON_S3_FORCE_PATH_STYLE` (default true).
- `DAMSON_STORAGE_MODE=local` → `DAMSON_STORAGE_LOCAL_PATH` (default `./data/blobs`).

Missing required values panic at startup (deliberate — see CONVENTIONS.md "Return early on error").
