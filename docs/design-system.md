# Damson Design System

**Status:** v3 — primitives from the `@telliott/shared-ui` package **Last updated:** 2026-08-22

The authoritative implementation contract for Damson's frontend. **Storybook is the
visual reference** (`just storybook`); this document is what the code actually does.
The old mockup now lives at `docs/archive/mockup/` and is consulted only for the *why*.

The token vocabulary is the shared contract's
(`design.smol.cloud/r/tokens.json`); the values are Damson's. See
`docs/specs/history/2026-08-15-shared-design-system.md` for how it got here.

**Contract components arrive as a dependency.** `@telliott/shared-ui` is a normal
npm package; nothing is copied into the tree, so `web/src/components/ui/` no longer
exists. Damson's own primitives stay in `web/src/design-system/primitives/` and wrap
or re-export what the package exports. A per-project difference belongs in a token
override or a wrapper, never in a fork. See **Where primitives come from**.

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

Two entries in `web/components.json` are worth knowing, because JSON cannot explain
itself and both look wrong at a glance:

- **`aliases.utils` points at `@/design-system/utils`, not the conventional
  `@/lib/utils`.** Damson's `cn()` registers its 19 composed `text-*` names with
  `tailwind-merge`, so its font sizes aren't deduplicated against colour classes.
  The alias is what a file copied in by `shadcn add` gets rewritten to import, so
  it lands on the extended `cn` rather than the stock one. A re-export at
  `lib/utils` would work and would invite someone to "simplify" it back to the
  stock two-liner, silently breaking every custom text size.
- **`tailwind.baseColor: "neutral"` is inert.** Only `init` and base-colour
  installs read it; it does not mean Damson uses shadcn's neutral palette, which
  it emphatically does not — see the warm scale below.

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

## Where primitives come from

`@telliott/shared-ui`, pinned in `web/package.json`. Four things make that work, and
three of them fail silently.

**`web/.npmrc`** maps the `@telliott` scope to the Forgejo registry. Reads are
anonymous — no token, in CI or in the image. Without the mapping the scope resolves
against npmjs.com, where `@telliott` is unclaimed: a 404 today, and someone else's
code the day anyone registers it.

**`@source` in `globals.css`** is what makes the package's classes reach the CSS.
Tailwind does not scan `node_modules`, so without it every package component renders
unstyled and the build still exits 0. `pnpm run lint` runs the package's own
`check-tailwind-source` against the stylesheet, which resolves the specifier rather
than matching it as a string. The directive sits **after** the last `@import` —
placed among them, postcss rejects the imports that follow.

**No CSS comes from the package.** `@telliott/shared-ui/styles.css` carries the
contract's reference palette *unlayered*, so importing it beats Damson's layered
`tokens.css` and repaints the app in reference greys. Nothing is lost by leaving it
out: every utility the package's components use resolves through the generated
`@theme inline` block, and the `prefers-reduced-motion` block the package would
supply is already in `globals.css`.

**`Dockerfile`'s install layer** copies `.npmrc` and `pnpm-workspace.yaml` alongside
`package.json`, before `pnpm install`. The `COPY web/ ./` below it lands too late.

### The `cn` on the far side of the boundary

A package component composes `className` with the `cn` bundled *in the package*, not
Damson's. It knows the contract's eight role sizes and none of Damson's 19, so
passing a role into one resolves nothing: `<Button className="text-mono">` keeps both
`text-mono` and the component's own `text-ui`, and which applies is stylesheet order
— per pair, not systematic. Damson's own colour tokens (`text-unknown`,
`text-unknown-soft`, `text-primary-hover`, `text-swatch-foreground`) duplicate rather
than dedupe there for the same reason. Neither is a drop, and no call site does
either today; `cn.package-boundary.test.tsx` pins both. Letting a consumer register
its roles with the package's `cn` is `shared-design`'s agreed follow-up — **don't
work around it by widening Damson's `cn`**, which is not the one running.

---

## Tokens

Source: `web/src/design-system/tokens/tokens.css`. **Light is the base (`:root`);
dark opts in via the `.dark` class**, which is the shape the registry `tokens` item
installs as. The initial theme follows `prefers-color-scheme` until the user picks
one; `web/index.html` sets the class before first paint so dark users see no flash.

### Neutrals + text

