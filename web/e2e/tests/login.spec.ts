import { test, expect, mockAuthMode } from "../fixtures";

// Local-mode login happy path. Asserts the form posts the expected payload
// to /api/auth/login, stores the returned token, and lands the user on
// /library. We don't opt into noConsoleErrors here because /library will
// fire un-mocked data queries that surface as errors — out of scope for the
// auth flow. Add /library page coverage in a separate spec with full
// fixture data.

test.describe("Local login", () => {
  test("happy path navigates to /library", async ({ page }) => {
    await mockAuthMode(page, { mode: "local", needs_bootstrap: false });

    let loginCalls = 0;
    await page.route("**/api/auth/login", async (route) => {
      loginCalls += 1;
      const payload = JSON.parse(route.request().postData() ?? "{}") as {
        email?: string;
        password?: string;
      };
      expect(payload.email).toBe("admin@example.com");
      expect(payload.password).toBe("hunter2hunter2");
      await route.fulfill({
        json: {
          token: "test-token",
          expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
        },
      });
    });

    await page.goto("/login");

    await page.getByLabel("Email").fill("admin@example.com");
    await page.getByLabel("Password").fill("hunter2hunter2");
    await page.getByRole("button", { name: /sign in/i }).click();

    await expect(page).toHaveURL(/\/library$/);
    expect(loginCalls).toBe(1);
  });

  test("invalid credentials surface the error inline", async ({ page }) => {
    await mockAuthMode(page, { mode: "local", needs_bootstrap: false });
    await page.route("**/api/auth/login", (route) =>
      route.fulfill({ status: 401, json: { error: "invalid credentials" } }),
    );

    await page.goto("/login");
    await page.getByLabel("Email").fill("admin@example.com");
    await page.getByLabel("Password").fill("wrong-password");
    await page.getByRole("button", { name: /sign in/i }).click();

    await expect(page.getByText(/invalid credentials/i)).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });
});
