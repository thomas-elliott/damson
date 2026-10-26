import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const server = vi.hoisted(() => ({
  assets: [] as { id: number; name: string }[],
  total: 0,
  calls: [] as { filters: { q?: string }; page?: { limit: number; offset: number } }[],
}));

vi.mock("@/hooks/useDataQueries", () => ({
  useAssets: (
    filters: { q?: string },
    _enabled: boolean,
    page?: { limit: number; offset: number },
  ) => {
    server.calls.push({ filters, page });
    return { data: server.assets, total: server.total, loading: false, placeholder: false };
  },
}));

import { AssetPicker, PICKER_PAGE_SIZE } from "./AssetPicker";

const lastCall = () => server.calls[server.calls.length - 1];

beforeEach(() => {
  server.assets = [
    { id: 1, name: "Alpha" },
    { id: 2, name: "Bravo" },
  ];
  server.total = 2;
  server.calls = [];
});

describe("AssetPicker", () => {
  it("searches the server for what is typed, one page at a time", async () => {
    render(<AssetPicker aria-label="Asset" selectedId={null} onSelect={vi.fn()} />);
    expect(lastCall()).toEqual({
      filters: { q: undefined },
      page: { limit: PICKER_PAGE_SIZE, offset: 0 },
    });

    fireEvent.change(screen.getByRole("searchbox", { name: "Asset" }), {
      target: { value: "cast" },
    });
    await waitFor(() => expect(lastCall().filters).toEqual({ q: "cast" }));
    expect(lastCall().page).toEqual({ limit: PICKER_PAGE_SIZE, offset: 0 });
  });

  it("asks for a narrower search only when there are more matches than shown", () => {
    const { rerender } = render(
      <AssetPicker aria-label="Asset" selectedId={null} onSelect={vi.fn()} />,
    );
    expect(screen.queryByText(/narrow the search/)).toBeNull();

    server.total = 300;
    rerender(<AssetPicker aria-label="Asset" selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText(/Showing the first 2 of 300 matches/)).toBeInTheDocument();
  });

  it("does not count an excluded asset as a hidden match", () => {
    render(<AssetPicker aria-label="Asset" selectedId={null} onSelect={vi.fn()} excludeId={1} />);
    expect(screen.queryByRole("button", { name: "Alpha" })).toBeNull();
    expect(screen.queryByText(/narrow the search/)).toBeNull();
  });

  it("reports the picked asset and marks it", () => {
    const onSelect = vi.fn();
    const { rerender } = render(
      <AssetPicker aria-label="Asset" selectedId={null} onSelect={onSelect} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Bravo" }));
    expect(onSelect).toHaveBeenCalledWith({ id: 2, name: "Bravo" });

    rerender(<AssetPicker aria-label="Asset" selectedId={2} onSelect={onSelect} />);
    expect(screen.getByRole("button", { name: "Bravo" })).toHaveAttribute("aria-pressed", "true");
  });
});