| Token | Dark L | Light L | Purpose |
|---|---|---|---|
| `--background` | 0.195 | 0.985 | Page background |
| `--card` | 0.225 | 0.965 | Sidebars, panels, cards |
| `--popover` | 0.225 | 0.965 | Floating surfaces — equal to `--card` on purpose |
| `--muted` | 0.258 | 0.945 | Controls, inputs, chips, skeletons |
| `--accent` | 0.295 | 0.92 | Hover / selected surface — **not** the brand |
| `--border` | 0.325 | 0.88 | Hairline dividers |
| `--border-strong` | 0.385 | 0.82 | Stronger borders |
| `--foreground` | 0.93 | 0.20 | Primary text |
| `--muted-foreground` | 0.78 | 0.38 | Secondary |
| `--subtle-foreground` | 0.70 | 0.52 | Tertiary — meets WCAG AA on `background`/`card` |
| `--faint-foreground` | 0.60 | 0.65 | Decorative / labels; meets AA on `background` only |

All OKLCH, hue 70–80 (very slight warm bias). `faint-foreground` is deliberately
at the edge of legibility — reserve it for decorative/ornamental text
(ChevronRight separators, unused states); reach for `subtle-foreground` for anything
informational.

**`--card` and `--popover` are set equal in both themes.** They differ in the
reference palette, which means a floating element misclassified as a card renders
correctly in one theme and wrongly in the other, and nothing catches it. Equal values
make the classification unable to render wrongly while leaving them free to diverge
later. The rule for choosing: *the element positions itself* — popovers, dropdowns,
dialogs, tooltips, context menus and selection bars take `--popover`; everything else
takes `--card`.

### Accent + status

| Token | Dark | Purpose |
|---|---|---|
| `--primary` | `oklch(0.82 0.15 86)` | Primary action, current selection, **the brand** |
| `--primary-hover` | `oklch(0.74 0.15 86)` | Hover / pressed. Local — the contract has no name |
| `--primary-soft` | `.../ 0.16` | Subtle brand background |
| `--ring` | `.../ 0.35` | Focus ring |
| `--primary-foreground` | `oklch(0.18 0.02 86)` | Foreground on filled brand bg |
| `--success` / `--success-soft` | `oklch(0.72 0.12 155)` | Compliant licence |
| `--warning` / `--warning-soft` | `oklch(0.78 0.13 85)` | Needs review |
| `--destructive` / `--destructive-soft` | `oklch(0.68 0.17 28)` | Violation / destructive |
| `--unknown` / `--unknown-soft` | `oklch(0.72 0.01 80)` | Unknown status. Local — see below |

**Watch the `accent` trap.** In shadcn's vocabulary `--accent` is the hover/selected
*surface* and `--primary` is the brand. Damson's old `--ds-accent` was the brand, so
it became `--primary`. Getting this backwards produces a palette that looks *wrong*
rather than *broken*, which is much harder to spot.

**Local extensions**, where the contract has no equivalent: `--unknown` /
`--unknown-soft` (the licence axis is four-valued and the contract carries three),
`--primary-hover` (a lighter yellow, not `--primary` at reduced alpha — half its
sites are the `from-primary to-primary-hover` gradient, where opacity is not
lightness), `--swatch-foreground` (light ink for server-supplied
`glyph_color` swatches, which are dark in both themes), `--elevation`,
`--sidebar-w` / `--rail-w` / `--detail-w`, and `--code-canvas` (GitHub Dark's
background, which must track `HIGHLIGHT_THEME` rather than Damson's palette). Each is
a candidate to contribute upstream if a second project sights one.

### Typography

- `--font-sans`: Inter (self-hosted via `@fontsource/inter`)
- `--font-mono`: JetBrains Mono (self-hosted via `@fontsource/jetbrains-mono`)
- Body: 15px / 1.55 line-height, `cv11 + ss01` OpenType features.

The font-size scale is two-layered: the shared contract's eight role sizes, which
Damson composes into its own weight and treatment variants. Authors reach for
whichever layer names what they mean — the contract's roles carry sizes only, and
Damson's composed tokens add a weight, family or transform on top.

**Layer 1 — the contract's role sizes** (from `@shared/tokens`; size +
line-height only, no weight)

| Utility | px / line-height | Used for |
|---|---|---|
| `text-display` | 28 / 1.2 | Page and hero titles. **Installed, currently unused** — Damson's largest is `title`. |
| `text-title` | 22 / 1.25 | Dialog and section titles. |
| `text-heading` | 20 / 1.3 | Subsection headings. |
| `text-subhead` | 18 / 1.35 | Card headings, list group headers. |
| `text-field` | 16 / 1.4 | **Anything you type into — a hard floor**, see below. |
| `text-body` | 15 / 1.55 | Default running text. Also the `<body>` default. |
| `text-ui` | 14 / 1.45 | Buttons, list rows, table cells, chip / tag / badge text. |
| `text-label` | 12 / 1.4 | **Absolute floor.** Captions, badges, column headers, mono hashes, kbd hints. |

