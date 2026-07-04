# Design — Scoped file browser

## Context

The mockup (`docs/mockup/src/filebrowser.jsx`, `filetree.jsx`) is a static Babel
prototype: it fabricates believable Unity-style trees with seeded RNG because it
has no backend. The real app already has the data the prototype invents. This
design records how the mockup maps onto real machinery and the decisions taken
during exploration.

## Mockup → real-app mapping

| Mockup construct | Real-app source | Notes |
|---|---|---|
| `buildPackTree(asset)` (fabricated) | `useAssetDetail(id).files: AssetFileView[]` | Build the tree client-side by splitting `relative_path` on `/`. No new endpoint. |
| `node.size`, `node.modified` | `size_bytes`; version `import_date` | Per-file modified time isn't tracked; use the version import date or omit. |
| `node.ftype` / `FTYPES` taxonomy | derived from file **extension** | `detected_type` is empty for extracted files (only set for loose uploads). |
| `FileThumb` waveform/font/sprite art | `GET /api/files/:objectKey` | Real previews: images render inline; everything else gets a type placeholder. |
| `node.thumb` procedural renders | — | Dropped; they were prototype filler. |
| "Add to collection" / `CollectionDock` | `POST /assets/:id/versions/:vid/subset` | **Misnomer in mockup.** The real verb is *subset*, not collection. |
| `SubsetVersionDialog` (flat list) | retired | The browser becomes the file-picking surface. |
| `damson.css` `.app` grid-row fix | AppShell layout | Verify the real AppShell already bounds its content area; port only if needed. |

## Decisions

### D1 — "Collection" interactions are subset creation
The mockup's collection dock, drag-to-collection, and "new collection from
selection" are all relabelings of **subset** creation. Collections remain
asset-scoped (`collection_assets`); no `collection_files` membership is
introduced. A file selection in the browser becomes a new asset *version* whose
`asset_files` are the chosen subset (blobs reused, no re-upload) via the existing
endpoint.

> Open sub-question for implementation: the mockup dock shows *multiple* named
> drop targets. Subsets are versions of one asset, so the natural reduction is a
> single primary verb — "Create subset from selection" — rather than a dock of
> targets. Resolve the exact selection→subset affordance when building Phase 3;
> the spec requires the capability, not the dock chrome.

### D2 — The browser supersedes `SubsetVersionDialog`
Rather than maintain a flat picker and a tree browser, the tree browser becomes
the single file-selection surface and the flat dialog is removed once the browser
ships. Sequenced last (Phase 3) so the browse experience can land and be verified
before the existing subset path is touched.

### D3 — Tree built client-side from `relative_path`
No structured manifest/tree API. The flat `files[]` array already on the asset
detail is grouped into a tree in the browser. Keeps the feature backend-free and
avoids a second source of truth for the manifest. Trade-off: very large packages
build their tree in the client — acceptable given the manifest already ships in
the same payload, but list/grid rendering should virtualize (the project already
uses `@tanstack/react-virtual`).

### D4 — File-type taxonomy is extension-derived
A client-side helper maps file extension → type (`.prefab`→prefab, `.fbx`→mesh,
`.png`→texture/sprite, `.mat`→material, `.unity`→scene, `.cs`→script,
`.shader`→shader, `.anim`→anim, audio/font/doc/data, fallback `file`). This drives
the icon (`FileGlyph`), the type filter, and whether a file previews as an image.
`detected_type` is consulted when present (loose uploads carry a content-type) but
is not relied upon.

### D5 — Previews: image-or-placeholder
Image files (`texture`/`sprite` types) preview inline; all other types render a
type-styled placeholder (icon + extension), matching the mockup's
`FilePlaceholder`. No server-side thumbnailing of meshes/scripts/scenes — out of
scope.

**Implementation note (discovered during Phase 2):** a plain
`<img src="/api/files/:objectKey">` does **not** work. The endpoint deliberately
forces `Content-Type: application/octet-stream` + `Content-Disposition:
attachment` + `nosniff` ("served blobs are hostile"), and auth is bearer-only
(no cookie/query token), so the browser can neither render nor authenticate such
a request. The backend-free workaround: `fetchBlob` (api.ts) GETs the bytes with
the auth header, re-types the Blob with a known raster MIME, and the caller
builds an object URL (`useFileObjectUrl`). Scoped to the **detail panel only**
(on file open) — the grid/list use placeholders to avoid a fetch storm. SVG is
excluded from preview (rendering attacker-controlled SVG is the risk the nosniff
policy guards against). Cost: one authenticated fetch per opened file; a future
per-file thumbnail endpoint could replace this.

### D6 — Selection is modifier-click; drag-to-target dropped
The mockup selected files by shift/meta/ctrl-click **and** by dragging onto a
collection dock. With collections reinterpreted as subset (D1), the dock
collapsed to a single "Create subset" verb, so no drop target remains — drag-to-
target was dropped. Selection is modifier-click only (file cards and list rows),
surfaced in the floating `FileSelectionBar`.

## Component split (design system)

Ported into `web/src/design-system` (or a feature folder composing the system),
each with a Storybook story:

- `FileTreeSidebar` — scope header, search, tree, file-type filter. Composes the
  existing `FilterSidebar`/`SearchInput`/`Section` patterns.
- `FileBrowserToolbar` — breadcrumb path + result/filter state + grid/list toggle.
  Composes the existing `Breadcrumb`, `SegmentedControl`.
- `FolderCard` / `FileCard` — grid tiles; `FileThumb` / `FilePlaceholder` for the
  preview surface.
- `FileBrowserList` — list/table view.
- `FileDetailPanel` — reuses the existing `DetailPanel` chrome; file hero,
  metadata, path bar, download/subset actions.
- `FileTypeFilter` — type rows with counts (composes existing filter-row pattern).
- Icons: `FolderOpen`, `Cube`, `Code`, `Scene`, `Sphere`, `Play`, `CornerArrow`
  added through `Glyph`/lucide.

## Risks / open questions

- **Cross-file globals.** The mockup shares helpers via global script scope
  (`Section`, `FGroupHead`, `FRow`, `formatSize`). The real port must use proper
  imports from `@/design-system`; no behavioural risk, just a rewrite.
- **Modified time.** Per-file modified date isn't stored. Either surface the
  version import date or drop the "Modified" column/field.
- **Large packages.** Virtualize grid/list and consider lazy tree expansion.
- **Subset affordance** (see D1) — resolved in Phase 3.
