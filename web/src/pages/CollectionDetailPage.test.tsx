import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

const collection = vi.hoisted(() => ({
  found: true,
  settingsLoading: false,
  total: 0,
  pages: [] as ({ limit: number; offset: number } | undefined)[],
}));

vi.mock("@/hooks/useDataQueries", () => ({
  useMe: () => ({ data: { id: 1, current_library: { id: 1, name: "Main" } } }),
  useCollection: () =>
    collection.found
      ? { data: { id: 4, name: "Props" }, loading: false, failed: false }
      : { data: null, loading: false, failed: false },
  useLibrarySettings: () => ({ data: null, loading: collection.settingsLoading }),
  useAssets: (_filters: unknown, _enabled: boolean, page?: { limit: number; offset: number }) => {
    collection.pages.push(page);
    return {
      data: collection.total > 0 ? [{ id: 1, name: "Barrel" }] : [],
      total: collection.total,
      loading: false,
      placeholder: false,
      error: null,
    };
  },
}));
vi.mock("@/hooks/useMutations", () => ({ useDeleteCollection: () => ({ mutate: vi.fn() }) }));
vi.mock("@/components/AssetGrid", () => ({ AssetGrid: () => <div>grid</div> }));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));

import { CollectionDetailPage } from "./CollectionDetailPage";

const lastPage = () => collection.pages[collection.pages.length - 1];

function mount(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/collections/:id" element={<CollectionDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  collection.found = true;
  collection.settingsLoading = false;
  collection.total = 0;
  collection.pages = [];
});

describe("CollectionDetailPage paging", () => {
  it("fetches the page in the URL and pages across the collection", () => {
    collection.total = 120;
    mount("/collections/4?page=2&page_size=25");
    expect(lastPage()).toEqual({ limit: 25, offset: 25 });
    expect(screen.getByRole("navigation", { name: "Collection pages" })).toHaveTextContent(
      "Page 2 of 5",
    );
  });

  it("shows the empty state for a collection with no assets", () => {
    mount("/collections/4");
    expect(screen.getByText("No assets in this collection")).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Collection pages" })).toBeNull();
  });
});

// Only the grid needs the library's density, so only the grid waits for it.
describe("CollectionDetailPage while the settings load", () => {
  beforeEach(() => {
    collection.settingsLoading = true;
  });

  it("shows a missing collection", () => {
    collection.found = false;
    mount("/collections/999");
    expect(screen.getByText("Collection not found")).toBeInTheDocument();
  });

  it("shows an empty collection", () => {
    mount("/collections/4");
    expect(screen.getByText("No assets in this collection")).toBeInTheDocument();
  });

  it("holds the grid back", () => {
    collection.total = 3;
    mount("/collections/4");
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("grid")).toBeNull();
  });
});
