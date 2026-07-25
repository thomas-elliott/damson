# Damson Design System

**Status:** v1 (M2.5) **Last updated:** 2026-04-25

The authoritative implementation contract for Damson's frontend. The mockup at
`docs/mockup/` is the visual reference; this document is what the code
actually does.

Companions: `docs/architecture.md` (architecture, data model, API),
`docs/CONVENTIONS.md` (stack, developer commands),
`docs/archive/ui-design.md` (the original information-architecture
brief — kept for historical reference).

---

## Overview

The design system lives at `web/src/design-system/` and is the only place new
UI primitives are built. Feature code in `web/src/pages/` and
`web/src/components/` composes the system rather than re-rolling it.

Four principles drive every decision:

1. **Information density over decoration.** Tight spacing, restrained
   type scale, flat surfaces. Zed is the nearest reference.
2. **Tokens first, utilities second.** All colour/spacing/motion is a
   named CSS custom property. Tailwind utilities map to those tokens.
3. **Headless behaviour, hand-rolled style.** Radix primitives supply
   a11y/keyboard/focus; we supply every visual rule.
4. **Every component ships a story.** Stories are the visual contract
   *and* the a11y test — Vitest runs them as real-browser tests.

---

## Architecture

```
web/src/design-system/
├── tokens/              OKLCH tokens (CSS vars) + TS helpers
├── utils/               cn(), cva re-export, formatBytes
├── primitives/          Atoms: Button, Input, Dialog, Tabs, …
├── compositions/        Shell frames: AppShell, NavRail, DetailPanel, …
├── domain/              Damson-specific: LicencePill, AssetCard, …
├── icons/               (reserved for centralised lucide re-exports)
└── index.ts             Barrel — consumers import from "@/design-system"
web/.storybook/          Storybook config + a11y parameter + vitest setup
```

Components:

- Use `cn()` from `design-system/utils` to compose classes (via
  `tailwind-merge` — custom font sizes are registered so they don't get
  deduplicated against colour classes).
- Use `cva()` for typed variants (`variant`, `size`, `tone`, etc.).
- Forward refs where callers might need them (form controls, Radix
  composition).
- Set `data-*` attributes for state so stories can exercise them without
  real interaction (e.g. `data-pressed`, `data-active`).

---

## Tokens

Source: `web/src/design-system/tokens/tokens.css`. Dark is the default
(`:root`); light overrides on `html[data-theme="light"]`.

### Neutrals + text

| Token | Dark L | Light L | Purpose |
|---|---|---|---|
| `--ds-bg` | 0.195 | 0.985 | Page background |
| `--ds-panel` | 0.225 | 0.965 | Sidebars, panels |
| `--ds-surface` | 0.258 | 0.945 | Cards, hover, surface |
| `--ds-surface-2` | 0.295 | 0.92 | Higher elevation |
| `--ds-line` | 0.325 | 0.88 | Hairline dividers |
| `--ds-line-2` | 0.385 | 0.82 | Stronger borders |
| `--ds-text` | 0.93 | 0.20 | Primary text |
| `--ds-text-2` | 0.78 | 0.38 | Secondary |
| `--ds-text-3` | 0.70 | 0.52 | Tertiary — meets WCAG AA on `bg`/`panel` |
| `--ds-text-4` | 0.60 | 0.65 | Decorative / labels; meets AA on `bg` only |

All OKLCH, hue 70–80 (very slight warm bias). `text-4` is deliberately
at the edge of legibility — reserve it for decorative/ornamental text
(ChevronRight separators, unused states); reach for `text-3` for anything
informational.

### Accent + status

| Token | Dark | Purpose |
|---|---|---|
| `--ds-accent` | `oklch(0.82 0.15 86)` | Primary action, current selection, brand |
| `--ds-accent-2` | `oklch(0.74 0.15 86)` | Hover / pressed |
| `--ds-accent-tint` | `.../ 0.16` | Subtle accent background |
| `--ds-accent-ring` | `.../ 0.35` | Focus ring |
| `--ds-accent-fg` | `oklch(0.18 0.02 86)` | Foreground on filled accent bg |
| `--ds-ok` / `--ds-ok-tint` | `oklch(0.72 0.12 155)` | Compliant licence |
| `--ds-warn` / `--ds-warn-tint` | `oklch(0.78 0.13 85)` | Needs review |
| `--ds-err` / `--ds-err-tint` | `oklch(0.68 0.17 28)` | Violation / destructive |
| `--ds-unk` / `--ds-unk-tint` | `oklch(0.72 0.01 80)` | Unknown status |

