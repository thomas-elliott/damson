# Digital Asset Manager — Frontend Design Brief

**Status:** Draft v2 **Author:** Thomas **Last updated:** 2026-04-25 **Companion to:** `docs/product-design-doc.md`, `docs/design-system.md`

> **v2 change:** Open questions from §9 resolved during the M2.5 design-system build. The authoritative implementation contract now lives in `docs/design-system.md`; this brief focuses on the *why* and the information architecture.

---

## 1. Purpose

This document is the design brief for the digital asset manager's frontend. It is intended to be usable as input to a design process (Figma, Claude Design, or equivalent) and as a reference for frontend implementation. It covers information architecture, navigation, core views, interaction patterns, and visual direction — not component-level specification, which will follow from design work.

The tool is a self-hosted web application for managing game-development assets with licence tracking, Unity package extraction, and versioning. Desktop is the primary target. Mobile is a view-only, limited experience. The project will eventually be open-sourced, so the design should feel legible to users who are not Thomas.

## 2. Visual direction

### Tone

Flat, information-dense, subtle. The tool is used by one person (or a small team) managing large libraries of assets over long periods. It should reward familiarity: keyboard-friendly, low-chrome, generous information per screen. Decorative elements should be rare and purposeful.

The closest visual reference is **Zed** — monochrome-leaning, restrained accents, precise type, clear hierarchy through weight and spacing rather than colour or ornament. Not Linear's soft gradients, not Raycast's branded warmth, not Obsidian's theme-driven feel. Zed's editor chrome, palette, and modal pickers are the target register.

### Palette and theming

- **Dark and light modes**, user-selectable, dark as default. Switching is driven by a `data-theme` attribute on `<html>`; tokens swap automatically via CSS custom properties.
- Dark mode is the "home" experience — asset thumbnails benefit from a dark surround, and the tool leans into that.
- Surfaces: four luminance tiers (`bg`, `panel`, `surface`, `surface-2`) in a narrow range rather than heavy contrast. Borders use two tiers (`line`, `line-2`).
- Accent: **yellow, one accent only** (`oklch(0.82 0.15 86)` in dark mode). Used sparingly for interactive affordances, current-selection state, and the brand mark. No theme-swap variants.
- Licence status has its own small palette: `ok` (green), `warn` (amber), `err` (red), `unk` (neutral grey) — each with a matching `-tint` for low-saturation backgrounds. These are the only places colour carries semantic load beyond the accent.
- Text tiers are `text`, `text-2`, `text-3`, `text-4`. The tokens were tuned during M2.5 so `text-3` meets WCAG AA on `bg`/`panel` and `text-4` stays legible enough for decorative labels; see `docs/design-system.md` for the contrast matrix.
- All colours are OKLCH; the token source is `web/src/design-system/tokens/tokens.css`.

### Typography

- Inter for all UI text, JetBrains Mono for hashes, file paths, version identifiers, licence SPDX IDs, and any "copy-exact" technical fragment. Both fonts are self-hosted via `@fontsource/*`.
- Body is 13px / 1.45 line-height. The full scale is 10.5/11/11.5/12/12.5/13/14/16/20/22 — dense, information-first, with weight doing most of the hierarchy work.

### Components

Custom components hand-rolled on **Radix UI headless primitives** (`@radix-ui/react-*`) for correct a11y / keyboard / focus behaviour, styled with Tailwind utilities over OKLCH tokens. Component variants use `class-variance-authority`; `cn()` composes classes through `tailwind-merge`.

The component library is organised in three tiers under `web/src/design-system/`:

- **Primitives** — Button, Input, Dialog, Tabs, Tooltip, etc. Atomic, no domain knowledge.
- **Compositions** — AppShell, NavRail, FilterSidebar, Toolbar, DetailPanel, etc. Reusable shell frames.
- **Domain** — LicencePill, AssetCard, ProjectCard, QueueItem, etc. Damson-specific vocabulary.

See `docs/design-system.md` for the full inventory, variants, and props.

## 3. Information architecture

### Top-level structure

The app is **library-first**. The library is the default and dominant surface. Other top-level destinations exist but are not peers to the library in daily use — they are accessed deliberately.

