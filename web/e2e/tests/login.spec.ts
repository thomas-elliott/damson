import { test, expect, ADMIN_EMAIL, ADMIN_PASSWORD, bootstrapAdmin } from "../fixtures";

// Tier-1 flow: local-mode login. Real backend, no mocks. Bootstraps an
// admin via API in beforeEach so the form has someone to log in as.

test.describe("Local login", () => {
  test.beforeEach(async ({ request }) => {
    await bootstrapAdmin(request);
  });

  test("happy path navigates to /library", async ({ page, noConsoleErrors }) => {
    void noConsoleErrors;
    await page.goto("/login");

    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD);
    await page.getByRole("button", { name: /sign in/i }).click();

    await expect(page).toHaveURL(/\/library$/);
  });

  test("invalid credentials surface the inline error", async ({ page }) => {
    await page.goto("/login");

    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password", { exact: true }).fill("wrong-password");
    await page.getByRole("button", { name: /sign in/i }).click();

    await expect(page.getByText(/invalid credentials/i)).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });
});
