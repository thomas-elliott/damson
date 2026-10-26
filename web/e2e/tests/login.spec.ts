import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import type { Page } from "@playwright/test";

import {
  test,
  expect,
  ADMIN_EMAIL,
  ADMIN_PASSWORD,
  bootstrapAdmin,
  uploadImport,
} from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

async function signIn(page: Page) {
  await page.getByLabel("Email").fill(ADMIN_EMAIL);
  await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: /sign in/i }).click();
}

// Tier-1 flow: local-mode login. Real backend, no mocks. Bootstraps an
// admin via API in beforeEach so the form has someone to log in as.

test.describe("Local login", () => {
  let session: Awaited<ReturnType<typeof bootstrapAdmin>>;
  test.beforeEach(async ({ request }) => {
    session = await bootstrapAdmin(request);
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

  test("signed out, a deep link survives sign-in with its query and hash", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const { asset_id } = await uploadImport(
      request,
      session,
      "tiny.unitypackage",
      await readFile(FIXTURE_PATH),
    );
    const requested = `/library/${asset_id}?q=tiny#files`;

    await page.goto(requested);
    await expect(page).toHaveURL(/\/login$/);
    await signIn(page);

    await expect(page).toHaveURL((url) => url.pathname + url.search + url.hash === requested);
  });

  test("signing in again after signing out opens the library", async ({ page }) => {
    await page.goto("/library");
    await expect(page).toHaveURL(/\/login$/);
    await signIn(page);
    await expect(page).toHaveURL(/\/library$/);

    await page.getByRole("button", { name: "Account" }).click();
    await page.getByRole("menuitem", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/login$/);

    await signIn(page);
    await expect(page).toHaveURL(/\/library$/);
  });
});
