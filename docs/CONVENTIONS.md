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
- Tailwind CSS with OKLCH design tokens
- Radix UI headless primitives (`@radix-ui/react-*`) + hand-rolled components under `web/src/design-system/`
- `class-variance-authority` for component variants; `clsx` + `tailwind-merge` via a `cn()` helper
- `lucide-react` icons; `@fontsource/inter` + `@fontsource/jetbrains-mono` self-hosted fonts

**Tooling**
- Task (`Taskfile.yml`) for all developer workflows
- `air` for Go hot-reload
- Vitest 3 with two projects: `unit` (jsdom) for component unit tests and `storybook` (Playwright Chromium) that runs every `*.stories.tsx` as a real-browser test with axe-core a11y checks
- Storybook 10 (`@storybook/react-vite`) for the component gallery, with `@storybook/addon-a11y`, `@storybook/addon-vitest`, and `storybook-addon-pseudo-states`
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
| `task web:test` | Vitest unit project (jsdom, fast) |
| `task web:test:stories` | Vitest storybook project — every `*.stories.tsx` in real Chromium with axe a11y |
| `task web:test:all` | Both Vitest projects |
| `task web:test:watch` | Vitest watch mode (unit project) |
| `task web:storybook` | Storybook dev server (port 6006) |
| `task web:storybook:build` | Static Storybook build (smoke test) |
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
- Feature-level compositions live under `components/` and consume the design system. See `docs/design-system.md` for the canonical component inventory.
- All authenticated routes are wrapped in `<AuthGuard>` in `App.tsx`.
- `TooltipProvider` is mounted once in `main.tsx` — tooltips anywhere in the tree (nav rail, icon buttons) rely on it.

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
- OKLCH design tokens live in `src/design-system/tokens/tokens.css`. Dark is the default (`:root`); light opts in via `html[data-theme="light"]`.
- Tailwind's theme extends the tokens as utility classes (`bg-bg`, `bg-panel`, `text-text-2`, `border-line`, `bg-accent-tint`, `text-ok`, etc.) — prefer these over raw `var(--ds-*)`.
- `@tailwindcss/typography` for markdown content.
- Use the design-system primitives (`Button`, `Input`, `Dialog`, `Pill`, `Tabs`, …) from `@/design-system` rather than hand-rolling equivalents. New components go in `src/design-system/{primitives,compositions,domain}/` alongside a `*.stories.tsx` with variants + states; see `docs/design-system.md`.
- `cn()` composes classes: `clsx` for conditional joins, `tailwind-merge` for dedup. The custom font-size keys (`2xs`, `xs-plus`, `sm-plus`, `base`, `md`) are registered with `tailwind-merge` in `utils/cn.ts` — new size utilities belong in the same group.
- Do not reintroduce CDN Tailwind; everything goes through the Vite pipeline.

### Frontend tests (Vitest)
- Config: `vitest.config.ts` defines two projects:
  - **`unit`** — `jsdom`, globals enabled, setup file `src/test/setupTests.ts`, pattern `src/**/*.{test,spec}.{ts,tsx}`. Run with `task web:test`.
  - **`storybook`** — real Chromium via Playwright and `@storybook/addon-vitest/vitest-plugin`; every `*.stories.tsx` runs as a test with axe-core a11y checks (`a11y.test = "error"`). Run with `task web:test:stories`. `task web:test:all` runs both.
- Libraries: `@testing-library/react`, `@testing-library/user-event` for the unit project; `storybook/test` helpers for story interactions.
- Browsers are installed via `npx playwright install chromium`; the cache lives at `~/Library/Caches/ms-playwright/` on macOS.

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
