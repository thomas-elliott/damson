import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LibraryFilterSidebar } from "./LibraryFilterSidebar";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

const BODIES: Record<string, unknown> = {
  "/api/tags": { tags: [{ id: 1, path: "environment", asset_count: 4 }] },
  "/api/projects": { projects: [] },
  "/api/assets/facets": { publishers: [], stores: [], engines: [] },
};

// How the tags endpoint answers; the others always succeed.
let tags: "ok" | "empty" | "pending" | "failed";

beforeEach(() => {
  tags = "ok";
  fetchJsonMock.mockReset();
  fetchJsonMock.mockImplementation((url: string) => {
    if (url === "/api/tags") {
      if (tags === "pending") return new Promise(() => {});
      if (tags === "failed") return Promise.reject(new Error("boom"));
      if (tags === "empty") return Promise.resolve({ tags: [] });
    }
    return Promise.resolve(BODIES[url]);
  });
});

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <LibraryFilterSidebar filters={{}} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("LibraryFilterSidebar tags", () => {
  it("says it is loading rather than that there are no tags", async () => {
    tags = "pending";
    mount();

    expect(await screen.findByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText(/No tags yet/)).not.toBeInTheDocument();
  });

  it("offers a Retry when the tags fail to load, and recovers", async () => {
    tags = "failed";
    const user = userEvent.setup();
    mount();

    expect(await screen.findByText("Couldn't load tags")).toBeInTheDocument();
    expect(screen.queryByText(/No tags yet/)).not.toBeInTheDocument();

    tags = "ok";
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: /environment/ })).toBeInTheDocument();
  });

  it("says there are no tags only once the list has loaded empty", async () => {
    tags = "empty";
    mount();

    expect(await screen.findByText(/No tags yet/)).toBeInTheDocument();
  });
});
