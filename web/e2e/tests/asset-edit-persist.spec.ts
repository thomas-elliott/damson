import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import { test, expect, bootstrapAdmin, installSession, uploadImport } from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

// Liberal MIT-shaped text — the licence detector keys off the "MIT
// License" header and the boilerplate phrase. Any failure to detect
// still saves with detected_type "custom" via use_detected, so the
// reload assertion only depends on persistence, not detection accuracy.
const LICENCE_TEXT = `MIT License

Copyright (c) 2026 Damson Test

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software.`;

const TAG_PATH = "qa/persist-test";

// PER-77 — most common write path post-import. Add a tag, save a licence,
// reload the page, confirm both stuck. Catches the optimistic-update
// masking-a-failed-backend-write class of bug: the assertions run after a
// real reload so the SPA has to refetch from the backend rather than
// reading its own in-memory cache.
test.describe("Asset edits persist across reload", () => {
  test("tag and licence survive a hard reload", async ({ page, request, noConsoleErrors }) => {
    void noConsoleErrors;
    // Five backend round-trips (import + tag + detect + licence + reload)
    // plus a Vite cold-start can blow past the default 30s budget on a
    // fresh worker; bump headroom rather than chase per-step waits.
    test.setTimeout(60_000);

    const session = await bootstrapAdmin(request);
    const fileBytes = await readFile(FIXTURE_PATH);
    await uploadImport(request, session, "persist-asset.unitypackage", fileBytes);

    await installSession(page, session);
    await page.goto("/library");

    // Open the detail panel via the asset card. URL becomes /library/:id;
    // panel mounts and useAssetDetail fetches.
    await page.getByRole("button", { name: /\bpersist asset\b/i }).click();
    await expect(page).toHaveURL(/\/library\/\d+$/);

    // Add a tag. The tag input commits on Enter (see AssetDetailPanel).
    const tagInput = page.getByLabel("New tag path");
    await tagInput.fill(TAG_PATH);
    await Promise.all([
      page.waitForResponse(
        (r) => /\/api\/assets\/\d+\/tags$/.test(r.url()) && r.request().method() === "POST",
      ),
      tagInput.press("Enter"),
    ]);
    await expect(page.getByLabel(`Remove ${TAG_PATH}`)).toBeVisible();

    // Save the licence — paste, detect, save-detected. The detect call is
    // a server round-trip but the save is what we care about; await both
    // so an unsaved licence can't sneak past the reload.
    await page.getByPlaceholder(/MIT License\.\.\./).fill(LICENCE_TEXT);
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().endsWith("/api/licences/detect") && r.request().method() === "POST",
      ),
      page.getByRole("button", { name: "Detect", exact: true }).click(),
    ]);
    await Promise.all([
      page.waitForResponse(
        (r) => /\/api\/assets\/\d+\/licence$/.test(r.url()) && r.request().method() === "PUT",
      ),
      page.getByRole("button", { name: "Save detected" }).click(),
    ]);

    // After save, LicenceEditor flips into the read-only "existing" view —
    // the textarea disappears, the LicencePill + "Original text"
    // disclosure render. That state must survive the reload below.
    await expect(page.getByPlaceholder(/MIT License\.\.\./)).toHaveCount(0);
    await expect(page.getByText("Original text")).toBeVisible();

    // The hard reload — this is the actual test. SPA loses every in-memory
    // cache; the assertions below are pure server-state reads.
    await page.reload();

    await expect(page.getByLabel(`Remove ${TAG_PATH}`)).toBeVisible();
    await expect(page.getByText("Original text")).toBeVisible();
    await expect(page.getByPlaceholder(/MIT License\.\.\./)).toHaveCount(0);
  });
});
