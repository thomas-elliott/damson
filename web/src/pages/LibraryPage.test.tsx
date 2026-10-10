import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { LibraryPage } from "./LibraryPage";

const assets = Array.from({ length: 12 }, (_, i) => ({ id: i + 1, name: `Asset ${i + 1}` }));

vi.mock("@/hooks/useDataQueries", () => ({
  useMe: () => ({ data: { id: 1, current_library: { id: 1, name: "Main" } } }),
  useAssets: () => ({ data: assets, total: assets.length, loading: false }),
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
  return <output aria-label="location">{useLocation().pathname}</output>;
}

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
