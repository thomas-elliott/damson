import { test, expect, bootstrapAdmin, installSession } from "../fixtures";

// PER-83 — library settings persistence. The class of bug the issue calls
// out is "PATCH succeeded but the GET cache wasn't invalidated": the SPA
// optimistically updates state, the user moves on, but a hard reload
// rehydrates from the cache (or a stale GET response) and the change
// vanishes. The hard reload at the end is the actual test — every
// assertion after it is a pure server-state read.
test.describe("Library settings persist across reload", () => {
  test("toggling the grid density default sticks after a hard reload", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    await installSession(page, session);
    await page.goto("/settings");

    const densityMedium = page.getByRole("radio", { name: "Medium" });
    const densityLarge = page.getByRole("radio", { name: "Large" });

    // Default is "medium" — bootstrap creates a row of defaults via
    // loadOrCreateLibrarySettings on first GET.
    await expect(densityMedium).toHaveAttribute("aria-checked", "true");
    await expect(densityLarge).toHaveAttribute("aria-checked", "false");

    // Drive the change via UI; pin to the PATCH so the reload below can't
    // race the network. The autosave fires from onValueChange.
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().endsWith("/api/settings") && r.request().method() === "PATCH",
      ),
      densityLarge.click(),
    ]);
    await expect(densityLarge).toHaveAttribute("aria-checked", "true");

    // Hard reload — SPA loses every in-memory cache; the assertion below
    // is a pure server-state read via the refetch on mount.
    await page.reload();

    await expect(page.getByRole("radio", { name: "Large" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    await expect(page.getByRole("radio", { name: "Medium" })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });
});
