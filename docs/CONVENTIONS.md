# Conventions

Coding standards, conventions, and setup for go/react repositories.

This document complements:
- `AGENTS.md` — shorter quick-start rules for agentic coding
- `docs/architecture.md` — current architecture, data model, API surface, shipped decisions
- `docs/design-system.md` — frontend design system spec
- `docs/progress.md` — what's actually in the tree, slice by slice
- `docs/specs/` — signed-off specs driving post-MVP work
- `docs/archive/` — original design docs and point-in-time research, kept for historical reference

---

## Stack

**Backend**
- Go 1.26 with the Gin web framework
- GORM (PostgreSQL in both production and tests; each test gets its own schema)
- Pluggable auth: OIDC bearer tokens (`coreos/go-oidc` + `oauth2`) or local HS256 JWTs (`golang-jwt/jwt/v5`)
- Pluggable storage: local filesystem or S3-compatible (`internal/services/storage`)
- River (`riverqueue/river` on `pgx/v5`) for background jobs — imports and previews

**Frontend**
- React 19 + TypeScript 6 (strict mode). Two compilers are installed side by side: the `typescript` dependency is aliased to `@typescript/typescript6`, which is what `tsc -b` runs; `@typescript/native` (TS 7) sits alongside it. Check `web/package.json` before assuming which one a given script uses.
- Vite 8 build tool
- TanStack Query v5 for server state
- React Router 8
- Tailwind CSS with OKLCH design tokens
- Radix UI headless primitives (`@radix-ui/react-*`) + hand-rolled components under `web/src/design-system/`
- `class-variance-authority` for component variants; `clsx` + `tailwind-merge` via a `cn()` helper
- `lucide-react` icons; `@fontsource/inter` + `@fontsource/jetbrains-mono` self-hosted fonts
- CodeMirror 6 (`@codemirror/*`) for markdown editing and `@lezer/markdown` for the matching read-only renderer — see the `MarkdownEditor` / `Markdown` primitives

**Tooling**
- Task (`Taskfile.yml`) for all developer workflows
- `air` for Go hot-reload
- Vitest 4 with two projects: `unit` (jsdom) for component unit tests and `storybook` (Playwright Chromium) that runs every `*.stories.tsx` as a real-browser test with axe-core a11y checks
- Storybook 10 (`@storybook/react-vite`) for the component gallery, with `@storybook/addon-a11y`, `@storybook/addon-vitest`, and `storybook-addon-pseudo-states`
- Playwright for e2e against `cmd/e2e-server` (a Postgres-backed test binary mirroring production wiring)
- Multi-stage Dockerfile (Node → Go → Alpine)
- Woodpecker CI (`.woodpecker.yml`)

---

## Developer Commands

All workflows run through `task` (see `Taskfile.yml`) — don't invoke `go`, `pnpm` or
`docker compose` directly for build/test/lint. `task --list` discovers everything.

Names follow a canonical vocabulary — `build`, `test`, `lint`, `format`, `check`,
`clean`, `run`/`dev`, `install` — with `:` for subtasks and a per-language split
below each umbrella (`lint:go` / `lint:web`). Reach for the umbrella when in doubt.

| Task | Purpose |
|---|---|
| `task` | alias for `task --list` |
| `task install` | Go modules + pnpm + Playwright browsers (fresh clone) |
| `task dev` | Go API only, `air` hot-reload |
| `task dev:spa` | Backend + Vite SPA in parallel |
| `task dev:web` | Vite dev server only |
| `task build` | SPA + Go production binary |
| `task build:web` | Build SPA into `web/static` |
| `task check` | read-only preflight: lint + format:check + mod:tidy:check |
| `task lint` | `go vet` + eslint (`lint:go`, `lint:web`) |
| `task format` | `go fmt` + prettier --write (`format:go`, `format:web`) |
| `task format:check` | verify formatting without writing |
| `task mod:tidy:check` | `go mod tidy` + fail on `go.mod` / `go.sum` drift |
| `task test` | everything: unit + e2e |
| `task test:unit` | Go (Postgres-backed) + Vitest unit |
| `task test:unit:go` | Go tests only |
| `task test:unit:web` | Vitest unit project (jsdom, fast) |
| `task test:stories` | Vitest storybook project — every `*.stories.tsx` in real Chromium with axe a11y |
| `task test:e2e` | Playwright (boots Postgres + e2e-server + vite internally) |
| `task test:e2e:ui` | Playwright interactive runner |
| `task test:e2e:install` | Playwright browser binaries (once, after install) |
| `task test:watch` | Vitest watch mode |
| `task test:coverage` | Coverage report |
| `task storybook` | Storybook dev server (port 6006) |
| `task storybook:build` | Static Storybook build (smoke test) |
| `task ci` | full preflight: check + test:unit + test:e2e |
| `task clean` | remove build artefacts |
| `task docker:build` | Container image |
| `task release:{patch,minor,major}` | Tag a release |

