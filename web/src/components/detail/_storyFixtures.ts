import type { AssetDetailView, AuditEntry } from "@/hooks/useDataQueries";

// Fixtures and a fake API shared by the app-level stories of the asset panel
// and the package page.

export const DESCRIPTION = `**POLYGON Fantasy Kingdom** is a low-poly asset pack of
characters, props, and environment pieces.

### Contents

- 1,284 prefabs, all with LODs
- 12 modular building kits
- Demo scene (\`Scenes/Demo.unity\`)

Terms follow the [Unity Asset Store EULA](https://unity.com/legal/as-terms).

> Requires the Universal Render Pipeline. Built-in RP users should re-import
> the materials.`;

export const LISTING_URL =
  "https://assetstore.unity.com/packages/3d/environments/fantasy-kingdom-164532";

export const ASSET: AssetDetailView = {
  id: 1,
  name: "POLYGON - Fantasy Kingdom",
  description: DESCRIPTION,
  source_url: LISTING_URL,
  needs_review: true,
  is_reference: false,
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
    { id: 11, path: "3d/environment/fantasy" },
    { id: 12, path: "3d/props/low-poly" },
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

export const AUDIT: AuditEntry[] = [
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

export function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

// Serves every endpoint the panel and the package page touch, so the stories
// exercise the app's own fetch path. PATCH writes back into the fixture map, so
// saving the Overview form in Storybook behaves like the real thing; other
// writes just succeed.
export const fakeApi =
  (assets: Map<number, AssetDetailView>) =>
  (realFetch: typeof window.fetch) =>
  async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
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
      const asset = assets.get(Number(detail[1]));
      if (!asset) return new Response("not found", { status: 404 });
      if (init?.method === "DELETE") return new Response(null, { status: 204 });
      if (init?.method === "PATCH" && typeof init.body === "string") {
        assets.set(asset.id, { ...asset, ...(JSON.parse(init.body) as Partial<AssetDetailView>) });
      }
      return json(assets.get(asset.id));
    }
    return json({ ok: true });
  };
