import path from "node:path";
import { fileURLToPath } from "node:url";

import { test, expect, bootstrapAdmin, installSession } from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// Tier-1 flow: end-to-end .unitypackage import via the UI dropzone. This
// covers the full pipeline — multipart upload → storage layer → Unity
// extractor → DB writes → asset list invalidation — none of which can be
// verified in unit tests on their own. The DropZone lives on /library
// (see web/src/components/DropZone.tsx); the toolbar Upload button calls
// open() from context, but we drive the hidden file input directly so
// the test isn't sensitive to dropzone-trigger refactors.

test.describe("Asset import", () => {
  test("dropping a .unitypackage uploads, extracts, and surfaces in the library", async ({
    page,
    request,
  }) => {
    const session = await bootstrapAdmin(request);
    await installSession(page, session);

    await page.goto("/library");

    // Wait for the import POST so we know the backend finished extraction
    // (postImport in internal/handlers/api/assets.go runs Unity extraction
    // synchronously inside the request).
    const importResponse = page.waitForResponse(
      (res) =>
        /\/api\/imports$/.test(new URL(res.url()).pathname) &&
        res.request().method() === "POST",
    );

    await page.locator('input[type="file"]').setInputFiles(FIXTURE_PATH);

    const response = await importResponse;
    expect(response.status()).toBe(201);

    // The asset list invalidates on upload completion (see
    // web/src/components/DropZone.tsx onDrop). Wait for the card to
    // appear; Playwright's locator auto-retries until the timeout.
    // Substring match — accessible name includes the meta line below
    // the asset name.
    await expect(page.getByRole("button", { name: /\btiny\b/i })).toBeVisible();
  });
});
