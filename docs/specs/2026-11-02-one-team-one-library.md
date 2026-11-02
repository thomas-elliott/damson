# One team, one library

Covers no board Task.

## Why are we doing this

The repo is going public. A team shares one library: the half-built parts of multi-library support (creating, switching and deleting libraries) go, so nobody can end up with no way into their library.

## Steering

- "I am wanting to make it public on my github" — repository, 2026-10-21, session 477946a3-d94e-482d-a814-e822cc4041f0
- "My gut feeling with those two is if we drop support at least for now, there will be a lot fewer gaps/functions missing that would need to be built, but we need to validate that" — multi-user and multi-library, 2026-11-02, session 17c4a8e2-1075-49c7-85d3-c8a8149cbcea

## Design

none — operator chose "C: team, one library" for section 0 of the go-public plan: keep users, roles and library membership; drop creating, switching and deleting libraries.

## Decisions

### What stays of multi-user support?
Answer: Users, server roles, library roles, the members table and invites all stay. Library membership remains the access grant, so in OIDC mode a new sign-in sees nothing until an admin invites them.
By: operator — chose "C: team, one library (Recommended)" (2026-11-02)

### Does the schema change?
Answer: No. `library_id`, its query filters, the `scoped` route group, `requireLibrary()` and token pinning stay as they are, so adding libraries back later is mostly routes and UI.
By: operator — chose "C: team, one library (Recommended)" (2026-11-02)

### What happens to existing data?
Answer: No data migration. Existing installs have one library and keep it.
By: operator — chose "No: one library everywhere" (2026-11-02)

### Where does a member with no current library land?
Answer: A member whose current library is unset or points at a library they no longer belong to opens the library they do belong to. This covers an invited member, whose current library is never set today. "No library access yet" shows only to a signed-in user who belongs to no library.
By: recommended, operator said "go" (2026-11-02)

### Which routes go?
Answer: `POST /libraries`, `DELETE /libraries/:id`, `GET /libraries` and `POST /me/current-library`. `PATCH /libraries/:id` and the `/libraries/:id/members` routes stay.
By: recommended, operator said "go" (2026-11-02)

### What replaces the library picker?
Answer: The picker goes. The library's name shows as a plain page heading in its place.
By: recommended, operator said "go" (2026-11-02)

### What stays in the library's settings?
Answer: Rename and description stay. The Delete button goes.
By: recommended, operator said "go" (2026-11-02)

### What happens to the docs?
Answer: Rewrite `docs/how-to/separate-libraries.md` as a how-to on inviting your team, keeping its parts on adding people, roles and the library's settings. Remove the README's "Keeps libraries apart" claim, its "multiple libraries" link and the gap line about creating a library, and fix the "more than one library" link in `docs/getting-started.md`. The README's "small game team" positioning stays.
By: recommended, operator said "go" (2026-11-02)

### What happens to the e2e suite?
Answer: Remove `library-switch.spec.ts` and the `switchCurrentLibrary` fixture. Add an e2e test in which an admin invites a member, the member signs in, and the member sees the library.
By: recommended, operator said "go" (2026-11-02)

## Acceptance

- The Library page shows the library's name as a heading and has no library picker.
- Settings has no way to create, switch or delete a library; the library's name and description can still be edited there.
- `POST /api/libraries`, `DELETE /api/libraries/:id`, `GET /api/libraries` and `POST /api/me/current-library` return 404.
- An admin invites a new member; the member signs in and sees the library's assets without any other step.
- A member removed from the library sees "No library access yet" on their next request, and a signed-in user who was never invited sees it too.
- An existing install keeps its library, its assets and its members after upgrading, with no migration.
- `dmsn` commands, including those using a personal access token, work as before.
- No doc under `docs/` and nothing in the README describes creating, switching or deleting libraries, or keeping libraries apart.
- `just check` passes.

## Out of scope

- Deleting or disabling a user, the last-sysadmin race (plan item 3.7), the unaudited password reset (3.8) and the admin handler tests.
- Removing `library_id` or any schema change.
- Renaming bootstrap's default "Personal" library.
- Every other item in the go-public plan.

## Start here

