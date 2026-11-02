import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

import { test, expect, bootstrapAdmin, installSession, uploadImport } from "../fixtures";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_PATH = path.resolve(dirname, "../fixtures/tiny.unitypackage");

const MEMBER_EMAIL = "member@e2e.local";
const MEMBER_PASSWORD = "member-pass-1234";

// A team shares one library. An invited member has never chosen a library, so
// signing in has to land them in the team's library with nothing else to do.
test.describe("Inviting a team member", () => {
  test("an invited member signs in and sees the team's library", async ({
    page,
    browser,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    await uploadImport(request, session, "team-asset.unitypackage", await readFile(FIXTURE_PATH));

    // The admin invites the member from Settings.
    await installSession(page, session);
    await page.goto("/settings");
    await page.getByRole("button", { name: "Invite" }).click();
    const dialog = page.getByRole("dialog", { name: "Invite a member" });
    await dialog.getByLabel("Email").fill(MEMBER_EMAIL);
    await dialog.getByLabel("Display name").fill("Mira");
    await dialog.getByLabel("Initial password").fill(MEMBER_PASSWORD);
    await dialog.getByRole("button", { name: "Invite" }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Mira", { exact: true })).toBeVisible();

    // The member signs in on their own browser and lands in the library.
    const memberContext = await browser.newContext();
    try {
      const member = await memberContext.newPage();
      await member.goto("/login");
      await member.getByLabel("Email").fill(MEMBER_EMAIL);
      await member.getByLabel("Password", { exact: true }).fill(MEMBER_PASSWORD);
      await member.getByRole("button", { name: /sign in/i }).click();
      await expect(member).toHaveURL(/\/library$/);
      await expect(member.getByRole("heading", { level: 1, name: "Personal" })).toBeVisible();
      await expect(member.getByRole("button", { name: /\bteam asset\b/i })).toBeVisible();
      await expect(member.getByText("No library access yet")).toHaveCount(0);
    } finally {
      await memberContext.close();
    }
  });
});
