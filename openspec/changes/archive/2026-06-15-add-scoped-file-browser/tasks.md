# Tasks — Scoped file browser

## Phase 1 — Design system + Storybook

- [x] 1.1 Add icons through the `Glyph`/lucide surface: `FolderOpen`, `Cube`, `Code`, `Scene`, `Sphere`, `Play`, `CornerArrow`; story coverage in the existing icon/display story. _(lucide equivalents wired in `fileTypes.tsx` `FILE_TYPE_META`: Box=Cube, Circle=Sphere, Clapperboard=Scene, etc.; shown in the `Glyphs` story.)_
- [x] 1.2 Build `FileThumb` / `FilePlaceholder` (inline image preview vs type-styled placeholder) + story.
- [x] 1.3 Build `FolderCard` and `FileCard` (grid tiles, selected/open states) + stories.
- [x] 1.4 Build `FileBrowserList` (list/table view, folders + files) + story.
- [x] 1.5 Build `FileTreeSidebar` (scope header, search, tree, file-type filter) composing existing `FilterSidebar`/`SearchInput`/`Section` patterns + story.
- [x] 1.6 Build `FileBrowserToolbar` (breadcrumb path, results/filter chips, grid/list toggle) composing existing `Breadcrumb`/`SegmentedControl` + story.
- [x] 1.7 Build `FileDetailPanel` reusing the existing `DetailPanel` chrome (hero, metadata, path bar, download/subset actions) + story.
- [x] 1.8 Build `FileSelectionBar` (count, clear, create-subset) + story.
- [x] 1.9 Verify all stories pass Storybook a11y; confirm tokens (no hard-coded colours/sizes). _(9/9 FileBrowser story tests pass incl. axe a11y; fixed a nested-interactive violation in the tree rows. eslint/prettier not installed in this env — not run.)_

## Phase 2 — Browse experience (read-only)

- [x] 2.1 Add an extension→type taxonomy helper (drives icon, filter, preview style; consults `detected_type` only when present). _(`fileTypeFromName`/`extOf` in `fileTypes.tsx`; unit-tested.)_
- [x] 2.2 Add a tree-from-manifest helper: group `useAssetDetail().files[]` into a folder tree by splitting `relative_path`; helpers for nodes-at-path, file counts, flatten, type counts. _(`buildFileTree`/`nodesAtPath`/`flattenFiles`/`typeCounts` in `fileTree.ts`; unit-tested.)_
- [x] 2.3 Add `isBrowsable(asset)` (current version `files > 1`) and the "Browse files" trigger on `AssetGrid` toolbar and `AssetDetailPanel`. _(`isBrowsable(fileCount)`; trigger in the LibraryPage toolbar (selected browsable asset) + AssetDetailPanel header icon and Files-section "Browse all N files" button.)_
- [x] 2.4 Wire enter/exit browser state into the library page (swap view; clear state on exit and on nav-rail route change). _(Route `/library/:assetId/browse` → `LibraryPage browse`; renders `FileBrowser` in place of grid+detail. Nav-rail navigation changes route, exiting browse.)_
- [x] 2.5 Implement tree navigation, breadcrumb, and grid/list rendering of the current folder; virtualize large listings. _(Folder nav/breadcrumb/grid/list done. Folders are naturally bounded; flat search results are capped at 300 with a visible "refine" note rather than virtualized — see note below.)_
- [x] 2.6 Implement in-pack search + file-type filter (flat results mode with folder paths; clearable).
- [x] 2.7 Implement file open → `FileDetailPanel`: image preview, placeholder otherwise, prev/next stepping, download. _(Preview can't use `<img src>` — the endpoint forces octet-stream/attachment/nosniff + bearer-only auth; uses `fetchBlob`→object URL via `useFileObjectUrl`, detail-panel only. See design.md D5.)_
- [x] 2.8 Implement browser-mode keyboard shortcuts (Esc steps up the hierarchy; arrows step files). _(LibraryPage's grid shortcuts stand down in browse mode to avoid Escape/arrow conflicts.)_
- [x] 2.9 Verify the AppShell bounds its content area so floating bars stay in the viewport. _(AppShell already bounds via `grid h-screen grid-rows-1 overflow-hidden`; the browser's content column uses `relative` + `min-h-0` scroll child. No app-wide CSS change needed.)_

## Phase 3 — Subset from the browser (supersedes the flat dialog)

- [x] 3.1 Implement file selection in the browser (shift/meta/ctrl-click) with `FileSelectionBar` visibility/clear. _(Drag-to-target dropped — the dock collapsed to a single subset verb, leaving no drop target; see design.md D6.)_
- [x] 3.2 Resolve the selection→subset affordance (single "Create subset" verb) and wire it to `POST /assets/:id/versions/:vid/subset`. _(Single verb in `FileSelectionBar`, via existing `useCreateSubsetVersion`.)_
- [x] 3.3 Show success confirmation (toast) and refresh affected asset/version queries. _(Transient "Subset version created" flash via `useSavedFlash`; the mutation invalidates the asset query, rebuilding the tree.)_
- [x] 3.4 Retire `SubsetVersionDialog` and its trigger; remove dead code and stories. _(Deleted `SubsetVersionDialog.tsx`; removed its trigger/state/imports from `AssetDetailPanel`. It had no story.)_
- [x] 3.5 Update any e2e/flow docs that reference the flat subset picker. _(e2e tests use the subset *endpoint* via fixtures, not the dialog UI — unaffected. Updated `architecture.md` to point at the browser.)_

## Cross-cutting

- [x] C.1 Unit tests for the taxonomy helper and tree-building helpers. _(`fileTypes.test.ts` + `fileTree.test.ts`, 19 tests.)_
- [x] C.2 Story tests (Vitest) for the new components. _(Phase 1 components covered by `FileBrowser.stories.tsx`, run under the storybook vitest project.)_
- [x] C.3 Update `docs/design-system.md` with the new components. _(Added a "File browser" subsection under Domain components; also updated `architecture.md` for the subset UI.)_
