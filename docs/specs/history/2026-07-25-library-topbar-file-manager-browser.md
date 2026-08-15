# Library top bar, rail account menu, and a file-manager file browser

**Status:** implemented — signed off and merged 2026-07-25 (PR #34, `d2453f8`).
Front-end only — no API, schema, or handler changes.

## Problem

Three related problems in the library and scoped-file-browser views.

1. **The library page always spends 248px on filters.** `LibraryPage` renders
   `LibraryFilterSidebar` unconditionally, so the asset grid is permanently narrowed
   whether or not any filter is in use. The toolbar then adds a `Library · N assets`
   breadcrumb that repeats what the library switcher (sitting in the sidebar header)
   already says.
2. **Account controls are scattered.** Sign-out lives in the filter sidebar's footer —
   which disappears with the sidebar — and is duplicated in `SignedOutSidebar`, a
   component that exists only so the no-library state has a way out. The theme toggle
   occupies its own slot in the nav rail.
3. **The scoped file browser doesn't behave like a file browser.** Its root view
   (`showAll`) is a flat listing of every file in the pack at every depth, and the
   sidebar tree lists every file leaf alongside folders. Neither matches how anyone
   expects to navigate a package's contents.

A fourth, smaller problem surfaced while reading the code: `LibraryFilterSidebar` writes
a `project_id` search param that `LibraryPage`'s `filters` memo never reads, so clicking
a project row highlights it, resets the grid selection, and filters nothing.

## Chosen approach

**Filters move behind a `Filter` toggle in the library toolbar, and the toolbar becomes
the page's identity row.** Left to right: library switcher (doubling as the title, with
the asset count as muted text beside it), `Filter` toggle, search input, spacer, Upload.

The filter surface stays the existing left sidebar, rendered conditionally. `AppShell`
already collapses its middle grid column to `0px` when `sidebar` is undefined, so this
needs a persisted boolean and a conditional prop — no new composition, and the resize
handle plus persisted width keep working while it's open. A popover was considered
(grid never loses width, but a deep tag tree fights a bounded height and outside-click
dismissal is hostile to multi-filter sessions) and an overlay sheet (new composition,
new a11y surface, obscures the grid it filters).

**Account controls consolidate into a rail-foot avatar menu** built on the existing
`DropdownMenu` primitive — the same one `LibraryPicker` uses. Identity block, theme
choice, Settings, Sign out.

**The file browser becomes a file manager.** Its root path takes the same
`nodesAtPath` route as any other folder (folders first, then files — `buildFileTree`
already sorts that way), and the sidebar tree renders folders only, pre-expanded to
depth 3. The caret keeps expand/collapse; the label navigates.

## Task breakdown

### 1 — Rail account menu

New `web/src/components/UserMenu.tsx`:

- Trigger: a `NavRailItem`-shaped avatar showing initials derived from `display_name`,
  falling back to `email`. Pinned last in the rail, after Settings.
- Content: identity block (display name + email), a theme radio group
  (`Light` / `Dark` — `ThemeMode` is exactly those two), separator, `Settings`, and
  `Sign out` calling the existing `lib/logout.ts:logout()`.
- `Shell.tsx`: mount `UserMenu`, drop the standalone theme `NavRailItem`.

Tests to update: `Shell.test.tsx` (its `useTheme` mock and any assertion about the
theme rail item) and `web/e2e/tests/oidc-callback.spec.ts:350`, which clicks
`Sign out` directly and must now open the menu first. That is the only e2e touching
sign-out.

Lands first because stage 2 removes sign-out's current home.

### 2 — Library toolbar and toggleable filter panel

`web/src/pages/LibraryPage.tsx`:

- Toolbar: `LibraryPicker` first (page title role) with the asset count beside it,
  then the `Filter` toggle, then the search input, spacer, `Browse files` when
  applicable, Upload. Remove the `Breadcrumb` / `BreadcrumbItem` usage.
- Filter panel open-state in `localStorage` under `damson.library.filters`, defaulting
  to closed. Pass `sidebar` to `Shell` only when open.
- The `Filter` button renders its active-filter count as a badge and holds its active
  (accent) state whenever any filter is set, so nothing filters invisibly.
- Add `project_id` to the `filters` memo (parsed as a number) and count it in the badge.
- Delete `SignedOutSidebar` and its stray `sr-only` `Search` icon; the no-library empty
  state keeps working because the rail — and therefore sign-out — renders regardless of
  library access.

`web/src/components/LibraryFilterSidebar.tsx`: drop the `header` (switcher moved) and
`footer` (sign-out moved) props; the component becomes filter sections only. `me` is
no longer needed as a prop if nothing else in it uses `me`.

`LibraryPicker`'s accessible name stays `Switch library`, so
`web/e2e/tests/library-switch.spec.ts` continues to pass unchanged.

### 3 — Root view is the root folder

`web/src/components/filebrowser/FileBrowser.tsx`:

