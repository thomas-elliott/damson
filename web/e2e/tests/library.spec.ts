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
});
