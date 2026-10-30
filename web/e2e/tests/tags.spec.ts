import type { APIRequestContext, Page } from "@playwright/test";

import {
  test,
  expect,
  bootstrapAdmin,
  createReferenceAsset,
  installSession,
  tagAsset,
} from "../fixtures";

type Session = Awaited<ReturnType<typeof bootstrapAdmin>>;

// Four assets under store-style tags. `Sword Pack` sits under a sibling whose
// name starts like `3d/characters`, so a prefix match that forgets the slash
// would let it in.
const ASSETS: [string, string][] = [
  ["Knight Pack", "3d/characters/humanoid"],
  ["Slime Pack", "3d/characters/creatures"],
  ["Oak Pack", "3d/vegetation/trees"],
  ["Sword Pack", "3d/charactersets"],
];

async function seed(request: APIRequestContext): Promise<Session> {
  const session = await bootstrapAdmin(request);
  for (const [name, path] of ASSETS) {
    const { asset_id } = await createReferenceAsset(request, session, name);
    await tagAsset(request, session, asset_id, path);
  }
  return session;
}

const card = (page: Page, name: string) =>
  page.getByRole("button", { name: new RegExp(`^${name}`) });

async function expectShown(page: Page, shown: string[]) {
  for (const [name] of ASSETS) {
    if (shown.includes(name)) await expect(card(page, name)).toBeVisible();
    else await expect(card(page, name)).toHaveCount(0);
  }
}

// Ticks two tags as fast as a person double-clicking down a column: two
// separate clicks, no wait between them for the URL to catch up.
async function tickTwo(page: Page, first: string, second: string) {
  const picker = page.getByRole("dialog", { name: "Tags" });
  const a = await picker
    .getByRole("checkbox", { name: `Select ${first}`, exact: true })
    .boundingBox();
  const b = await picker
    .getByRole("checkbox", { name: `Select ${second}`, exact: true })
    .boundingBox();
  if (!a || !b) throw new Error("tick boxes not on screen");
  await page.mouse.click(a.x + a.width / 2, a.y + a.height / 2);
  await page.mouse.click(b.x + b.width / 2, b.y + b.height / 2);
}

test.describe("Tags", () => {
  test("ticked tags in the picker filter the library to any of them", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await seed(request);
    await installSession(page, session);
    await page.goto("/library");
    await expectShown(page, ["Knight Pack", "Slime Pack", "Oak Pack", "Sword Pack"]);

    const toolbar = page.getByRole("toolbar");
    await toolbar.getByRole("button", { name: /^Tags/ }).click();
    const picker = page.getByRole("dialog", { name: "Tags" });
    // The first level is too broad to tick; open it to reach the second.
    await picker
      .getByRole("list", { name: "Top-level tags" })
      .getByRole("button", { name: /^3d/ })
      .click();
    await expect(
      picker
        .getByRole("list", { name: "Tags in 3d" })
        .getByRole("button", { name: /^characters\s*2/ }),
    ).toBeVisible();

    await Promise.all([
      page.waitForResponse(
        (r) => /tag_path=3d%2Fcharacters&tag_path=3d%2Fvegetation/.test(r.url()) && r.ok(),
      ),
      tickTwo(page, "3d/characters", "3d/vegetation"),
    ]);
    await picker.getByRole("button", { name: "Show results" }).click();
    await expect(picker).toBeHidden();

    await expectShown(page, ["Knight Pack", "Slime Pack", "Oak Pack"]);
    await expect(toolbar.getByRole("button", { name: /^Tags/ })).toHaveText(/2/);
    const chips = page.getByRole("list", { name: "Active filters" });
    await expect(chips.getByRole("listitem")).toHaveText([
      "Tag3d / characters",
      "Tag3d / vegetation",
    ]);

    // The ticked tags live in the URL, so a reload keeps them.
    await page.reload();
    await expectShown(page, ["Knight Pack", "Slime Pack", "Oak Pack"]);

    await page.getByRole("button", { name: "Remove tag filter: 3d / vegetation" }).click();
    await expectShown(page, ["Knight Pack", "Slime Pack"]);

    await page.getByRole("button", { name: "Clear all" }).click();
    await expectShown(page, ["Knight Pack", "Slime Pack", "Oak Pack", "Sword Pack"]);
    await expect(chips).toHaveCount(0);
  });

  test("a tag index card opens the library filtered to its tag", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    const session = await seed(request);
    await installSession(page, session);
    await page.goto("/library");
    await page.getByRole("link", { name: "Tag index" }).click();

    await expect(page.getByRole("heading", { level: 1, name: "Tag index" })).toBeVisible();
    const threeD = page.getByRole("region", { name: "3d" });
    await expect(threeD.getByRole("button")).toHaveText([
      /characters\s*2\s*creatures 1 · humanoid 1/,
      /charactersets\s*1/,
      /vegetation\s*1\s*trees 1/,
    ]);

    await threeD.getByRole("button", { name: "3d / characters, 2 assets" }).click();
    await expect(page).toHaveURL(/\/library\?tag=3d%2Fcharacters$/);
    await expectShown(page, ["Knight Pack", "Slime Pack"]);
  });

  test("the picker fits a 400px screen and steps back up a level", async ({
    page,
    request,
    noConsoleErrors,
  }) => {
    void noConsoleErrors;
    await page.setViewportSize({ width: 400, height: 800 });
    const session = await seed(request);
    await installSession(page, session);
    await page.goto("/library?tag=3d%2Fcharacters%2Fhumanoid");
    await expectShown(page, ["Knight Pack"]);

    await page.keyboard.press("t");
    const picker = page.getByRole("dialog", { name: "Tags" });
    const box = await picker.boundingBox();
    expect(box && box.x >= 0 && box.x + box.width <= 400).toBe(true);

    // Two columns: the ticked tag's and its parent's; the top level is off screen.
    await expect(
      picker.getByRole("checkbox", { name: "Select 3d/characters/humanoid" }),
    ).toBeChecked();
    await expect(picker.getByRole("list", { name: "Top-level tags" })).toBeHidden();
    await picker.getByRole("button", { name: /Up a level from\s*characters/ }).click();
    await expect(picker.getByRole("list", { name: "Top-level tags" })).toBeVisible();

    await picker.getByRole("checkbox", { name: "Select 3d/vegetation" }).click();
    await picker.getByRole("button", { name: "Show results" }).click();
    await expectShown(page, ["Knight Pack", "Oak Pack"]);
  });
});
