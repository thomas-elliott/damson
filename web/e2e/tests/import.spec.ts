import path from "node:path";
import { fileURLToPath } from "node:url";

import { test, expect, bootstrapAdmin, installSession } from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// Tier-1 flow: end-to-end .unitypackage import via the UI dropzone. The
// upload goes multipart → storage → DB stub rows → background worker
// (extract + thumbnail). POST /api/imports returns 202 once the stub is
// persisted; the placeholder asset is then visible in the library while
// the worker fills in asset_files and the thumbnail asynchronously.

test.describe("Asset import", () => {
  test("dropping a .unitypackage enqueues an import and surfaces in the library", async ({
    page,
    request,
  }) => {
    const session = await bootstrapAdmin(request);
    await installSession(page, session);

    await page.goto("/library");

    const importResponse = page.waitForResponse(
      (res) =>
        /\/api\/imports$/.test(new URL(res.url()).pathname) &&
        res.request().method() === "POST",
    );

    await page.locator('input[type="file"]').setInputFiles(FIXTURE_PATH);

    const response = await importResponse;
    expect(response.status()).toBe(202);

    // DropZone invalidates the assets query on upload completion. The
    // placeholder asset shows up immediately because the stub row was
    // written in the handler transaction.
    await expect(page.getByRole("button", { name: /\btiny\b/i })).toBeVisible();
  });
});