### Typography

- `--ds-font-sans`: Inter (self-hosted via `@fontsource/inter`)
- `--ds-font-mono`: JetBrains Mono (self-hosted via `@fontsource/jetbrains-mono`)
- Body: 15px / 1.5 line-height, `cv11 + ss01` OpenType features.

The font-size scale is two-layered: raw size primitives compose into
role-named semantic tokens. Authors should reach for the role tokens
(`text-title`, `text-label`, etc.) by default; the size primitives exist
for the rare cases where role tokens don't fit.

**Layer 1 — size primitives** (Tailwind `fontSize` entries; size +
line-height only)

| Utility | px / line-height | Used for |
|---|---|---|
| `text-body` | 15 / 1.5 | Default running text. Also the `<body>` default. |
| `text-ui` | 14 / 1.4 | Buttons, inputs, list rows, table cells, chip / tag / badge text. |
| `text-small` | 12 / 1.4 | Captions, labels, mono hashes, kbd hints. |

**Layer 2 — composed semantic tokens** (declared in `globals.css` under
`@layer components`; bake in size + weight + transform + tracking + family)

Headings (each has 4 weight variants; the bare alias = `-semibold`):

| Token | px / line-height | Pairs with |
|---|---|---|
| `text-title-{reg,medium,semibold,bold}` | 22 / 1.25 | `<h1>` (page heading) |
| `text-heading-{reg,medium,semibold,bold}` | 20 / 1.3 | `<h2>` (section heading) |
| `text-subhead-{reg,medium,semibold,bold}` | 18 / 1.35 | `<h3>` (dialog title, asset detail title) |

Generic role tokens:

| Token | Composes | Used for |
|---|---|---|
| `text-label` | `text-small font-medium uppercase tracking-wide` | Section labels, kbd-row labels. |
| `text-mono` | `font-mono text-small` | Sha hashes, IDs, file paths, short code refs. |
| `text-caption` | `text-small text-text-3` | Helper text under inputs, muted captions. |
| `text-num` | `text-ui font-mono` + tabular numerals | Inline numeric data. |

View-specific role tokens:

| Token | Composes | Used for |
|---|---|---|
| `text-table-row` | `text-ui` | Body cells in `<table>`. |
| `text-table-header` | `text-ui font-medium` | `<thead>` cells. |
| `text-list-row` | `text-ui` | Rows of dense list views (e.g. `AssetListRow`). |

### Sizing, radii, motion, z-index

- Sizes: `--ds-sidebar-w: 248px`, `--ds-rail-w: 52px`, `--ds-detail-w: 440px`
- Radii: `--ds-r-sm: 4px`, `--ds-r-md: 6px`, `--ds-r-lg: 10px`, `--ds-r-pill: 999px`
- Transitions: `--ds-transition-fast/std/slow` = 80/100/120ms
- Easings: `--ds-ease` (0.4, 0, 0.2, 1), `--ds-ease-bounce` (0.34, 1.56, 0.64, 1)
- Z-index: `resizer=5`, `detail=10`, `drop=20`, `selbar=30`, `tooltip=100`,
  `dialog=500`, `tweaks=1000`

### Tailwind mapping

Tokens become Tailwind utilities via `tailwind.config.ts`. **Prefer the
utility over `var(--ds-*)`** — e.g. `bg-panel`, `text-text-2`,
`border-line`, `rounded-md`, `animate-slide-in-right`.

---

## Primitives

Atomic components with no domain knowledge. All live under
`src/design-system/primitives/`.

### Action

- **`Button`** — `variant: primary|secondary|ghost|danger`, `size: sm|md`.
  `leadingIcon`/`trailingIcon` slots. `pressed` state via
  `data-pressed`. Focus ring on `focus-visible`.
- **`IconButton`** — square `size: sm|md` (24/34px), `active` highlights
  with surface bg, used in toolbars and rails.
- **`SegmentedControl<T>`** — generic over a value type; options can be
  icon-only or text. Used for grid/list toggles and density picks.

### Input

- **`Input`** — leading icon slot + trailing slot, `invalid` state sets
  `aria-invalid`, focus-within applies the accent ring.
- **`SearchInput`** — preset over `Input`: Search icon + optional `kbd`
  hint (⌘K).
- **`Textarea`** — resizable vertical, same invalid/disabled semantics
  as `Input`.
