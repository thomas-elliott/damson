# Conventions

Coding standards, conventions, and setup for go/react repositories.

---

## Developer Commands

All workflows run through `just` (see `justfile` and `tools/*.just`) — don't invoke
`go`, `pnpm` or `docker` directly for build/test/lint. Bare `just` lists everything.

Names follow a canonical vocabulary — `build`, `test`, `lint`, `format`, `check`,
`clean`, `dev`, `install` — with the backend/frontend split as the second word
(`lint backend` / `lint frontend`). Reach for the umbrella when in doubt.

**`just check` is the full preflight** and what CI gates on. `just check fast` skips the build
and the tests, so running only it before a push is what lets a red build through.

Go tests need Postgres. `just test backend` sets `DAMSON_TEST_POSTGRES_DSN` itself,
defaulting to the shared local instance on 5432 via the `damson_test` database, and
its arguments go straight to `go test`:

```
just test backend ./internal/handlers/api -run '^TestName$'
just test backend ./internal/handlers/api -run '^TestName$/SubtestName$'
```

Playwright browsers come from `just install`; there is no separate recipe for them.

---

## Go Backend Conventions

### Formatting & linting
- Always run `just format` (or `just format backend`) before committing. No hand-formatting.
- `just lint` must pass — on the Go side, `go vet` and golangci-lint with `errorlint`, `contextcheck` and `unused` (`.golangci.yml`). golangci-lint is pinned as a Go tool in `go.mod`, so `just lint backend` needs no separate install.
- `just check fast` is the read-only gate: lint + format check + `go mod tidy` drift. CI fails on `go.mod` / `go.sum` drift, so run it before pushing.
- Standard gofmt import grouping: stdlib, third-party, local.
- Package names: lowercase, short, descriptive. File names snake_case when needed.

### Handler structure
API handlers are receiver methods on a shared exported `Handler` struct that carries its dependencies (`*gorm.DB`, `config.AuthMode`, `*auth.LocalAuthenticator`, `storage.Service`, the metadata registry, the job enqueuers, the SSE broadcasters). Construct it with `api.New(api.Deps{...})`. Routes are registered in `internal/handlers/api/handler.go`. One file per feature (`assets.go`, `library.go`, `me.go`, etc.); when a feature grows past a comfortable file, split by concern rather than by layer — `assets.go` / `assets_crud.go` / `assets_bulk.go` / `assets_rescan.go`.

Access the current user with `auth.GetUser(c)`.

### Error responses
Respond through the helpers in `helpers.go` (`respond400/400Msg/404/409/500/502/503`, `respondForbidden`; `respondGORM` for transaction errors). The other cross-handler helpers live there too: `parseIDParam`, `parsePagination`, `requireAdmin`, `requireSysadmin`, `isUniqueViolation`. Never call `AbortWithStatusJSON` for a 4xx/5xx with a cause: only `respond400`/`respond500` attach it via `c.Error`, which is what puts `err` on the request log line.

- `400` — bad request (e.g. validation failure) — include `err.Error()`
- `401` — unauthenticated
- `403` — forbidden (`respondForbidden`)
- `404` — not found
- `409` — conflict
- `500` — internal error — use the generic string `"internal error"`, don't leak internals

Rarer statuses have their own homes: upload transport failures (413, 408, 507, …) go through
`respondUploadFailure` in `upload_errors.go`; store-metadata errors through `respondLookupFailed` in
`metadata.go`.

