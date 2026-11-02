import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import {
  test,
  expect,
  bootstrapAdmin,
  createReferenceAsset,
  installSession,
  uploadImport,
} from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// J3, look at an asset and get to it: the quick look leads to the package
// page, whose files open past the pack's lone root folder, and whose next and
// previous walk the library results it was opened from.
test.describe("Package page", () => {
  test("opens from the quick look onto the pack's files and goes back", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    // tiny.unitypackage holds one file, Assets/E2EFixture.txt.
    const { asset_id } = await uploadImport(
      request,
      session,
      "tiny.unitypackage",
      await readFile(FIXTURE_PATH),
    );

    await installSession(page, session);
    await page.goto("/library?q=tiny");
    await page.getByRole("button", { name: /\btiny\b/i }).click();

    const panel = page.getByRole("complementary", { name: "tiny" });
    await expect(panel.getByRole("button", { name: "Download original" })).toBeVisible();
    await panel.getByRole("button", { name: "Open package page" }).click();

    await expect(page).toHaveURL(new RegExp(`/library/${asset_id}/package\\?q=tiny$`));
    await expect(page.getByRole("heading", { level: 1, name: "tiny" })).toBeVisible();
    // The pack wraps everything in Assets/, so the files open inside it.
    const files = page.getByRole("region", { name: "Files" });
    await expect(
      files.getByRole("toolbar").getByRole("button", { name: "Assets" }),
    ).toHaveAttribute("aria-current", "page");
    await expect(files.getByText("E2EFixture.txt")).toBeVisible();
    // The only asset in the results has nowhere to step to.
    await expect(page.getByText("1 of 1")).toBeVisible();
    await expect(page.getByRole("button", { name: "Next package" })).toBeDisabled();

    // Back returns to the results with the quick look still open.
    await page.getByRole("button", { name: "Back to library" }).click();
    await expect(page).toHaveURL(new RegExp(`/library/${asset_id}\\?q=tiny$`));
    await expect(panel).toBeVisible();
  });

  test("steps through the library results across a page boundary", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    test.setTimeout(60_000);

    // One more than the smallest page size, so `ref 26` is alone on page 2.
    const session = await bootstrapAdmin(request);
    const ids: number[] = [];
    for (let i = 1; i <= 26; i++) {
      const { asset_id } = await createReferenceAsset(
        request,
        session,
        `ref ${String(i).padStart(2, "0")}`,
      );
      ids.push(asset_id);
    }

    await installSession(page, session);
    await page.goto(`/library/${ids[24]}/package?sort=name_asc&page_size=25`);
    const heading = page.getByRole("heading", { level: 1 });
    await expect(heading).toHaveText("ref 25");
    await expect(page.getByText("25 of 26")).toBeVisible();
    // A reference asset's page leads with where it is.
    await expect(page.getByRole("button", { name: "Copy path" })).toBeVisible();

    await page.getByRole("button", { name: "Next package" }).click();
    await expect(heading).toHaveText("ref 26");
    await expect(page).toHaveURL(
      new RegExp(`/library/${ids[25]}/package\\?sort=name_asc&page_size=25&page=2$`),
    );
    await expect(page.getByText("26 of 26")).toBeVisible();
    await expect(page.getByRole("button", { name: "Next package" })).toBeDisabled();

    // The arrow keys step too, back across the boundary.
    await page.keyboard.press("ArrowLeft");
    await expect(heading).toHaveText("ref 25");
    await expect(page).toHaveURL(
      new RegExp(`/library/${ids[24]}/package\\?sort=name_asc&page_size=25$`),
    );
    await expect(page.getByText("25 of 26")).toBeVisible();
  });

  test("fits a 400px screen", async ({ page, request, noConsoleErrors }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    const { asset_id } = await uploadImport(
      request,
      session,
      "tiny.unitypackage",
      await readFile(FIXTURE_PATH),
    );

    await page.setViewportSize({ width: 400, height: 800 });
    await installSession(page, session);
    await page.goto(`/library/${asset_id}/package`);

    // The columns stack: the files follow the summary, at the screen's width.
    const files = page.getByRole("region", { name: "Files" });
    const file = files.getByText("E2EFixture.txt");
    await file.scrollIntoViewIfNeeded();
    await expect(file).toBeInViewport();
    const box = await files.boundingBox();
    expect(box && box.x + box.width).toBeLessThanOrEqual(400);

    // The folder tree is a drawer behind a toggle.
    const search = page.getByPlaceholder("Search files in this pack…");
    await expect(search).toBeHidden();
    await files.getByRole("button", { name: "Folders and search" }).click();
    await expect(search).toBeVisible();
    // It covers its own toggle, so Escape closes it.
    await page.keyboard.press("Escape");
    await expect(search).toBeHidden();
  });
});