Go tests need Postgres. `task test:unit:go` defaults to the shared local instance on
5432 via the `damson_test` database; to run one test directly, set the DSN yourself:

```
DAMSON_TEST_POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/damson_test?sslmode=disable \
  go test ./internal/handlers/api -run '^TestName$'
go test ./internal/handlers/api -run '^TestName$/SubtestName$'
```

> `Taskfile.yml` runs under mvdan/sh, Task's embedded interpreter — not bash.
> `trap INT TERM`, process substitution and `set -o pipefail` fail silently or
> confusingly; wrap them in `bash -c '...'`.

---

## Go Backend Conventions

### Formatting & linting
- Always run `task format` (or `task format:go`) before committing. No hand-formatting.
- `task lint` must pass — `go vet` on the Go side. No additional Go linter is configured.
- `task check` is the read-only gate: lint + format check + `go mod tidy` drift. CI fails on `go.mod` / `go.sum` drift, so run it before pushing.
- Standard gofmt import grouping: stdlib, third-party, local.
- Package names: lowercase, short, descriptive. File names snake_case when needed.

### Handler structure
API handlers are receiver methods on a shared exported `Handler` struct that carries its dependencies (`*gorm.DB`, `config.AuthMode`, `*auth.LocalAuthenticator`, `storage.Service`, the metadata registry, the job enqueuers, the SSE broadcasters). Construct it with `api.New(api.Deps{...})`. Routes are registered in `internal/handlers/api/handler.go`. One file per feature (`assets.go`, `library.go`, `me.go`, etc.); when a feature grows past a comfortable file, split by concern rather than by layer — `assets.go` / `assets_crud.go` / `assets_bulk.go` / `assets_rescan.go`.

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

- `newTestHandler(t)` — returns `(*Handler, *gorm.DB)` backed by a per-test Postgres schema (OIDC mode) with the real SQL migrations applied (via `internal/testdb`) and a `LocalFS` storage rooted in `t.TempDir()`. Requires `DAMSON_TEST_POSTGRES_DSN` (see AGENTS.md "Migrations vs tests")
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
- Paginated lists: `limit`/`offset` query params on the server. No `useInfiniteQuery` hook yet — the grid virtualises a single page. Add one when a list actually needs to page.
- Server-sent events (imports, previews) are subscriptions, not queries: the DB row is the source of truth and the stream is a nudge to refetch, so a dropped event must always be recoverable.

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
  - **`unit`** — `jsdom`, globals enabled, setup file `src/test/setupTests.ts`, pattern `src/**/*.{test,spec}.{ts,tsx}`. Run with `task test:unit:web`.
  - **`storybook`** — real Chromium via Playwright and `@storybook/addon-vitest/vitest-plugin`; every `*.stories.tsx` runs as a test with axe-core a11y checks (`a11y.test = "error"`). Run with `task test:stories`.
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
- River's `river_*` tables are the exception: River owns that schema and migrates it from `queue.New`. Don't hand-write migrations for them.
- Schema changes must land in BOTH the SQL migration AND the corresponding GORM struct tag — production and tests both run the SQL migrations, but GORM still maps columns from the struct tags, so a migration/tag mismatch surfaces at runtime (and now in tests). See AGENTS.md "Migrations vs tests".

## Docker & Release

**Dockerfile** (three-stage, no CGo):
1. `node:24-alpine` — `pnpm install --frozen-lockfile` + `vite build` → `web/static/`
2. `golang:1.26-alpine` — `CGO_ENABLED=0 go build` with `-ldflags="-s -w"`, embedding the built SPA via `go:embed`
3. `alpine:3.20` runtime — `ca-certificates` + `tzdata`, non-root `damson` user, `HEALTHCHECK` hitting `/api/health`

**CI/CD is Woodpecker** (`.woodpecker.yml`), not GitHub Actions. A push to `main` runs the full suite then builds and pushes `ghcr.io/thomas-elliott/damson:main-<sha>` and bumps the test environment's kustomize tag. Pushing a `v*.*.*` tag does **not** rebuild — it promotes that already-tested `main-<sha>` image to the `{version}`, `{major}.{minor}` and `{major}` tags with `crane`, then bumps prod. So a tag is only releasable if its commit was previously built on `main`.

`task release:{patch,minor,major}` cuts the tag from an up-to-date `main`; they don't re-run checks locally. Run `task ci` yourself first. Note that the Playwright step's image tag must match `@playwright/test` in `web/package.json` exactly — bumping one means bumping the other.

Don't commit `web/static/` build output — it's gitignored except for the `.gitkeep` placeholder that lets `//go:embed` succeed when the SPA hasn't been built yet.

## When in Doubt

- Mirror existing patterns
- Keep changes minimal and focused. Don't refactor unrelated code.
- Read `docs/` for deeper context before making architectural changes.
