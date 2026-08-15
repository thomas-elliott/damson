# Adopting the shared design system

**Status `[2026-08-15]`:** superseded, and the work has since shipped —
[`specs/history/2026-08-15-shared-design-system.md`](specs/history/2026-08-15-shared-design-system.md)
(PR #40, `f91d654`). **Read that instead**, and `docs/design-system.md` for the live shape. Where it and this file disagree, **the spec wins**: it verified its claims
against the working trees, and several of the ones below turned out to be stale.

Known stale here, corrected in the spec: the `--ds-surface-2` gap (its twelve sites decompose into
three roles that already have names, so no new contract name is needed); the guardrail script (the
one in `personal-practices` is the Svelte variant and has no retired-name check); the `@theme inline`
step (the CLI generates that block itself); the radius scale (multiplicative, not additive); the
font-size layer's directive (`@utility`, not `@layer components`); and `EmptyState`'s location (a
composition, not a primitive).

This file is kept for the **estate context that lives outside this repo** — what the registry is, why
damson is consumer two, and the reasoning behind the vocabulary decisions. That part has not changed.

## What the shared design system is

`~/Projects/shared-design` (`git.smol.cloud/telliott/shared-design`) is a shadcn **custom registry**
plus the Storybook that renders it, from one build:

- `https://design.smol.cloud/r/<name>.json` — prod ring.
- `https://design.test.smol.cloud/r/<name>.json` — test ring.
- Install with the CLI: `pnpm dlx shadcn@latest add @shared/<name>`, after registering
  `registries: { "@shared": "https://design.smol.cloud/r/{name}.json" }` in `components.json`.

Four items today: `tokens`, `button`, `dialog`, `empty-state`. Components are **copy-in source you
own** — the CLI is the provenance, not a version range. `registryDependencies` resolve at install
time and chain (`dialog → button → tokens`).

The contract is the **token names**, not the palette values. `shared-design/src/styles/tokens.css`
ships a neutral reference palette so a greenfield app renders on install; damson overrides every
value and keeps the names.

Authoritative reading, in order. All paths are siblings of this repo under `~/Projects/`, except
`personal-practices`, which is `~/Projects/code-skills-marketplace/personal-practices/`:

- `shared-design/docs/consuming.md` — how a repo installs, and the porting table.
- `shared-design/src/styles/tokens.css` — the contract itself, with its three traps in the header.
- `personal-practices/standards/design-system.md` — the measurements and reasoning.
- `personal-practices/decisions/2026-08-15-design-system-vocabulary.md` — what was chosen.
- `dialogue-db/docs/specs/2026-08-15-shared-design-system.md` — the first consumer's spec.
  **Shipped**, merged as PR #39 (`5e6b4ba`) `[re-checked 2026-08-15]` — so it is a completed worked
  example rather than a plan, and `dialogue-db/web/src/styles/tokens.css` is the more reliable
  reading of what it actually settled on.

## Where damson sits

Consumer **two of two in the proof**, and the brownfield half of it: dialogue-db tests a clean
install, damson tests installing into an existing token vocabulary and 27 existing primitives. The
estate sequence is dialogue-db → **damson** → taskmanager, and taskmanager is the payoff rather than
the proof.

damson also carries **one exit criterion nobody else can**: a deliberate local override that survives
updating the *other* registry items — its soft-red `destructive`, kept as a **wrapper** around the
registry `Button` rather than an edit to the installed file. That is the crux of copy-in ownership
and the only place it gets tested.

## Settled elsewhere — do not reopen at alignment

All decided 2026-08-15, recorded in `personal-practices/decisions/`:

- **shadcn's vocabulary is primary, not aliased.** shadcn/ui *is* the primitive layer, so every
  component arriving from the registry expects these names. Keeping `--ds-*` as the real vocabulary
  would mean translating upstream forever.
- **Drop the `--ds-` prefix.** damson's zero overlap with the other three projects is a prefix, not a
  disagreement — `--ds-bg` *is* `--bg`.
- **`destructive`, not `danger`.**
- **Default `Button` variant is `primary`.**
- **Upstream direct for anything shadcn has; `@shared` only for what it lacks.** A bootstrapping
  stance, explicitly not the mature-library rule. Accepted cost: an upstream component never appears
  in the canonical Storybook and never goes through its a11y gate.
- **Never overwrite `cn`.** damson's is extended for its font-size system (`design-system/utils/cn.ts`,
  24 registered `text-*` class names); overwriting it deduplicates `text-mono` against a colour class
  and breaks damson silently.
