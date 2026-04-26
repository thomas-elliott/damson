import { test, expect, mockAuthMode } from "../fixtures";

// Smoke tests: the SPA mounts, the router resolves, and AuthGuard dispatches
// to the correct unauthenticated landing page based on /api/auth/mode. These
// run with the `noConsoleErrors` fixture so a transform error or render
// crash from a dependency upgrade fails the test loudly.

test.describe("App boot", () => {
  test("redirects an unauthenticated user to /login", async ({
    page,
    noConsoleErrors,
  }) => {
    await mockAuthMode(page, { mode: "local", needs_bootstrap: false });

    await page.goto("/library");

    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole("heading", { name: /damson/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /sign in/i })).toBeVisible();
  });

  test("routes a fresh local-mode deployment to /bootstrap", async ({
    page,
    noConsoleErrors,
  }) => {
    await mockAuthMode(page, { mode: "local", needs_bootstrap: true });

    await page.goto("/library");

    await expect(page).toHaveURL(/\/bootstrap$/);
    await expect(
      page.getByRole("heading", { name: /create the first admin/i }),
    ).toBeVisible();
  });
});
