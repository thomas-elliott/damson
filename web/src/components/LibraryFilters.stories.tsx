import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, useSearchParams } from "react-router";
import { expect, screen, userEvent, within } from "storybook/test";

import type { AssetFacetsView, ProjectView, TagView } from "@/hooks/useDataQueries";
import { parseAssetFilters } from "@/lib/libraryFilters";
import { stubFetch } from "@/components/_storyStubs";
import { Toolbar } from "@/design-system";

import { ActiveFilterChips } from "./ActiveFilterChips";
import { LibraryFiltersButton } from "./LibraryFilters";
import { LibraryTagsButton } from "./LibraryTagsButton";

// The library toolbar's find controls, wired to the URL as the page wires
// them: the Tags and Filters buttons, and the chips for what is set.
const meta: Meta = {
  title: "App/LibraryFilters",
  parameters: { layout: "fullscreen" },
  beforeEach: () => stubFetch(fakeApi),
};
export default meta;
type Story = StoryObj;

const project = (id: number, name: string, asset_count: number): ProjectView => ({
  id,
  name,
  description: "",
  target_distribution: "commercial",
  glyph: name.slice(0, 2).toUpperCase(),
  glyph_color: "amber",
  asset_count,
  updates_available: 0,
  licence_counts: { ok: asset_count, warn: 0, err: 0, unk: 0 },
  created_at: "2026-09-01T09:00:00Z",
  updated_at: "2026-10-01T09:00:00Z",
});

const TAGS: TagView[] = [
  ["3d", 0],
  ["3d/characters", 12],
  ["3d/characters/humanoid", 31],
  ["3d/characters/creatures", 14],
  ["3d/environments", 9],
  ["3d/environments/fantasy", 22],
  ["3d/environments/sci-fi", 11],
  ["3d/props", 18],
  ["3d/props/furniture", 7],
  ["2d", 0],
  ["2d/gui", 16],
  ["2d/textures", 25],
  ["audio", 0],
  ["audio/sound-fx", 19],
].map(([path, count], i) => ({ id: i + 1, path: path as string, asset_count: count as number }));

const PROJECTS = [project(1, "Ashfall", 64), project(2, "Tidebreak", 12)];

const FACETS: AssetFacetsView = {
  publishers: [
    { value: "Synty Studios", count: 24 },
    { value: "Kenney", count: 11 },
  ],
  stores: [
    { value: "unity", count: 31 },
    { value: "fab", count: 6 },
  ],
  engines: [],
};

// The lists have no id to key fixtures on, so each story picks how the stub
// answers before it renders.
type Mode = "populated" | "empty" | "loading" | "failed";
let mode: Mode = "populated";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const BODIES: Record<string, { populated: unknown; empty: unknown }> = {
  "/api/tags": { populated: { tags: TAGS }, empty: { tags: [] } },
  "/api/projects": { populated: { projects: PROJECTS }, empty: { projects: [] } },
  "/api/assets/facets": {
    populated: FACETS,
    empty: { publishers: [], stores: [], engines: [] },
  },
};

const fakeApi =
  (realFetch: typeof window.fetch) =>
  async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const path = raw.startsWith("http") ? new URL(raw).pathname : raw.split("?")[0]!;
    const body = BODIES[path];
    if (!body) return realFetch(input, init);
    if (mode === "loading") return new Promise<Response>(() => {});
    if (mode === "failed") return json({ error: "internal error" }, 500);
    return json(body[mode]);
  };

function Controls() {
  const [search] = useSearchParams();
  const filters = parseAssetFilters(search);
  const [tagsOpen, setTagsOpen] = useState(false);
  return (
    <>
      <Toolbar>
        <LibraryTagsButton
          selected={filters.tag_path ?? []}
          open={tagsOpen}
          onOpenChange={setTagsOpen}
        />
        <LibraryFiltersButton filters={filters} />
      </Toolbar>
      <ActiveFilterChips filters={filters} />
    </>
  );
}

function Harness({ search = "", width }: { search?: string; width?: number }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/library${search}`]}>
        <div className="h-[640px] bg-background" style={{ width }}>
          <Controls />
        </div>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const SET = "?tag=3d%2Fcharacters%2Fhumanoid&tag=2d%2Fgui&licence=warn&project_id=1&store=unity";

/** Tags and filters set: each has a chip, and the buttons count them. */
export const ActiveFilters: Story = {
  beforeEach: () => {
    mode = "populated";
  },
  render: () => <Harness search={SET} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("button", { name: /Tags/ })).toHaveTextContent("Tags2");
    await expect(canvas.getByRole("button", { name: /Filters/ })).toHaveTextContent("Filters3");
    await expect(await canvas.findByText("Ashfall")).toBeVisible();
    await userEvent.click(canvas.getByRole("button", { name: "Remove tag filter: 2d / gui" }));
    await expect(canvas.getByRole("button", { name: /Tags/ })).toHaveTextContent("Tags1");
  },
};

/** The Tags popover, opened on the column of the first ticked tag. */
export const TagsOpen: Story = {
  beforeEach: () => {
    mode = "populated";
  },
  render: () => <Harness search="?tag=3d%2Fcharacters%2Fhumanoid" />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: /Tags/ }));
    await expect(
      await screen.findByRole("checkbox", { name: "Select 3d/characters/humanoid" }),
    ).toBeChecked();
    await userEvent.click(screen.getByRole("checkbox", { name: "Select 3d/characters/creatures" }));
    await expect(
      within(canvasElement).getByRole("list", { name: "Active filters" }),
    ).toHaveTextContent("3d / characters / creatures");
  },
};

/** The Filters popover with a project set: its section opens on it. */
export const FiltersOpen: Story = {
  beforeEach: () => {
    mode = "populated";
  },
  render: () => <Harness search="?project_id=1&licence=warn" />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: /Filters/ }));
    const popover = within(await screen.findByRole("dialog", { name: "Filters" }));
    await expect(await popover.findByRole("button", { name: /Ashfall/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await expect(popover.getByRole("button", { name: "Review" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  },
};

/** A fresh library: the picker and every open list say how they get filled. */
export const Empty: Story = {
  beforeEach: () => {
    mode = "empty";
  },
  render: () => <Harness search="?project_id=1" />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: /Tags/ }));
    await expect(await screen.findByText("No tags yet")).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    await userEvent.click(within(canvasElement).getByRole("button", { name: /Filters/ }));
    await expect(await screen.findByText("No projects yet")).toBeInTheDocument();
  },
};

/** The lists failed to load: each offers a Retry instead of an empty state. */
export const Failed: Story = {
  beforeEach: () => {
    mode = "failed";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: /Tags/ }));
    await expect(await screen.findByText("Couldn't load tags")).toBeInTheDocument();
    await expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    await expect(screen.queryByText("No tags yet")).not.toBeInTheDocument();
  },
};