- `docs/architecture.md` (auth and library sections)
- `internal/handlers/api/handler.go`
- `internal/handlers/api/middleware_library.go`
- `internal/handlers/api/members.go`
- `web/src/pages/LibraryPage.tsx`

## Deferred follow-ups

None

## Implementation report

Mode: committing — branch `feat/one-team-one-library`, one commit per stage (AGENTS.md); no push, no PR.

### Stages

1. Server: a signed-in member whose current library is unset or stale opens the library they belong to (`requireLibrary`, `GET /api/me`).
2. Server: remove `POST /libraries`, `DELETE /libraries/:id`, `GET /libraries` and `POST /me/current-library`, with their handlers and tests.
3. SPA: the library's name replaces the picker as the Library page heading; Settings loses Delete library; drop the hooks and query key only those used.
4. E2E: remove `library-switch.spec.ts` and the `createLibrary`/`switchCurrentLibrary` fixtures; add invite → member signs in → sees the library.
5. Docs: rewrite `docs/how-to/separate-libraries.md` as inviting your team; fix README, `docs/getting-started.md` and any standing doc that describes multiple libraries.
6. SPA (added during real proof): a revoked local session goes to sign-in instead of hanging on Loading…, so a removed member reaches "No library access yet".

### Choices made

- `internal/handlers/api/middleware_library.go`: `resolveLibrary` keeps the current library while the user is a member, otherwise takes their membership with the lowest library id; it never writes `current_library_id`, and a token principal never falls back from its pinned library.
- `docs/how-to/invite-your-team.md`: renamed from `separate-libraries.md` so the filename matches the rewritten content; every link updated, and `docs/images/library-switcher.png` deleted with its row.
- `web/src/components/AuthGuard.tsx`, `web/src/lib/queryClient.ts`: an `AuthRedirectError` from any query or mutation invalidates `me`; `ProfileGate` sends a local session to `/login` with the requested page, and OIDC keeps its loading state.

### Review — stages 1…5

#### Verdict
No blockers