- **`MarkdownEditor`** — markdown source editor on CodeMirror 6
  (`@codemirror/lang-markdown`) with source highlighting mapped onto the
  tokens, wrapping, and undo history. CodeMirror is code-split behind
  `lazy` (`MarkdownEditorView.tsx` is the host), so read-only views never
  load it. Its surface is a `contenteditable` with `role="textbox"`, which a
  `FieldRow` `<label for>` cannot name — callers **must** pass `aria-label`
  (the prop is required for that reason).

### Display

- **`Badge`** — `tone: neutral|accent|ok|warn|err|unk`,
  `shape: rect|pill`, `size: sm|md`, optional `mono`.
- **`Pill`** — preset over Badge with `shape="pill"`, kept as a separate
  name because it reads better at call sites.
- **`Chip`** — dismissable filter chip: `label` prefix + bold value +
  close button. Calls `onRemove`.
- **`Tag`** — `variant: solid|add`. The `add` variant is dashed-border
  with a Plus icon for "add new tag" affordances.
- **`Kbd`** — inline keyboard hint, mono 10.5px with a bordered surface.
- **`Markdown`** — renders CommonMark + GFM as React elements, parsed with
  CodeMirror's own `@lezer/markdown` grammar so the read view and
  `MarkdownEditor` can't disagree about a document. Safe for untrusted
  input by construction: no HTML is interpreted (raw tags render as text),
  link targets are restricted to `http(s)`/`mailto`, and images render as
  labelled links out rather than `<img>` so a store description can't
  hot-link third-party requests. Covered by `Markdown.test.tsx`.
- **`StatusDot`** — 6/8px coloured dot, optional `ring` for "on a
  background" use.
- **`ProgressBar`** — 3/4px high, `tone: accent|ok|warn|err`. Caller
  controls `value` 0–100.
- **`Glyph`** — coloured identity square, `size: sm|md|lg`. Accepts
  `color` (bg) and `textColor` overrides so callers can maintain
  contrast for low-L backgrounds.

### Radix-backed composites

Each of these exports the composable Radix parts (Root / Trigger /
Content / …) with styling applied, so consumers can compose naturally
while inheriting the design.

- **`Checkbox`** — Radix Checkbox with lucide Check indicator; supports
  indeterminate.
- **`Slider`** — thin 3px track, circle thumb, accent fill.
- **`Separator`** — horizontal (full width, 1px) or vertical (18px high,
  1px wide).
- **`Tabs`** + **`TabsList`** + **`TabsTrigger`** + **`TabsContent`** +
  **`TabCount`** — underline-style active indicator, optional count
  badge next to the label.
- **`Tooltip`** (+ `TooltipProvider`, `TooltipTrigger`,
  `TooltipContent`, `SimpleTooltip`) — 250ms delay, mono text, surface-2
  background. `TooltipProvider` is mounted once in `main.tsx`.
- **`DropdownMenu`** + `Content` + `Item` + `CheckboxItem` +
  `RadioItem` + `Label` + `Separator` + `Shortcut`.
- **`Dialog`** (+ `DialogTrigger`, `DialogContent`, `DialogOverlay`,
  `DialogHeader`, `DialogBody`, `DialogFooter`, `DialogTitle`,
  `DialogDescription`, `DialogClose`) — scrim with backdrop-blur, 440px
  default width, pop-up animation.
- **`Popover`** + `Content` — lighter surface container than
  DropdownMenu, for rich content (detail previews, filter helpers).
- **`Select`** + `Trigger` + `Content` + `Item` + `Label` + `Separator`
  — Radix Select with ChevronDown trigger.

### Layout

- **`ScrollArea`** — Radix ScrollArea with custom thin scrollbar
  matching the mockup. Viewport is `tabIndex={0}` to satisfy axe's
  `scrollable-region-focusable` rule.

---

## Compositions

Shell frames under `src/design-system/compositions/`. Feature pages
orchestrate these rather than rebuilding the layout each time.

- **`AppShell`** — 3-column CSS grid: rail (52px) | sidebar (adjustable,
  default 248px) | main (fills). Sidebar is optional; when omitted the
  grid collapses to rail + main.
- **`NavRail`** + **`NavRailItem`** + **`NavRailSpacer`** — vertical
  icon nav with tooltip labels (250ms delay), `badge` prop for counts,
  `active` renders a left accent bar. `brand` slot at top.
- **`ResizeHandle`** — 4px draggable separator with keyboard support
  (Arrow keys = 8px, Shift+Arrow = 32px) and min/max clamp. Consumer
  owns the resized element's width.
