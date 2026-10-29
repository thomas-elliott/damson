import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LibraryPage } from "./LibraryPage";

const assets = Array.from({ length: 12 }, (_, i) => ({ id: i + 1, name: `Asset ${i + 1}` }));

// The library's total is set per test; the loaded page is always `assets`.
const library = vi.hoisted(() => ({
  total: 12,
  failed: false,
  // The first page is still in flight.
  loading: false,
  // While a new search loads, the hook keeps answering with the old results.
  placeholder: false,
  pages: [] as ({ limit: number; offset: number } | undefined)[],
}));
// The library settings as useLibrarySettings returns them, and every density
// the grid was drawn with.
const settings = vi.hoisted(() => ({
  data: null as { grid_density_default: string } | null,
  loading: false,
  densities: [] as (string | undefined)[],
}));

vi.mock("@/hooks/useDataQueries", () => ({
  useMe: () => ({ data: { id: 1, current_library: { id: 1, name: "Main" } } }),
  useLibrarySettings: () => ({ data: settings.data, loading: settings.loading }),
  useAssets: (_filters: unknown, _enabled: boolean, page?: { limit: number; offset: number }) => {
    library.pages.push(page);
    if (library.failed) {
      const error = new Error("network");
      return { data: [], total: 0, loading: false, placeholder: false, error, failed: true };
    }
    if (library.loading) {
      return { data: [], total: 0, loading: true, placeholder: false };
    }
    const data = library.total === 0 ? [] : assets;
    return { data, total: library.total, loading: false, placeholder: library.placeholder };
  },
}));
// The grid is what knows how many columns it shows; this one reports three.
vi.mock("@/components/AssetGrid", () => ({
  AssetGrid: ({
    onColumnsChange,
    onToggleSelect,
    density,
  }: {
    onColumnsChange?: (n: number) => void;
    onToggleSelect?: (id: number) => void;
    density?: string;
  }) => {
    settings.densities.push(density);
    onColumnsChange?.(3);
    return (
      <div>
        grid
        <button type="button" onClick={() => onToggleSelect?.(1)}>
          Select asset 1
        </button>
      </div>
    );
  },
}));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));
vi.mock("@/components/DropZone", () => ({
  DropZone: ({ children }: { children: ReactNode }) => <>{children}</>,
  useDropZoneOpen: () => () => {},
}));
vi.mock("@/components/detail/AssetDetailPanel", () => ({
  AssetDetailPanel: ({ assetId }: { assetId: number | null }) =>
    assetId == null ? null : <aside aria-label="Details">panel</aside>,
}));
vi.mock("@/components/LibraryFilterSidebar", () => ({ LibraryFilterSidebar: () => null }));
vi.mock("@/components/LibraryPicker", () => ({ LibraryPicker: () => null }));
vi.mock("@/components/AddByLocationDialog", () => ({ AddByLocationButton: () => null }));
vi.mock("@/components/BulkAssetBar", () => ({
  BulkAssetBar: () => <div role="status">bulk bar</div>,
}));
vi.mock("@/components/AssetSortMenu", () => ({
  AssetSortMenu: () => null,
  DEFAULT_ASSET_SORT: "newest",
}));

function Location() {
  const { pathname, search } = useLocation();
  return (
    <>
      <output aria-label="location">{pathname}</output>
      <output aria-label="query">{search}</output>
    </>
  );
}

beforeEach(() => {
  library.total = 12;
  library.failed = false;
  library.loading = false;
  library.placeholder = false;
  library.pages = [];
  settings.data = { grid_density_default: "medium" };
  settings.loading = false;
  settings.densities = [];
});

const lastPage = () => library.pages[library.pages.length - 1];
const query = () => screen.getByLabelText("query").textContent;
const pager = () => screen.queryByRole("navigation", { name: "Library pages" });
const typeSearch = (text: string) =>
  userEvent.type(screen.getByRole("searchbox", { name: "Search assets" }), text);

function page(path: string) {
  return (
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/library/:assetId?" element={<LibraryPage />} />
      </Routes>
      <Location />
    </MemoryRouter>
  );
}

function mount(path: string) {
  return render(page(path));
}