- Remove the `showAll` branch. `flatMode` becomes `resultsMode`, so the flat listing —
  and the `RESULTS_CAP` of 300 that guards it — applies only to search / file-type
  results, which is where it was always load-bearing.
- Update the empty-state copy (the `showAll` arm goes away) and the toolbar count.
- Add a unit test asserting the root listing yields folders before files.

Knock-on to state plainly: `FileDetailPanel`'s prev/next stepper walks `listingFiles`,
which is now the current folder's files rather than the whole pack — stepping no longer
crosses folder boundaries. That is the file-manager behaviour, but it is a change.

### 4 — Folders-only tree, depth-3 expansion, caret semantics

`web/src/design-system/domain/FileTreeSidebar.tsx`:

- Render folders only. Delete the file-leaf branch of `TreeNavNode` and the
  `openFileKey` / `onOpenFile` props; opening a file is the grid's job. Update the
  `Section` title from `Files` to `Folders`.
- Keep `All files` as the root row: same label, same pack-total count, but it navigates
  to the root folder listing.
- Initial expanded set: every folder at depth 0–2 (so depth-3 folders are visible but
  shut), computed from the tree in a `useState` initialiser so re-navigating doesn't
  stomp manual collapses. The depth walk is a pure helper in `domain/fileTree.ts`, unit
  tested alongside `fileTree.test.ts`.
- Caret stays a sibling button that only expands/collapses. The label navigates *and*
  reveals that folder's children.
- **Reveal-on-navigate becomes a state write.** `expanded` is currently derived as
  `openFolders ∪ ancestorKeys(path)` at render time, which makes collapsing anything on
  the current path impossible — it re-opens on the next paint. Instead, union the new
  ancestors into `openFolders` when `path` changes; after that, manual toggles win, and
  the grid can keep showing a folder's contents while its subtree is shut in the tree.

Update `domain/FileBrowser.stories.tsx` for the changed props.

### 5 — Docs and verification

- `docs/design-system.md` — the `FileTreeSidebar` entry (folders-only, depth-3 default,
  caret/label split).
- `docs/progress.md` — a post-MVP slice entry pointing at this spec.
- `docs/e2e-flows.md` — only if the sign-out flow's documented steps change.
- Run `task check`, plus `web/e2e/tests/oidc-callback.spec.ts`,
  `library-switch.spec.ts`, `library.spec.ts` and `search.spec.ts`.

## Reuse

Everything here builds on components already in the tree; no new dependency, and one
new component total (`UserMenu`).

- `AppShell` — already collapses its sidebar column to `0px` when `sidebar` is
  undefined, and already persists sidebar width under `damson.sidebar.w`. The toggle is
  a conditional prop, not a layout change.
- `DropdownMenu` (+ `DropdownMenuLabel` / `Item` / `Separator`) — the account menu uses
  exactly the pattern `LibraryPicker` established.
- `NavRail` / `NavRailItem` — the avatar trigger is a rail item with a tooltip.
- `Toolbar` / `ToolbarSpacer` / `SearchInput` / `Button` — the new toolbar is a
  re-ordering of components already there.
- `FilterSidebar` / `FilterRow` / `Section` — the filter panel keeps its internals; only
  its header and footer slots are dropped.
- `fileTree.ts` — `buildFileTree` already sorts folders before files at every level, and
  `nodesAtPath` already returns a folder's children, so stage 3 is deletion rather than
  new logic. The only addition is the depth-walk helper for the initial expanded set.
- `lib/logout.ts` / `useTheme` — reused as-is by `UserMenu`.

Searched for an existing sheet/drawer composition (`FilterSidebar`, `DetailPanel`,
`DropOverlay`, `FullPageView`, the Radix primitives) — there is no `Sheet`, which is
part of why option 3 lost.

## Resolved questions

| Question | Decision |
|---|---|
| Filter surface | Toggleable left sidebar — matches the described interaction and reuses `AppShell`'s existing collapse. |
| Panel default state | Closed, persisted per browser under `damson.library.filters`. Reclaiming the space on first paint is the point of the change. |
| Folder row click | **Caret stays separate.** Caret = expand/collapse, label = navigate + reveal. Rejected the single-button (navigate + toggle) variant because it makes peeking into a subtree impossible without moving the grid. This is what forces reveal-on-navigate into state (stage 4). |
| "All files" row | Kept as the root row — same label, same pack-total count, now navigating to the root listing. No second flat-view row. |
| `project_id` filter bug | Fixed in this slice (stage 2), since `LibraryPage` and `LibraryFilterSidebar` are both open anyway. |
| A permanent flat "every file" view | Out of scope. After stage 3 the flat listing is reachable via a search term or a file-type filter. If it's wanted as a first-class mode later, the honest place is a third option on the grid/list segmented control. |
| Settings in both the rail and the account menu | Kept in both — cheap, and the menu is where people look. |
