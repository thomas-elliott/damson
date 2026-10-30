import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { MemoryRouter, useLocation, useSearchParams } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { LibraryTagsButton } from "./LibraryTagsButton";

const fetchJsonMock = vi.fn();
vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return { ...actual, fetchJson: (...args: unknown[]) => fetchJsonMock(...args) };
});

const TAGS = [
  { id: 1, path: "3d", asset_count: 0, subtree_count: 6 },
  { id: 2, path: "3d/props", parent_id: 1, asset_count: 4, subtree_count: 4 },
  { id: 3, path: "3d/vegetation", parent_id: 1, asset_count: 2, subtree_count: 2 },
];

let tags: "ok" | "empty" | "pending" | "failed";

beforeEach(() => {
  tags = "ok";
  fetchJsonMock.mockReset();
  fetchJsonMock.mockImplementation(() => {
    if (tags === "pending") return new Promise(() => {});
    if (tags === "failed") return Promise.reject(new Error("boom"));
    return Promise.resolve({ tags: tags === "empty" ? [] : TAGS });
  });
});

function Harness() {
  const [search] = useSearchParams();
  const { search: query } = useLocation();
  const [open, setOpen] = useState(false);
  return (
    <>
      <LibraryTagsButton selected={search.getAll("tag")} open={open} onOpenChange={setOpen} />
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

async function openPicker() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: /Tags/ }));
  return user;
}

describe("LibraryTagsButton", () => {
  it("counts the ticked tags", () => {
    mount("/library?tag=3d/props&tag=3d/vegetation");
    expect(screen.getByRole("button", { name: /Tags/ })).toHaveTextContent("Tags2");
  });

  it("ticks tags into the URL, any of them matching, from the first page", async () => {
    mount("/library?page=2&q=rock");
    const user = await openPicker();
    await user.click(await screen.findByRole("button", { name: /^3d/ }));
    await user.click(screen.getByRole("checkbox", { name: "Select 3d/props" }));
    expect(query()).toBe("?q=rock&tag=3d%2Fprops");
    await user.click(screen.getByRole("checkbox", { name: "Select 3d/vegetation" }));
    expect(query()).toBe("?q=rock&tag=3d%2Fprops&tag=3d%2Fvegetation");
    await user.click(screen.getByRole("checkbox", { name: "Select 3d/props" }));
    expect(query()).toBe("?q=rock&tag=3d%2Fvegetation");
  });

  it("counts everything under a tag, not only what carries it", async () => {
    mount();
    await openPicker();
    expect(await screen.findByRole("button", { name: /^3d/ })).toHaveTextContent("3d6");
  });

  it("closes on Show results", async () => {
    mount();
    const user = await openPicker();
    await user.click(await screen.findByRole("button", { name: "Show results" }));
    expect(screen.queryByRole("dialog", { name: "Tags" })).toBeNull();
  });

  it("says it is loading rather than that there are no tags", async () => {
    tags = "pending";
    mount();
    await openPicker();
    expect(await screen.findByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("No tags yet")).toBeNull();
  });

  it("offers a Retry when the tags fail to load, and recovers", async () => {
    tags = "failed";
    mount();
    const user = await openPicker();
    expect(await screen.findByText("Couldn't load tags")).toBeInTheDocument();

    tags = "ok";
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("list", { name: "Top-level tags" })).toBeInTheDocument();
  });

  it("says there are no tags only once the list has loaded empty", async () => {
    tags = "empty";
    mount();
    await openPicker();
    expect(await screen.findByText("No tags yet")).toBeInTheDocument();
  });
});
