import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { expect, within } from "storybook/test";

import type { AssetFacetsView, AssetFilters, ProjectView, TagView } from "@/hooks/useDataQueries";
import { contrastDebt } from "@/design-system/_a11yDebt";
import { stubFetch } from "@/components/_storyStubs";

import { LibraryFilterSidebar } from "./LibraryFilterSidebar";

const meta: Meta<typeof LibraryFilterSidebar> = {
  title: "App/LibraryFilterSidebar",
  component: LibraryFilterSidebar,
  parameters: { layout: "fullscreen" },
  beforeEach: () => stubFetch(fakeApi),
};
export default meta;
type Story = StoryObj<typeof LibraryFilterSidebar>;

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
  { id: 1, path: "environment", asset_count: 42 },
  { id: 2, path: "environment/fantasy", parent_id: 1, asset_count: 18 },
  { id: 3, path: "environment/rocks", parent_id: 1, asset_count: 9 },
  { id: 4, path: "style", asset_count: 30 },
  { id: 5, path: "style/low-poly", parent_id: 4, asset_count: 30 },
];

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

// The three lists have no id to key fixtures on, so each story picks how the
// stub answers before it renders.
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

function Harness({ filters = {}, search = "" }: { filters?: AssetFilters; search?: string }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/library${search}`]}>
        <div className="h-[720px] w-sidebar border-r border-border bg-card">
          <LibraryFilterSidebar filters={filters} />
        </div>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

/** A library with tags, projects and storefront facets. */
export const Populated: Story = {
  beforeEach: () => {
    mode = "populated";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByRole("button", { name: /fantasy/ })).toBeVisible();
  },
};

/** A tag and a licence status picked: both rows read as pressed. */
export const ActiveFilters: Story = {
  beforeEach: () => {
    mode = "populated";
  },
  render: () => (
    <Harness
      filters={{ tag_path: "environment/rocks", licence_status: "warn" }}
      search="?tag=environment%2Frocks&licence=warn"
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByRole("button", { name: /rocks/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await expect(canvas.getByRole("button", { name: "Review" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  },
};

/** A fresh library: every list says how it gets filled. */
export const Empty: Story = {
  parameters: {
    // Known contrast debt: faint-foreground on card, 4.34:1 (SMO-3 finding 5).
    a11y: contrastDebt("faint-foreground"),
  },
  beforeEach: () => {
    mode = "empty";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText(/No tags yet/)).toBeVisible();
  },
};

/** The lists are still loading: each open list says so, and none claims to be empty. */
export const Loading: Story = {
  beforeEach: () => {
    mode = "loading";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByRole("status")).toHaveTextContent("Loading…");
    await expect(canvas.queryByText(/No tags yet/)).not.toBeInTheDocument();
  },
};

/** The lists failed to load: each open list offers a Retry instead of an empty state. */
export const Failed: Story = {
  beforeEach: () => {
    mode = "failed";
  },
  render: () => <Harness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Couldn't load tags")).toBeVisible();
    await expect(canvas.getByRole("button", { name: "Retry" })).toBeVisible();
    await expect(canvas.queryByText(/No tags yet/)).not.toBeInTheDocument();
  },
};