```
Library       ← default, dominant
Projects      ← project-scoped views
Collections   ← user-curated groupings
Import queue  ← pending imports needing review
Audit log     ← historical record
Settings      ← configuration
```

Navigation between these lives in a persistent, narrow left rail (icon + label, or icon-only in a collapsed state). The rail is the only global navigation; there is no top bar beyond a search/command affordance.

The top of the nav rail carries a **library switcher** — the current library's name (truncated if long), clickable to reveal the user's other library memberships. If the user belongs to exactly one library the switcher is visible but inert (no dropdown), which keeps the layout stable between solo and shared installations. Switching library reloads the library view; filter and selection state do not carry across.

### Library structure

The library view has a three-column layout on desktop:

```
[ nav rail ] [ filter sidebar ] [ asset grid / asset detail panel ]
```

- **Nav rail** — global navigation between top-level destinations. Narrow, persistent.
- **Filter sidebar** — library-scoped filters and search. Contains the search input, tag tree, collections, projects, licence filters, source filters, file-type filters. Persistent; collapsible.
- **Main area** — the asset grid. When an asset is selected, the detail panel replaces or splits this area (see §4.3).

### Mobile adaptation

On narrow viewports the layout collapses:

- Nav rail becomes a bottom bar or hamburger menu
- Filter sidebar becomes a drawer, opened from a filter button
- Asset grid becomes the full surface
- Asset detail is a full-screen route
- Editing affordances are hidden (no upload, no extraction, no version changes, no licence editing)
- The mobile experience is explicitly viewer-only for v1

## 4. Core views

### 4.1 Library (default view)

The asset grid is the heart of the app. Thumbnails at three selectable densities (small/medium/large), square aspect ratio, with the asset name and a compact licence indicator beneath. File-type is conveyed by a small icon or badge for non-image assets; images show their own thumbnail, audio shows a waveform strip, fonts show a rendered sample, unknown types show a generic icon.

Sorting and view controls sit above the grid in a thin bar: sort field, sort direction, density toggle, grid/list toggle. List view shows the same information in a denser table: thumbnail, name, source, licence, tags, version, size, imported.

The filter sidebar drives what appears in the grid. Filters compose: selecting a tag and a licence type shows assets matching both. The search input at the top of the sidebar does full-text over name, description, and tag paths; it composes with the other filters rather than replacing them.

Empty state: a minimal message ("Drop files here to begin, or click to select") centred in the grid area, with the grid-area itself serving as a drop target. No wizard, no tooltip tour.

### 4.2 Asset grid interaction

Assets are selectable — single-click selects (and opens the detail panel), shift- or cmd-click extends selection for bulk operations. Bulk operations available from the selection bar: add to project, add to collection, add tags, delete. Bulk licence editing is deliberately excluded in v1 (too risky without per-asset confirmation).

Drag-and-drop from the OS onto the grid area triggers import at any time, regardless of whether an asset is selected.

### 4.3 Asset detail

Opens as a right-side panel over the grid when an asset is clicked. The grid remains visible and scrollable behind the panel so the user retains context; navigation between adjacent assets via keyboard arrows is supported.

The detail panel is organised into sections, stacked vertically, each collapsible:

1. **Header** — thumbnail, name, current version label, licence pill, tag chips. Inline-editable name and tags.
2. **Versions** — list of all versions with file count, import date, notes. Selecting a version scopes the rest of the panel to that version. Actions: pin to project, download original package (if retained), re-extract (for Unity packages).
3. **Files** — the manifest for the selected version, as a tree. Per-file: relative path, size, detected type, hash. Per-file actions: download, preview where supported.
4. **Licence** — detected type, confidence, diff status (if modified from canonical), structured attributes (commercial, modification, redistribution, attribution, share-alike, open-source), and the full original licence text (collapsible, monospace).
5. **Projects** — list of projects using this asset, with the pinned version per project, and the user who pinned it.
6. **Collections** — list of collections containing this asset.
7. **Source** — origin URL, free-form notes.
8. **Audit** — recent audit entries for this asset, with a link to the full audit log filtered to this asset.

An "open as full page" action promotes the panel to a dedicated route for deep work on a single asset, preserving the same section structure.

### 4.4 Projects

Projects have their own top-level destination. The projects index is a simple list or card view with name, description, distribution profile, asset count, last updated.

