# E2E Flow Roadmap

**Status:** v1 (tier 1 implemented; tier 2 + 3 pending) **Last updated:** 2026-04-26

The Playwright scaffold (PR #11) now includes the full tier-1 set, backed
by a dedicated SQLite-driven test binary (`cmd/e2e-server`) that mirrors
the production wiring. This document captures what's covered, what's
intentionally deferred, and the design principles guiding both. Treat the
tiering as a sequencing recommendation, not a hard contract.

## Implementation status

- **Real-backend harness:** `cmd/e2e-server` (test-only Go binary, SQLite +
  AutoMigrate, `/api/test/reset` to drop+remigrate between tests). Built
  via `task web:e2e:build`; Playwright launches it alongside `vite dev`
  via the `webServer` array.
- **Tier 1 specs** in `web/e2e/tests/`: bootstrap routing, local login
  (happy + 401), library renders with seeded asset, full UI import via
  dropzone. All 7 tests pass — the previously `test.fixme`'d
  bootstrap → /library flow (PER-68) is fixed: `BootstrapPage.onSubmit`
  no longer awaits `queryClient.invalidateQueries`, the parallel
  module-level cache singleton in `lib/authMode.ts` is gone, and the
  navigate fires reliably.
- **Tier 2 + 3:** not yet implemented; sketched below.

Companions:
- `docs/CONVENTIONS.md` — testing libraries, fixtures, coverage expectations
- `web/playwright.config.ts` — webServer target (`vite dev` by default,
  `vite preview` with `E2E_TARGET=preview`)

---

## Selection criteria

A flow earns a tier-1 slot when it satisfies most of the following:

1. **Critical path.** If it breaks, the product is unusable for the
   majority of users in their first session.
2. **Multi-system integration.** Touches frontend + HTTP + DB + storage /
   worker — a class of bug unit and component tests fundamentally cannot
   reach.
3. **Upgrade-fragile.** Hits surfaces that have historically broken on
   dependency bumps (router, auth, file upload, async worker plumbing).
4. **Hard to detect manually.** Requires multi-step setup or async waits
   that a developer is unlikely to reproduce on every PR.

---

## Tier 1 — land first

### 1. First-run bootstrap → first login

**What.** Fresh deployment with zero users. `/api/auth/mode` returns
`needs_bootstrap=true`. User submits the bootstrap form, server creates
the first admin, the SPA stores the session token and lands the user in
`/library`. A second visit goes straight to `/login`, not `/bootstrap`.

**Why high-value.** This is the very first thing every operator
experiences. It exercises the full auth surface
(`/api/auth/mode`, `/api/auth/bootstrap`, `/api/auth/login`),
post-login routing, token storage, and the `needs_bootstrap` flag flip
that requires the cache invalidation in `lib/authMode.ts` to work
correctly. AuthGuard regressions (the 6 → 7 react-router rewrite was
exactly this surface) land here first.

**How to test now.** Real backend. Boot the Go binary against an empty
SQLite DB in the Playwright `webServer` (alongside `vite dev`). Drive
the form, assert URL transitions and that a follow-up
`fetch('/api/auth/mode')` returns `needs_bootstrap=false`. Tear down by
deleting the SQLite file between tests.

### 2. Local login → library page renders with content

**What.** Bootstrapped server with a seeded user and one library
containing N assets. User logs in, lands on `/library`, and sees asset
cards rendered with thumbnails and metadata. Click into a detail panel,
assert the right asset loads.

**Why high-value.** Most common authenticated session. Covers `LoginPage`,
AuthGuard's "ready" path, `/api/me`, `/api/libraries`, the library
switch into `/api/me/current-library`, then the scoped fetch
`/api/scoped/assets`. Catches react-query cache key regressions, virtualized
grid breakage (`@tanstack/react-virtual`), broken thumbnail proxying
(`/thumbs/:objectKey`), and any router-related URL/state desync.

**How to test now.** Real backend with a seed fixture (one user,
one library, ~5 assets). Load fixtures via a small Go test helper that
the Playwright `globalSetup` shells out to (or via a `/test/seed` endpoint
gated to test mode). Assertions: at least one asset card visible by role,
detail panel opens on click, no console errors throughout.

### 3. Asset import — `.unitypackage` upload to "ready"

**What.** From the Queue page, drop a small fixture `.unitypackage` file.
Watch the import row move queued → extracting → done. Refresh `/library`
and find the new asset.

**Why high-value.** Damson's signature feature. Touches the entire
import pipeline: dropzone → multipart upload (`POST /api/scoped/imports`)
→ storage layer (`internal/services/storage`) → async extract worker
(`internal/services/extract`) → DB writes → thumbnail generation
→ library refresh. Most failure modes here are silent: the upload
"works" but extraction fails, or the library doesn't refresh, or the
thumbnail is missing. Unit tests cover each stage; only an E2E catches
the wiring.

**How to test now.** Real backend, real storage on a tmpdir
(`STORAGE_LOCAL_ROOT=$(mktemp -d)`), real extractor. Ship a
~50KB fixture `.unitypackage` in `web/e2e/fixtures/`. Use
`page.locator('input[type=file]').setInputFiles()` to bypass the actual
dropzone (more reliable than simulating drag/drop). Poll the import row
for "done" with a 30s timeout, then navigate to library and assert
the new asset is visible. Clean up the tmpdir in afterEach.

---

## Tier 2 — next wave

### 4. Library switch isolates scope

**What.** User with two libraries A and B (each with distinct assets)
switches active library and verifies the asset list refetches with the
new scope — none of A's assets leak into B's view.

**Why high-value.** Multi-tenancy correctness. A subtle react-query cache
key bug would silently show one library's data while the URL says
another, which is a security/privacy issue, not just a UX one.

**How to test now.** Real backend, two seeded libraries with disjoint
assets. Switch via the library picker, assert the asset IDs visible after
the switch are exactly B's, then switch back and re-assert.

### 5. Tag and licence edits persist

**What.** Open an asset detail. Add a new tag, change the licence, save.
Reload the page (or remount the SPA) and confirm both stuck.

**Why high-value.** Most common write path after import. Exercises the
PATCH/PUT handlers, optimistic update logic in `useMutations.ts`, and
the licence detection auto-suggest. Reload-then-reassert catches the
class of "optimistic update masks a backend write failure" bug.

**How to test now.** Real backend, one seeded asset. Use Testing Library
roles to find the tag input and licence selector. After save, call
`page.reload()` and re-query the same elements.

### 6. OIDC sign-in callback

**What.** With the server in OIDC mode, navigate to `/auth/callback`
with a stub `code`+`state`, expect the SPA to exchange and land on
`/library`.

**Why high-value.** OIDC is one of two supported auth modes and has
zero E2E coverage today. The full code-exchange dance is hard to fake,
but the SPA-side callback handling is testable in isolation.

**How to test now.** Mocked. Stand up either a fake OIDC issuer (e.g.
`oidc-provider` running in `globalSetup`) or `page.route` the
`oidc-client-ts` token endpoint to return a synthetic JWT. Skip a
real provider in CI; the value is in catching SPA-side regressions in
`LoginCallbackPage` and `lib/oidc.ts`.

---

## Tier 3 — nice-to-have

Defer until tier 1 + 2 are stable. Brief notes only:

- **Project create + pin assets** — exercises the projects CRUD and
  pin/unpin write path.
- **Collection create + share link** — collections CRUD + sharing.
- **Search returns filtered results** — covers the search route and the
  Postgres `tsvector` index (only when running against Postgres, not
  SQLite — flag this clearly in the spec).
- **Admin audit log visibility** — sysadmin role gating.
- **Settings persistence** — change theme/library setting, reload, value
  sticks.

---

## Cross-cutting principles

**Real backend over mocked API for tier 1.** Mocked-API tests are fast
but don't catch contract drift between the SPA and the Go handlers.
Once we're spinning up the Go binary anyway (a one-time cost in
`globalSetup`), the marginal cost per real-backend test is small.