- **The old `--accent` is the brand and becomes `--primary`.** shadcn's `--accent` is the
  hover/selected surface. Backwards produces a palette that looks *wrong* rather than *broken*.

## Starting state, measured 2026-08-15 at `c86084f`

Re-measure at build time; these size the work rather than being targets.

| | |
| --- | --- |
| Stack | React 19, Tailwind 4.3, Vite, Storybook 10.5 + `addon-a11y` + `addon-pseudo-states` |
| shadcn stack, assembled by hand | **10** Radix packages, `cva`, `clsx`, `tailwind-merge`, `lucide-react` |
| `components.json` | **absent** |
| Token declarations | **57** `--ds-*` in `web/src/design-system/tokens/tokens.css` |
| Token *utility* usages | **832** across **116** `.ts`/`.tsx` files (`bg-panel`, `text-text-2`, `border-line`, …) |
| Raw `var(--ds-*)` | **69** — 31 in `.ts`/`.tsx`, 38 in `.css` |
| Primitives | **27** under `design-system/primitives/`, plus compositions and domain tiers |
| Stories | **21** `*.stories.tsx` |
| Radii in use | `rounded-md` 51, `rounded-sm` 29, `rounded-full` 13, `rounded-pill` 11, `rounded-lg` 6 |

**The sweep here is a utility rename, not a `var()` rename.** damson's `@theme` block maps `--ds-*`
onto unprefixed Tailwind utilities and `docs/design-system.md` tells authors to prefer the utility —
so 832 of the ~900 sites are class names in `className`, and only 69 are raw `var()`. That is the
opposite shape from dialogue-db, which had ~1,050 `var()` calls and 46 utilities.

## The hazards, damson-specific

Seven, in rough order of how expensive getting them wrong is.

### 1. `@theme` is not `@theme inline` — a migration step, not a live bug

`web/src/styles/globals.css:16`. **damson currently works**: `design-system/tokens/index.ts` sets
`data-theme` on `document.documentElement`, and `html` *is* `:root`, so `--color-bg: var(--ds-bg)`
resolves against the overridden value and both themes render.

It works *only because the switch is at root*. Plain `@theme` bakes the value into the utility at
build time; the contract themes on a **class** selector, which Storybook's `addon-themes` and any
scoped preview apply to a wrapper element, not `html`. The moment a theme is scoped below root,
non-inline `@theme` stops following it.

Switch to `@theme inline` **before** adoption, then verify the toggle both at root and in a scoped
wrapper. Line 14's `@custom-variant dark (&:is(.dark *))` is dead config today — nothing sets `.dark`
and there are zero `dark:` utilities — so it is not a conflict, but it is what the CLI will regenerate.

### 2. damson inverts the default theme, and the contract does not

`tokens.css:83`: dark is `:root`, light opts in via `html[data-theme="light"]`. The contract is
shadcn's shape — light at `:root`, dark on `.dark, [data-theme="dark"]`.

The registry `tokens` item carries only `light` and `dark` `cssVars`, which **the CLI writes as
`:root` and `.dark`** — the second `[data-theme="dark"]` selector exists in `shared-design`'s own
`tokens.css`, which is not what installs. So the reassurance that "neither app has to change its
theme toggle" does not actually reach a consumer.

Three ways out, and the spec has to pick one:

