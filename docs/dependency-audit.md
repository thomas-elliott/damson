# Pre-Alpha Dependency Upgrade Audit

## Context

`damson` is approaching alpha. Before the cut we want to land any **major-version upgrades** that are worth doing now rather than immediately after the alpha release, since post-alpha breaking rebases are more painful than pre-alpha ones.

This document is the research deliverable: a per-dependency audit of `go.mod` and `web/package.json`, current-vs-latest comparison, and a recommended subset to bump before alpha. Implementation is sequenced in the last section.

Scope: major bumps only. Patch / minor sweeps (`go get -u ./...`, `npm update`) are a separate housekeeping pass at the end.

Audited: 2026-04-26.

---

## Backend (Go) — `go.mod`

Go toolchain is `go 1.26` (latest 1.26.2 as of 2026-04-07). Already on the current major.

| Module | Pinned | Latest (Apr 2026) | Major behind? | Coupling | Action |
|---|---|---|---|---|---|
| `github.com/gin-gonic/gin` | v1.10.0 | v1.12.0 | No (minor bumps) | **Deep** — every handler | Minor bump (v1.11 added HTTP/3, `AbortWithStatusPureJSON`) |
| `gorm.io/gorm` | v1.25.12 | v1.31.1 | No — there is no v2 line yet; "v2" in the docs means GORM v1 generation 2 (already in v1.x) | **Very deep** — entire data layer | Minor bump only; v1.30 introduced a Generics API we may want later |
| `gorm.io/driver/postgres` | v1.5.11 | current major | No | Light | Minor bump |
| `gorm.io/driver/sqlite` | v1.5.6 | current major | No | Light (test path only) | Minor bump |
| `github.com/aws/aws-sdk-go-v2` (and config / credentials / service/s3 / smithy-go) | v1.41.6 | current major (aws-sdk-go-v2 is the v2 line; no v3) | No | **Light** — `internal/services/storage/s3.go` only | Minor bump |
| `github.com/coreos/go-oidc/v3` | v3.11.0 | v3.17.0 | No (still v3) | Light — `internal/auth/oidc.go` only | Minor bump |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | v5.x | No | Light — `internal/auth/local.go` only | Minor bump |
| `github.com/google/licensecheck` | v0.3.1 | v0.x | No (still pre-1.0) | Very light | None |
| `github.com/hajimehoshi/go-mp3` | v0.3.4 | v0.x | No | Light | None |
| `github.com/jfreymuth/oggvorbis` | v1.0.5 | v1.x | No | Light | None |
| `github.com/joho/godotenv` | v1.5.1 | v1.x | No | One call site | None |
| `golang.org/x/crypto` | v0.25.0 | v0.x (Go convention) | No | Light (bcrypt only) | Minor bump |
| `golang.org/x/image` | v0.39.0 | v0.x | No | Light (thumbnail) | Minor bump |

**Backend conclusion:** *No major-version upgrades available.* Every Go dependency we ship is already on its current major line. Recommend a single `go get -u` + `go mod tidy` housekeeping commit before alpha.

---

## Frontend (`web/package.json`) — the actual major-bump decisions

