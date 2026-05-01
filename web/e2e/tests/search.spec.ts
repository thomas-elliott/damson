import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import {
  test,
  expect,
  bootstrapAdmin,
  installSession,
  uploadImport,
} from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// PER-81 — search returns filtered results, exercising the Postgres
// `to_tsquery` branch in applyAssetFilters and the trigger-maintained
// `assets.search_vector` (gin-indexed). The SQLite fallback in the same
// file uses a LIKE on name/description; tier-1/2/3 SQLite specs cover
// that path. This spec only matters against a real Postgres backend, so
// it is gated on DAMSON_TEST_POSTGRES_DSN — without it the e2e binary
// boots SQLite and the assertion below would just retest LIKE.
test.describe("Library search (Postgres tsvector)", () => {
  test.skip(
    !process.env.DAMSON_TEST_POSTGRES_DSN,
    "Postgres-only — set DAMSON_TEST_POSTGRES_DSN to run",
  );

  test("typing in the search box narrows the asset grid", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    test.setTimeout(60_000);

    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    // Two assets with deliberately distinct prefixes so the simple-parser
    // tokenisation produces non-overlapping leading words. `alpha-pack`
    // tokenises into `alpha-pack`, `alpha`, `pack`; `beta-pack` similarly.
    // A query of "alpha" must match the first only.
    await uploadImport(request, session, "alpha-pack.unitypackage", fileBytes);
    await uploadImport(request, session, "beta-pack.unitypackage", fileBytes);

    await installSession(page, session);
    await page.goto("/library");

    const alpha = page.getByRole("button", { name: /\balpha-pack\b/i });
    const beta = page.getByRole("button", { name: /\bbeta-pack\b/i });
    await expect(alpha).toBeVisible();
    await expect(beta).toBeVisible();

    // Drive the search input. The library page writes into URL search
    // params; useAssets refetches when its query key includes the new q.
    // Pin the next /api/assets?q=alpha GET so the visibility assertion
    // doesn't race the network.
    const searchBox = page.getByLabel("Search assets");
    await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes("/api/assets") &&
          r.url().includes("q=alpha") &&
          r.request().method() === "GET",
      ),
      searchBox.fill("alpha"),
    ]);
    // The tsvector match-or-miss is the actual assertion — `alpha-pack`
    // tokenises into `alpha-pack` + `alpha` + `pack`, so `alpha:*` hits
    // it; `beta-pack` has no `alpha` token, so it must drop out.
    await expect(alpha).toBeVisible();
    await expect(beta).toHaveCount(0);
  });
});
