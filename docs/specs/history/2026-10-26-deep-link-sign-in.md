# Keep the deep link through sign-in

Covers DAM-49

## Why are we doing this

Someone who opens a link to a page, for example `/library/42`, while signed out is sent to `/` after
signing in, and has to find the asset again.

## Steering

None recorded

## Design

none: operator approved the approach in the spec round on 2026-10-26

## Decisions

### Which sign-in paths keep the deep link?
Answer: both. Local login and the OIDC redirect each return to the page that was asked for.
By: recommended, operator said "go" (2026-10-26)

### Which destinations are honoured?
Answer: only paths inside the app. Anything else, such as an outside URL, falls back to `/`.
By: settled by the open-redirect risk of honouring any URL (2026-10-26)

## Acceptance

- Signed out, opening `/library/42` and signing in with local login opens `/library/42`.
- Signed out under OIDC, opening `/library/42` and completing sign-in at the IdP opens
  `/library/42`.
- The query string and hash of the requested page survive sign-in.
- Signing in from `/login` with no requested page, or after logging out, opens `/`.
- A requested destination outside the app is ignored, and sign-in opens `/`.
- A first-login user who is held on profile setup reaches the requested page once setup is done.

## Out of scope

- Returning to the page after a session expires mid-use and a 401 renewal fails.

## Start here

- web/src/components/AuthGuard.tsx
- web/src/pages/LoginPage.tsx
- web/src/pages/LoginCallbackPage.tsx
- docs/architecture.md (SPA auth flow)

## Deferred follow-ups

None

### Cleanup

- Built by the stagecraft `spec-to-pr` factory as work item `dam-49`; no `/cleanup` pass ran.
  `just check` passed on 119628c (lint, format, tidy, build, Go, Vitest, stories, e2e).
- No `docs/intent/` in the repo, so nothing graduated. Retired with these findings from the
  reviews:
  - MED · plan review F1: reading the location through a ref in AuthGuard would trip
    `react-hooks/refs`. Addressed: the effect reads `window.location`.
  - MED · plan review F2: the OIDC outbound leg was not tested with a query and hash. Addressed:
    the authorize-intercept e2e opens `/library?q=x#files` and asserts the stored `returnTo`.
  - LOW · plan review F3: reading local storage after leaving for the IdP sees the wrong origin.
    Addressed: the e2e answers the authorize request with 204, which keeps the app's document
    (`route.abort()` destroyed the context).
  - LOW · plan review F4: `safeReturnTo` must compare and return the parsed URL's parts.
    Addressed, with unit cases for `/login/`, `/auth/callback?…`, and a tab or newline.
  - LOW · plan review F5: the OIDC e2e should not open an asset the catch-all mock cannot serve.
    Addressed: it uses `/library?q=x#files`, and `login.spec` seeds the asset it opens.
  - LOW · code review R1: after a failed OIDC sign-in, "Sign in again" goes to `/`, so the retry
    loses the deep link. Not addressed; the spec does not cover the retry path
    (web/src/pages/LoginCallbackPage.tsx).
  - LOW · code review R2: no unit case passes a router location with an off-site pathname such
    as `//evil.example`; the code already sends it to `/`. Not addressed
    (web/src/lib/returnTo.test.ts).
