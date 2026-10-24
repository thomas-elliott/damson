import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import { test, expect, bootstrapAdmin, installSession, uploadImport } from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// Tier-1 flow: post-login library page renders with content. Seeds an
// asset via the import API in beforeEach (the same wire format the UI
// uses, just bypassing the dropzone) so the library page has something
// to render.

test.describe("Library page", () => {
  test("renders a seeded asset and opens its detail panel", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    await uploadImport(
      request,
      session,
      "tiny.unitypackage",
      fileBytes,
      "application/octet-stream",
    );

    await installSession(page, session);
    await page.goto("/library");

    // Asset name comes from the upload filename minus extension; see
    // assetNameFromFilename in internal/handlers/api/assets.go. Match by
    // substring because the card's accessible name includes the meta
    // line under the name (file count, size).
    const card = page.getByRole("button", { name: /\btiny\b/i });
    await expect(card).toBeVisible();

    await card.click();

    // Detail panel mounts at /library/:assetId. Don't assert a specific id
    // (it's a fresh autoincrement); a numeric trailing segment is enough.
    await expect(page).toHaveURL(/\/library\/\d+$/);
  });

  test("pages through the library and keeps the page in the URL", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    test.setTimeout(120_000);

    // One more asset than the smallest page size, so 25 a page makes two
    // pages. The import turns `pack-01.unitypackage` into the asset `pack 01`;
    // sorting by name puts pack 01-25 on page 1 and pack 26 alone on page 2.
    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    for (let i = 1; i <= 26; i++) {
      const n = String(i).padStart(2, "0");
      await uploadImport(request, session, `pack-${n}.unitypackage`, fileBytes);
    }

    await installSession(page, session);
    await page.goto("/library?sort=name_asc&page_size=25");

    const pager = page.getByRole("navigation", { name: "Library pages" });
    const first = page.getByRole("button", { name: /\bpack 01\b/i });
    const last = page.getByRole("button", { name: /\bpack 26\b/i });

    await expect(page.getByText("26 assets")).toBeVisible();
    await expect(pager).toContainText("Page 1 of 2");
    await expect(pager.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await expect(first).toBeVisible();
    await expect(last).toHaveCount(0);

    await pager.getByRole("button", { name: "Next page" }).click();
    await expect(page).toHaveURL(/[?&]page=2\b/);
    await expect(last).toBeVisible();
    await expect(first).toHaveCount(0);
    await expect(pager.getByRole("button", { name: "Page 2" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    await expect(pager.getByRole("button", { name: "Next page" })).toBeDisabled();
    // The count is the library's, not the page's.
    await expect(page.getByText("26 assets")).toBeVisible();

    // Reload and history restore the same page.
    await page.reload();
    await expect(last).toBeVisible();
    await page.goBack();
    await expect(page).not.toHaveURL(/[?&]page=/);
    await expect(first).toBeVisible();
    await page.goForward();
    await expect(last).toBeVisible();

    // A bigger page size starts again from page 1, now the only page.
    await pager.getByRole("combobox", { name: "Page size" }).click();
    await page.getByRole("option", { name: "50 per page" }).click();
    await expect(page).not.toHaveURL(/[?&]page=/);
    await expect(pager).toContainText("Page 1 of 1");
    await expect(first).toBeVisible();

    // A page past the end lands on the last page.
    await page.goto("/library?sort=name_asc&page_size=25&page=9");
    await expect(page).toHaveURL(/[?&]page=2\b/);
    await expect(last).toBeVisible();

    // A new search starts again from page 1.
    await page.getByLabel("Search assets").fill("pack");
    await expect(page).not.toHaveURL(/[?&]page=/);
    await expect(pager).toContainText("Page 1 of 2");
  });
});