// Arrow Up and Down move one visible row: as many assets as the grid shows
// columns, whatever the window or panel widths.
describe("LibraryPage arrow keys", () => {
  it("Arrow Down moves by the grid's column count", async () => {
    mount("/library/2");
    await userEvent.keyboard("{ArrowDown}");
    expect(screen.getByLabelText("location")).toHaveTextContent("/library/5");
  });

  it("Arrow Up moves back by the grid's column count", async () => {
    mount("/library/8");
    await userEvent.keyboard("{ArrowUp}");
    expect(screen.getByLabelText("location")).toHaveTextContent("/library/5");
  });

  it("Arrow Down stops at the last asset", async () => {
    mount("/library/11");
    await userEvent.keyboard("{ArrowDown}");
    expect(screen.getByLabelText("location")).toHaveTextContent("/library/12");
  });
});

describe("LibraryPage paging", () => {
  it("asks for the first page at the default size", () => {
    library.total = 120;
    mount("/library");
    expect(lastPage()).toEqual({ limit: 50, offset: 0 });
    expect(screen.getByRole("navigation", { name: "Library pages" })).toHaveTextContent(
      "Page 1 of 3",
    );
    expect(screen.getByRole("button", { name: "Page 1" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    // The count is the library's, not the page's.
    expect(screen.getByText("120 assets")).toBeInTheDocument();
  });

  it("writes the page to the URL and fetches it", async () => {
    library.total = 120;
    mount("/library?q=castle");
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(query()).toBe("?q=castle&page=2");
    expect(lastPage()).toEqual({ limit: 50, offset: 50 });
    expect(screen.getByRole("button", { name: "Page 2" })).toHaveAttribute("aria-current", "page");

    await userEvent.click(screen.getByRole("button", { name: "Page 3" }));
    expect(lastPage()).toEqual({ limit: 50, offset: 100 });
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
  });

  it("reads the page and page size from the URL", () => {
    library.total = 120;
    mount("/library?page=3&page_size=25");
    expect(lastPage()).toEqual({ limit: 25, offset: 50 });
    expect(screen.getByRole("navigation", { name: "Library pages" })).toHaveTextContent(
      "Page 3 of 5",
    );
  });

  it("falls back to the defaults for invalid values", () => {
    library.total = 120;
    mount("/library?page=abc&page_size=7");
    expect(lastPage()).toEqual({ limit: 50, offset: 0 });
  });

  it("moves a page past the end to the last page", () => {
    library.total = 120;
    mount("/library?page=9");
    expect(query()).toBe("?page=3");
    expect(lastPage()).toEqual({ limit: 50, offset: 100 });
  });

  it("goes back to the first page when the search changes", async () => {
    library.total = 120;
    mount("/library?page=2");
    await userEvent.type(screen.getByRole("searchbox", { name: "Search assets" }), "x");
    expect(query()).toBe("?q=x");
    expect(lastPage()).toEqual({ limit: 50, offset: 0 });
  });

  it("hides the pager for an empty library", () => {
    library.total = 0;
    mount("/library");
    expect(screen.queryByRole("navigation", { name: "Library pages" })).toBeNull();
  });
});

// The count is the first number on the screen, so it never shows a total the
// library doesn't have.
describe("LibraryPage heading", () => {
  it("names the page with an h1", () => {
    mount("/library");
    expect(screen.getByRole("heading", { level: 1, name: "Library" })).toBeInTheDocument();
  });
});

describe("LibraryPage asset count", () => {
  it("shows an ellipsis, not 0, while the first page loads", () => {
    library.loading = true;
    mount("/library");
    expect(screen.getByText("… assets")).toBeInTheDocument();
    expect(screen.queryByText("0 assets")).toBeNull();
  });

  it("shows the total once the page settles", () => {
    library.total = 120;
    mount("/library");
    expect(screen.getByText("120 assets")).toBeInTheDocument();
  });

  it("keeps the last total when a page change fails", async () => {
    library.total = 120;
    mount("/library");
    library.failed = true;
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(screen.getByText("120 assets")).toBeInTheDocument();
  });
});

// The selection bar is absolutely positioned, so it must share a positioned
// box with the grid and not the pager, or it floats over the pager.
describe("LibraryPage selection bar", () => {
  it("sits in the grid's box, apart from the pager", async () => {
    library.total = 120;
    mount("/library");
    await userEvent.click(screen.getByRole("button", { name: "Select asset 1" }));
    const box = screen.getByText("bulk bar").parentElement!;
    expect(box).toContainElement(screen.getByText("grid"));
    expect(box).not.toContainElement(pager());
  });
});

// The detail panel overlays the grid, never the toolbar, so Upload and Add
// by location stay reachable with an asset open.
describe("LibraryPage detail panel", () => {
  it("sits in the grid's box, apart from the toolbar", () => {
    mount("/library/3");
    const box = screen.getByRole("complementary", { name: "Details" }).parentElement!;
    expect(box).toContainElement(screen.getByText("grid"));
    expect(box).not.toContainElement(screen.getByRole("toolbar"));
  });
});

describe("LibraryPage load failure", () => {
  it("shows the error with a Retry and keeps the requested page", () => {
    library.failed = true;
    mount("/library?page=3");
    expect(screen.getByText("Couldn't load assets")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.queryByText("grid")).toBeNull();
    expect(query()).toBe("?page=3");
  });

  it("keeps the pager below the error when a page change fails", async () => {
    library.total = 120;
    mount("/library");
    library.failed = true;
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(screen.getByText("Couldn't load assets")).toBeInTheDocument();
    expect(query()).toBe("?page=2");
    expect(screen.getByRole("button", { name: "Page 2" })).toHaveAttribute("aria-current", "page");

    library.failed = false;
    await userEvent.click(screen.getByRole("button", { name: "Page 3" }));
    expect(query()).toBe("?page=3");
    expect(lastPage()).toEqual({ limit: 50, offset: 100 });
    expect(screen.getByText("grid")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load assets")).toBeNull();
  });

  // The remembered total belongs to the search it was loaded for.
  it("hides the pager when a changed search fails", async () => {
    library.total = 120;
    mount("/library");
    expect(pager()).toHaveTextContent("Page 1 of 3");
    library.failed = true;
    await typeSearch("x");
    expect(screen.getByText("Couldn't load assets")).toBeInTheDocument();
    expect(pager()).toBeNull();
  });

  it("does not remember the old search's total while the new one loads", async () => {
    library.total = 120;
    const { rerender } = mount("/library");
    library.placeholder = true;
    await typeSearch("x");
    expect(query()).toBe("?q=x");
    expect(pager()).toHaveTextContent("Page 1 of 3");

    library.placeholder = false;
    library.failed = true;
    rerender(page("/library"));
    expect(screen.getByText("Couldn't load assets")).toBeInTheDocument();
    expect(pager()).toBeNull();
  });
});

describe("LibraryPage density", () => {
  it.each(["small", "large"])("draws the grid at the library's %s density", (density) => {
    settings.data = { grid_density_default: density };
    mount("/library");
    expect(settings.densities).not.toHaveLength(0);
    expect(settings.densities.every((d) => d === density)).toBe(true);
  });

  it("waits for the settings before drawing the grid", () => {
    settings.data = null;
    settings.loading = true;
    const { rerender } = mount("/library");
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("grid")).toBeNull();

    settings.data = { grid_density_default: "small" };
    settings.loading = false;
    rerender(page("/library"));
    expect(screen.getByText("grid")).toBeInTheDocument();
    expect(settings.densities.every((d) => d === "small")).toBe(true);
  });

  it("shows a failed load without waiting for the settings", () => {
    settings.loading = true;
    library.failed = true;
    mount("/library");
    expect(screen.getByText("Couldn't load assets")).toBeInTheDocument();
  });

  it("shows an empty library without waiting for the settings", () => {
    settings.loading = true;
    library.total = 0;
    mount("/library");
    expect(screen.queryByText("Loading…")).toBeNull();
    expect(screen.getByText("grid")).toBeInTheDocument();
  });

  it("draws the grid at the default density when the settings failed", () => {
    settings.data = null;
    settings.loading = false;
    mount("/library");
    expect(screen.getByText("grid")).toBeInTheDocument();
    expect(settings.densities.every((d) => d === undefined)).toBe(true);
  });
});