There is no 13px step and nothing below 12px.

**The 16px field floor is enforced, not merely applied.** Below 16px, iOS Safari
zooms the viewport on focus and does not zoom back, leaving the layout broken for
the rest of the session. `Input`, `Textarea` and `SearchInput` carry `text-field`,
and `web/scripts/check-tokens.mjs` fails the build if any of them drops below it.

**Layer 2 — Damson's composed tokens** (declared in `globals.css` as `@utility`
rules; bake in size + weight + transform + tracking + family)

Headings — four weight variants each. There is deliberately **no bare alias**:
`text-title` is the contract's plain 22px, and `text-title-semibold` is Damson's
22px at 600. The two used to collide.

| Token | px / line-height | Pairs with |
|---|---|---|
| `text-title-{reg,medium,semibold,bold}` | 22 / 1.25 | `<h1>` (page heading) |
| `text-heading-{reg,medium,semibold,bold}` | 20 / 1.3 | `<h2>` (section heading) |
| `text-subhead-{reg,medium,semibold,bold}` | 18 / 1.35 | `<h3>` (dialog title, asset detail title) |

Generic role tokens:

| Token | Composes | Used for |
|---|---|---|
| `text-overline` | `text-label font-medium uppercase tracking-wide` | Section labels, kbd-row labels. |
| `text-mono` | `font-mono text-label` | Sha hashes, IDs, file paths, short code refs. |
| `text-caption` | `text-label text-subtle-foreground` | Helper text under inputs, muted captions. |
| `text-num` | `text-ui font-mono` + tabular numerals | Inline numeric data. |

View-specific role tokens:

| Token | Composes | Used for |
|---|---|---|
| `text-table-row` | `text-ui` | Body cells in `<table>`. |
| `text-table-header` | `text-ui font-medium` | `<thead>` cells. |
| `text-list-row` | `text-ui` | Rows of dense list views (e.g. `AssetListRow`). |

**All 19 Layer 2 names must stay registered with `tailwind-merge`** in
`design-system/utils/cn.ts`, alongside the contract's eight from
`@shared/type-scale`. A role name fails tailwind-merge's t-shirt-size test, falls
through to the `text-color` group, and `cn("text-mono", "text-muted-foreground")`
silently returns the colour alone. `cn.test.ts` pins both halves.

### Sizing, radii, motion, elevation, z-index

- Sizes: `--sidebar-w: 248px`, `--rail-w: 52px`, `--detail-w: 440px`
- Radii: the contract's **literal** scale — `xs` 4, `sm` 6, `md` 12, `lg` 16, `xl` 24,
  `2xl` 32, `3xl` 40, `4xl` 48px. No pill; use `rounded-full`
- Transitions: `duration-fast` 120ms is the only step in use. `normal` (180ms) and
  `slow` (260ms) are installed and unused
- Easing: `ease-standard` (0.2, 0.8, 0.2, 1). `ease-bounce` (0.34, 1.56, 0.64, 1)
  stays Damson-local and is consumed by `--animate-pop-up`, not used as a utility
- Elevation: `shadow-lg` for overlays (portals, sliding panels, toasts), `shadow-md`
  for resting raised surfaces (auth cards, the PDF canvas). Values differ per theme
- Z-index: `resizer=5`, `detail=10`, `drop=20`, `selbar=30`, `tooltip=100`,
  `dialog=500`, `popover=600`, `tweaks=1000`
- A global `prefers-reduced-motion` block ships with `@shared/tokens` and is written
  into `globals.css` by the CLI

**Radii are literal, not derived.** Corners nest by subtraction (`inner = outer −
padding`) rather than by ratio, which no multiplicative scale expresses. Adopting
them reflowed 88 sites and made Damson visibly rounder; that was accepted rather
than fought.

### Tailwind mapping

`web/src/styles/globals.css` carries **two `@theme inline` blocks, in order**:

1. **CLI-owned**, generated by `shadcn add @shared/tokens` — the contract's colour,
   type, radius, motion and elevation mappings. Re-running the CLI regenerates it
   wholesale, so never hand-edit it.
2. **Damson's** — the extension colours, the z-indexes, widths, animations and
   keyframes, `--ease-bounce`, and the contract's scales restated as literals.
   Tailwind merges `@theme` blocks with the later winning, so this is the only
   place that can override block 1, and the CLI cannot see it.

