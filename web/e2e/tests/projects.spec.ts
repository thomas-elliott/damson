import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import {
  test,
  expect,
  bootstrapAdmin,
  installSession,
  uploadImport,
  subsetVersion,
  getAssetDetail,
  listAuditEntries,
  createReferenceAsset,
} from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// Project pin/upgrade arithmetic. Two failure modes the spec
// guards against:
//   1. updates_available drifts — a pin pointing at v1 of an asset whose
//      current_version is now v2 must show as "1 update available". Lose
//      the SQL `<>` comparison and this counter goes to 0 silently.
//   2. project.pin.change audit row missing — the audit feed is the only
//      built-in trail for "who upgraded what to which version, when". A
//      same-version repin must NOT emit; a true upgrade must emit exactly
//      one row with the previous + new version_ids in the diff.
test.describe("Project pin + upgrade flow", () => {
  test("pinning v1, subsetting to v2, upgrading the pin emits audit", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    test.setTimeout(60_000);

    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    const upload = await uploadImport(request, session, "pin-target.unitypackage", fileBytes);

    // The subset endpoint takes file_ids belonging to the source version;
    // grab the first asset_file from the detail payload (the unitypackage
    // extracts to several asset_files on import).
    const detail = await getAssetDetail(request, session, upload.asset_id);
    const firstFileId = detail.files[0]?.id;
    expect(firstFileId, "expected at least one file on imported asset").toBeGreaterThan(0);

    await installSession(page, session);
    await page.goto("/projects");

    // Empty-state CTA opens the create dialog.
    await page.getByRole("button", { name: "New project" }).first().click();
    await page.getByPlaceholder("Ashfall").fill("Pinning project");
    await page.getByRole("combobox", { name: "Target distribution" }).click();
    await page.getByRole("option", { name: "Open source" }).click();
    await Promise.all([
      page.waitForURL(/\/projects\/\d+$/),
      page.getByRole("button", { name: "Create", exact: true }).click(),
    ]);
    const projectId = Number(new URL(page.url()).pathname.split("/").pop());
    expect(projectId).toBeGreaterThan(0);
    // The stored target distribution comes back and shows as its label.
    await expect(page.locator("header").getByText("Open source", { exact: true })).toBeVisible();

    // Pin the asset's current version (v1), which the dialog preselects.
    await page.getByRole("button", { name: "Pin asset" }).first().click();
    await page.getByRole("searchbox", { name: "Asset" }).fill("pin");
    await page.getByRole("button", { name: "pin target" }).click();
    await Promise.all([
      page.waitForResponse(
        (r) =>
          new RegExp(`/api/projects/${projectId}/pins$`).test(r.url()) &&
          r.request().method() === "POST",
      ),
      page.getByRole("button", { name: "Pin version" }).click(),
    ]);
    // Pin lands as "Up to date" — current_version still equals pinned_version.
    await expect(page.getByText("Up to date")).toBeVisible();

    // Create v2 via subset. The subset handler also flips current_version
    // to the new id, so updates_available should jump from 0 to 1. Version
    // labels are no longer auto-incremented (store_version is the identity),
    // so pass the "v2" label explicitly — the upgrade affordance renders it.
    const v2 = await subsetVersion(
      request,
      session,
      upload.asset_id,
      upload.asset_version_id,
      [firstFileId!],
      "v2",
    );
    expect(v2.version_label).toBe("v2");

    // Reload the project page so useProject refetches against the new
    // current_version. The "Upgrade to v2" affordance is rendered by
    // ProjectPinRow when pin.has_update is true.
    await page.reload();
    await expect(page.getByText(/1\s+updates available/)).toBeVisible();
    const upgradeButton = page.getByRole("button", { name: /Upgrade to v2/ });
    await expect(upgradeButton).toBeVisible();

    // Click upgrade, await the second POST /pins (re-post with version_id),
    // assert the row collapses back to "Up to date".
    await Promise.all([
      page.waitForResponse(
        (r) =>
          new RegExp(`/api/projects/${projectId}/pins$`).test(r.url()) &&
          r.request().method() === "POST",
      ),
      upgradeButton.click(),
    ]);
    await expect(page.getByText("Up to date")).toBeVisible();

    // Audit assertion. Library audit feed should now contain exactly one
    // project.pin.change row whose diff captures the version delta.
    const audit = await listAuditEntries(request, session, {
      action_type: "project.pin.change",
    });
    expect(audit, "exactly one project.pin.change row expected").toHaveLength(1);
    const diff = audit[0].diff as {
      after?: { asset_id: number; version_id: number };
      before?: { asset_id: number; version_id: number };
    };
    expect(diff.after?.version_id).toBe(v2.asset_version_id);
    expect(diff.before?.version_id).toBe(upload.asset_version_id);
  });
});

