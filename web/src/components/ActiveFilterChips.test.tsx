import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation, useSearchParams } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { parseAssetFilters } from "@/lib/libraryFilters";

import { ActiveFilterChips } from "./ActiveFilterChips";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

beforeEach(() => {
  fetchJsonMock.mockReset();
  fetchJsonMock.mockResolvedValue({
    projects: [{ id: 7, name: "Ashfall", asset_count: 3, licence_counts: {} }],
  });
});

function Harness() {
  const [search] = useSearchParams();
  const { search: query } = useLocation();
  return (
    <>
      <ActiveFilterChips filters={parseAssetFilters(search)} />
      <output aria-label="query">{query}</output>
    </>
  );
}

function mount(path: string) {
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
const chips = () =>
  within(screen.getByRole("list", { name: "Active filters" }))
    .getAllByRole("listitem")
    .map((li) => li.textContent);

describe("ActiveFilterChips", () => {
  it("shows nothing for an unfiltered library, or one only searched or sorted", () => {
    mount("/library?q=castle&sort=name_asc");
    expect(screen.queryByRole("list", { name: "Active filters" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Clear all" })).toBeNull();
  });

  it("shows a chip per tag and per filter, in words", async () => {
    mount("/library?tag=3d/props&tag=2d/ui&licence=err&kind=zip&project_id=7&needs_review=true");
    expect(chips()).toEqual([
      "Tag3d / props",
      "Tag2d / ui",
      "LicenceViolation",
      "StatusNeeds review",
      "Project#7",
      "File kindZip archive",
    ]);
    // The project's name replaces its id once the projects load.
    expect(await screen.findByText("Ashfall")).toBeInTheDocument();
  });

  it("removes one tag and keeps the others", async () => {
    const user = userEvent.setup();
    mount("/library?page=2&tag=3d/props&tag=2d/ui");
    await user.click(screen.getByRole("button", { name: "Remove tag filter: 3d / props" }));
    expect(query()).toBe("?tag=2d%2Fui");
  });

  it("removes a filter", async () => {
    const user = userEvent.setup();
    mount("/library?tag=3d/props&licence=err");
    await user.click(screen.getByRole("button", { name: "Remove licence filter: Violation" }));
    expect(query()).toBe("?tag=3d%2Fprops");
  });

  it("clears every tag and filter, keeping the search and sort", async () => {
    const user = userEvent.setup();
    mount("/library?q=castle&tag=3d/props&licence=err&sort=name_asc&page=4");
    await user.click(screen.getByRole("button", { name: "Clear all" }));
    expect(query()).toBe("?q=castle&sort=name_asc");
  });
});
