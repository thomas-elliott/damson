import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { TagIndexEntryView } from "@/hooks/useDataQueries";

import { TagIndexPage } from "./TagIndexPage";

const index = vi.hoisted(() => ({
  data: [] as TagIndexEntryView[],
  loading: false,
  failed: false,
  reload: vi.fn(),
}));

vi.mock("@/hooks/useDataQueries", () => ({ useTagIndex: () => index }));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));
vi.mock("@/components/filebrowser/useFileObjectUrl", () => ({ useBlobUrl: () => undefined }));

const entry = (path: string, count: number): TagIndexEntryView => ({
  path,
  count,
  top_children: [{ name: "humanoid", count: 3 }],
  covers: [{ id: 1, name: "POLYGON_Knights", publisher: "Synty" }],
});

beforeEach(() => {
  index.data = [
    entry("favourites", 2),
    entry("audio/sound-fx", 4),
    entry("3d/characters", 12),
    entry("3d/props", 7),
  ];
  index.loading = false;
  index.failed = false;
});

function Location() {
  const { pathname, search } = useLocation();
  return <output aria-label="location">{pathname + search}</output>;
}

function mount() {
  return render(
    <MemoryRouter initialEntries={["/tags"]}>
      <Routes>
        <Route path="/tags" element={<TagIndexPage />} />
        <Route path="/library" element={<Location />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("TagIndexPage", () => {
  it("groups the cards under their top-level tag, the library's own tags last", () => {
    mount();
    expect(screen.getByRole("heading", { level: 1, name: "Tag index" })).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent)).toEqual([
      "3d",
      "audio",
      "Other tags",
    ]);
    const threeD = screen.getByRole("region", { name: "3d" });
    const cards = within(threeD).getAllByRole("button");
    expect(cards).toHaveLength(2);
    expect(cards[0]).toHaveAccessibleName("3d / characters, 12 assets");
    expect(cards[1]).toHaveAccessibleName("3d / props, 7 assets");
    // A cover's folder-style name reads with spaces.
    expect(cards[0]).toHaveTextContent("POLYGON Knights");
  });

  it("opens the library filtered to a card's tag", async () => {
    mount();
    await userEvent.click(screen.getByRole("button", { name: /characters/ }));
    expect(screen.getByLabelText("location")).toHaveTextContent("/library?tag=3d%2Fcharacters");
  });

  it("says how tags arrive when there are none", () => {
    index.data = [];
    mount();
    expect(screen.getByText("No tags yet")).toBeInTheDocument();
  });

  it("offers a Retry when the index fails to load", async () => {
    index.data = [];
    index.failed = true;
    mount();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(index.reload).toHaveBeenCalled();
  });
});
