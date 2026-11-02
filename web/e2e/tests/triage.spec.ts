import type { APIRequestContext } from "@playwright/test";

import { test, expect, bootstrapAdmin, installSession, type Session } from "../fixtures";

// recordForReview records a reference asset flagged needs review: one that
// never went through an import, which the queue used to miss.
async function recordForReview(
  request: APIRequestContext,
  session: Session,
  name: string,
): Promise<number> {
  const res = await request.post("/api/assets/references", {
    headers: { Authorization: `Bearer ${session.token}` },
    data: { name, location_path: `/library/${name}`, needs_review: true },
  });
  if (!res.ok()) throw new Error(`record reference: ${res.status()} ${await res.text()}`);
  return ((await res.json()) as { asset_id: number }).asset_id;
}

// J4, triage: every asset marked needs review is in the queue, and marking it
// reviewed there moves it to Recently reviewed.
test.describe("Triage", () => {
  test("lists a needs-review reference and marks it reviewed", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    await recordForReview(request, session, "recorded-pack");

    await installSession(page, session);
    await page.goto("/queue");
    await expect(page.getByRole("heading", { level: 1, name: "Triage" })).toBeVisible();
    await expect(page.getByText("recorded-pack")).toBeVisible();
    await expect(page.getByText("Needs review", { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "Mark reviewed" }).click();
    await expect(page.getByRole("heading", { name: "Recently reviewed" })).toBeVisible();
    await expect(page.getByText("Nothing waiting for review.")).toBeVisible();
    await expect(page.getByText("Reviewed", { exact: true })).toBeVisible();
  });

  test("fits a 400px screen", async ({ page, request, noConsoleErrors }) => {
    void noConsoleErrors;
    const session = await bootstrapAdmin(request);
    await recordForReview(request, session, "a-pack-with-a-long-enough-name-to-wrap");

    await installSession(page, session);
    await page.setViewportSize({ width: 400, height: 800 });
    await page.goto("/queue");
    await expect(page.getByText("a-pack-with-a-long-enough-name-to-wrap")).toBeVisible();

    // Every action is whole and on screen, not cut off at the edge.
    for (const name of ["Open", "Find listing", "Mark reviewed"]) {
      const box = await page.getByRole("button", { name, exact: true }).boundingBox();
      expect(box, name).not.toBeNull();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(400);
    }
  });
});
