import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import {
  test,
  expect,
  bootstrapAdmin,
  createLibrary,
  installSession,
  switchCurrentLibrary,
  uploadImport,
} from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// PER-76 — multi-tenancy correctness. The asset list is server-scoped to
// the user's current_library_id; a stale react-query cache key here would
// silently leak one library's data into another's view. Two libraries
// with disjoint assets, drive the switch through the picker, assert the
// asset grid changes and the previous scope's asset is gone.
test.describe("Library switch isolates scope", () => {
  test("switching libraries via the picker swaps the asset grid", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);

    // Create A, switch into A, drop an asset only A can see.
    const libA = await createLibrary(request, session, "Library A");
    await switchCurrentLibrary(request, session, libA.id);
    await uploadImport(request, session, "asset-a.unitypackage", fileBytes);

    // Same for B.
    const libB = await createLibrary(request, session, "Library B");
    await switchCurrentLibrary(request, session, libB.id);
    await uploadImport(request, session, "asset-b.unitypackage", fileBytes);

    // Land the SPA in A so the first render is "A's view, B's asset must
    // be invisible". The picker drives every later switch.
    await switchCurrentLibrary(request, session, libA.id);

    await installSession(page, session);
    await page.goto("/library");

    const assetA = page.getByRole("button", { name: /\basset a\b/i });
    const assetB = page.getByRole("button", { name: /\basset b\b/i });
    await expect(assetA).toBeVisible();
    await expect(assetB).toHaveCount(0);

    // Switch to B via the picker. waitForResponse pins to the next
    // /api/assets refetch so the visibility assertions don't race the
    // cache invalidation.
    await page.getByRole("button", { name: "Switch library" }).click();
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes("/api/assets") && r.request().method() === "GET",
      ),
      page.getByRole("menuitem", { name: "Switch to Library B" }).click(),
    ]);
    await expect(assetB).toBeVisible();
    await expect(assetA).toHaveCount(0);

    // And back to A — the ticket explicitly calls out the round-trip so a
    // one-way invalidation bug doesn't slip through.
    await page.getByRole("button", { name: "Switch library" }).click();
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes("/api/assets") && r.request().method() === "GET",
      ),
      page.getByRole("menuitem", { name: "Switch to Library A" }).click(),
    ]);
    await expect(assetA).toBeVisible();
    await expect(assetB).toHaveCount(0);
  });
});
