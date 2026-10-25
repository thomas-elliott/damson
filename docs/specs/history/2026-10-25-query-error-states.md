# Show query errors instead of empty pages

Covers DAM-44

## Why are we doing this

A failed fetch currently looks the same as an empty library, queue or project list. The user
should see that loading failed and be able to retry.

## Steering

None recorded

## Design

none — operator approved the approach in the DAM-44 spec round on 2026-10-25

## Decisions

### Which screens get an error state?
Answer: Library, Queue, Processing, Projects, Collections, Audit and Admin audit. Project detail
and Collection detail get the same split as the asset panel: a 404 shows "not found", and any
other failure shows an error with a retry.
By: recommended, operator said "go" (2026-10-25)

### When does the error replace what is on screen?
Answer: only when there is no data to show, as on a failed first load or a failed page change. A
background refetch that fails while earlier data is on screen leaves the screen as it is.
By: recommended, operator said "go" (2026-10-25)

### What happens when the OIDC callback fails?
Answer: a failed top-level callback stays on the callback page. It shows the error message, such
as the IdP's description, and a "Sign in again" button. It no longer navigates to `/`, which made
AuthGuard redirect straight back to the IdP. The silent-renew iframe is unchanged.
By: recommended, operator said "go" (2026-10-25)

### How is the error state built, and what does it say?
Answer: one shared `ErrorState` composition, built on `EmptyState` and offering a Retry action.
Each screen gives it fixed wording, and the server's error message is never shown.
By: settled by docs/design-system.md and docs/CONVENTIONS.md (generic 500 body) (2026-10-25)

### Does a failed page fetch keep the requested page?
Answer: this is already built. `UrlPager` does not clamp while `error` is set, and Library uses
it. No work is needed beyond the Library error state.
By: settled by web/src/components/UrlPager.tsx (DAM-51, f296852) (2026-10-25)

## Acceptance

- When the list fetch fails on Library, Queue, Processing, Projects, Collections, Audit or Admin
  audit, the page shows an error state with a Retry button instead of an empty page. Retry loads
  the list once the fetch succeeds.
- A background refetch that fails while data is on screen leaves that data showing.
- The asset panel shows "not found" for a missing asset and an error with a Retry button for any
  other failure. It never shows a blank body titled "Asset".
- Project detail and Collection detail behave the same way: a 404 shows "not found", and any other
  failure shows an error with a Retry button.
- A failed sign-in callback stays on the callback page, shows the error, and offers "Sign in
  again". It does not loop back to the IdP.
- On a failed page fetch in Library, the URL stays on the requested page.

## Out of scope

- Error handling for mutations and dialogs, which already show inline errors.
- Settings and other screens not named above.
- Changing the query retry policy.

## Start here

- docs/design-system.md
- web/src/hooks/useDataQueries.ts
- web/src/pages/LoginCallbackPage.tsx

## Deferred follow-ups

None
