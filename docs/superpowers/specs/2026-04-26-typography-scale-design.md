# Typography Scale Redesign

**Status:** design
**Date:** 2026-04-26
**Owner:** Thomas Elliott

## Motivation

Today's font-size scale has 10 tokens spanning 10.5–22px. The cluster at the
small end (10.5 / 11 / 11.5 / 12 / 12.5 / 13) is too dense — call sites pick
between near-identical sizes by feel, and the smallest tiers fail readability
on copy that isn't fleeting. Body text at 13px is below the 14px floor we
want for sustained reading.

This redesign collapses the scale to **6 raw sizes** (12 / 14 / 15 / 18 / 20 /
22) and reframes the design system around **two layers**:

- **Layer 1 — size primitives:** raw size + line-height, no weight, used for
  body / UI / small fine print.
- **Layer 2 — composed semantic tokens:** named by role (heading, label,
  caption, table-row, …) and bake in size + weight + transform + tracking +
  family.

Heading sizes only ever appear inside Layer 2 heading tokens — they are not
exposed as Layer 1 primitives. This keeps the heading sizes locked to their
role and lets the heading scale move (e.g. to 24/20/16 later) without
touching call sites.

## Goals

- Raise the readability floor: smallest text is 12px, body is 15px.
- Eliminate near-duplicate sizes (10.5/11/11.5 collapse to 12; 12/12.5
  collapse to 14).
- Replace size-named tokens with role-named tokens (`text-title`,
  `text-heading`, `text-subhead`, `text-label`, `text-mono`, …) so call
  sites express intent.
- Pair Layer 2 heading tokens with the matching HTML element
  (`text-title` ↔ `<h1>`, `text-heading` ↔ `<h2>`, `text-subhead` ↔ `<h3>`).
- Provide weight variants for headings (`-reg`/`-medium`/`-semibold`/
  `-bold`) as a future-proofed system, even where some variants are unused
  today.

## Non-goals

- WCAG-AA contrast audit of `text-text-3` / `text-text-4` against panel
  backgrounds at the new sizes. The size bump improves accessibility, but a
  comprehensive contrast review is a separate follow-up.
- Re-thinking the colour scale, spacing scale, or any other tokens.
- Backwards-compatibility shims. The old token names (`text-2xs`,
  `text-xs`, …) are removed entirely; every call site is updated in this
  migration.

## Design

### Layer 1 — size primitives

Tailwind `fontSize` entries in `web/tailwind.config.ts`. Size + line-height
only, no weight, no transform.

| Token | px / line-height | Used for |
|---|---|---|
| `text-body` | 15 / 1.5 | Default running text. Also becomes the `<body>` default in `globals.css`. |
| `text-ui` | 14 / 1.4 | Buttons, inputs, list rows, table cells, chip / tag / badge text. |
| `text-small` | 12 / 1.4 | Captions, labels, mono hashes, kbd hints — accessibility floor. |

The 18 / 20 / 22 px sizes do **not** live in Layer 1. They are baked into
the Layer 2 heading tokens and only appear there.

### Layer 2 — composed semantic tokens

`@layer components` classes in `web/src/styles/globals.css`, declared with
`@apply` so they consume Layer 1 utilities and standard Tailwind weight /
transform utilities.

#### Headings (12 tokens + 3 bare aliases)

| Bare token (alias of `-semibold`) | `-reg` (400) | `-medium` (500) | `-semibold` (600) | `-bold` (700) |
|---|---|---|---|---|
| `text-title` | `text-title-reg` | `text-title-medium` | `text-title-semibold` | `text-title-bold` |
| `text-heading` | `text-heading-reg` | `text-heading-medium` | `text-heading-semibold` | `text-heading-bold` |
| `text-subhead` | `text-subhead-reg` | `text-subhead-medium` | `text-subhead-semibold` | `text-subhead-bold` |

Sizes / line-heights:
- `text-title-*` — 22 / 1.25 (page heading; pairs with `<h1>`)
- `text-heading-*` — 20 / 1.3 (section heading; pairs with `<h2>`)
- `text-subhead-*` — 18 / 1.35 (dialog title, asset detail title; pairs with `<h3>`)

The bare token (`text-title`, `text-heading`, `text-subhead`) is an alias
for the `-semibold` variant — the dominant pattern in the current codebase
(8 of 10 heading sites).

