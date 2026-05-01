import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import {
  test,
  expect,
  bootstrapAdmin,
  installSession,
  uploadImport,
  addAssetToCollection,
  removeAssetFromCollection,
} from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// PER-80 — collections CRUD + membership rendering. The product surface
// today only exposes a UI for collection create + delete + detail view; the
// add/remove side runs through the API (the SPA hooks exist but no page
// wires them yet). The flow this spec protects: create a collection
// through the UI, hydrate it via the API, navigate to the detail page and
// verify the asset grid reflects the live membership state. This catches
// stale collection_id query-key bugs in useAssets.
test.describe("Collection create + asset add/remove", () => {
  test("detail page reflects membership after add and remove", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    // Two backend round-trips per import + a Vite cold-start can be tight
    // on the default 30s budget when paired with a UI flow.
    test.setTimeout(60_000);

    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    const a = await uploadImport(request, session, "asset-a.unitypackage", fileBytes);
    const b = await uploadImport(request, session, "asset-b.unitypackage", fileBytes);

    await installSession(page, session);
    await page.goto("/collections");

    // Empty-state CTA opens the dialog. The "New collection" button only
    // exists in the toolbar variant after at least one collection lives;
    // start from empty so we click the empty-state primary instead.
    await page.getByRole("button", { name: "New collection" }).first().click();
    await page.getByPlaceholder("Boss fight audio").fill("Audio sweep");

    // Submit + wait for navigation to /collections/:id.
    await Promise.all([
      page.waitForURL(/\/collections\/\d+$/),
      page.getByRole("button", { name: "Create", exact: true }).click(),
    ]);
    const url = new URL(page.url());
    const collectionId = Number(url.pathname.split("/").pop());
    expect(collectionId).toBeGreaterThan(0);

    // Empty collection — detail page shows the "no assets" state.
    await expect(
      page.getByRole("heading", { name: "No assets in this collection" }),
    ).toBeVisible();

    // Hydrate via API: add A and B. Detail panel UI for collections doesn't
    // exist yet (per CollectionDetailPage's "Add assets from their detail
    // panels" copy — that affordance is planned but not built); the spec
    // exercises the membership-rendering path the SPA already has.
    await addAssetToCollection(request, session, collectionId, a.asset_id);
    await addAssetToCollection(request, session, collectionId, b.asset_id);

    // Reload so useAssets refetches with the new membership. /collections/:id
    // doesn't re-poll on its own, and the test backend has no realtime push.
    await page.reload();
    await expect(page.getByRole("button", { name: /\basset-a\b/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /\basset-b\b/i })).toBeVisible();

    // Remove A and confirm only B remains. This is the assertion that
    // exercises the cache key the issue calls out — a stale key would keep
    // showing A even after the membership row is gone server-side.
    await removeAssetFromCollection(request, session, collectionId, a.asset_id);
    await page.reload();
    await expect(page.getByRole("button", { name: /\basset-a\b/i })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /\basset-b\b/i })).toBeVisible();
  });
});
