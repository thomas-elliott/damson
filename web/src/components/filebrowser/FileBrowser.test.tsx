import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/design-system/primitives/Tooltip";
import { FileBrowser } from "./FileBrowser";

const FILES = [
  { relative_path: "Assets/Meshes/Castle.fbx" },
  { relative_path: "Assets/Textures/Wall.png" },
  { relative_path: "Docs/Readme.txt" },
  { relative_path: "Licence.txt" },
  { relative_path: "Changelog.md" },
].map((f, i) => ({
  id: i + 1,
  relative_path: f.relative_path,
  size_bytes: 1024,
  object_key: `obj-${i + 1}`,
  detected_type: undefined,
  preview_object_key: undefined,
  preview_status: "unsupported",
}));

vi.mock("@/hooks/useDataQueries", () => ({
  useAssetDetail: () => ({
    data: {
      id: 7,
      name: "Castle Pack",
      current_version_id: 3,
      versions: [],
      files: FILES,
    },
    loading: false,
  }),
}));

vi.mock("@/hooks/useMutations", () => {
  const idle = () => ({ mutate: vi.fn(), isPending: false });
  return {
    useCreateSubsetVersion: idle,
    useReExtractVersion: idle,
    useRequeueAssetPreviews: idle,
    useRequeueFilePreview: idle,
  };
});

vi.mock("./useFileObjectUrl", () => ({
  useFileThumbUrl: () => undefined,
  downloadFile: vi.fn(),
}));

function mount() {
  return render(
    <TooltipProvider>
      <FileBrowser assetId={7} onExit={vi.fn()} />
    </TooltipProvider>,
  );
}

/**
 * Names of the folder/file cards in the main listing, in render order —
 * scoped away from the tree sidebar, which lists the same folder names.
 */
function listing(): string[] {
  const sidebar = screen.getByRole("complementary", { name: "Filters" });
  return screen
    .getAllByRole("button")
    .filter((el) => !sidebar.contains(el))
    .map((el) => el.querySelector("div.truncate.text-ui.font-medium")?.textContent ?? "")
    .filter(Boolean);
}

/** The listing card whose name matches exactly. */
function listingCard(name: string): HTMLElement {
  const sidebar = screen.getByRole("complementary", { name: "Filters" });
  const card = screen
    .getAllByRole("button")
    .filter((el) => !sidebar.contains(el))
    .find((el) => el.querySelector("div.truncate.text-ui.font-medium")?.textContent === name);
  if (!card) throw new Error(`no listing card named ${name}`);
  return card;
}

describe("FileBrowser root listing", () => {
  it("shows the root folder's own contents, not every file in the pack", () => {
    mount();
    const names = listing();

    expect(names).toContain("Assets");
    expect(names).toContain("Docs");
    expect(names).toContain("Licence.txt");
    expect(names).toContain("Changelog.md");

    expect(names).not.toContain("Castle.fbx");
    expect(names).not.toContain("Wall.png");
    expect(names).not.toContain("Readme.txt");
  });

  it("orders folders before files", () => {
    mount();
    const names = listing();
    const lastFolder = Math.max(names.indexOf("Assets"), names.indexOf("Docs"));
    const firstFile = Math.min(names.indexOf("Changelog.md"), names.indexOf("Licence.txt"));
    expect(lastFolder).toBeLessThan(firstFile);
  });

  it("descends into a folder and lists its children", async () => {
    mount();
    const card = listingCard("Assets");
    await userEvent.click(card);

    const names = listing();
    expect(names).toContain("Meshes");
    expect(names).toContain("Textures");
    expect(names).not.toContain("Castle.fbx");
    expect(names).not.toContain("Licence.txt");
  });

  it("falls back to a flat, path-labelled listing when searching", async () => {
    mount();
    await userEvent.type(screen.getByPlaceholderText("Search files in this pack…"), "castle");

    const names = listing();
    expect(names).toContain("Castle.fbx");
    expect(names).not.toContain("Assets");
  });
});