| Package | Pinned | Latest | Majors behind | Coupling | Verdict |
|---|---|---|---|---|---|
| **`vite`** | ^5.4.0 | 8.0.x | **3** | Shallow (config only, single plugin) | **UPGRADE** |
| **`vitest`** + `@vitest/browser` | ^3.2.4 | 4.1.x | **1** | Test infra, ~76 story tests + unit | **UPGRADE** (paired with Vite 8) |
| **`tailwindcss`** | ^3.4.0 | 4.2.x | **1** | Deep (every component) — but codemod assists | **UPGRADE** |
| **`react-router-dom`** | ^6.26.0 | 7.14.x | **1** | 17 files; classic API only (no data router) | **UPGRADE** (low-risk) |
| **`react`** + `react-dom` + `@types/react`(-dom) | ^18.3.1 | 19.2.x | **1** | Core; codebase already 19-clean per audit | **UPGRADE** |
| `@tanstack/react-query` | ^5.51.0 | 5.99.x | 0 (no v6 for React) | Deep | Minor bump only |
| `@tanstack/react-virtual` | ^3.8.0 | v3.x current | 0 | Light | Minor bump |
| `react-dropzone`, `oidc-client-ts` | current majors | current | 0 | Light | None |
| Radix UI primitives | mixed ^1/^2 | current | 0 (already on each pkg's current major) | Wrapped in `src/design-system/primitives/` | None |
| `lucide-react` | ^1.9.0 | 1.x | 0 | 22 files; named imports only | None |
| `class-variance-authority`, `clsx`, `tailwind-merge` | current | current | 0 | Light | None |
| `storybook` + addons | ^10.3.5 | 10.3.5 | 0 | Heavy (76 stories) | None — currently latest |
| `@vitejs/plugin-react` | ^4.3.0 | bumped with Vite 8 | — | Config only | Move with Vite |
| `typescript` | ^5.5.0 | 5.x | 0 | — | Minor bump |
| `playwright` | ^1.59.1 | 1.x | 0 | Test runner | Minor bump |
| `jsdom` | ^24.1.0 | v24/25 | possibly 1 | Vitest env | Bump alongside Vitest 4 if required |
| `@types/node` | ^24.0.0 | 24.x | 0 | — | Minor bump |
| `autoprefixer`, `postcss` | current | — | — | Drops out with Tailwind v4 (which has its own engine) | Re-evaluate post-Tailwind 4 |

---

## Recommended pre-alpha upgrades (in execution order)

Five concrete major-version moves, ordered by dependency. Each is a separate commit so a regression bisects cleanly.

### 1. React 18 → 19  *(foundation)*
- **Why first:** every other frontend bump cares about React's peer range. Doing it first lets us pick the React-19-aligned version of each subsequent dep in one shot.
- **Files:** `web/package.json` (`react`, `react-dom`, `@types/react`, `@types/react-dom`); no source changes expected — the codebase already uses hooks-only function components, `createRoot`, no `defaultProps`/`PropTypes`/`findDOMNode`.
- **Risk:** Low. Optionally run `npx types-react-codemod@latest preset-19 ./src` for any subtle type changes.
- **Verify:** `npm run build`, `npm run test:all`, `npm run storybook` smoke.

### 2. Vite 5 → 8 + `@vitejs/plugin-react` peer  *(build)*
- **Why next:** Vitest 4 needs Vite 8. Our `vite.config.ts` is minimal (alias + dev proxy + single plugin).
- **Files:** `web/package.json`, `web/vite.config.ts` (smoke check; CJS-default removal already complied with).
- **Risk:** Low — no custom plugins, no Environment API usage.
- **Verify:** `npm run dev`, `npm run build`, confirm `/api` proxy still works against `task dev`.

### 3. Vitest 3 → 4 + `@vitest/browser` 4  *(test)*
- **Why next:** depends on Vite 8. v4 stabilizes Browser Mode (already in use via `vitest --project=storybook`).
- **Files:** `web/package.json`, `web/vitest.config.ts`, `web/.storybook/vitest.setup.ts`. `playwright` may need a bump alongside.
- **Risk:** Medium — browser-mode test config can drift. Pre-existing 76 story tests are the canary.
- **Verify:** `npm run test`, `npm run test:stories`, `npm run test:all`.

### 4. React Router 6 → 7  *(routing)*
- **Why:** v6 is on the deprecation path; v7 is largely backward-compatible for our usage. We use only the classic surface — no data-router APIs, no loaders/actions.
- **Files:** Replace `react-router-dom` with `react-router` in ~17 files (the module consolidates in v7). No code changes beyond import paths.
- **Risk:** Low. Optionally enable v7 future flags first as a smoke test, then flip.
- **Verify:** Manual click-through of each page route + `AuthGuard` redirect; existing tests; SPA login redirect.

### 5. Tailwind 3 → 4  *(styling)*
- **Why last:** highest blast radius (every component + globals.css + design tokens) but Tailwind v4 is mature (4.2.x, 15+ months out) and the `@tailwindcss/upgrade` codemod handles most of the rewrite.
- **Files to review:**
  - `web/tailwind.config.ts` — disappears or becomes a thin shim; theme moves to CSS.
  - `web/src/styles/globals.css` — `@tailwind base/components/utilities` → `@import "tailwindcss"`. `@layer components` and `@apply` blocks (6 of them) stay valid but should be sanity-checked.
  - `web/src/design-system/tokens/tokens.css` — already CSS-vars-driven, transitions naturally to v4's `@theme` block.
  - `web/postcss.config.js` and `autoprefixer` — Tailwind v4 has its own engine; both likely become removable.
  - `@tailwindcss/typography` peer — bump to its v4-compatible release.
  - `tailwind-merge` and `class-variance-authority` — verify against new utility names; v4 renames a handful of utilities.
- **Risk:** High footprint, low per-call-site complexity. Run codemod first, then visual-diff Storybook page-by-page.
- **Verify:** Storybook visual sweep; `npm run build-storybook`; `task dev:spa` and click through key pages.

### 6. Housekeeping pass *(non-major, but run as part of the same release)*
- **Backend:** `go get -u ./... && go mod tidy && task pre-release-checks`. Brings Gin to v1.12, GORM to v1.31, go-oidc to v3.17, AWS SDK and `golang.org/x/*` current.
- **Frontend:** `npm update` for everything not bumped above (TanStack Query 5.51 → 5.99, TypeScript 5.5 → current, Playwright, jsdom, `@types/node`, lucide-react, Radix UI).

---

## Verification (end-to-end)

After **each** numbered upgrade above, run, in order:

1. `task fmt && task vet` — Go side untouched but cheap to confirm.
2. `task test` — Go tests.
3. `cd web && npm install && npm run build` — TypeScript + Vite build must succeed.
4. `cd web && npm run test:all` — unit + storybook tests.
5. `task dev:spa` — open in a browser, exercise:
   - Local-mode login flow (`/login` → library page) — covers React Router + auth guard.
   - Library grid scroll — covers `@tanstack/react-virtual`, Tailwind layout.
   - Asset detail panel — covers Radix Dialog + Tailwind components.
   - Settings → Library Members — covers forms + Radix primitives.
6. `task pre-release-checks` — full gate (fmt, vet, Go tests, web tests, `go mod tidy` drift check). Same gate CI runs.

If any step fails, isolate to the most recent commit and bisect within that upgrade rather than rolling back the whole sweep.

---

## Out of scope

- GORM v1 → v2: there is no GORM v2. The "v2" in the docs is the second generation of v1.x. No action.
- Gin v2: not released. No action.
- Storybook v11: not released; we are already on the latest stable v10.3.5.
- TanStack Query v6: not released for React. No action.
- AWS SDK v3: not released. No action.