- **`FilterSidebar`** + **`FilterRow`** — scrollable frame with sticky
  header + footer. Rows take `icon`/`dot`/`label`/`count`/`active` and
  auto-apply the accent-tint highlight when pressed.
- **`Section`** — collapsible section with rotating caret. Controlled
  (`open`/`onOpenChange`) or uncontrolled (`defaultOpen`); wires
  `aria-expanded`/`aria-controls`.
- **`Toolbar`** + **`ToolbarSpacer`** — 42px min-height toolbar bar with
  border-bottom. Compose breadcrumbs + chips + buttons + segmented
  controls inside.
- **`Breadcrumb`** + **`BreadcrumbItem`** + **`BreadcrumbSeparator`** —
  ancestry text with optional count suffix.
- **`SelectionBar`** — floating pill that appears when `count > 0`,
  centered bottom of container, pop-up animation. `actions` slot on the
  right.
- **`DetailPanel`** — slide-in panel with sticky header (title + action
  slot + close button) and scrollable body. `variant: overlay|flush`
  (overlay floats above main with shadow; flush sits in flow). When
  `resizable` is set on an overlay, a left-edge drag handle exposes
  `width` / `onWidthChange` for caller-controlled resize (use
  `minWidth` / `maxWidth` to clamp). `fullscreen` makes the panel
  fill its positioned ancestor and hides the resize handle.
- **`FullPageView`** — full-page alternative to `DetailPanel`: grid of
  (preview | 440px side), both independently scrollable. Shared header
  on top.
- **`DropOverlay`** — absolute-inset dashed drop target indicator,
  `pointer-events: none` so the drop handler stays on the parent.
- **`EmptyState`** — centered icon + title + description + optional
  action.

---

## Domain components

Damson-specific vocabulary under `src/design-system/domain/`. Still
reusable across features, but pre-assume the application's concepts
(licences, assets, projects, queue items).

- **`LicencePill`** — SPDX identifier with status tone; optional leading
  `StatusDot`. Always mono.
- **`LicenceBar`** — stacked proportional bar (ok/warn/err/unk) with
  auto-generated a11y label.
- **`AssetThumb`** — entry point for placeholder thumbnails. Dispatches
  to one of:
  - `Waveform` — deterministic SVG bars (seeded RNG), used for audio.
  - `SpriteThumb` — 9 procedural patterns (`abstract`, `landscape`,
    `character`, `tiles`, `ui`, `portrait`, `gradient`, `fire`, `icons`)
    parameterised by a 1–4 colour palette. Ported byte-identical from
    `docs/mockup/src/thumbnails.jsx`.
  - `FontThumb` — typeface sample text.
  - `UnknownThumb` — file-icon fallback.
- **`AssetCard`** — grid tile with thumb + name + sub line + trailing
  slot (licence pill usually). `typeBadge` overlay on thumb,
  `needsReview` dot in corner, `selected` state.
- **`AssetListRow`** + **`AssetListHead`** — table-row variant for
  dense list views. 22px mini-thumb + name + caller-provided cells.
- **`ProjectGlyph`** — wrapper over `Glyph` semantically tied to a
  project.
- **`ProjectCard`** + **`ProjectCardNew`** — project card (glyph +
  name + path + engine + stats slot) and dashed "new project" card.
- **`ProjectListRow`** — grid-column row variant.
- **`CollectionCard`** — manual or `smart` variant. Smart shows a
  SMART badge and a mono query description.
- **`QueueItem`** — import queue row with 56px thumb, progress bar,
  actions.
- **`KeyValueList`** — two-column `dl` with configurable label width
  and a `mono` option per entry.
- **`FieldRow`** — labeled form field; auto-wires
  `htmlFor`/`aria-describedby`/`aria-invalid` across
  label/description/hint/error.
- **`VersionItem`** — version-history entry. Accent left bar for
  `current`. Optional `onSelect` makes it a button.
- **`ActivityRow`** — audit timeline row with timestamp/actor/action/target
  grid.

### File browser

Components for the scoped file browser (a multi-file asset's manifest, browsed
as a directory tree). Pure/presentational — the orchestration that wires real
data + state lives in `web/src/components/filebrowser/`.

- **`fileTypes`** — `FileType` taxonomy, `FILE_TYPE_META` (label + lucide
  glyph), `FileGlyph`, `fileTypeMeta`, and `fileTypeFromName(path, detectedType?)`
  which classifies a file by extension (package-extracted files have no
  `detected_type`).