A project detail page uses a similar three-column layout: nav rail, project-scoped sidebar (filters scoped to assets pinned in this project), and main area showing pinned assets. The project detail is explicitly its own view rather than a filter on the library because projects carry their own concerns — distribution profile, release state later, audit scoped to the project.

Pinning and upgrading are surfaced in the project view: a pinned asset shows its pinned version, and if a newer version exists, a non-intrusive indicator flags that an upgrade is available. Upgrading is an explicit action with a confirmation and an audit entry.

### 4.5 Collections

Similar structure to projects but lighter — collections exist for organisation rather than licence scope. An asset's presence in a collection has no functional consequence; it only affects browsability. A collection detail page is a filtered library view with collection-specific add/remove affordances.

### 4.6 Import queue

Because import is background-first (see §5.1), newly imported assets land in an import queue until they have been reviewed and have their metadata confirmed. The queue is a dedicated view accessible from the nav rail with a badge showing unreviewed count.

Queue items show: filename, import timestamp, detected type, detected licence (if any), and a "review" action that opens the asset detail panel focused on the fields needing confirmation. Assets can be used before review, but unreviewed status is visible in the grid via a subtle indicator.

### 4.7 Audit log

A chronological list of all write actions, filterable by user, action type, target type, date range. Each entry shows who, when, what, and a structured diff where applicable. Entries link to the target (asset, project, collection) when it still exists. The audit log is read-only.

### 4.8 Settings

Runtime-configurable settings live here. Settings split across two scopes, each with their own sections in the UI:

**Library settings** (library admins) — apply to the currently active library:
- Appearance (theme, grid density default)
- Storage (retain originals, deduplication within the library)
- Thumbnails (per-type toggles)
- Audit (retention policy)
- Members (list, invite by email, role changes — library admins can invite members and other admins)
- Library (name, description, delete library)

**Server settings** (sysadmins only) — apply to the whole server:
- Users (list of all users across libraries, role changes between `user` and `sysadmin`)
- Libraries (list, create new library, assign an admin)
- Licences (knowledge base version, custom licence templates)
- Limits (max upload size)

See §6 for the split between in-app and env-var configuration. Authentication (OIDC provider) is env-var only.

## 5. Interaction patterns

### 5.1 Import flow

Import is **background-first**. The user drops a file or files onto the grid area (or a persistent drop zone in the nav rail, for when the grid is not in focus). The upload starts immediately. The user is not interrupted; they continue browsing.

For `.unitypackage` files, extraction runs in the background after upload. The extraction itself produces metadata (folder structure, preview images embedded in the package, detected asset names) that is used to pre-fill the new asset's fields. The result: most assets arrive with enough metadata that the user confirms rather than fills.

All newly imported assets land in the import queue, visible with a badge on the nav rail. Assets in the queue behave normally — they can be tagged, added to projects, searched — but they carry a "needs review" marker until the user confirms. Confirmation is a single click on the asset detail panel, once metadata looks right.

Unity package subset selection is **not** a blocking picker in this model. Full extraction happens on import. If the user later wants only a subset of the extracted files as "the asset," they can do so through the asset detail via a file selection and subset action. This keeps the import path fast and lets subset decisions happen with full information visible.

### 5.2 Search and filter

- Search input always visible at the top of the filter sidebar; `/` or `cmd+k` focuses it
- Filters below the search input, composable: tag tree, collections, projects, licence types, source, file type
- Active filters show as dismissable chips above the grid, giving a clear view of what's currently applied
- Clearing all filters returns the library view to unfiltered
- Saved searches are out of scope for v1

### 5.3 Keyboard

Keyboard support is a first-class concern given the information-dense, daily-use nature of the tool. Minimum:

- `/` or `cmd+k` — focus search
- Arrow keys — navigate grid
- `enter` — open selected asset detail
- `escape` — close detail panel, clear selection
- `cmd+a` — select all in current view
- Standard navigation shortcuts for adjacent assets in detail view

A full keyboard shortcut surface is a later refinement; v1 should cover the common path.

### 5.4 Drag-and-drop

- OS file → grid or drop zone → import
- Asset → project in sidebar → add to project with version pin prompt
- Asset → collection in sidebar → add to collection

