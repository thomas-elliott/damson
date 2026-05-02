import { test, expect, ADMIN_EMAIL, ADMIN_PASSWORD, bootstrapAdmin } from "../fixtures";

// Tier-1 flow: first-run bootstrap. A freshly-reset backend reports
// needs_bootstrap=true; the SPA should route any unauthenticated visit
// through the bootstrap form, and submitting the form should land the
// user in /library with a working session.

test.describe("First-run bootstrap", () => {
  test("AuthGuard routes a fresh deployment to /bootstrap", async ({ page, noConsoleErrors }) => {
    await page.goto("/library");
    await expect(page).toHaveURL(/\/bootstrap$/);
    await expect(page.getByRole("heading", { name: /create the first admin/i })).toBeVisible();
  });

  // PER-68 fix: BootstrapPage.onSubmit no longer awaits invalidateQueries
  // (which previously hung under React 19 + RQ5 when the authMode query
  // had no active subscriber). It seeds the new mode synchronously via
  // queryClient.setQueryData and fires invalidate without awaiting. The
  // separate authMode singleton in lib/authMode.ts is gone — TanStack is
  // the only cache.
  test("submitting the bootstrap form lands the user in /library", async ({
    page,
    noConsoleErrors,
  }) => {
    await page.goto("/bootstrap");

    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD);
    await page.getByLabel(/confirm password/i).fill(ADMIN_PASSWORD);
    await page.getByRole("button", { name: /create account/i }).click();

    await expect(page).toHaveURL(/\/library$/);
  });

  test("once bootstrapped, second visit goes to /login not /bootstrap", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    // Seed an admin via API to flip needs_bootstrap=false, then visit as
    // an unauthenticated user — AuthGuard should bounce to /login.
    await bootstrapAdmin(request);

    await page.goto("/library");
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole("button", { name: /sign in/i })).toBeVisible();
  });
});