#### Generic role tokens (4)

| Token | Composition | Purpose |
|---|---|---|
| `text-label` | `text-small font-medium uppercase tracking-wide` | Section labels, column headers, kbd-row labels. |
| `text-mono` | `font-mono text-small` | Sha hashes, IDs, file paths, short code refs. |
| `text-caption` | `text-small text-text-3` | Helper text under inputs, muted captions. |
| `text-num` | `text-ui font-mono` + `font-feature-settings: 'tnum'` | Inline numeric data (file sizes, counts, durations) where digit alignment matters without going full mono on the row. |

#### View-specific role tokens (3)

| Token | Composition | Purpose |
|---|---|---|
| `text-table-row` | `text-ui` | Body cells of `<table>` rows. Dedicated token so table typography can change without touching every UI text site. |
| `text-table-header` | `text-ui font-medium` | `<thead>` cells. 14px / medium-weight (no uppercase) — same size as rows, weight differentiates. |
| `text-list-row` | `text-ui` | Rows of dense list views (e.g. `AssetListRow`). Distinct from table-row so lists and tables can diverge. |

### Body default

`web/src/styles/globals.css` currently sets `body { font-size: 13px;
line-height: 1.45; }`. After migration:

```css
body {
  font-size: 15px;       /* matches text-body */
  line-height: 1.5;
  /* … existing font-family, color, etc. */
}
```

## Migration

### Layer 1 → Layer 2 mapping

| Old token (current px) | Sweep rule |
|---|---|
| `text-2xs` (10.5) | → `text-label` if the site combines `text-2xs` with `uppercase` and `tracking-*` (the existing label pattern); otherwise `text-small`. (~19 sites) |
| `text-xs` (11) | → `text-small` by default; → `text-caption` if the site is muted helper text under another element. (~59 sites — per-site judgment) |
| `text-xs-plus` (11.5) | → `text-mono` if the existing class set includes `font-mono`; otherwise `text-small`. (~41 sites) |
| `text-sm` (12) | → `text-ui`. (~5 sites) |
| `text-sm-plus` (12.5) | → `text-table-row` if inside a `<table>` body; → `text-list-row` if inside a list-view row component (e.g. `AssetListRow`); otherwise `text-ui`. (~69 sites — per-site judgment) |
| `text-base` (13) | Remove entirely. The 1 site is redundant with the body default; new body default is 15px and inheritance handles it. |
| `text-md` (14) | Heading sites (`<h1>` / `<h2>` / `<h3>` / dialog title) → `text-subhead`. Non-heading sites → `text-ui`. `Glyph` size=`md` → `text-[14px]` (icon size, not typography — see Glyph note below). (~7 sites) |
| `text-lg` (16) | Asset detail title → `text-subhead` (16 → 18). `Glyph` size=`lg` → `text-[16px]` (icon size — see Glyph note below). (~2 sites) |
| `text-xl` (20) | → `text-heading`. (~4 sites) |
| `text-2xl` (22) | → `text-title`. (~1 site, the Tokens story — gets rewritten anyway) |

After the sweep, the old tokens (`text-2xs`, `text-xs`, `text-xs-plus`,
`text-sm`, `text-sm-plus`, `text-base`, `text-md`, `text-lg`, `text-xl`,
`text-2xl`) are removed from `tailwind.config.ts`. No alias / shim period.

`text-mono` and `text-num` get their own targeted sweeps:

- Any `font-mono text-{small-tier}` pair → `text-mono`.
- Any future tabular-numeral column → `text-num` (no current sites).

`text-table-header` gets a targeted sweep: column-header sites today use
`text-2xs uppercase tracking-wide font-medium text-text-3` (or close
variants). Identify these in `<thead>` rows and replace with
`text-table-header`.

### Component height adjustments

Bumping the smallest tier from 10.5/11/11.5px to 12px requires growing a
few primitive heights so text doesn't crowd the box.

