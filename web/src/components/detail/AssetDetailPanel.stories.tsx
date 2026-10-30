import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, fn, userEvent, within } from "storybook/test";

import type { AssetDetailView } from "@/hooks/useDataQueries";
import { stubFetch } from "@/components/_storyStubs";

import { AssetDetailPanel } from "./AssetDetailPanel";
import { ASSET, fakeApi } from "./_storyFixtures";

const NO_DESCRIPTION: AssetDetailView = {
  ...ASSET,
  id: 2,
  name: "unsorted_kitbash_props",
  description: "",
  source_url: "",
  source_metadata: undefined,
  licence: undefined,
  images: [],
  tags: [],
};

const SPARSE: AssetDetailView = {
  ...ASSET,
  id: 3,
  name: "metal_impact_01",
  description: "Single-shot metal impact, 48 kHz mono.",
  needs_review: false,
  licence: undefined,
  source_metadata: undefined,
  images: [],
  tags: [],
  versions: [ASSET.versions[0]!],
  projects: [],
  collections: [],
};

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
};

const ASSETS = new Map([ASSET, NO_DESCRIPTION, SPARSE, REFERENCE].map((a) => [a.id, a]));

const onOpenPackage = fn();

/**
 * Stories for the quick look the library opens an asset into, as the app
 * renders it: the wired component with its own TanStack hooks, not a mock
 * reassembled from primitives. `/api/*` is served by the stub in
 * `_storyFixtures.ts`; each story gets its own asset id so the fixtures stay
 * independent of render order.
 */
const meta: Meta<typeof AssetDetailPanel> = {
  title: "App/AssetDetailPanel",
  component: AssetDetailPanel,
  parameters: { layout: "fullscreen" },
  beforeEach: () => {
    onOpenPackage.mockClear();
    return stubFetch(fakeApi(ASSETS));
  },
};
export default meta;
type Story = StoryObj<typeof AssetDetailPanel>;

function PanelHarness({ assetId }: { assetId: number }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  const [open, setOpen] = useState(true);
  return (
    <QueryClientProvider client={client}>
      <div className="relative h-[900px] w-full overflow-hidden bg-background">
        <AssetDetailPanel
          assetId={open ? assetId : null}
          onClose={() => setOpen(false)}
          onOpenPackage={onOpenPackage}
        />
      </div>
    </QueryClientProvider>
  );
}

/**
 * As it opens from the library grid: how to get the package first, then the
 * gallery, description, tags and licence line. Needs review is a status, not a
 * button, and a licence with its own terms is a plain label.
 */
export const Default: Story = {
  render: () => <PanelHarness assetId={ASSET.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const open = await canvas.findByRole("button", { name: "Open package page" });
    await expect(await canvas.findByRole("button", { name: "Download original" })).toBeVisible();
    await expect(canvas.getByText("Needs review")).toBeVisible();
    await expect(canvas.queryByRole("button", { name: /Needs review/ })).not.toBeInTheDocument();
    const licence = canvas.getByText("Unity-EULA");
    await expect(licence).toBeVisible();
    await expect(licence.querySelector("[aria-hidden]")).toBeNull();
    // Housekeeping lives on the package page.
    await expect(canvas.queryByRole("button", { name: "Delete asset" })).not.toBeInTheDocument();
    await expect(canvas.queryByRole("button", { name: /^Versions/ })).not.toBeInTheDocument();
    await userEvent.click(open);
    await expect(onOpenPackage).toHaveBeenCalledWith(ASSET.id);
  },
};

/** No description: the panel says so and still leads with the package, not a form. */
export const MissingDescription: Story = {
  render: () => <PanelHarness assetId={NO_DESCRIPTION.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("No description yet.")).toBeVisible();
    await expect(canvas.queryByRole("textbox", { name: "Name" })).not.toBeInTheDocument();
    await expect(canvas.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
  },
};

/** Reviewed, unlicensed, one version, no gallery: the sparse end of the panel. */
export const Sparse: Story = {
  render: () => <PanelHarness assetId={SPARSE.id} />,
};

/** Recorded by its location: the path and store link come first. */
export const Reference: Story = {
  render: () => <PanelHarness assetId={REFERENCE.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText("Reference · not stored")).toBeVisible();
    const copy = canvas.getByRole("button", { name: "Copy path" });
    await expect(copy).toBeVisible();
    await expect(canvas.getByRole("link", { name: "Open store link" })).toBeVisible();
    // The path comes before the description.
    const description = canvas.getByText(/is a low-poly asset pack/);
    await expect(
      copy.compareDocumentPosition(description) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  },
};