- **Invert to match** — light values at `:root`, dark on `.dark`, and the toggle sets/clears `.dark`
  instead of `data-theme="light"`. What dialogue-db did (its stage 2), so the path is proven and the
  estate converges on one mechanism. Costs a decorator change in `.storybook/preview.tsx:12` and a
  toggle change in `design-system/tokens/index.ts:7`.
- **Keep dark-default and re-anchor the selectors by hand** after each install — cheap once, a
  standing merge hazard on every re-run of `add`.
- **Ask `shared-design` to ship the second selector in the item.** Already a recorded follow-up on
  dialogue-db's spec, unresolved: *"if damson cannot move, the item needs the second selector rather
  than the docs claiming it already has it."*

Recommendation for the spec to accept or overrule: **invert**, on the same reasoning dialogue-db
used — take whatever both defaults converge on and settle it once. Dark-by-default is a *palette*
choice and survives the inversion; damson's dark values simply move into the `.dark` block and
`prefers-color-scheme` decides the initial class.

### 3. `cn` is not at `@/lib/utils`

Shipped registry files import `{ cn } from "@/lib/utils"` verbatim — that layout mirrors what a
consumer has after `shadcn init`, and `shared-design` deliberately never ships its own `utils.ts`.
damson has `src/lib/` (19 files, no `utils.ts`) and its extended `cn` at
`src/design-system/utils/cn.ts`. `@/*` → `./src/*` already resolves (`tsconfig.json:23`).

Two options; the CLI rewrites imports from `components.json`'s aliases, so both work mechanically:

- Point `aliases.utils` at `@/design-system/utils` — installed files pick up damson's extended `cn`,
  nothing moves, and the font-size class groups keep working.
- Create `src/lib/utils.ts` re-exporting the extended `cn` — matches every other consumer, at the cost
  of a second name for one function.

Recommendation: **the alias.** The whole reason `cn` is extended lives in `design-system/utils`, and a
re-export invites someone to later "simplify" it back to the stock two-liner.

Either way: run `shadcn init` and **read the diff before accepting it**. This is the same hazard
dialogue-db recorded for its `downloadBlob` export.

### 4. Where installed files land

Registry files land at `aliases.ui` — `@/components/ui/` by default. damson's primitives are at
`design-system/primitives/`, already flat, one file per component.

`personal-practices/standards/storybook.md` settled the convention on 2026-08-15: **registry-distributed
primitives are flat**, a deliberate exception to folder-per-component. That covers the installed files
regardless of which folder they land in. What the spec has to say is whether installed and owned
primitives share a directory (one import path, no way to tell at a glance which files the CLI will
overwrite) or sit apart (`design-system/primitives/` stays damson's, `components/ui/` is the CLI's).

Recommendation: **keep them apart**, and say so in `docs/design-system.md`. "Which files does
re-running `add` overwrite" is the question copy-in ownership makes people ask, and a directory
answers it for free.

### 5. The surface split is a judgement, not a rename

damson's ramp is `--ds-bg` → `--ds-panel` → `--ds-surface` → `--ds-surface-2`. The contract has
`--background`, `--card`, `--popover`, `--muted` — and **no name for a second upward step**, which is
what `--ds-surface-2` is. (`shared-design/src/styles/tokens.css:92` records that gap explicitly:
`--ds-surface-2` / `--bg-elev-2` is sighted in three projects and unnamed on purpose.)

`--card` and `--popover` are the *same value in light and different in dark* in the reference palette,
so a wrong call passes every check and only shows when someone toggles the theme. dialogue-db split 82
sites with the rule **"the element positions itself"** — floating things (`Popover`, `DropdownMenu`,
`Dialog`, `Tooltip`, context menus, selection bars) take `--popover`; everything else takes `--card` —
and set the two to equal values in both themes so the classification cannot render wrongly, leaving
them free to diverge later. Steal both the rule and the equal-values trick.

Open on damson's side: whether `--ds-surface-2` maps onto `--popover`, onto `--muted`, or becomes the
second sighting that finally puts a name in the contract. **The contract's rule is a name enters on
its second sighting** — this one has three, so it is a legitimate ask of `shared-design`, and it is a
stage-0 style dependency if the answer is yes.

