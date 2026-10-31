import type { Page } from "@playwright/test";

import { test, expect } from "../fixtures";

// PER-78 — OIDC sign-in callback. The full code-exchange dance is faked
// purely on the SPA side: /api/auth/mode is intercepted to return "oidc"
// (the e2e backend itself stays in local mode), the IdP discovery, JWKS,
// and token endpoints are all served by page.route() against a fake
// authority URL, and a SigninState pre-seeded into sessionStorage closes
// the loop with oidc-client-ts. The value is in catching SPA-side
// regressions in LoginCallbackPage and lib/oidc.ts; nothing here exercises
// the backend's OIDC bearer-token validation.
//
// The auth env vars used here come from playwright.config.ts. Authority
// must match — oidc-client-ts compares state.authority against settings.

const AUTHORITY = "https://oidc-mock.test";
const CLIENT_ID = "damson-test";
// Port default matches FRONTEND_PORT in playwright.config.ts.
const REDIRECT_URI = `http://localhost:${process.env.E2E_PORT ?? 5174}/auth/callback`;
const SCOPE = "openid profile email";

const STATE_ID = "state-per78-fixed";
const NONCE = "nonce-per78-fixed";
const SUBJECT = "oidc-test-subject";

// SigninState storage shape comes from oidc-client-ts v3
// (src/SigninState.ts toStorageString); WebStorageStateStore prefixes
// each key with `oidc.` and defaults to localStorage for both
// `stateStore` and `userStore` when neither is explicitly configured.
const SIGNIN_STATE = {
  id: STATE_ID,
  data: undefined,
  created: Math.floor(Date.now() / 1000),
  request_type: "si:r",
  url_state: undefined,
  // PKCE verifier is required so _processCode runs the token exchange.
  // Value is opaque to the mock token endpoint, which always returns the
  // synthetic id_token regardless of inputs.
  code_verifier: "pkce-verifier-per78-fixed",
  authority: AUTHORITY,
  client_id: CLIENT_ID,
  redirect_uri: REDIRECT_URI,
  scope: SCOPE,
  client_secret: undefined,
  extraTokenParams: undefined,
  response_mode: undefined,
  skipUserInfo: true,
  nonce: NONCE,
};

const DISCOVERY_DOC = {
  issuer: AUTHORITY,
  authorization_endpoint: `${AUTHORITY}/authorize`,
  token_endpoint: `${AUTHORITY}/token`,
  userinfo_endpoint: `${AUTHORITY}/userinfo`,
  jwks_uri: `${AUTHORITY}/jwks`,
  response_types_supported: ["code"],
  subject_types_supported: ["public"],
  id_token_signing_alg_values_supported: ["RS256"],
};

// Build a minimal id_token. oidc-client-ts decodes the JWT to read claims
// (sub, nonce, exp) but does not verify the signature locally — the third
// segment can be anything that base64url-decodes.
function fakeIdToken(): string {
  const header = { alg: "RS256", typ: "JWT", kid: "test" };
  const now = Math.floor(Date.now() / 1000);
  const payload = {
    iss: AUTHORITY,
    aud: CLIENT_ID,
    sub: SUBJECT,
    nonce: NONCE,
    exp: now + 3600,
    iat: now,
    auth_time: now,
    email: "oidc-user@example.com",
    name: "OIDC Test User",
  };
  return `${b64url(JSON.stringify(header))}.${b64url(JSON.stringify(payload))}.signature`;
}