- **`fileTree`** — node model (`FileNode`/`FolderNode`/`FlatFile`) + pure
  helpers: `buildFileTree(manifest)` (groups a flat manifest into a sorted
  tree), `nodesAtPath`, `flattenFiles`, `typeCounts`, `countFiles`, `pathKey`,
  `isBrowsable(fileCount)`. Covered by unit tests.
- **`FileThumb`** / **`FilePlaceholder`** — image-or-placeholder preview
  surface; image-type files render from a caller-supplied `imageSrc`.
- **`FilePreview`** — per-type preview *dispatcher* for the file detail panel:
  picks the richest viewer a file type supports, falling back to `FileThumb`
  when none applies or the bytes aren't loaded. Currently only the inline
  image viewer is wired; later viewers slot in above the fallback and should
  be `lazy`-loaded so heavy deps stay out of the main bundle (the component's
  doc comment carries the pattern). The caller supplies `previewUrl` from an
  authenticated blob fetch — the component does no fetching of its own.
- **`FolderCard`** / **`FileCard`** — grid tiles (file card has a type badge +
  selection check + `open` highlight, mirroring `AssetCard`).
- **`FileBrowserList`** — dense table view of folders + files.
- **`FileTreeSidebar`** — scope header + in-pack search + directory tree +
  file-type filter (composes `FilterSidebar`/`SearchInput`/`Section`). Tree rows
  keep the caret toggle and the navigate target as **sibling** buttons to stay
  free of the axe nested-interactive violation.
- **`FileBrowserToolbar`** — breadcrumb path + active filter chips + grid/list
  toggle (composes `Toolbar`/`Breadcrumb`/`SegmentedControl`).
- **`FileDetailPanel`** — per-file detail reusing the detail-panel chrome
  (hero + metadata + path bar + download/subset actions + prev/next stepper).
- **`FileSelectionBar`** — floating selection bar wrapping `SelectionBar` with
  the "Create subset" action.

Fixture data for stories lives in `_fixtures.ts` (ASSET_FIXTURES,
PROJECT_FIXTURES). Shapes mirror `docs/mockup/src/data.jsx`. The file-browser
stories build their own fixture tree inline.

---

## Testing

Every component ships a `*.stories.tsx` next to it. Stories serve three
purposes: (1) visual contract browsed in Storybook, (2) a11y test via
`@storybook/addon-a11y` (`test: "error"` — violations fail the run),
(3) pseudo-state preview via `storybook-addon-pseudo-states` (the
toolbar flips `:hover`/`:focus-visible`/`:active`/`:disabled` on
demand, and stories can wrap elements in `pseudo-<state>-all` to show
matrices).

Two Vitest projects:

- **`unit`** (`task test:unit:web`) — jsdom, component unit tests.
- **`storybook`** (`task test:stories`) — Playwright Chromium, every
  `*.stories.tsx` runs as a browser test with axe-core checks. Requires
  `npx playwright install chromium` once.

The story glob (`.storybook/main.ts`) covers `src/design-system/**` **and**
`src/components/**`. The second entry is for app-level stories: wired
components rendered with their real hooks against a stubbed `window.fetch`,
which is how the library sidebar is captured (`App/AssetDetailPanel`).
Prefer a design-system story where the component is presentational — an
app-level story is for the cases where the value is in seeing the wired
thing, gallery fetches and all.

Contrast exceptions: if a story legitimately uses a decorative
low-contrast element (e.g. an all-caps 10.5px label in `text-text-4`),
override at the story level with `parameters.a11y.test = "todo"` and
explain the rationale in a comment.

---

## Contributing a new component

Follow the pattern of the nearest existing component.

1. Choose the tier: primitive (atomic, no domain), composition (reusable
   shell), or domain (Damson-specific). If it could live in multiple
   places, push toward the lower tier.
2. Create `Name.tsx` with props interface, `cn()` + `cva()` for
   variants, ref forwarding if it could be form-linked or
   Radix-composed.
3. Create `Name.stories.tsx` with: default story, all variants, all
   states. Add a "States" matrix story wrapping sections in
   `pseudo-<state>-all` when `:hover`/`:focus-visible`/`:active` matter.
4. Export from the tier's `index.ts` barrel.
5. Run `task test:unit:web` and `task test:stories` — the storybook project must pass with
   `a11y.test = "error"`.

If axe flags a contrast issue, fix the token or reach for a brighter
tier (e.g. `text-3` instead of `text-4`) before adding a story-level
exception. Exceptions should be the exception.
