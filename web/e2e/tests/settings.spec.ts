import {
  test,
  expect,
  bootstrapAdmin,
  installSession,
} from "../fixtures";

// PER-83 — library settings persistence. The class of bug the issue calls
// out is "PATCH succeeded but the GET cache wasn't invalidated": the SPA
// optimistically updates state, the user moves on, but a hard reload
// rehydrates from the cache (or a stale GET response) and the change
// vanishes. The hard reload at the end is the actual test — every
// assertion after it is a pure server-state read.
test.describe("Library settings persist across reload", () => {
  test("toggling the theme default sticks after a hard reload", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    const session = await bootstrapAdmin(request);
    await installSession(page, session);
    await page.goto("/settings");

    const themeDark = page.getByRole("radio", { name: "Dark" });
    const themeLight = page.getByRole("radio", { name: "Light" });

    // Default is "dark" — bootstrap creates a row of defaults via
    // loadOrCreateLibrarySettings on first GET.
    await expect(themeDark).toHaveAttribute("aria-checked", "true");
    await expect(themeLight).toHaveAttribute("aria-checked", "false");

    // Drive the change via UI; pin to the PATCH so the reload below can't
    // race the network. The autosave fires from onValueChange.
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().endsWith("/api/settings") && r.request().method() === "PATCH",
      ),
      themeLight.click(),
    ]);
    await expect(themeLight).toHaveAttribute("aria-checked", "true");

    // Hard reload — SPA loses every in-memory cache; the assertion below
    // is a pure server-state read via the refetch on mount.
    await page.reload();

    await expect(page.getByRole("radio", { name: "Light" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    await expect(page.getByRole("radio", { name: "Dark" })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });
});
