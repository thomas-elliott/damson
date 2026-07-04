# Add scoped file browser

## Why

Multi-file assets (`.unitypackage` archives, multi-file packs) are stored with a
full per-file manifest — `asset_files` rows carry `relative_path`, `size_bytes`,
`object_key`, and `unity_guid` for every extracted file, and that manifest is
already delivered to the frontend on `useAssetDetail().files[]`. Today the only
way to act on individual files is the `SubsetVersionDialog`: a flat checkbox list
whose own implementation note concedes it skips directory grouping because
"grouping costs more in dialog complexity than it saves."

The result is that a rich, real dataset is presented as an undifferentiated list.
Users can't browse a pack's structure, preview a file, see what a folder contains,
or understand a package before subsetting it. The mockup at `docs/mockup/`
(`src/filebrowser.jsx`, `src/filetree.jsx`) prototypes the answer: a full-page
**scoped file browser** — a directory tree, breadcrumb navigation, grid/list
views, type filtering, search, and a per-file detail/preview panel — entered by
drilling into a browsable asset.

The mockup fabricates its trees because it is a static prototype with no backend.
The real feature needs **no new backend and no new data model**: the tree is built
client-side by splitting `relative_path`, previews come from the existing
`GET /api/files/:objectKey` endpoint, and selecting files to keep maps onto the
existing `POST /assets/:id/versions/:vid/subset` endpoint. (The mockup labels this
interaction "collection"; that is a misnomer — the real verb is **subset**.)

## What Changes

- **Design system + Storybook (first).** Port the mockup's browser components into
  `web/src/design-system` as token-aligned, story-backed components: file tree
  sidebar, browser toolbar/breadcrumb, folder/file cards, grid and list views,
  file thumbnail/placeholder, file-type filter, and a file detail panel that
  reuses the existing `DetailPanel` chrome. New icons (`FolderOpen`, `Cube`,
  `Code`, `Scene`, `Sphere`, `Play`, …) are added through the existing `Glyph`
  surface. Every component ships a Storybook story (the project's visual + a11y
  contract).
- **Browse experience.** A browsable asset (`files > 1`) gains a "Browse files"
  entry point on the asset grid toolbar and detail panel. Entering it swaps the
  library view for the scoped browser: navigate the tree, switch grid/list, search
  within the pack, filter by file type, open a file to preview it, and download
  individual files — all keyboard-navigable.
- **File-type taxonomy from extension.** Because `detected_type` is empty for
  unitypackage-extracted files, the file-type classification (prefab, mesh,
  texture, material, scene, script, …) is derived client-side from the file
  extension. Drives the icon, the type filter, and the preview style.
- **Subset from the browser, replacing the flat dialog.** Selecting files in the
  browser and choosing "Create subset" calls the existing subset endpoint. The
  tree browser becomes the file-picking surface and the flat-list
  `SubsetVersionDialog` is retired.

## Impact

- Affected specs: new `file-browser` capability.
- Affected code (frontend-only):
  - **New:** design-system file-browser components + stories; a scoped-browser
    view/route within the library page; an extension→type taxonomy helper.
  - **Modified:** `AssetGrid` toolbar and `AssetDetailPanel` gain the "Browse
    files" trigger; library page wiring for enter/exit browser state and
    browser-mode keyboard shortcuts; `Glyph`/icon additions; subset creation moves
    into the browser.
  - **Removed:** `SubsetVersionDialog` (flat-list picker) once the browser path
    ships.
- **No backend changes.** Manifest already on the asset-detail payload; previews
  and downloads use the existing `/api/files/:objectKey` endpoint; subset creation
  uses the existing `POST /assets/:id/versions/:vid/subset` endpoint.
- Non-goals: no `collection_files` membership (collections stay asset-scoped); no
  new files/manifest API; no server-side thumbnailing of non-image files; no edits
  to a pack's on-disk structure.
