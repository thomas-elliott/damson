# E2E flows

The Playwright suite (`web/e2e/`) drives the real SPA against a real Go backend and Postgres. It
exists for the bugs unit and component tests cannot reach: contract drift between the SPA and the
handlers, router and auth regressions on dependency bumps, and wiring across HTTP, DB, storage and
the job queue. A flow earns a spec when it is on the critical path, crosses those systems, and is
too slow to reproduce by hand on every change.

## How it runs

`just test e2e` builds `cmd/e2e-server` into `web/e2e/.bin`, then Playwright starts it and `vite dev`
through the `webServer` array in `web/playwright.config.ts` (`E2E_TARGET=preview` runs the built
bundle instead). Arguments go straight to Playwright, so `just test e2e --ui` gets the interactive
runner. The backend needs `DAMSON_TEST_POSTGRES_DSN`; `just test e2e` defaults it to the shared local
Postgres. One worker, no parallelism: every test shares the one backend and its database.

`cmd/e2e-server` mounts the production `api.Handler` and River queue in local-auth mode, against
the real SQL migrations. It does not mount `cmd/server`'s middleware (request id, request logging,
security headers, JSON body cap), the metadata service or store lookups, and its login throttle is
effectively off. It adds two test-only routes: `POST /api/test/reset` and `POST /api/test/users`
(seed a user, optionally sysadmin or with a library membership).

Specs import `test` from `web/e2e/fixtures.ts`, which resets the backend before every test. They
seed through the real API (`bootstrapAdmin`, `createLibrary`, `switchCurrentLibrary`,
`uploadImport`, `createReferenceAsset`, `tagAsset`, `createTestUser`) and authenticate by writing the session
into local storage before the SPA loads (`installSession`); only the login and bootstrap specs drive the forms.
`uploadImport` blocks until the import row is `done`. The OIDC spec runs entirely in the browser:
it intercepts `/api/auth/mode` and the IdP endpoints with `page.route`.

## Contract

Covered, one spec per flow in `web/e2e/tests/`: first-run bootstrap and the AuthGuard redirects,
local login, a deep link surviving local and OIDC sign-in, the library page rendering a seeded
asset, paging through the library with the page in the URL (reload, back/forward, page size,
clamping, reset on search), ticking two tags in the Tags picker to filter the library to either (a
sibling tag left out, the chips, a reload, removing one chip, Clear all) and the picker at a 400px
viewport, a tag index card opening the library filtered to its tag, a UI upload accepted and shown
as a placeholder, library switching without leaking the other library's assets, the package page
(opened from the quick look onto a pack's files inside its lone root folder, Back to the results,
the old `/browse` address, next and previous across a library page boundary by button and arrow
key, a reference asset's page, and the stacked layout at 400px), tag and licence edits surviving a
reload (a licence edit, made on the package page, also clearing its review), the OIDC callback and
sign-out, project pins (finding an asset past the first page or by its exact name behind newer
matches, choosing a version, changing a pinned version and its audit row, the unpin confirm, the
asset link), collection add/remove and paging, search, sysadmin gating of the audit page, paging
both audit pages (reload, back/forward, reset on search and scope, a search reaching a later page),
and library settings surviving a reload.

Not covered:

- No spec watches an import reach `done` through the UI. The upload spec stops at the 202 and the
  placeholder; `uploadImport` reaches `done` only as API seeding.
- Accessibility is checked only by the story suite (`just test stories`, axe at
  `a11y.test = "error"`), which `just check` and CI run but `just test` does not.
- Visual regression has no owner.
- Chunked uploads, below.

### Deliberately not covered: chunked uploads

The resumable transport (`docs/architecture.md` section 8.0) has no Playwright flow, by decision.
Triggering it needs a file at or above `CHUNKED_UPLOAD_THRESHOLD` (`web/src/lib/chunkedUpload.ts`).
Generating one per run costs more than the suite's runtime budget, and lowering the threshold for
tests would ship a test-only path through the logic the test is meant to prove.

What covers it instead:

- `web/src/lib/chunkedUpload.test.ts` — the browser half against a recorded fetch: chunk boundaries
  and ordering, the dedupe short-circuit, resume from stored state, retry on a transient failure,
  no retry on a rejected chunk, abort.
- `internal/handlers/api/uploads_test.go` — the server half against real Postgres and storage, with
  the chunk size injected small so a few bytes exercise the multi-chunk path.
- `internal/storage/multipart_test.go` — one contract run against both storage backends.
- `cmd/dmsn/chunked_test.go` — the CLI half against a fake session server.

Still unproven in a browser: the Web Worker hashing path (the unit test stubs the worker), and
`Blob`-body streaming under a real network stack. Both are worth a manual pass against a real large
file before trusting a release.

## Traps

- **Add a new table to `dataTables` in `cmd/e2e-server/main.go`.** Reset truncates that list
  (`CASCADE` reaches tables with a foreign key into it) rather than re-migrating, so a table outside
  both keeps its rows from one test into the next.
- **Don't assume reset clears stored objects.** It clears the database only; blobs persist in
  `E2E_DATA_DIR` (default `web/e2e/.e2e-data`) across tests and runs, so a re-uploaded fixture can
  dedupe onto an object an earlier test wrote.
- **Kill a leftover e2e-server before `just test e2e`.** Outside CI, Playwright reuses whatever
  already answers on `:8081` and `:5174`, so a stale server runs the old code. `just dev` uses
  other ports and can stay up.
- **Don't write a spec that needs store lookups or metadata enrichment.** The e2e server does not
  wire them, so those endpoints answer 503.
- **List `noConsoleErrors` in the test's arguments.** It is opt-in, not automatic; a happy-path spec
  without it lets console errors, the first sign of most upgrade regressions, pass.
- **Wait on a response or a locator, never `waitForTimeout`.** Use `page.waitForResponse` or the
  auto-waiting `expect`; a sleep turns a race into a flake.
- **Select by role, not `data-testid`.** Roles double as an accessibility check.