### 6. `Button` changes shape, and the default flips

| | damson today | Becomes |
| --- | --- | --- |
| Variants | `primary secondary ghost danger` | `danger` → `destructive` |
| Default variant | **`secondary`** (`Button.tsx:49`) | **`primary`** |
| Sizes | `sm` = `h-6`, `md` = `h-7` | registry is `h-7 / h-8 / h-9` |

The default flip is the dangerous one: **every `<Button>` with no `variant` prop changes appearance**,
and damson uses that call shape widely. Grep for prop-less `Button` before, not after.

The heights are an open design question on `shared-design`'s board, not a damson decision: `h-7/8/9` is
a midpoint between damson's very compact `h-6/h-7` and dialogue-db's 30px `.btn`, and **nobody has
looked at them side by side**. Adopting the registry Button here is what puts them side by side.

`IconButton` (`h-6` / `h-[34px]`) is the other half: `@shared/icon-button` is on `shared-design`'s
board but was **not published** for dialogue-db, which turned out not to use it. damson's copy is now
the only remaining evidence for publishing it at all — so this spec effectively decides it.

### 7. The font-size system is damson's alone

Two layers — three size primitives (`text-body/ui/small`) composing into 19 role tokens
(`text-title-*`, `text-label`, `text-mono`, `text-caption`, `text-num`, `text-table-row`, …), declared
in `globals.css` under `@layer components` and taught to `tailwind-merge` in `cn.ts`.

The contract has **no type scale beyond `--font-sans` / `--font-mono`**. This stays local, untouched,
and is the reason `cn` must not be overwritten. Worth stating in the spec so nobody reads "adopt the
token contract" as licence to flatten it.

## Questions for alignment

Everything above marked *recommendation* is one. Beyond those:

- **Does `docs/mockup/` follow the rename, or stop being the source of truth?** dialogue-db hit the
  same fork and **demoted** its mockup — code becomes the source of truth, the mockup keeps the old
  names and stops being upstream of the tokens. damson's `docs/design-system.md` names `docs/mockup/`
  as the visual reference, and `SpriteThumb` is ported *byte-identical* from
  `docs/mockup/src/thumbnails.jsx`, so demotion here is not free the way it was there.
- **One commit or several?** dialogue-db did nine stages, one commit each, and the vocabulary sweep
  was deliberately a single commit — a half-swapped vocabulary was the thing it most wanted to avoid.
  damson has no clone deadline forcing that, so it can be staged more finely if the review is easier.
- **A token guardrail?** dialogue-db added `web/scripts/check-tokens.mjs` to `task check` *after* its
  sweep, failing on any surviving old name and on raw hex outside `tokens.css`. damson has no
  equivalent gate. Cheap, and it is what makes "no half-swapped vocabulary" enforced rather than hoped
  for.
- **Which primitives retire?** dialogue-db retired four of thirteen. damson has 27, ten Radix-backed,
  and the registry has four items — so the honest answer is probably "`Button`, `Dialog`,
  `EmptyState`, and grow the rest during adoption". The estate rule is that **the inventory grows when
  a consumer needs it**; anything primitive-shaped and not damson-specific goes to the registry rather
  than staying here.
- **Dark-mode a11y.** damson overrides every contract value, so `shared-design`'s a11y gate has
  checked *its* palette, not damson's — in either theme. damson already runs stories at
  `a11y.test = "error"`, which is most of the mechanism.

## Docs this will make wrong

- `docs/design-system.md` — the whole Tokens section, the `Button`/`IconButton` entries, and the
  "Contributing a new component" tiering. It also says tokens map via `tailwind.config.ts`, which does
  not exist — the mapping is the `@theme` block in `globals.css` `[checked 2026-08-15]`.
- `docs/architecture.md:1508` — *"Radix headless primitives — no shadcn/ui."* True today, wrong after.
- `AGENTS.md` — no change expected beyond this file's pointer, but check the frontend bullets.
