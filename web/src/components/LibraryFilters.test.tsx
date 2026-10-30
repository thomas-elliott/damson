import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation, useSearchParams } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { parseAssetFilters } from "@/lib/libraryFilters";

import { LibraryFiltersButton } from "./LibraryFilters";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

const BODIES: Record<string, unknown> = {
  "/api/projects": {
    projects: [{ id: 7, name: "Ashfall", asset_count: 3, licence_counts: {} }],
  },
  "/api/assets/facets": { publishers: [], stores: [], engines: [] },
};

// How the projects endpoint answers; the others always succeed.
let projects: "ok" | "failed";

beforeEach(() => {
  projects = "ok";
  fetchJsonMock.mockReset();
  fetchJsonMock.mockImplementation((url: string) => {
    if (url === "/api/projects" && projects === "failed") return Promise.reject(new Error("boom"));
    return Promise.resolve(BODIES[url]);
  });
});

// The button as the page uses it: filters read from the URL it writes.
function Harness() {
  const [search] = useSearchParams();
  const { search: query } = useLocation();
  return (
    <>
      <LibraryFiltersButton filters={parseAssetFilters(search)} />
      <output aria-label="query">{query}</output>
    </>
  );
}

function mount(path = "/library") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Harness />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const query = () => screen.getByLabelText("query").textContent;

describe("LibraryFiltersButton", () => {
  it("counts the filters set, but not the tags or the search", () => {
    mount("/library?q=castle&tag=3d/props&licence=err&needs_review=true");
    expect(screen.getByRole("button", { name: /Filters/ })).toHaveTextContent("Filters2");
  });

  it("sets a filter from the popover and goes back to the first page", async () => {
    const user = userEvent.setup();
    mount("/library?page=3&tag=3d/props");
    await user.click(screen.getByRole("button", { name: /Filters/ }));
    await user.click(await screen.findByRole("button", { name: "Violation" }));
    expect(query()).toBe("?tag=3d%2Fprops&licence=err");

    await user.click(screen.getByRole("button", { name: "Violation" }));
    expect(query()).toBe("?tag=3d%2Fprops");
  });

  it("opens a section that holds a set filter", async () => {
    const user = userEvent.setup();
    mount("/library?project_id=7");
    await user.click(screen.getByRole("button", { name: /Filters/ }));
    expect(await screen.findByRole("button", { name: /Ashfall/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("offers a Retry when a list fails to load", async () => {
    projects = "failed";
    const user = userEvent.setup();
    mount("/library?project_id=7");
    await user.click(screen.getByRole("button", { name: /Filters/ }));
    expect(await screen.findByText("Couldn't load projects")).toBeInTheDocument();

    projects = "ok";
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: /Ashfall/ })).toBeInTheDocument();
  });
});