Return early on error. Log with `log/slog` (see `cmd/server/main.go`'s JSON handler) including operation context as structured fields. Don't panic. A startup failure is an error returned from `run()`; `main` logs it with `slog.Error` and exits 1.

### Database patterns
- GORM models live in `internal/models/`. Each model implements `TableName() string` so the production table name is explicit and survives a struct rename (e.g. `Asset` → `assets`, `LibraryMembership` → `library_memberships`).
- IDs are `uint` with `gorm:"primarykey;auto_increment;not_null"`. Singleton settings rows are the exception (`ServerSettings` is `primarykey;default:1`).
- Timestamps are `time.Time` fields populated by GORM.
- Response shapes that differ from DB rows (enriched views, joins) use dedicated view models — see `assetGridRow` / `meResponse` / `libraryMembershipView` and similar.
- Prefer the GORM query builder (`Find`, `Where`, `Preload`, `Joins`, `Scan`); use `Table` + `Select` for joins. Raw SQL only when necessary.
- Complex queries use **batch loading**: fetch IDs first, then load relations in bulk to avoid N+1.

### Tests
Tests live alongside handlers (`assets_test.go` next to `assets.go`). Shared helpers live in the package's `*_test.go` files — the core ones in `api_test.go`, seed helpers beside the feature they seed:

- `newTestHandler(t)` — returns `(*Handler, *gorm.DB)` backed by a per-test Postgres schema (OIDC mode) with the real SQL migrations applied (via `internal/testdb`) and a `LocalFS` storage rooted in `t.TempDir()`. Requires `DAMSON_TEST_POSTGRES_DSN` (see `docs/architecture.md` §4.6)
- Import and preview pools are synchronous (`ingesttest.NewSync`, `previewtest.NewSync`), so a 202 import is already terminal when the handler returns; River's async behaviour is only exercised by e2e.
- `newTestHandlerLocal(t)` — same but in local-auth mode, with a `*auth.LocalAuthenticator` returned alongside
- `setupRouter(h, asUser)` — registers routes under `/api` behind the real `auth.Middleware`, with an authenticator that signs every request in as `asUser` (401 when nil)
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
- Custom hooks live in `hooks/useDataQueries.ts` and wrap `useQuery`, returning a normalized `{ data, loading, error, reload }` shape. A hook behind a screen also returns `failed`: an error with no data to show, which the screen renders as `ErrorState`. A failed refetch over data already on screen leaves `failed` false.
- Cache policy uses constants from `lib/cachePolicy` (`SECOND`, `MINUTE`, `HOUR`, `DAY`). Set `staleTime` and `gcTime` explicitly per endpoint.
- Client config in `lib/queryClient.ts`: 5 min default stale, no refetch on focus.
- Paginated lists: `limit`/`offset` query params on the server. End every paged `ORDER BY` on a unique column (`a.id`), or tied rows can land on two pages or none. The library pages with the `Pager` composition, not infinite scroll: `useAssets` takes a `{ limit, offset }` page, keeps the previous page on screen while the next loads (`keepPreviousData`), and the grid virtualises the one page it holds.
- Server-sent events (imports, previews) are subscriptions, not queries: the DB row is the source of truth and the stream is a nudge to refetch, so a dropped event must always be recoverable.

### API layer (`src/api.ts`)
- `fetchJson<T>(url, options)` is the main wrapper: attaches bearer token, handles 401 via silent token renewal with single-flight deduping, retries once, parses JSON, and throws `ApiError` on non-2xx.
- `uploadFile(url, file, onProgress?)` handles multipart uploads with XHR progress events (fetch lacks upload progress).
- `fetchMaybeJson` is the same for responses that may have no body (204); `fetchBlob` for binary downloads.
- Error types: `ApiError` (with status), `AuthRedirectError` (401 after renewal fails — triggers redirect to login). Narrow with `isAuthRedirectError(err)`.
- Don't call `fetch` directly from components; always go through these helpers so auth/renewal behavior is consistent. Exceptions live in `lib/` and `hooks/` only: the pre-auth calls (`lib/authMode.ts`, `lib/localAuth.ts`), SSE (`fetchEventSource` in `hooks/use*Events.ts`) and the chunk uploader (`lib/chunkedUpload.ts`). The SSE hooks and the uploader attach the bearer themselves and get no 401 renewal.

### Auth / OIDC
- OIDC client config comes from `GET /api/auth/mode` at runtime (`lib/oidc.ts`); there are no build-time OIDC env vars.
- `automaticSilentRenew: true` — token refresh happens transparently in the background.
- Never log tokens or raw session values.

### Styling
- The token **vocabulary** is shadcn's, extended; the **values** are Damson's. OKLCH colour values live in `src/design-system/tokens/tokens.css`. Light is the base (`:root`); dark opts in via a `.dark` class on `<html>` (`applyTheme` in `design-system/tokens/`). There is no `data-theme` attribute.
- The size, radius, motion and elevation scales are **not** in `tokens.css` — they are literals in the second `@theme inline` block of `src/styles/globals.css`. `docs/design-system.md` explains why; the short version is that the first block's `--radius-md: var(--radius-md)` is a self-reference that destroys any declaration of that name.
- Tailwind's theme exposes the tokens as utility classes (`bg-background`, `bg-card`, `text-muted-foreground`, `text-subtle-foreground`, `border-border`, `bg-primary-soft`, `text-success`, `rounded-md`, `shadow-lg`, …) — prefer these over raw `var(--token)`.
- `web/scripts/check-tokens.mjs` fails the build on a retired token name, a raw colour outside `tokens.css`, or a typing surface below the 16px floor. It runs as part of `just lint frontend`.
- `@tailwindcss/typography` for markdown content.
- Use the design-system primitives (`Button`, `Input`, `Dialog`, `Pill`, `Tabs`, …) from `@/design-system` rather than hand-rolling equivalents. New components go in `src/design-system/{primitives,compositions,domain}/` alongside a `*.stories.tsx` with variants + states; see `docs/design-system.md`.
- `cn()` composes classes: `clsx` for conditional joins, `tailwind-merge` for dedup. The role-named font sizes are registered with `tailwind-merge` in `utils/cn.ts` — the contract's eight via `@shared/type-scale`, Damson's 19 composed roles (`text-mono`, `text-caption`, the weight variants, …) via a local group merged onto it. A new size utility belongs in that local group, or it is silently dropped when it meets a text colour. See `docs/design-system.md`.
- Do not reintroduce CDN Tailwind; everything goes through the Vite pipeline.

### Frontend tests (Vitest)
- Config: `vitest.config.ts` defines two projects:
  - **`unit`** — `jsdom`, globals enabled, setup file `src/test/setupTests.ts`, pattern `src/**/*.{test,spec}.{ts,tsx}`. Run with `just test frontend`.
  - **`storybook`** — real Chromium via Playwright and `@storybook/addon-vitest/vitest-plugin`; every `*.stories.tsx` runs as a test with axe-core a11y checks (`a11y.test = "error"`). Run with `just test stories`.
- Libraries: `@testing-library/react`, `@testing-library/user-event` for the unit project; `storybook/test` helpers for story interactions.
- Browsers come from `just install`; the cache lives at `~/Library/Caches/ms-playwright/` on macOS.

---

## Database Migrations

- Location: `database/migrations/`.
- Naming: `NNN_descriptive_name.sql` (zero-padded 3-digit sequence). Example: `002_tags.sql`.
- Each migration is wrapped in `BEGIN; ... COMMIT;`.
- Use `IF NOT EXISTS` for idempotency.
- Primary keys: `bigserial PRIMARY KEY`. `created_at` / `updated_at` are `timestamptz NOT NULL DEFAULT now()`. Event timestamps are nullable `timestamptz`.
- Migrations are SQL-only — no Go migration framework. See `internal/db/migrate.go` for the runner (advisory lock + `schema_migrations` tracking). The file is recorded only after its own COMMIT, so a crash in between re-runs it. Data `UPDATE`s must be idempotent too.
- River's `river_*` tables are the exception: River owns that schema and migrates it from `queue.New`. Don't hand-write migrations for them.
- Schema changes must land in BOTH the SQL migration AND the GORM struct. A `default:` tag makes GORM omit a zero value on Create, so writing `false` to a default-true column needs an explicit `Update`.

## Docker & Release

**`Dockerfile`** is three-stage with no CGo: the SPA build into `web/static/`, a static Go build that
embeds it via `go:embed`, and a minimal runtime as a non-root `damson` user with a `HEALTHCHECK` on
`/api/health`. Base-image versions live in the Dockerfile.

`just release {patch,minor,major}` cuts the tag from an up-to-date `main`; it doesn't re-run checks locally. Run `just check` yourself first.

Don't commit `web/static/` build output — it's gitignored except for the `.gitkeep` placeholder that lets `//go:embed` succeed when the SPA hasn't been built yet.

## When in Doubt

- Mirror existing patterns
- Keep changes minimal and focused. Don't refactor unrelated code.
- Read `docs/` for deeper context before making architectural changes.