There is no `tailwind.config.ts` — Tailwind 4 takes its theme from CSS.

**A generated mapping is a dependency cycle, and this is the trap in the whole
setup.** The CLI emits `--radius-md: var(--radius-md)` — the same name on both
sides. A custom property that references itself is invalid at computed-value time,
**and that invalidates every declaration of the name, whatever the cascade layer
says**. So a value of the contract's own name placed in `tokens.css` does not
merely lose; it is destroyed along with the mapping. This is why:

- The scales are restated as literals in block 2, not as values in `tokens.css`.
- The font stacks are `--sans-stack` / `--mono-stack`, and the per-theme shadows
  are `--elevation-sm/md/lg`. Both are mapped across in block 2. **Renaming these
  back to the contract's spelling silently breaks them** — Inter stops loading and
  every shadow disappears, with no error anywhere.

Colour tokens are unaffected: they map `--color-x: var(--x)`, where the two names
differ, so `tokens.css` remains where every colour value lives.

**Prefer the utility over `var(--token)`** — e.g. `bg-card`, `text-muted-foreground`,
`border-border`, `rounded-md`.

**`var(--radius-sm)` is a real custom property now.** The scale ships literal
values, so `var(--radius-sm)` resolves and the old `calc(var(--radius) * 0.6)`
idiom is gone. `--radius` survives as an unreferenced 16px alias, kept only so a
registry component installed direct still resolves.

**Every `shadcn add` re-appends the contract's reference palette** to the end of
`globals.css` as an unlayered `:root`/`.dark`, which beats `tokens.css` in the
cascade and silently overrides every Damson value. Delete it after each install —
`web/scripts/check-tokens.mjs` fails the build if you forget.

---

## Primitives

Atomic components with no domain knowledge. All live under
`src/design-system/primitives/`.

### Action

- **`Button`** — a **wrapper** around the package's `Button`.
  `variant: primary|secondary|ghost|destructive` (plus `outline`/`default`
  compatibility aliases for pasted upstream blocks), `size: sm|md|lg` (28/32/36px).
  **Defaults to `primary`.** `leadingIcon`/`trailingIcon`, `loading`, `fullWidth`,
  `asChild`, and `pressed` via `data-pressed`.

  The wrapper exists for exactly one reason: `destructive` renders the registry's
  `secondary` with Damson's soft red tint over it, because a filled red reads fine in
  a confirm dialog and shouts in a toolbar. **That override lives in the wrapper**,
  which is the only place a package component can be changed at all; see
  `Button.test.tsx`, which fails if the wrapper is bypassed.
- **`IconButton`** — the package's, and a **pure re-export** rather than a
  wrapper: unlike `Button`, Damson has nothing to override. Square `size: sm|md|lg`
  (28/32/36px), matching `Button`'s heights so the two line up in a toolbar; same four
  variants, defaulting to `ghost`; `pressed` via `data-pressed`. `aria-label` is
  required by the type — that guard is why it is a component and not `size="icon"`.

  Two things to know when calling it. The size step also sets the icon
  (`sm` → 14px, `md`/`lg` → 16px) via `[&_svg:not([class*='size-'])]`, so a `size={n}`
  prop on a Lucide icon inside it is overridden and should be dropped — pass a
  `size-*` class if you really need a different one. And `variant="destructive"` is
  **filled**, not the soft red `Button` gives you: the destructive softening lives in
  `Button.tsx`'s wrapper, which this component does not share. No call site uses it yet.
- **`SegmentedControl<T>`** — generic over a value type; options can be
  icon-only or text. Used for grid/list toggles and density picks.

### Input

All three typing surfaces render at `text-field` (16px) and the token guardrail
enforces it — see **Typography**.

**The package's `Input` and `Textarea` are deliberately unused.** This is a
divergence from the usual "take what the contract has" stance, alongside the
`Button` wrapper precedent, and it is a decision rather than an oversight. The
registry's `Input` is a bare `<input>` carrying its own border and background;
Damson's is a wrapping `<div>` that carries them, so nesting the two double-borders,
and the icon and keyboard-hint slots have no registry equivalent. What Damson takes
from the contract is the `--text-field` token and the floor rule, not the component.
Contributing Damson's icon-slot API upstream — the path `IconButton` took — is the
long-run answer and is on the roadmap.

- **`Input`** — leading icon slot + trailing slot, `invalid` state sets
  `aria-invalid`, focus-within applies the focus ring. Both the wrapper and the
  inner field carry `text-field`.
