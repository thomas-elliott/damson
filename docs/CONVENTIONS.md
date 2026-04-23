# Conventions

Coding standards, conventions, and setup for go/react repositories.

This document complements:
- `AGENTS.md` — shorter quick-start rules for agentic coding
- `docs/` — deeper architecture notes (testing, ingest, whatsync, etc.)

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
| `task docker-build` / `docker-push` | Container image |
| `task release VERSION=vX.Y.Z` | Tag and push release |
| `task release-patch` / `release-minor` / `release-major` | Bumped releases |

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
API handlers are receiver methods on a shared `handler` struct that carries dependencies (`*gorm.DB`, `*services.S3Service`, `*whatsync.ObservabilityHub`, `*whatsync.RuntimeController`). Routes are registered in `internal/handlers/api/routes.go` via `AddRoutes(...)`. One file per feature (`people.go`, `messages.go`, `documents.go`, etc.).

Access the current user with `auth.GetUser(c)`.

### Error responses
Use `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`.

- `400` — bad request (e.g. validation failure) — include `err.Error()`
- `401` — unauthorized (middleware)
- `404` — not found
- `409` — conflict
- `500` — internal error — use the generic string `"internal error"`, don't leak internals

Return early on error. Log with `log.Printf` including operation context. Don't panic for request-level failures; reserve panics for startup config (`MustGetEnv`).

### Database patterns
- GORM models live in `internal/models/`. Each model with a non-default table name implements `TableName() string` (e.g. `Person` → `people`).
- IDs are `uint` with `gorm:"primarykey;auto_increment;not_null"`.
- Timestamps are `time.Time` fields populated by GORM.
- Response shapes that differ from DB rows (enriched views, joins) use dedicated view models — see `MessageView`, `MessageWithMedia`, `adminPerson` and similar.
- Prefer the GORM query builder (`Find`, `Where`, `Preload`, `Joins`, `Scan`); use `Table` + `Select` for joins. Raw SQL only when necessary.
- Complex queries use **lateral joins + batch loading**: fetch IDs first, then load relations in bulk to avoid N+1.

### Tests
Tests live alongside handlers (`people_test.go` next to `people.go`). Shared helpers are in `internal/handlers/api/api_test.go`:

- `newTestHandler(t)` — returns `(*handler, *gorm.DB)` backed by in-memory SQLite with `AutoMigrate` applied to all models
- `setupRouter(h)` — registers routes under `/api`
- `performGET(r, path)`, `performRequest(r, method, path, body)` — issue requests
- `requireStatus(t, w, code)`, `jsonResponse`, `jsonArrayResponse` — assertions
- `seedPerson`, `seedChat`, `seedMessage`, `seedIdentity`, `seedProvider` — seed data

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
    people: ["people"] as const,
    personProfile: (id: number) => ["person", id, "profile"] as const,
  };
  ```
- Custom hooks live in `hooks/useDataQueries.ts` and wrap `useQuery` / `useInfiniteQuery`, returning a normalized `{ data, loading, error, reload, ... }` shape.
- Cache policy uses constants from `lib/cachePolicy` (`MINUTE`, `HOUR`, `DAY`). Set `staleTime` and `gcTime` explicitly per endpoint.
- Client config in `lib/queryClient.ts`: 5 min default stale, no refetch on focus.
- Paginated lists use `useInfiniteQuery` + the `InfiniteScrollList` component with `limit`/`offset` URL params.

### API layer (`src/api.ts`)
- `fetchJson<T>(url, options)` is the main wrapper: attaches bearer token, handles 401 via silent token renewal with single-flight deduping, retries once, parses JSON, and throws `ApiError` on non-2xx.
- `fetchMaybeJson<T>` permits empty bodies.
- Error types: `ApiError` (with status), `AuthRedirectError` (401 after renewal fails — triggers redirect to login). Narrow with `isAuthRedirectError(err)`.
- Don't call `fetch` directly from components; always go through these helpers so auth/renewal behavior is consistent.

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
- Naming: `NNN_descriptive_name.sql` (zero-padded 3-digit sequence). Example: `007_add_message_revisions.sql`.
- Each migration is wrapped in `BEGIN; ... COMMIT;`.
- Use `IF NOT EXISTS` for idempotency.
- Primary keys: `generated always as identity`. Timestamps: `timestamptz default now()`.
- Migrations are SQL-only — no Go migration framework.

## Docker & Release

**Dockerfile** (multi-stage):
1. Node — build SPA
2. Go with `build-base` / `vips` — build binary

**Releases** use `task release VERSION=vX.Y.Z`:
- Runs `pre-release-checks` (fmt, vet, Go tests, web tests)
- Fails if the working tree is dirty or the tag exists
- Tags and pushes to origin

Don't commit `web/static/` build output unless explicitly asked.

## When in Doubt

- Mirror existing patterns
- Keep changes minimal and focused. Don't refactor unrelated code.
- Read `docs/` for deeper context before making architectural changes.