#### Checked
repo rules (AGENTS.md, docs/architecture.md request lifecycle and auth), documented hazards (library_id filter on every scoped query, 403-never-404 on `scoped`, token pinning, 500 returns generic "internal error", no tokens in logs, doc updated with the change), mechanical checks (shape conformance of `currentLibrary` in `/api/me`, coverage, cross-boundary contract: SPA, `dmsn` and e2e no longer call the removed routes, 403 error strings matched by no client), logic (`resolveLibrary` ORDER BY `m.library_id = ? DESC, m.library_id` prefers the current library, then the lowest id; nil current binds 0 and matches nothing), error handling, silent failure, input validation, security (session fallback limited to the user's own memberships; token principal still filtered to its pinned library, so a removed token owner gets 403), idempotency, credentials, performance (`library_memberships_user_idx` covers the new `user_id`-only filter on the hot path), docs (no README or `docs/` text left on creating, switching or deleting libraries), tests (resolver covered for invited, stale, kept-current, no-membership and pinned-token cases; e2e invite flow) · issues in: none · n/a: concurrency (no shared state), external contracts (no external calls), resource management (no new resources), logging (no new log sites), shared-library precedence (no new components) · validation: in-context, 0 candidates, 0 confirmed, 0 refuted

### Review — stages 6…6

#### Verdict
MED · introduced — the redirect keys only on `useMe`'s error, so a removed member whose `/api/me` is still fresh (fetched under 5 min ago, `staleTime: 5 * MINUTE`, no focus refetch) and who then changes a filter or page gets a 401 on that page's query: `renewOrReject` clears the local session, `failedToLoad` ignores the `AuthRedirectError`, nothing navigates, and the Library page shows an empty "0 assets" instead of sign-in. Fix: in local mode, react to any query's `AuthRedirectError`, for example a `QueryCache` `onError` in `lib/queryClient.ts` that invalidates `queryKeys.me` so ProfileGate sees it, or sends the tab to `/login` (web/src/components/AuthGuard.tsx:109) → fixed: a `QueryCache` `onError` (and the mutation cache) invalidates `me` on `AuthRedirectError`; `src/lib/queryClient.test.ts`

#### Checked
repo rules (AGENTS.md frontend API discipline, docs/architecture.md SPA auth section and traps), documented hazards (no direct `fetch`, query keys only in `lib/queryKeys.ts`, no token logging, sign-in return goes through `safeReturnTo`; all hold), mechanical checks (the `from` router-state shape matches what `LoginPage` reads; `ProfileGate`'s `mode` prop typed the same as `useAuthMode().mode`), logic (local redirect ordered ahead of the loading branch; OIDC keeps loading; after sign-in the cached `me` error does not loop back, because in TanStack v5 a remount refetches a query in the error state with no data and resets it to pending), error handling, silent failure (the remaining `AuthRedirectError` path on non-`me` queries; see verdict), security (the stale token is already cleared by `renewOrReject` before the redirect), tests (new test covers local redirect, existing OIDC test now pins mode explicitly; nothing covers a 401 on a page query after `me` loaded), docs (architecture.md auth bullets stay true) · issues in: logic, silent failure · n/a: concurrency (no shared state), performance (no hot path), resource management (no new resources), shared-library precedence (no new component), completeness honesty (no pagination touched) · validation: in-context, 1 candidate, 1 confirmed, 0 refuted

### Results

| Item | Status | Notes |
| --- | --- | --- |
| Library page: name as heading, no picker | passed | `LibraryPage.test.tsx` "names the page after the library, with no way to switch library" fails if the picker or the sr-only "Library" heading comes back; `team-invite.spec.ts` asserts the `Personal` h1 |
| Settings: no create/switch/delete; name and description editable | passed | `LibraryProfileSection.stories.tsx` Admin asserts no Delete button (fails if the danger zone returns); `LibraryProfileSection.test.tsx` saves a rename keeping the description; no create control existed, the switch was the picker |
| Four routes return 404 | passed | `TestLibraryLifecycleRoutesAreGone`; with HEAD~ handlers it got 200/201/204/403 (confirmed). Real `cmd/server` (running `just dev`, commit 060f24d): all four 404 by curl |
| Invited member signs in and sees the assets | passed | `team-invite.spec.ts` against e2e-server and Postgres; fails at the `Personal` heading with stage 1 reverted (confirmed). `TestRequireLibrary_InvitedMemberOpensTheirLibrary` fails on the old middleware (confirmed) |
| Removed member, and never-invited user, see "No library access yet" | passed | Never invited: `TestRequireLibrary_NoMembershipIsForbidden`, plus a temporary e2e proof (real server, sign-in shows the message). Removed: removal revokes their sessions, so the next request lands on sign-in and signing in shows the message; before stage 6 the tab hung on Loading… Proved by temporary e2e specs (reload → `/login` → message; in-app navigation → `/login`), deleted afterwards. `AuthGuard.test.tsx` "sends a revoked local session to sign in" and `queryClient.test.ts` both failed before their fixes (confirmed) |
| Existing install keeps library, assets, members; no migration | passed | `git diff main --name-only` touches no migration or model; `TestRequireLibrary_KeepsCurrentLibraryWhileAMember` fails without the current-library ordering (mutation confirmed); the dev server's existing data still answers `/api/me` with Personal/admin |
| `dmsn`, including a personal access token | passed | `TestRequireLibrary_TokenStaysPinned` fails with the token filter and ordering removed (mutation confirmed). Real: a read PAT minted on the dev server, `dmsn home` showed library Personal, token revoked (204). Only `home` was run |
| No doc or README describes creating, switching, deleting or keeping libraries apart | passed | grep over README.md and docs/ (excluding specs) for those phrases: no match. `docs/images/library-overview.png` (README) still shows the old picker's role badge and chevron; retaking it needs the same seed data |
| `just check` | passed | `E2E_PORT=5175 E2E_BACKEND_PORT=8082 just check`, EXIT=0: Go 24 packages ok, Vitest 573 passed, stories 154 passed, Playwright 37 passed; lint 22 warnings, 0 errors, none in changed files |
| review: stages 1…5 | passed | `### Review — stages 1…5`, no findings |
| review: stages 6…6 | passed | `### Review — stages 6…6`, 1 MED → fixed (commit b8751dc) |

Real proof ran against the operator's running `just dev` server and the e2e server's `damson_test` database. The only state left in the dev database is the revoked token row `one-library proof`.