- **`SearchInput`** — preset over `Input`: Search icon + optional `kbd`
  hint (⌘K). Inherits the field size from `Input`.
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
- **`Kbd`** — inline keyboard hint, `text-mono` in a 20px-high bordered surface.
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
  grid collapses to rail + main, which is how pages implement a toggleable
  sidebar (see the library's `Filter` toggle). The persisted width
  (`damson.sidebar.w`) is read whether or not a sidebar is currently
  mounted, so toggling one back open restores the user's width instead of
  overwriting it with the default; an explicit `sidebarWidth` still wins.
- **`NavRail`** + **`NavRailItem`** + **`NavRailSpacer`** — vertical
  icon nav with tooltip labels (250ms delay), `badge` prop for counts,
  `active` renders a left accent bar. `brand` slot at top.
- **`ResizeHandle`** — 4px draggable separator with keyboard support
  (Arrow keys = 8px, Shift+Arrow = 32px) and min/max clamp. Consumer
  owns the resized element's width.
- **`FilterSidebar`** + **`FilterRow`** — scrollable frame with sticky
  header + footer. Rows take `icon`/`dot`/`label`/`count`/`active` and
  auto-apply the `bg-primary-soft` highlight when pressed.
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
    `docs/archive/mockup/src/thumbnails.jsx`.
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
- **`FileTreeSidebar`** — scope header + in-pack search + folder tree +
  file-type filter (composes `FilterSidebar`/`SearchInput`/`Section`). Tree rows
  keep the caret toggle and the navigate target as **sibling** buttons to stay
  free of the axe nested-interactive violation. **Folders only** — files are
  opened from the grid, so a folder with no subfolders renders no caret.
  Folders shallower than depth 3 are expanded on first render (from
  `folderKeysToDepth`), computed in a `useState` initialiser so later
  navigation can't stomp a manual collapse. The caret expands/collapses; the
  label navigates *and* reveals. Reveal-on-navigate is a **state write** on
  `path` change, not a render-time union with the path's ancestors — the
  union made a folder on the current path impossible to collapse.
- **`FileBrowserToolbar`** — breadcrumb path + active filter chips + grid/list
  toggle (composes `Toolbar`/`Breadcrumb`/`SegmentedControl`).
- **`FileDetailPanel`** — per-file detail reusing the detail-panel chrome
  (hero + metadata + path bar + download/subset actions + prev/next stepper).
- **`FileSelectionBar`** — floating selection bar wrapping `SelectionBar` with
  the "Create subset" action.

Fixture data for stories lives in `_fixtures.ts` (ASSET_FIXTURES,
PROJECT_FIXTURES). Shapes mirror `docs/archive/mockup/src/data.jsx`. The file-browser
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

- **`unit`** (`just test frontend`) — jsdom, component unit tests.
- **`storybook`** (`just test stories`) — Playwright Chromium, every
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
low-contrast element (e.g. a `text-overline` label in `text-faint-foreground`),
override at the story level with `parameters.a11y.test = "todo"` and
explain the rationale in a comment.

---

## Contributing a new component

Follow the pattern of the nearest existing component.

**First: does it belong here at all?** The estate's order of preference is
upstream shadcn/ui → `@telliott/shared-ui` → Damson. If shadcn has it, install it
rather than writing one. If it is primitive-shaped and *not* Damson-specific, it
belongs in `shared-design` so every consumer gets it — the package's inventory
grows when a consumer needs something. Only genuinely Damson-specific components
are written here.

**Never fork a package component into the tree.** A per-project difference goes in
a token override or a wrapper, both of which survive a version bump; a copy does
not. `shadcn add @shared/<name>` still works and remains the escape hatch for a
one-off, but it takes the component off the upgrade path — a decision, not an
accident, so record it if you take it.

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
5. Run `just test frontend` and `just test stories` — the storybook project must pass with
   `a11y.test = "error"`. `just lint frontend` runs the token guardrail
   (`web/scripts/check-tokens.mjs`), which fails on a retired token name, a raw hex
   outside `tokens.css`, an arbitrary colour utility, or a `:root`/`.dark` block
   left in `globals.css` by `shadcn add`.

If axe flags a contrast issue, fix the token or reach for a brighter
tier (e.g. `subtle-foreground` instead of `faint-foreground`) before adding a
story-level exception. Exceptions should be the exception.

**Check both themes.** `STORYBOOK_THEME=light pnpm run test:stories` runs the suite
in light. It is not in `just check` yet — see the note in
`docs/progress.md` about the Storybook vitest project not emitting Tailwind
utilities, which makes its contrast checks weaker than they look.
