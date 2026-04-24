# Conventions

Coding standards, conventions, and setup for go/react repositories.

This document complements:
- `AGENTS.md` — shorter quick-start rules for agentic coding
- `docs/product-design-doc.md`, `docs/ui-design.md`, `docs/technical-design.md` — product scope, UI brief, architecture
- `docs/progress.md` — milestone-by-milestone status of what's actually in the tree

---

## Stack

**Backend**
- Go 1.26 with the Gin web framework
- GORM (PostgreSQL in production, in-memory SQLite for tests)
- OIDC bearer-token auth (`coreos/go-oidc` + `oauth2`)
- S3-compatible media storage (`internal/services`)

**Frontend**
- React + TypeScript (strict mode)
- Vite build tool
- TanStack Query v5 for server state
- React Router
- Tailwind CSS + shadcn/ui primitives

**Tooling**
- Task (`Taskfile.yml`) for all developer workflows
- `air` for Go hot-reload
- Vitest (+ Testing Library) for frontend tests
- Multi-stage Dockerfile (Node → Go → Alpine)

---

## Developer Commands

All workflows run through `task` (see `Taskfile.yml`).

| Task | Purpose |
|---|---|
| `task dev` | Go server with `air` hot-reload |
| `task dev:spa` | Backend + Vite SPA in parallel |
| `task web:dev` | Vite dev server only |
| `task build` | Production binary (SPA + Go) |
| `task web:build` | Build SPA into `web/static` |
| `task fmt` | `go fmt ./...` |
| `task vet` | `go vet ./...` |
| `task test` | `go test ./...` |
| `task test:short` | Non-verbose Go tests |
| `task test:coverage` | Generate `coverage.html` |
| `task web:test` | Vitest run (frontend) |
| `task web:test:watch` | Vitest watch mode |
| `task pre-release-checks` | fmt + vet + tests (Go & web) |
| `task docker-build` | Container image |

Running single Go tests:
```
go test ./internal/handlers/api -run '^TestName$'
go test ./internal/handlers/api -run '^TestName$/SubtestName$'
```

---

## Go Backend Conventions

### Formatting & linting
- Always run `task fmt` before committing. No hand-formatting.
- `task vet` must pass. No additional linter is configured.
- Standard gofmt import grouping: stdlib, third-party, local.
- Package names: lowercase, short, descriptive. File names snake_case when needed.

### Handler structure
API handlers are receiver methods on a shared unexported `handler` struct that carries its dependencies (`*gorm.DB`, `config.AuthMode`, `*auth.LocalAuthenticator`, `storage.Service`). Routes are registered in `internal/handlers/api/handler.go` via `AddRoutes(...)`. One file per feature (`assets.go`, `library.go`, `me.go`, etc.).

Access the current user with `auth.GetUser(c)`.

### Error responses
Use `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`.

- `400` — bad request (e.g. validation failure) — include `err.Error()`
- `401` — unauthorized (middleware)
- `404` — not found
- `409` — conflict
- `500` — internal error — use the generic string `"internal error"`, don't leak internals

