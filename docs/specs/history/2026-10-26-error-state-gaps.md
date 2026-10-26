# Fix the gaps left open in the DAM-44 error states

Covers DAM-62

## Why are we doing this

A few failed loads still look empty or leave the user stuck: a collection whose asset list fails
reads as empty, and a failed page change in Library leaves no pager to step back with.

## Steering

None recorded

## Design

none: follows docs/specs/history/2026-10-25-query-error-states.md; operator approved the remaining
calls in the spec round on 2026-10-26

## Decisions

### What do the remaining detail lists show when they fail?
Answer: Collection detail's asset list and Project detail's audit section show the same error with
Retry as the other DAM-44 screens, in place of their empty state.
By: settled by docs/specs/history/2026-10-25-query-error-states.md (2026-10-26)

### What happens to Library's pager when a page change fails?
Answer: the pager stays on screen below the error, so the user can step to another page.
By: recommended, operator said "go" (2026-10-26)

### What does the callback page say when sign-in fails?
Answer: the IdP's own error description when the IdP returned one; otherwise a fixed line, never
an oidc-client-ts message.
By: recommended, operator said "go" (2026-10-26)

## Acceptance

- When Collection detail's asset list fails to load, the page shows an error with Retry, not "No
  assets in this collection". Retry loads the assets once the fetch succeeds.
- When Project detail's audit section fails to load, it shows an error with Retry, not an empty
  list.
- When a page change in Library fails, the error shows with the pager still below it, and stepping
  to another page loads that page.
- A test covers a 400 on a detail fetch showing "not found".
- A failed sign-in that the IdP described shows the IdP's description; any other failure, such as
  "No matching state found in storage", shows a fixed line instead.

## Out of scope

None

## Start here

- docs/specs/history/2026-10-25-query-error-states.md
- web/src/pages/CollectionDetailPage.tsx
- web/src/pages/ProjectDetailPage.tsx
- web/src/pages/LibraryPage.tsx
- web/src/pages/LoginCallbackPage.tsx

## Deferred follow-ups

None

### Review findings

Built by the stagecraft `spec-to-pr` factory as work item `dam-62`. Open medium and low findings
from its reviews, and whether this change addressed them:

- MED · plan review: Library's remembered total could reset on every page change or record a
  placeholder total from the previous filter. Addressed: it is kept per search and filter without
  page and page size, and recorded only when the load has settled (web/src/pages/LibraryPage.tsx).
- LOW · plan review: the existing callback test expected a plain error's message on screen.
  Addressed: it now rejects with an IdP `ErrorResponse` (web/src/pages/LoginCallbackPage.test.tsx).
- LOW · plan review: the detail tests' fetch stub answered every other endpoint with `{}`.
  Addressed: other endpoints take a body per path (web/src/pages/ErrorStates.test.tsx).
- LOW · plan review: the 400 test passes on main, since `isNotFound` already counts 400. Addressed
  as intended: the test pins existing behaviour; no code change.
- LOW · plan review: `instanceof ErrorResponse` could miss across module copies. Addressed: matched
  by the name oidc-client-ts sets (web/src/pages/LoginCallbackPage.tsx).
- LOW · code review: Collection detail's Retry test answers the retry with an empty list, so it
  shows the error clearing but not assets loading. Not addressed (web/src/pages/ErrorStates.test.tsx).
- LOW · code review: Collection detail's loading line while its assets load has no test. Not
  addressed (web/src/pages/CollectionDetailPage.tsx).

### Follow-up: fac-5

An independent review of PR #20 found two more low gaps. Follow-up `fac-5` (branch
`fac-5-board/library-pager-and-asset-activity-errors`) fixed both:

- LOW · independent review: nothing tested the guard on the Library pager's remembered total. The
  mock never returned a placeholder load, so dropping `!placeholder` from `settledTotal` would show
  a stale pager on a failed new search. Addressed: one test covers a placeholder load followed by a
  failure, and one covers a changed search that fails. Each fails if its guard is removed
  (web/src/pages/LibraryPage.test.tsx).
- LOW · independent review: the asset panel's activity showed "No activity yet" when its load
  failed. Addressed: it shows "Couldn't load recent activity" with a Retry, as Project detail does
  (web/src/hooks/useDataQueries.ts, web/src/components/detail/AssetDetailPanel.tsx,
  web/src/pages/ErrorStates.test.tsx).

Its reviews left these low findings:

- LOW · plan review: the plan checked only the placeholder guard by removing it, not the search
  check in `pagerTotal`. Addressed: both guards were removed in turn, and each removal failed its
  test.
- LOW · plan review: the activity test asserted only a Retry, and the panel has another. Addressed:
  it asserts the activity error's own title (web/src/pages/ErrorStates.test.tsx).
