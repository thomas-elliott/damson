import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { contrastDebt } from "@/design-system/_a11yDebt";
import type { AssetDetailView } from "@/hooks/useDataQueries";
import { stubFetch } from "@/components/_storyStubs";

import { PackageSummary } from "./PackageSummary";
import { ASSET, fakeApi } from "./_storyFixtures";

const EDITING: AssetDetailView = { ...ASSET, id: 4 };

const REFERENCE: AssetDetailView = {
  ...ASSET,
  id: 5,
  name: "POLYGON_Fantasy_Kingdom",
  is_reference: true,
  location_path: "/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage",
  needs_review: false,
  current_version_id: undefined,
  versions: [],
  files: [],
  images: [],
  projects: [
    { project_id: 1, project_name: "Ashfall", pinned_version_id: null, version_label: "" },
  ],
};

const DELETING: AssetDetailView = {
  ...ASSET,
  id: 6,
  name: "Throwaway Pack",
  images: [],
  needs_review: false,
};

const LICENCE_EDITING: AssetDetailView = { ...ASSET, id: 7 };

// An upload auto-enrich couldn't match: the store's search returned one
// plausible listing among its best sellers.
const UNMATCHED: AssetDetailView = {
  ...ASSET,
  id: 8,
  name: "Sample Terrain Textures",
  description: "",
  source_url: "",
  images: [],
  source_metadata: {
    store: "Unity Asset Store",
    fetched_at: "2026-04-20T11:02:00Z",
    matched: false,
    candidates: [
      {
        title: "Gaia Pro 2023 - Terrain & Scene Generator",
        store: "Unity Asset Store",
        listing_url: "https://assetstore.unity.com/packages/tools/terrain/gaia-pro",
      },
      {
        title: "Behavior Designer",
        store: "Unity Asset Store",
        listing_url: "https://assetstore.unity.com/packages/tools/visual-scripting/behavior",
      },
      {
        title: "Terrain Textures Sample Pack",
        store: "Unity Asset Store",
        publisher: "Nature Manufacture",
        listing_url: "https://assetstore.unity.com/packages/2d/textures/terrain-textures",
      },
    ],
  },
};

const ASSETS = new Map(
  [ASSET, EDITING, REFERENCE, DELETING, LICENCE_EDITING, UNMATCHED].map((a) => [a.id, a]),
);

const onDeleted = fn();

/**
 * The package page's left column, wired to its real hooks against the stub
 * API: everything about an asset except its files.
 */
const meta: Meta<typeof PackageSummary> = {
  title: "App/PackageSummary",
  component: PackageSummary,
  parameters: { layout: "fullscreen" },
  beforeEach: () => {
    onDeleted.mockClear();
    return stubFetch(fakeApi(ASSETS));
  },
};
export default meta;
type Story = StoryObj<typeof PackageSummary>;

function Harness({ id }: { id: number }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <div className="w-[380px] bg-card">
          <PackageSummary asset={ASSETS.get(id)!} onDeleted={onDeleted} />
        </div>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

// Delete is on every story here: destructive on destructive-soft, 4.42:1.
const deleteDebt = contrastDebt("destructive-soft");

/** A stored asset: cover, get-it, overview, versions; projects and collections link out. */
export const Default: Story = {
  parameters: { a11y: deleteDebt },
  render: () => <Harness id={ASSET.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: /^Projects/ }));
    await expect(await canvas.findByRole("link", { name: "Ashfall" })).toHaveAttribute(
      "href",
      "/projects/1",
    );
    await userEvent.click(canvas.getByRole("button", { name: /^Collections/ }));
    await expect(await canvas.findByRole("link", { name: "Kingdom build" })).toHaveAttribute(
      "href",
      "/collections/3",
    );
  },
};

/** Overview in edit mode on an asset that already has content. */
export const EditingOverview: Story = {
  parameters: { a11y: deleteDebt },
  render: () => <Harness id={EDITING.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Edit" }));
    await expect(await canvas.findByLabelText("Name")).toBeVisible();
    // The description editor loads lazily; wait for it so the a11y check
    // always sees it, not only when the chunk happens to arrive in time.
    await expect(await canvas.findByRole("textbox", { name: "Description" })).toBeVisible();
  },
};

/** Rescan, re-extract and regenerate previews sit in one menu. */
export const Maintenance: Story = {
  parameters: { a11y: deleteDebt },
  render: () => <Harness id={ASSET.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(canvas.getByRole("button", { name: "Maintenance" }));
    // The menu animates open, so wait for it rather than catching it mid-way.
    await waitFor(() =>
      expect(body.getByRole("menuitem", { name: "Rescan package" })).toBeVisible(),
    );
    await expect(body.getByRole("menuitem", { name: "Re-extract files" })).toBeVisible();
    await userEvent.click(body.getByRole("menuitem", { name: "Regenerate previews" }));
    await expect(await canvas.findByText("Previews queued.")).toBeVisible();
  },
};

/** Unmatched: only the listing that shares the asset's name is offered. */
export const UnmatchedSuggestions: Story = {
  parameters: { a11y: deleteDebt },
  render: () => <Harness id={UNMATCHED.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Terrain Textures Sample Pack")).toBeVisible();
    await expect(canvas.queryByText(/Gaia Pro/)).not.toBeInTheDocument();
    await expect(canvas.queryByText("Behavior Designer")).not.toBeInTheDocument();
  },
};

/** A reference asset: location in place of versions, and no merge or maintenance. */
export const Reference: Story = {
  parameters: { a11y: deleteDebt },
  render: () => <Harness id={REFERENCE.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("button", { name: "Copy path" })).toBeVisible();
    await expect(canvas.queryByRole("button", { name: /^Versions/ })).not.toBeInTheDocument();
    await expect(canvas.queryByRole("button", { name: "Merge…" })).not.toBeInTheDocument();
    await expect(canvas.queryByRole("button", { name: "Maintenance" })).not.toBeInTheDocument();
    // A reference asset's pin has no version, so it reads as Reference, not blank.
    await userEvent.click(canvas.getByRole("button", { name: /^Projects/ }));
    await expect(await canvas.findByText("Reference")).toBeVisible();
  },
};

/** Deleting hands back to the page, which leaves for the library. */
export const DeleteLeaves: Story = {
  parameters: { a11y: deleteDebt },
  render: () => <Harness id={DELETING.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(canvas.getByRole("button", { name: "Delete asset" }));
    const dialog = await body.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete asset" }));
    await waitFor(() => expect(onDeleted).toHaveBeenCalled());
  },
};

/** Editing a set licence: its text and each attribute, prefilled; Save waits for a change. */
export const EditingLicence: Story = {
  parameters: { a11y: deleteDebt },
  render: () => <Harness id={LICENCE_EDITING.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Licence" }));
    await userEvent.click(await canvas.findByRole("button", { name: "Edit licence" }));
    const redistribution = await canvas.findByRole("radiogroup", { name: "Redistribution" });
    await expect(redistribution).toBeVisible();
    const save = canvas.getByRole("button", { name: "Save licence" });
    await expect(save).toBeDisabled();
    await userEvent.click(within(redistribution).getByRole("radio", { name: "Yes" }));
    await expect(save).toBeEnabled();
  },
};