// The pin dialog searches the whole library, not its first page,
// lets the user pin an older version, and the pin table links and confirms.
test.describe("Pinning from the whole library", () => {
  test("finds an asset past the first page and pins a chosen version", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    test.setTimeout(60_000);

    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    // Imported first, so the newest-first list puts it behind the fillers.
    const upload = await uploadImport(request, session, "zeppelin-hangar.unitypackage", fileBytes);
    const detail = await getAssetDetail(request, session, upload.asset_id);
    await subsetVersion(
      request,
      session,
      upload.asset_id,
      upload.asset_version_id,
      [detail.files[0].id],
      "v2",
    );
    for (let i = 1; i <= 60; i++) {
      await createReferenceAsset(request, session, `Filler ${String(i).padStart(2, "0")}`);
    }

    await installSession(page, session);
    await page.goto("/projects");
    await page.getByRole("button", { name: "New project" }).first().click();
    await page.getByPlaceholder("Ashfall").fill("Whole library");
    await Promise.all([
      page.waitForURL(/\/projects\/\d+$/),
      page.getByRole("button", { name: "Create", exact: true }).click(),
    ]);

    await page.getByRole("button", { name: "Pin asset" }).first().click();
    const dialog = page.getByRole("dialog");
    // A broad search shows a page and asks for a narrower one.
    await expect(dialog.getByText(/Showing the first 25 of 61 matches/)).toBeVisible();
    await expect(dialog.getByRole("button", { name: "zeppelin hangar" })).toHaveCount(0);

    await dialog.getByRole("searchbox", { name: "Asset" }).fill("zepp");
    await dialog.getByRole("button", { name: "zeppelin hangar" }).click();
    // The current version, v2, is preselected; pick the older one.
    await expect(dialog.getByRole("combobox", { name: "Version" })).toContainText("v2 (current)");
    await dialog.getByRole("combobox", { name: "Version" }).click();
    await page.getByRole("option").filter({ hasNotText: "v2" }).click();
    await dialog.getByRole("button", { name: "Pin version" }).click();

    const row = page.getByRole("row", { name: /zeppelin hangar/ });
    await expect(row.getByText("Upgrade to v2")).toBeVisible();

    // Unpin asks first; cancelling keeps the pin.
    await row.getByRole("button", { name: "Unpin" }).click();
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(row).toBeVisible();

    // The asset's name opens it in the library.
    await row.getByRole("link", { name: "zeppelin hangar" }).click();
    await expect(page).toHaveURL(new RegExp(`/library/${upload.asset_id}$`));
  });

  test("finds an older asset by its exact name and changes its pinned version", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    test.setTimeout(60_000);

    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    // Imported first, behind 30 newer assets whose names start with its own.
    const upload = await uploadImport(request, session, "rock.unitypackage", fileBytes);
    const detail = await getAssetDetail(request, session, upload.asset_id);
    await subsetVersion(
      request,
      session,
      upload.asset_id,
      upload.asset_version_id,
      [detail.files[0].id],
      "v2",
    );
    for (let i = 1; i <= 30; i++) {
      await createReferenceAsset(request, session, `Rock Pack ${String(i).padStart(2, "0")}`);
    }

    await installSession(page, session);
    await page.goto("/projects");
    await page.getByRole("button", { name: "New project" }).first().click();
    await page.getByPlaceholder("Ashfall").fill("Quarry");
    await Promise.all([
      page.waitForURL(/\/projects\/\d+$/),
      page.getByRole("button", { name: "Create", exact: true }).click(),
    ]);

    await page.getByRole("button", { name: "Pin asset" }).first().click();
    let dialog = page.getByRole("dialog");
    await dialog.getByRole("searchbox", { name: "Asset" }).fill("rock");
    await expect(dialog.getByText(/Showing the first 25 of 31 matches/)).toBeVisible();
    await dialog.getByRole("button", { name: "rock", exact: true }).click();
    await dialog.getByRole("button", { name: "Pin version" }).click();
    const row = page.getByRole("row", { name: /^rock/ });
    await expect(row.getByText("Up to date")).toBeVisible();

    // Opened again, the picker marks it as pinned and offers a change.
    await page.getByRole("button", { name: "Pin asset" }).first().click();
    dialog = page.getByRole("dialog");
    await dialog.getByRole("searchbox", { name: "Asset" }).fill("rock");
    await dialog.getByRole("button", { name: "rock · Pinned · v2", exact: true }).click();
    await expect(dialog.getByText("rock is pinned at v2")).toBeVisible();
    const change = dialog.getByRole("button", { name: "Change pinned version" });
    await expect(change).toBeDisabled();
    await dialog.getByRole("combobox", { name: "Version" }).click();
    await page.getByRole("option").filter({ hasNotText: "v2" }).click();
    await change.click();
    await expect(row.getByText("Upgrade to v2")).toBeVisible();

    // A downgrade is audited under the neutral name.
    const audit = await listAuditEntries(request, session, { action_type: "project.pin.change" });
    expect(audit).toHaveLength(1);
  });
});