Return early on error. Log with `log/slog` (see `cmd/server/main.go`'s JSON handler) including operation context as structured fields. Don't panic for request-level failures; reserve panics for startup config (`MustGetEnv`).

### Database patterns
- GORM models live in `internal/models/`. Each model implements `TableName() string` so the production table name is explicit and survives a struct rename (e.g. `Asset` → `assets`, `LibraryMembership` → `library_memberships`).
- IDs are `uint` with `gorm:"primarykey;auto_increment;not_null"`.
- Timestamps are `time.Time` fields populated by GORM.
- Response shapes that differ from DB rows (enriched views, joins) use dedicated view models — see `assetGridRow` / `meResponse` / `libraryMembershipView` and similar.
- Prefer the GORM query builder (`Find`, `Where`, `Preload`, `Joins`, `Scan`); use `Table` + `Select` for joins. Raw SQL only when necessary.
- Complex queries use **lateral joins + batch loading**: fetch IDs first, then load relations in bulk to avoid N+1.

### Tests
Tests live alongside handlers (`assets_test.go` next to `assets.go`). Shared helpers are in `internal/handlers/api/api_test.go`:

- `newTestHandler(t)` — returns `(*handler, *gorm.DB)` backed by in-memory SQLite (OIDC mode) with `AutoMigrate` applied to all models and a `LocalFS` storage rooted in `t.TempDir()`
- `newTestHandlerLocal(t)` — same but in local-auth mode, with a `*auth.LocalAuthenticator` returned alongside
- `setupRouter(h, asUser)` — registers routes under `/api`; bypasses real auth by injecting `asUser` via `auth.SetUserForTest`
- `performGET(r, path)`, `performRequest(r, method, path, body)` — issue requests
- `requireStatus(t, w, code)`, `jsonResponse[T]` — assertions
- `seedUser`, `seedLocalUser`, `seedLibrary`, `seedMembership`, `seedLibraryAndSwitch` — seed data

Test mode is set once in `init()` with `gin.SetMode(gin.TestMode)`.

## Frontend Conventions

### TypeScript / React
- Strict mode on (`tsconfig.json`). `@` path alias maps to `src/`.
- Functional components only. Explicit props interfaces.
- Pages in `pages/` use React Router hooks (`useParams`, `useNavigate`, `useSearchParams`).
- Components grouped by feature under `components/`. Generic shadcn primitives go in `components/ui/`.
- All authenticated routes are wrapped in `<AuthGuard>` in `App.tsx`.

### Server state (TanStack Query)
- Query keys live in `lib/queryKeys.ts` — a factory-style object. Never hand-roll query keys in components.
  ```ts
  export const queryKeys = {
    authMode: ["authMode"] as const,
    me: ["me"] as const,
    assets: (filters?: Record<string, string | number>) =>
      filters ? (["assets", filters] as const) : (["assets"] as const),
  } as const;
  ```
- Custom hooks live in `hooks/useDataQueries.ts` and wrap `useQuery`, returning a normalized `{ data, loading, error, reload }` shape.
- Cache policy uses constants from `lib/cachePolicy` (`SECOND`, `MINUTE`, `HOUR`, `DAY`). Set `staleTime` and `gcTime` explicitly per endpoint.
- Client config in `lib/queryClient.ts`: 5 min default stale, no refetch on focus.
- Paginated lists: `limit`/`offset` query params on the server; a `useInfiniteQuery` hook lands when the list surfaces it (M5).

### API layer (`src/api.ts`)
- `fetchJson<T>(url, options)` is the main wrapper: attaches bearer token, handles 401 via silent token renewal with single-flight deduping, retries once, parses JSON, and throws `ApiError` on non-2xx.
- `uploadFile(url, file, onProgress?)` handles multipart uploads with XHR progress events (fetch lacks upload progress).
- Error types: `ApiError` (with status), `AuthRedirectError` (401 after renewal fails — triggers redirect to login). Narrow with `isAuthRedirectError(err)`.
- Don't call `fetch` directly from components; always go through these helpers so auth/renewal behavior is consistent. (Two exceptions in `lib/authMode.ts` and `lib/localAuth.ts` — they're public, pre-auth endpoints that can't depend on the wrapper.)

### Auth / OIDC
- Env vars (Vite): `VITE_OIDC_AUTHORITY`, `VITE_OIDC_CLIENT_ID`, `VITE_OIDC_SCOPE`, `VITE_OIDC_REDIRECT_URI`, `VITE_OIDC_SILENT_REDIRECT_URI`, `VITE_OIDC_POST_LOGOUT_REDIRECT_URI`.
- `automaticSilentRenew: true` — token refresh happens transparently in the background.
- Never log tokens or raw session values.

### Styling
- Tailwind CSS with CSS-variable theming; dark mode via a `class` on the root.
- `@tailwindcss/typography` for markdown content.
- Reuse shadcn primitives from `components/ui/` (button, card, dialog, input, badge, etc.) rather than building new ones.
- Do not reintroduce CDN Tailwind; everything goes through the Vite pipeline.

### Frontend tests (Vitest)
- Config: `vitest.config.ts`, environment `jsdom`, globals enabled.
- Setup file: `src/test/setupTests.ts` (imports `@testing-library/jest-dom/vitest`).
- Libraries: `@testing-library/react`, `@testing-library/user-event`.
- Pattern: `src/**/*.{test,spec}.{ts,tsx}`.

---

## Database Migrations

- Location: `database/migrations/`.
- Naming: `NNN_descriptive_name.sql` (zero-padded 3-digit sequence). Example: `002_tags.sql`.
- Each migration is wrapped in `BEGIN; ... COMMIT;`.
- Use `IF NOT EXISTS` for idempotency.
- Primary keys: `bigserial PRIMARY KEY`. Timestamps: `timestamptz NOT NULL DEFAULT now()`.
- Migrations are SQL-only — no Go migration framework. See `internal/db/migrate.go` for the runner (advisory lock + `schema_migrations` tracking).
- Schema changes must land in BOTH the SQL migration AND the corresponding GORM struct tag — tests use `AutoMigrate` against the models, production uses the SQL. See AGENTS.md "Migrations vs tests".

## Docker & Release

**Dockerfile** (three-stage, no CGo):
1. `node:24-alpine` — `npm ci` + `vite build` → `web/static/`
2. `golang:1.26-alpine` — `CGO_ENABLED=0 go build` with `-ldflags="-s -w"`, embedding the built SPA via `go:embed`
3. `alpine:3.20` runtime — `ca-certificates` + `tzdata`, non-root `damson` user, `HEALTHCHECK` hitting `/api/health`

**Releases** are automated via GitHub Actions (`.github/workflows/release.yml`): pushing a tag matching `v*.*.*` builds and pushes the image to `ghcr.io/<owner>/damson` with `{version}`, `{major}.{minor}`, and `{major}` tags. `main-build.yml` builds `main-<sha>` on every push to `main`. Locally, `task pre-release-checks` runs fmt + vet + Go tests + web tests before tagging.

Don't commit `web/static/` build output — it's gitignored except for the `.gitkeep` placeholder that lets `//go:embed` succeed when the SPA hasn't been built yet.

## When in Doubt

- Mirror existing patterns
- Keep changes minimal and focused. Don't refactor unrelated code.
- Read `docs/` for deeper context before making architectural changes.
