import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CollectionDetailPage } from "./CollectionDetailPage";

vi.mock("@/lib/token", () => ({ getAccessToken: vi.fn().mockResolvedValue(null) }));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));
// Every density the grid was drawn with.
const grid = vi.hoisted(() => ({ densities: [] as (string | undefined)[] }));
vi.mock("@/components/AssetGrid", () => ({
  AssetGrid: ({ density }: { density?: string }) => {
    grid.densities.push(density);
    return <div>grid</div>;
  },
}));

const collection = {
  id: 5,
  name: "Castles",
  description: "",
  asset_count: 1,
  created_at: "2026-10-26T00:00:00Z",
  updated_at: "2026-10-26T00:00:00Z",
};

// /api/settings answers when the test calls `answerSettings`, so the
// collection and its assets have loaded first, as on a cold load.
let answerSettings: (response: Response) => void;

beforeEach(() => {
  grid.densities = [];
  const settings = new Promise<Response>((resolve) => (answerSettings = resolve));
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string) => {
      const path = new URL(url, "http://x").pathname;
      if (path === "/api/settings") return settings;
      if (path === "/api/collections/5") return Promise.resolve(Response.json(collection));
      if (path === "/api/assets") {
        return Promise.resolve(Response.json({ assets: [{ id: 1, name: "Keep" }], total: 1 }));
      }
      return Promise.resolve(Response.json({}));
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

async function mountLoaded() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/collections/5"]}>
        <Routes>
          <Route path="/collections/:id" element={<CollectionDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  // The collection and its assets have loaded; the settings have not.
  await screen.findByText("Castles");
  await waitFor(() => expect(client.isFetching()).toBe(1));
}

describe("CollectionDetailPage density", () => {
  it.each(["small", "large"])(
    "draws the grid at the library's %s density once the settings load",
    async (density) => {
      await mountLoaded();
      expect(screen.queryByText("grid")).toBeNull();

      answerSettings(Response.json({ grid_density_default: density }));
      await screen.findByText("grid");
      expect(grid.densities.every((d) => d === density)).toBe(true);
    },
  );

  it("draws the grid at the default density when the settings fail", async () => {
    await mountLoaded();
    answerSettings(Response.json({ error: "internal error" }, { status: 500 }));
    await screen.findByText("grid");
    expect(grid.densities.every((d) => d === undefined)).toBe(true);
  });
});