function b64url(input: string): string {
  return Buffer.from(input, "utf8")
    .toString("base64")
    .replace(/=+$/, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

const jsonRoute = (body: unknown) => ({
  status: 200,
  contentType: "application/json",
  body: JSON.stringify(body),
});

// setupOidcRoutes wires the full SPA-side OIDC dance (mode flip, discovery,
// JWKS, token exchange, signin-state seed) plus the post-login reads. `me` is
// what GET /api/me returns; `patchedMe`, when given, is returned for the
// PATCH /api/me the profile step issues; `returnTo`, when given, is the page
// AuthGuard asked to come back to, seeded into the SigninState. Used by the
// profile, deep-link and logout tests; the first test keeps its inline routes
// for its stricter storage assertions.
async function setupOidcRoutes(
  page: Page,
  me: Record<string, unknown>,
  patchedMe?: Record<string, unknown>,
  returnTo?: string,
) {
  await page.route("**/api/auth/mode", (route) =>
    // The `oidc` block is what makes fetchAuthMode() call configureOidc();
    // without it getUserManager() throws "OIDC not configured".
    route.fulfill(
      jsonRoute({
        mode: "oidc",
        needs_bootstrap: false,
        oidc: { authority: AUTHORITY, client_id: CLIENT_ID },
      }),
    ),
  );
  await page.route(`${AUTHORITY}/.well-known/openid-configuration`, (route) =>
    route.fulfill(jsonRoute(DISCOVERY_DOC)),
  );
  await page.route(`${AUTHORITY}/jwks`, (route) => route.fulfill(jsonRoute({ keys: [] })));
  await page.route(`${AUTHORITY}/token`, (route) =>
    route.fulfill(
      jsonRoute({
        access_token: "synthetic-access-token-per78",
        id_token: fakeIdToken(),
        token_type: "Bearer",
        expires_in: 3600,
        scope: SCOPE,
      }),
    ),
  );
  await page.route("**/api/me", (route) =>
    route.fulfill(jsonRoute(route.request().method() === "PATCH" && patchedMe ? patchedMe : me)),
  );
  await page.route("**/api/libraries", (route) =>
    route.fulfill(
      jsonRoute({
        libraries: [{ library_id: 1, name: "OIDC Test Library", role: "admin", is_current: true }],
      }),
    ),
  );
  await page.route(
    /\/api\/(assets|tags|projects|collections|imports|triage|previews|settings|audit|admin|files|thumbs)/,
    (route) => route.fulfill(jsonRoute({})),
  );
  await page.addInitScript(
    ({ key, value }) => {
      try {
        window.localStorage.setItem(key, value);
      } catch {
        /* about:blank — storage isn't available here */
      }
    },
    {
      key: `oidc.${STATE_ID}`,
      value: JSON.stringify({ ...SIGNIN_STATE, data: returnTo ? { returnTo } : undefined }),
    },
  );
}

const ME = {
  id: 1,
  email: "oidc-user@example.com",
  display_name: "OIDC Test User",
  role: "user",
  needs_profile: false,
  current_library: { id: 1, name: "OIDC Test Library", role: "admin" },
};

// A page with no detail fetch, so the catch-all empty mocks are enough.
const REQUESTED = "/library?q=x#files";

const onRequestedPage = (url: URL) => url.pathname + url.search + url.hash === REQUESTED;

test.describe("OIDC sign-in callback", () => {
  test("exchanges code, stores user, and lands on /library", async ({ page, noConsoleErrors }) => {
    void noConsoleErrors;
    // The SPA calls /api/auth/mode at boot; flip it to oidc *before*
    // navigation so AuthGuard takes the OIDC branch.
    await page.route("**/api/auth/mode", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        // The `oidc` block carries the issuer + client_id the SPA needs to
        // configure oidc-client-ts at runtime; fetchAuthMode() only calls
        // configureOidc() when it's present, so getUserManager() throws
        // without it.
        body: JSON.stringify({
          mode: "oidc",
          needs_bootstrap: false,
          oidc: { authority: AUTHORITY, client_id: CLIENT_ID },
        }),
      }),
    );

    // OIDC discovery + JWKS. The SDK fetches discovery to find the token
    // endpoint; JWKS is mocked for resilience even though signature
    // verification doesn't happen client-side in oidc-client-ts v3.
    await page.route(`${AUTHORITY}/.well-known/openid-configuration`, (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(DISCOVERY_DOC),
      }),
    );
    await page.route(`${AUTHORITY}/jwks`, (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ keys: [] }),
      }),
    );

    // Token exchange — the post-callback POST. Returns the synthetic JWT
    // unconditionally; mock validates only that we hit the right URL.
    await page.route(`${AUTHORITY}/token`, (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          access_token: "synthetic-access-token-per78",
          id_token: fakeIdToken(),
          token_type: "Bearer",
          expires_in: 3600,
          scope: SCOPE,
        }),
      }),
    );

    // After the redirect lands on /library, AuthGuard renders the page,
    // which fetches /api/me and the library-scoped queries. The e2e
    // backend would 401 the synthetic bearer (it's in local mode), so
    // mock the reads at the /api boundary too. A catch-all empty-200
    // covers everything else (assets, tags, projects, imports, libraries).
    await page.route("**/api/me", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: 1,
          email: "oidc-user@example.com",
          display_name: "OIDC Test User",
          role: "user",
          current_library: { id: 1, name: "OIDC Test Library", role: "admin" },
        }),
      }),
    );
    await page.route("**/api/libraries", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          libraries: [
            { library_id: 1, name: "OIDC Test Library", role: "admin", is_current: true },
          ],
        }),
      }),
    );
    await page.route(
      /\/api\/(assets|tags|projects|collections|imports|triage|previews|settings|audit|admin|files|thumbs)/,
      (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({}),
        }),
    );

    // Pre-seed localStorage with the SigninState the SDK looks up by
    // `oidc.${state}`. Has to run before any SPA script touches the page —
    // addInitScript guarantees that ordering on every navigation. The
    // try/catch swallows the "access is denied" SecurityError that fires
    // on Chromium's initial about:blank context (no storage there); the
    // real http://localhost run hits the localStorage write fine.
    await page.addInitScript(
      ({ key, value }) => {
        try {
          window.localStorage.setItem(key, value);
        } catch {
          /* about:blank — storage isn't available here */
        }
      },
      {
        key: `oidc.${STATE_ID}`,
        value: JSON.stringify(SIGNIN_STATE),
      },
    );

    await page.goto(`/auth/callback?code=stub-code-per78&state=${encodeURIComponent(STATE_ID)}`);

    // The callback handler navigates / → /library. Wait on the final URL,
    // not the intermediate /, so a regression that strands the SPA on /
    // (e.g. AuthGuard not seeing the freshly-stored user) fails clearly.
    await page.waitForURL("**/library");

    // The OIDC user persisted under the SDK's deterministic storage key.
    // Failure here would mean storeUser() on the UserManager was skipped.
    const storedUser = await page.evaluate(
      ({ authority, clientId }) =>
        window.localStorage.getItem(`oidc.user:${authority}:${clientId}`),
      { authority: AUTHORITY, clientId: CLIENT_ID },
    );
    expect(storedUser, "OIDC user should be persisted in localStorage").not.toBeNull();
    const parsed = JSON.parse(storedUser!);
    expect(parsed.access_token).toBe("synthetic-access-token-per78");
    expect(parsed.profile?.sub).toBe(SUBJECT);
  });

  test("with no email, prompts for a profile before entering the app", async ({
    page,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    // IdP authenticated by `sub` but the backend has no email for this user
    // yet → /api/me reports needs_profile. The PATCH response fills it in.
    await setupOidcRoutes(
      page,
      {
        id: 1,
        email: "",
        display_name: "",
        role: "user",
        needs_profile: true,
        current_library: { id: 1, name: "OIDC Test Library", role: "admin" },
      },
      {
        id: 1,
        email: "filled-in@example.com",
        display_name: "filled-in@example.com",
        role: "user",
        needs_profile: false,
        current_library: { id: 1, name: "OIDC Test Library", role: "admin" },
      },
      REQUESTED,
    );

    await page.goto(`/auth/callback?code=stub-code-per78&state=${encodeURIComponent(STATE_ID)}`);

    // Profile step gates the app — the routed library view is not rendered.
    await expect(
      page.getByRole("heading", { name: /finish setting up your profile/i }),
    ).toBeVisible();

    await page.locator('input[type="email"]').fill("filled-in@example.com");
    await page.getByRole("button", { name: "Continue" }).click();

    // PATCH flips needs_profile → false; the gate falls through to the page
    // that was asked for, which the callback had already navigated to.
    await expect(page).toHaveURL(onRequestedPage);
    await expect(
      page.getByRole("heading", { name: /finish setting up your profile/i }),
    ).toBeHidden();
  });

  test("signed out, opening a page sends it to the IdP in the sign-in state", async ({ page }) => {
    await setupOidcRoutes(page, ME);
    // Answer the IdP with 204, which leaves the app's document in place, so
    // the SigninState oidc-client-ts stored can still be read.
    const authorize = new Promise<string>((resolve) => {
      void page.route(`${AUTHORITY}/authorize**`, async (route) => {
        await route.fulfill({ status: 204 });
        resolve(new URL(route.request().url()).searchParams.get("state") ?? "");
      });
    });

    await page.goto(REQUESTED);
    const stateId = await authorize;

    const stored = await page.evaluate(
      (key) => window.localStorage.getItem(key),
      `oidc.${stateId}`,
    );
    expect(stored, "SigninState should be stored for the redirect").not.toBeNull();
    expect(JSON.parse(stored!).data).toEqual({ returnTo: REQUESTED });
  });

  test("returns to the page that was asked for, with its query and hash", async ({
    page,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    await setupOidcRoutes(page, ME, undefined, REQUESTED);

    await page.goto(`/auth/callback?code=stub-code-per78&state=${encodeURIComponent(STATE_ID)}`);

    await expect(page).toHaveURL(onRequestedPage);
  });

  test("ignores a requested page outside the app and opens /library", async ({
    page,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    await setupOidcRoutes(page, ME, undefined, "https://evil.example/");

    await page.goto(`/auth/callback?code=stub-code-per78&state=${encodeURIComponent(STATE_ID)}`);

    await page.waitForURL("**/library");
  });

  test("sign out clears the stored session and returns to /login", async ({
    page,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    await setupOidcRoutes(page, {
      id: 1,
      email: "oidc-user@example.com",
      display_name: "OIDC Test User",
      role: "user",
      needs_profile: false,
      current_library: { id: 1, name: "OIDC Test Library", role: "admin" },
    });

    await page.goto(`/auth/callback?code=stub-code-per78&state=${encodeURIComponent(STATE_ID)}`);
    await page.waitForURL("**/library");

    await page.getByRole("button", { name: "Account" }).click();
    await page.getByRole("menuitem", { name: "Sign out" }).click();

    // DISCOVERY_DOC advertises no end_session_endpoint, so logout() falls back
    // to clearing the stored user locally and hard-redirecting to /login.
    await page.waitForURL("**/login");
    const storedUser = await page.evaluate(
      ({ authority, clientId }) =>
        window.localStorage.getItem(`oidc.user:${authority}:${clientId}`),
      { authority: AUTHORITY, clientId: CLIENT_ID },
    );
    expect(storedUser, "OIDC user should be cleared on sign out").toBeNull();
  });
});