**Console-error capture in every flow.** The `noConsoleErrors` fixture
in `web/e2e/fixtures.ts` should be opt-in for any flow that doesn't
intentionally exercise an error path. Most upgrade regressions surface
as silent console errors first.

**Stable selectors via roles, not test IDs.** Prefer
`getByRole('button', { name: /sign in/i })` over `data-testid`. Test
IDs rot; roles double as accessibility checks.

**Flake budget.** Target 0% flake. If a test starts flaking, fix the
test or the underlying race — never add a `waitFor(timeout)`. Use
Playwright's auto-waiting locators and `expect(...).toPass()` where
genuine retries are needed.

**A11y and visual regression deferred.** Both are valuable but lower
priority than the flow coverage above. Revisit after tier 1 + 2 ship.

---

## Suggested implementation order

1. Stand up a real-backend test harness (Go binary + SQLite + tmpdir
   storage + seed helper) — one-time infrastructure cost.
2. Convert the existing `login.spec.ts` happy path from mocked to real
   backend (keep the 401 test mocked — it's still valuable as is).
3. Add tier-1 flows in order: bootstrap → library landing → asset import.
4. Add tier-2 flows as time allows.
5. Reassess tier 3 once we have a feel for E2E suite runtime in CI
   (target: under 3 minutes for the full suite).
