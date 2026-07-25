import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { buildFileTree, type ManifestEntry } from "./fileTree";
import { FileTreeSidebar } from "./FileTreeSidebar";

const MANIFEST: ManifestEntry[] = [
  { relative_path: "Assets/Prefabs/Buildings/Small/Interior/Chair.prefab", size_bytes: 1 },
  { relative_path: "Assets/Prefabs/Buildings/Small/Hut.prefab", size_bytes: 1 },
  { relative_path: "Assets/Prefabs/Castle.prefab", size_bytes: 1 },
  { relative_path: "Assets/Textures/Stone.png", size_bytes: 1 },
  { relative_path: "Docs/Readme.md", size_bytes: 1 },
  { relative_path: "Licence.txt", size_bytes: 1 },
];

const TREE = buildFileTree(MANIFEST);

function view(path: string[], onNavigate = vi.fn()) {
  return (
    <FileTreeSidebar
      assetName="Castle Pack"
      query=""
      onQueryChange={vi.fn()}
      tree={TREE}
      totalFiles={5}
      path={path}
      onNavigate={onNavigate}
      typeCounts={{}}
      activeTypes={new Set()}
      onToggleType={vi.fn()}
    />
  );
}

function mount(path: string[] = [], onNavigate = vi.fn()) {
  return { ...render(view(path, onNavigate)), onNavigate };
}

/** The row button whose label matches exactly, or undefined when not rendered. */
function row(name: string): HTMLElement | undefined {
  return screen.queryAllByRole("button").find((el) => el.textContent?.startsWith(name));
}

/** The caret sibling of a folder row. */
function caret(name: string): HTMLElement {
  const label = row(name);
  if (!label) throw new Error(`no row named ${name}`);
  return within(label.parentElement as HTMLElement).getByRole("button", {
    name: /^(Expand|Collapse) folder$/,
  });
}

describe("FileTreeSidebar", () => {
  it("lists folders only — files belong to the grid", () => {
    mount();
    expect(row("Assets")).toBeDefined();
    expect(row("Textures")).toBeDefined();
    expect(row("Docs")).toBeDefined();

    expect(row("Licence.txt")).toBeUndefined();
    expect(row("Castle.prefab")).toBeUndefined();
    expect(row("Readme.md")).toBeUndefined();
  });

  it("titles the section Folders and keeps the All files root row", () => {
    mount();
    expect(screen.getByText("Folders")).toBeInTheDocument();
    expect(row("All files")).toBeDefined();
  });

  it("expands to depth 3 on first render, leaving depth-3 folders shut", () => {
    mount();
    // Assets (0) → Prefabs (1) → Buildings (2) are open, so Buildings' child
    // Small (3) is visible; Small itself is shut, so Interior (4) is not.
    expect(row("Prefabs")).toBeDefined();
    expect(row("Buildings")).toBeDefined();
    expect(row("Small")).toBeDefined();
    expect(row("Interior")).toBeUndefined();
    expect(caret("Small")).toHaveAttribute("aria-expanded", "false");
    expect(caret("Buildings")).toHaveAttribute("aria-expanded", "true");
  });

  it("navigates on the label without collapsing the folder", async () => {
    const { onNavigate } = mount();
    await userEvent.click(row("Textures")!);
    expect(onNavigate).toHaveBeenCalledWith(["Assets", "Textures"]);
    expect(row("Textures")).toBeDefined();
  });

  it("collapses a subtree from the caret without navigating", async () => {
    const { onNavigate } = mount();
    await userEvent.click(caret("Prefabs"));
    expect(onNavigate).not.toHaveBeenCalled();
    expect(row("Buildings")).toBeUndefined();
    expect(caret("Prefabs")).toHaveAttribute("aria-expanded", "false");
  });

  it("lets a folder on the current path stay collapsed", async () => {
    const { rerender } = mount(["Assets", "Prefabs"]);
    await userEvent.click(caret("Prefabs"));
    expect(row("Buildings")).toBeUndefined();

    // A re-render at the same path must not resurrect the subtree — the old
    // render-time ancestor union did exactly that.
    rerender(view(["Assets", "Prefabs"]));
    expect(row("Buildings")).toBeUndefined();
    expect(caret("Prefabs")).toHaveAttribute("aria-expanded", "false");
  });

  it("reveals the path again when navigation moves there", async () => {
    const { rerender } = mount([]);
    await userEvent.click(caret("Assets"));
    expect(row("Prefabs")).toBeUndefined();

    rerender(view(["Assets", "Prefabs"]));
    expect(row("Prefabs")).toBeDefined();
    expect(caret("Assets")).toHaveAttribute("aria-expanded", "true");
  });

  it("gives a folder with no subfolders no caret", () => {
    mount();
    const docs = row("Docs")!;
    expect(
      within(docs.parentElement as HTMLElement).queryByRole("button", {
        name: /folder$/,
      }),
    ).toBeNull();
  });
});
