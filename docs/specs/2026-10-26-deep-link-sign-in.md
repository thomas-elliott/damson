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
