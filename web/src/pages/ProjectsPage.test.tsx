import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ProjectsPage } from "./ProjectsPage";

vi.mock("@/lib/token", () => ({ getAccessToken: vi.fn().mockResolvedValue(null) }));
vi.mock("@/components/Shell", () => ({
  Shell: ({ main }: { main: ReactNode }) => <main>{main}</main>,
}));

function project(id: number, name: string, licence_counts: Record<string, number>) {
  return {
    id,
    name,
    description: "",
    target_distribution: "",
    glyph: "",
    glyph_color: "",
    asset_count: 10,
    updates_available: 0,
    licence_counts,
    created_at: "2026-10-26T00:00:00Z",
    updated_at: "2026-10-26T00:00:00Z",
  };
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string) => {
      const path = new URL(url, "http://x").pathname;
      if (path !== "/api/projects") return Promise.resolve(Response.json({}));
      return Promise.resolve(
        Response.json({
          projects: [
            project(1, "Keep", { ok: 1, warn: 2, err: 3, unk: 4 }),
            project(2, "Moat", { ok: 7, warn: 5, err: 6, unk: 8 }),
          ],
        }),
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

describe("ProjectsPage", () => {
  it("draws each card's licence bar from the project's licence counts", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <ProjectsPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(
      await screen.findByRole("img", {
        name: "1 compliant, 2 review, 3 violation, 4 unknown",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "7 compliant, 5 review, 6 violation, 8 unknown" }),
    ).toBeInTheDocument();
  });
});
