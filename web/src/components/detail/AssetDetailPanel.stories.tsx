import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, userEvent, within } from "storybook/test";

import type { AssetDetailView, AuditEntry } from "@/hooks/useDataQueries";

import { AssetDetailPanel } from "./AssetDetailPanel";

/**
 * Stories for the library sidebar as the app actually renders it — the wired
 * component with its own TanStack hooks, not a mock reassembled from
 * primitives. `/api/*` is served by the module-level stub below; each story
 * gets its own asset id so the fixtures stay independent of render order.
 */
const meta: Meta<typeof AssetDetailPanel> = {
  title: "App/AssetDetailPanel",
  component: AssetDetailPanel,
  parameters: { layout: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof AssetDetailPanel>;

const DESCRIPTION = `**POLYGON Fantasy Kingdom** is a low-poly asset pack of
characters, props, and environment pieces.

### Contents

- 1,284 prefabs, all with LODs
- 12 modular building kits
- Demo scene (\`Scenes/Demo.unity\`)

Terms follow the [Unity Asset Store EULA](https://unity.com/legal/as-terms).

> Requires the Universal Render Pipeline. Built-in RP users should re-import
> the materials.`;

const LISTING_URL = "https://assetstore.unity.com/packages/3d/environments/fantasy-kingdom-164532";

const ASSET: AssetDetailView = {
  id: 1,
  name: "POLYGON - Fantasy Kingdom",
  description: DESCRIPTION,
  source_url: LISTING_URL,
  needs_review: true,
  current_version_id: 21,
  created_at: "2026-03-12T09:14:00Z",
  updated_at: "2026-04-20T11:02:00Z",
  licence: {
    id: 5,
    asset_id: 1,
    detected_type: "Unity-EULA",
    detection_confidence: 0.92,
    original_text: "Unity Asset Store End User License Agreement…",
    attributes: {
      commercial_use: true,
      modification: true,
      redistribution: false,
      attribution_required: false,
      share_alike: false,
      open_source_required: false,
    },
    status: "warn",
  },
  source_metadata: {
    store: "Unity Asset Store",
    listing_url: LISTING_URL,
    fetched_at: "2026-04-20T11:02:00Z",
    matched: true,
    match_confidence: 0.97,
  },
  tags: [
    { id: 11, path: "environment/fantasy" },
    { id: 12, path: "style/low-poly" },
  ],
  versions: [
    {
      id: 21,
      version_label: "v2.1",
      store_version: "2.1",
      notes: "URP materials fixed",
      import_date: "2026-04-20T11:00:00Z",
      package_kind: "unitypackage",
      file_count: 1284,
      size_bytes: 684_221_440,
      has_original: true,
      original_package_object_key: "ab/abcdef",
      is_current: true,
    },
    {
      id: 20,
      version_label: "v2.0",
      store_version: "2.0",
      notes: "",
      import_date: "2026-03-12T09:14:00Z",
      package_kind: "unitypackage",
      file_count: 1190,
      size_bytes: 612_000_000,
      has_original: false,
      is_current: false,
    },
  ],
  files: [
    {
      id: 101,
      relative_path: "Assets/Polygon/Prefabs/Castle.prefab",
      size_bytes: 48_200,
      content_hash: "a1b2c3",
      object_key: "a1/a1b2c3",
    },
    {
      id: 102,
      relative_path: "Assets/Polygon/Textures/Atlas.png",
      size_bytes: 4_812_000,
      content_hash: "d4e5f6",
      object_key: "d4/d4e5f6",
      detected_type: "image/png",
      preview_status: "done",
    },
  ],
  images: [
    { id: 301, object_key: "gallery/01", content_type: "image/png" },
    { id: 302, object_key: "gallery/02", content_type: "image/png" },
  ],
  projects: [
    { project_id: 1, project_name: "Ashfall", pinned_version_id: 21, version_label: "v2.1" },
  ],
  collections: [{ collection_id: 3, collection_name: "Kingdom build" }],
};

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

const EDITING: AssetDetailView = { ...ASSET, id: 4 };

const ASSETS = new Map([ASSET, NO_DESCRIPTION, SPARSE, EDITING].map((a) => [a.id, a]));

const AUDIT: AuditEntry[] = [
  {
    id: 900,
    created_at: "2026-04-20T11:02:00Z",
    action_type: "asset.metadata_applied",
    user_display_name: "Thomas",
    user_email: "thomas@example.com",
    target_type: "asset",
    target_id: 1,
  },
  {
    id: 899,
    created_at: "2026-04-20T11:00:00Z",
    action_type: "asset.version_added",
    user_display_name: "Thomas",
    user_email: "thomas@example.com",
    target_type: "asset",
    target_id: 1,
  },
];

// 2x2 PNG — the gallery fetches thumbnails as authenticated blobs, so the stub
// has to hand back real image bytes rather than JSON.
const PNG_BYTES = Uint8Array.from(
  atob(
    "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAACp8Z5+AAAAFUlEQVR42mNkYPhfz0DFwEjNwAgAaZgD+8sVYVoAAAAASUVORK5CYII=",
  ),
  (c) => c.charCodeAt(0),
);

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

// Serves every endpoint the panel touches so the stories exercise the app's own
// fetch path. PATCH writes back into the fixture map, so saving the Overview
// form in Storybook behaves like the real thing; other writes just succeed.
const realFetch = window.fetch;
window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
  const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
  const path = raw.startsWith("http") ? new URL(raw).pathname : raw;
  if (!path.startsWith("/api/")) return realFetch(input, init);

  if (path.startsWith("/api/thumbs/")) {
    return new Response(PNG_BYTES, { status: 200, headers: { "Content-Type": "image/png" } });
  }
  const audit = /^\/api\/assets\/(\d+)\/audit$/.exec(path);
  if (audit) return json({ entries: AUDIT, total: AUDIT.length });

  const detail = /^\/api\/assets\/(\d+)$/.exec(path);
  if (detail) {
    const asset = ASSETS.get(Number(detail[1]));
    if (!asset) return new Response("not found", { status: 404 });
    if (init?.method === "PATCH" && typeof init.body === "string") {
      ASSETS.set(asset.id, { ...asset, ...(JSON.parse(init.body) as Partial<AssetDetailView>) });
    }
    return json(ASSETS.get(asset.id));
  }
  return json({ ok: true });
};

function PanelHarness({ assetId }: { assetId: number }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <div className="relative h-[900px] w-full overflow-hidden bg-background">
        <AssetDetailPanel assetId={assetId} onClose={() => {}} onBrowse={() => {}} />
      </div>
    </QueryClientProvider>
  );
}

/** As it opens from the library grid: name in the header, rendered overview, licence collapsed. */
export const Default: Story = {
  render: () => <PanelHarness assetId={ASSET.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // The collapsed Licence header carries the identifier…
    await expect(await canvas.findByText("Unity-EULA")).toBeVisible();
    // …and enrichment provenance stays out of the read view.
    await expect(canvas.queryByText(/Metadata from/)).not.toBeInTheDocument();
  },
};

/** No description yet — Overview opens straight into the edit form. */
export const MissingDescription: Story = {
  render: () => <PanelHarness assetId={NO_DESCRIPTION.id} />,
};

/** Overview in edit mode on an asset that already has content. */
export const EditingOverview: Story = {
  render: () => <PanelHarness assetId={EDITING.id} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Edit" }));
    await expect(await canvas.findByLabelText("Name")).toBeVisible();
    await expect(await canvas.findByText(/Metadata from Unity Asset Store/)).toBeVisible();
  },
};

/** Reviewed, unlicensed, one version, no gallery — the sparse end of the panel. */
export const Sparse: Story = {
  render: () => <PanelHarness assetId={SPARSE.id} />,
};
