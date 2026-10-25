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
  pages: [] as ({ limit: number; offset: number } | undefined)[],
}));

vi.mock("@/hooks/useDataQueries", () => ({
  useMe: () => ({ data: { id: 1, current_library: { id: 1, name: "Main" } } }),
  useLibrarySettings: () => ({ data: null }),
  useAssets: (_filters: unknown, _enabled: boolean, page?: { limit: number; offset: number }) => {
    library.pages.push(page);
    if (library.failed) {
      const error = new Error("network");
      return { data: [], total: 0, loading: false, placeholder: false, error, failed: true };
    }
    return { data: assets, total: library.total, loading: false, placeholder: false };
  },
}));
// The grid is what knows how many columns it shows; this one reports three.
vi.mock("@/components/AssetGrid", () => ({
  AssetGrid: ({ onColumnsChange }: { onColumnsChange?: (n: number) => void }) => {
    onColumnsChange?.(3);
    return <div>grid</div>;
  },
}));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));
vi.mock("@/components/DropZone", () => ({
  DropZone: ({ children }: { children: ReactNode }) => <>{children}</>,
  useDropZoneOpen: () => () => {},
}));
vi.mock("@/components/detail/AssetDetailPanel", () => ({ AssetDetailPanel: () => null }));
vi.mock("@/components/LibraryFilterSidebar", () => ({ LibraryFilterSidebar: () => null }));
vi.mock("@/components/LibraryPicker", () => ({ LibraryPicker: () => null }));
vi.mock("@/components/AddByLocationDialog", () => ({ AddByLocationButton: () => null }));
vi.mock("@/components/BulkAssetBar", () => ({ BulkAssetBar: () => null }));
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
  library.pages = [];
});

const lastPage = () => library.pages[library.pages.length - 1];
const query = () => screen.getByLabelText("query").textContent;

function mount(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/library/:assetId?" element={<LibraryPage />} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
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

describe("LibraryPage load failure", () => {
  it("shows the error with a Retry and keeps the requested page", () => {
    library.failed = true;
    mount("/library?page=3");
    expect(screen.getByText("Couldn't load assets")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.queryByText("grid")).toBeNull();
    expect(query()).toBe("?page=3");
  });
});
