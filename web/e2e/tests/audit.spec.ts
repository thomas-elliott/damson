import type { APIRequestContext } from "@playwright/test";

import {
  test,
  expect,
  bootstrapAdmin,
  createTestUser,
  installSession,
  loginViaApi,
  type Session,
} from "../fixtures";

const LATE_EMAIL = "late@e2e.local";
const LATE_PASSWORD = "hunter2hunter2";

async function api(
  request: APIRequestContext,
  session: Session,
  method: "post" | "patch" | "delete",
  url: string,
  data?: unknown,
): Promise<void> {
  const res = await request[method](url, {
    headers: { Authorization: `Bearer ${session.token}` },
    data,
  });
  if (!res.ok()) throw new Error(`${method} ${url}: ${res.status()} ${await res.text()}`);
}

// Seeds more entries than one 25-entry page, so the oldest sit on page 2.
test.describe("Audit log paging", () => {
  test("pages the library audit log and searches every entry", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    test.setTimeout(120_000);

    // The oldest entry is a tag by a second user; 26 tags by the admin push
    // it onto page 2.
    const admin = await bootstrapAdmin(request);
    await createTestUser(request, {
      email: LATE_EMAIL,
      password: LATE_PASSWORD,
      libraryId: 1,
      membershipRole: "admin",
      setCurrent: true,
    });
    const late = await loginViaApi(request, LATE_EMAIL, LATE_PASSWORD);
    await api(request, late, "post", "/api/tags", { path: "late-tag" });
    for (let i = 1; i <= 26; i++) {
      await api(request, admin, "post", "/api/tags", { path: `tag-${i}` });
    }

    await installSession(page, admin);
    await page.goto("/audit?page_size=25");

    const pager = page.getByRole("navigation", { name: "Audit log pages" });
    const lateRow = page.getByText(LATE_EMAIL);

    await expect(page.getByText("27", { exact: true })).toBeVisible();
    await expect(pager).toContainText("Page 1 of 2");
    await expect(lateRow).toHaveCount(0);

    await pager.getByRole("button", { name: "Next page" }).click();
    await expect(page).toHaveURL(/[?&]page=2\b/);
    await expect(lateRow).toBeVisible();
    // Labels, not raw action types.
    await expect(page.getByText("created tag").first()).toBeVisible();
    await expect(page.getByText(/\btag\.create\b/)).toHaveCount(0);

    // Reload and history restore the same page and size.
    await page.reload();
    await expect(lateRow).toBeVisible();
    await expect(page).toHaveURL(/[?&]page_size=25\b/);
    await page.goBack();
    await expect(page).not.toHaveURL(/[?&]page=/);
    await expect(lateRow).toHaveCount(0);
    await page.goForward();
    await expect(lateRow).toBeVisible();

    // The search reaches an entry that is only on page 2, from page 1.
    await page.getByLabel("Filter audit log by user").fill("late@");
    await expect(page).not.toHaveURL(/[?&]page=/);
    await expect(lateRow).toBeVisible();
    await expect(pager).toContainText("Page 1 of 1");
  });

  test("pages the admin audit log in both scopes", async ({ page, request, noConsoleErrors }) => {
    void noConsoleErrors;
    test.setTimeout(120_000);

    // The oldest server-level entry is a role change; 26 settings updates
    // follow it. Under All libraries a tag in the first library is older
    // still.
    const admin = await bootstrapAdmin(request);
    await api(request, admin, "post", "/api/tags", { path: "first-tag" });
    const user = await createTestUser(request, { email: LATE_EMAIL, password: LATE_PASSWORD });
    await api(request, admin, "patch", `/api/admin/users/${user.id}`, { role: "sysadmin" });
    for (let i = 0; i < 26; i++) {
      await api(request, admin, "patch", "/api/admin/settings", {});
    }

    await installSession(page, admin);
    await page.goto("/admin/audit?page_size=25");
    const pager = page.getByRole("navigation", { name: "Audit log pages" });

    await expect(pager).toContainText("Page 1 of 2");
    await pager.getByRole("button", { name: "Page 2" }).click();
    await expect(page.getByText("updated user")).toBeVisible();
    await expect(page.getByText("tag first-tag")).toHaveCount(0);

    // Changing the scope starts again from page 1.
    await page.getByRole("radio", { name: "All libraries" }).click();
    await expect(page).not.toHaveURL(/[?&]page=/);
    await expect(page).toHaveURL(/[?&]scope=all\b/);
    await expect(pager).toContainText("Page 1 of 2");
    await expect(page.getByText("tag first-tag")).toHaveCount(0);

    await pager.getByRole("button", { name: "Next page" }).click();
    await expect(page.getByText("tag first-tag")).toBeVisible();
    await expect(page.getByText("updated user")).toBeVisible();
  });
});