### 5.5 Thumbnails

Generated on import synchronously for v1 (simple, blocking only the thumbnail step, not the upload). Covered for MVP:

- Raster images (direct)
- Audio (waveform strip)
- Fonts (sample-text render)
- `.unitypackage` preview harvest (embedded previews used where present)
- All others: file-type icon fallback

A background job queue for thumbnails (for later addition of 3D, video) is deferred. The frontend should not assume thumbnails are synchronous; design patterns should tolerate a "thumbnail pending" state even if v1 never uses it, so the later shift to async generation is purely backend.

## 6. Configurability in the UI

The philosophy is: **env vars for bootstrap and secrets only; everything else in the database, surfaced in Settings.**

### Env-var configuration (not in UI)

- Database connection
- Object storage endpoint and credentials
- OAuth2 provider configuration and secrets
- Initial admin user bootstrap
- Application base URL

### In-app settings (database-backed, editable in Settings)

- Theme default, grid density default
- Retain original `.unitypackage` after extraction (default: on)
- Content-hash deduplication (default: on)
- Max upload size
- Per-file-type thumbnail generation toggles
- Audit log retention period
- Licence knowledge base update policy
- User management (add/remove, roles where applicable)

Settings changes take effect immediately where possible, are recorded in the audit log, and carry a brief description clarifying their consequence (e.g. "Disabling original retention will apply to future extractions only; existing originals are not deleted").

## 7. Open-source considerations

The design should remain legible to users who are not the primary author. Specific implications:

- **Empty states explain the concept, not just the action.** "No projects yet. A project is a game you're working on — it pins specific asset versions and tracks licence compliance." One sentence, not a wizard.
- **Settings descriptions are explicit about what each toggle does.** Assume a user who has not read the design doc.
- **Terminology is consistent.** Asset, version, project, collection, licence, grant, pin, import queue. These are the nouns; the UI uses them uniformly.
- **No Thomas-specific aesthetic flourishes** — the restraint of the visual direction pays off here. Flat, subtle, monochrome-leaning makes the tool feel like a neutral utility rather than a personal artifact.

## 8. Out of scope for design (v1)

- Detailed licence compliance report views (data captured, UI later)
- Release snapshot exports
- Upstream update indicators beyond "a newer version exists"
- 3D, video, or rendered-model thumbnail treatments
- Content-aware search result presentation
- Auto-tagging UI
- Mobile editing affordances
- Saved searches / filter presets
- Plugin / extension UI (for later extension point work)

## 9. Resolved design questions

Resolved during the M2.5 design-system build (2026-04-25). Implementation lives in the components listed.

- **Accent colour** → **yellow only** (`oklch(0.82 0.15 86)`). No theme-swap variants. Tokens: `--ds-accent`, `--ds-accent-2`, `--ds-accent-tint`, `--ds-accent-ring`.
- **Detail panel vs. full-page** → right slide-in is the dominant pattern; full-page is a promoted route for deep work. Both share the same section structure. Components: `DetailPanel` (`overlay` / `flush` variants) and `FullPageView`.
- **Import queue visibility** → badge on the nav rail, visible only when count > 0. Component: `NavRailItem` `badge` prop.
- **Versions UI** → linear list with current-version highlight (accent left bar + tint). Component: `VersionItem` with `current` prop.
- **Licence attribute presentation** → structured attribute rows inside the Licence section of the detail panel; `KeyValueList` pattern with boolean values rendered as `StatusDot` + label. Pill toggles rejected (too easy to mis-click for something with legal weight).
- **Dark/light toggle** → quick toggle in Settings surface and via a future global tweaks menu (M6). Storybook already provides a per-view toolbar toggle for design work.
- **Grid thumbnail aspect ratio** → square tiles with object-cover (crop). Preserving aspect ratios in a grid caused too much vertical jitter; the detail panel's full-preview area is the place for true ratios. Waveform, sprite, and font thumbnails all render square.

### Still open

- Licence attribute matrix — how to present commercial/modification/redistribution/attribution/share-alike/open-source in the detail panel. Currently falls out of the structured-attribute-rows decision above, but the exact row-vs-icon-grid question is deferred to the M3 licence-editor slice.
- Saved searches / filter presets — still out of scope for v1 per §8.