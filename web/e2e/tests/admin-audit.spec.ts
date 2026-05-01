import {
  test,
  expect,
  bootstrapAdmin,
  createTestUser,
  loginViaApi,
  type Session,
} from "../fixtures";

const REGULAR_EMAIL = "regular@e2e.local";
const REGULAR_PASSWORD = "hunter2hunter2";

// PER-82 — sysadmin gating for /admin/audit. The page is defended in
// depth: the SPA bounces non-sysadmins (Navigate to="/audit" replace) and
// the API returns 403 (requireSysadmin in audit.go). The spec proves both
// the redirect and the rendered page in a single test by swapping local-
// auth tokens between two users on the same page.
//
// This test deliberately avoids `installSession` — that helper uses
// addInitScript, which re-runs on every navigation and would overwrite
// the second user's tokens. setLocalTokens uses page.evaluate, which
// persists in localStorage and survives subsequent gotos as long as we're
// on the same origin.
test.describe("Admin audit visibility (sysadmin gating)", () => {
  test("regular user is redirected; sysadmin sees the page", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    // bootstrapAdmin promotes the first user to sysadmin and creates the
    // first library (id=1). The second user, created via /api/test/users,
    // gets role=user and a member-tier membership in the same library.
    const sysadmin = await bootstrapAdmin(request);
    await createTestUser(request, {
      email: REGULAR_EMAIL,
      password: REGULAR_PASSWORD,
      role: "user",
      libraryId: 1,
      membershipRole: "member",
      setCurrent: true,
    });
    const regular = await loginViaApi(request, REGULAR_EMAIL, REGULAR_PASSWORD);

    // Land on a public route first so we have a same-origin context for
    // the localStorage write. /login renders without an authed session.
    await page.goto("/login");

    // Sign in as the regular user, then attempt the admin route.
    // <Navigate replace /> rewrites the URL to /audit synchronously inside
    // AdminAuditPage's role check.
    await setLocalTokens(page, regular);
    await page.goto("/admin/audit");
    await expect(page).toHaveURL(/\/audit(\?.*)?$/);

    // Swap to the sysadmin and re-navigate. The full goto re-mounts the
    // SPA, so /api/me runs against the new bearer token and AdminAuditPage
    // makes it past the gate.
    await setLocalTokens(page, sysadmin);
    await page.goto("/admin/audit");
    await expect(page).toHaveURL(/\/admin\/audit/);

    // The radiogroup is sysadmin-only: it lives inside the toolbar of
    // AdminAuditPage, which only renders past the role check.
    await expect(page.getByRole("radiogroup", { name: "Audit scope" })).toBeVisible();
    await expect(page.getByRole("radio", { name: "Server" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });
});

async function setLocalTokens(
  page: import("@playwright/test").Page,
  session: Session,
): Promise<void> {
  await page.evaluate(({ token, expiresAt }) => {
    window.localStorage.setItem("damson.local.token", token);
    window.localStorage.setItem("damson.local.expiresAt", expiresAt);
  }, session);
}