| Component | Today | Change |
|---|---|---|
| `Badge` size=`sm` | `h-[18px] text-2xs` | `h-[20px] text-small` (via `text-mono` or direct) |
| `Kbd` | `h-[18px] mono text-2xs` | `h-[20px] text-mono` |
| `Tabs` count badge | `h-[18px] mono text-2xs` | `h-[20px] text-mono` |
| `NavRail` notification dot | `h-[15px] min-w-[15px] text-[9px]` | `h-[18px] min-w-[18px] text-mono` (the `9px` arbitrary is replaced; the dot becomes a 12px-mono pill) |

`Chip`, `Tag`, `Separator` vertical, `AssetListRow` mini-thumb keep their
22 / 18px boxes — 12px text fits 22px boxes, and the 18px separator stays
inline with `text-small`.

### Glyph (icon-sizing primitive)

`web/src/design-system/primitives/Glyph.tsx` uses `text-sm-plus` / `text-md`
/ `text-lg` to size icon glyphs (single chars / emoji rendered inside an
`inline-grid` box). This is icon sizing, not typography, and shouldn't pull
on Layer 2 role tokens. Migration:

| Glyph size | Today | After |
|---|---|---|
| `sm` | `h-7 w-7 text-sm-plus` (12.5) | `h-7 w-7 text-[14px]` |
| `md` | `h-9 w-9 text-md` (14) | `h-9 w-9 text-[14px]` |
| `lg` | `h-12 w-12 text-lg` (16) | `h-12 w-12 text-[16px]` |

Arbitrary values are correct here — Glyph is using font-size as an icon
sizing knob, not making a typographic statement, so it shouldn't reach
into the Layer 2 vocabulary.

### Heading bumps that affect surrounding layout

- Dialog title: 14 → 18 (28% larger). Re-eyeball the dialog header padding
  in `web/src/design-system/primitives/Dialog.tsx` once the title token is
  swapped — the close-button vertical alignment in particular.
- Asset detail title: 16 → 18 (slight bump). Eyeball
  `AssetDetailPanel.tsx:115` for spacing.

### Files touched (high level)

- `web/tailwind.config.ts` — replace `fontSize` block with the 3 Layer 1 tokens.
- `web/src/styles/globals.css` — bump body default to 15 / 1.5; add
  `@layer components` classes for all Layer 2 tokens.
- `web/src/design-system/tokens/Tokens.stories.tsx` — rewrite the
  typography section to demonstrate Layer 1 primitives, then the composed
  Layer 2 tokens.
- `docs/design-system.md` — replace the typography table.
- `docs/ui-design.md` — replace the inline scale list at line 36.
- ~210 call sites across `web/src/**/*.tsx` — token-by-token sweep per
  the mapping table above.
- A handful of primitives for height bumps and icon-size adjustments:
  `Badge.tsx`, `Kbd.tsx`, `Tabs.tsx`, `NavRail.tsx`, `Glyph.tsx`.

## Risks and trade-offs

**Risk: the body bump (13 → 15) breaks tightly-tuned layouts.** Most
risk surfaces are dialogs, panels, and dense settings copy. Dev-server
eyeball pass per surface during implementation; height-bumps for known
primitives are folded into this migration.

**Risk: per-site judgment in the `text-xs` and `text-sm-plus` sweeps.**
The `text-small` vs `text-caption` and `text-ui` vs `text-table-row` /
`text-list-row` calls aren't pure regex — some are inside table bodies,
some are inside cards, some are muted helpers. The implementation plan
should batch these into reviewable chunks rather than a single
all-files commit.

**Trade-off: 22 tokens is more than the 10 we have today.** The
Layer-1-vs-Layer-2 split adds tokens, but each one is named by role.
The cost is a slightly heavier reference doc; the win is that call
sites express intent and a future global typography change (e.g.
shifting headings to 24/20/16) only edits the Layer 2 definitions.

**Trade-off: 12 heading-weight variants when only 2 weights are used
today.** Accepted by design — the design system is being built for
the product's lifetime, not just today's call sites. Unused variants
cost effectively nothing in the compiled CSS (Tailwind's
`@layer components` classes are tree-shaken if unused, and
the source declaration is a few lines per variant).

## Out of scope

- Comprehensive WCAG-AA contrast audit at the new sizes.
- Spacing-scale review.
- Body / UI / small weight variants (intentionally weight-agnostic;
  authors compose `font-medium` etc inline as needed).
- Backwards-compatibility aliases for the old token names.

## Open questions

None at design time. All resolved during brainstorming.
