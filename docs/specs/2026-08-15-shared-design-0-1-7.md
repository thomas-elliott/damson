# Take up `shared-design` 0.1.7

**Status:** signed off 2026-08-15 — not yet implemented.
**Alignment artifact:** `.lavish/spec-shared-design-0-1-7.html`.
**Predecessor:** [`history/2026-08-15-shared-design-system.md`](history/2026-08-15-shared-design-system.md)
— the vocabulary adoption, merged the same morning (PR #40, `f91d654`). This spec revises two of its
resolved questions; per the standing rule it does not rewrite them.
**Roadmap item:** `Next` #2 in [`../roadmap.md`](../roadmap.md).

**Assumed base:** damson at `f91d654`; `shared-design` at `df21ae4` (the 0.1.7 tree). Every count in
this file was measured at those commits on 2026-08-15 and **must be re-measured at build time** —
they size the work, they are not targets. Nothing else is assumed to have landed.

## Problem

damson finished adopting the shared contract's *colour* vocabulary on the morning of 2026-08-15.
`shared-design` 0.1.7 landed the same day and extended the contract past colour: four scales that
were previously each project's own — radius, type, motion, elevation — plus two new components and a
`tailwind-merge` extension.

Nothing in damson breaks today. The drift is entirely latent: it materialises the next time anyone
runs `shadcn add`, at which point the CLI regenerates the `@theme inline` block and damson's corners,
type sizes and transition durations change underneath whatever else that install was for. That makes
this cheap to defer and expensive to forget — the failure mode is an unrelated component install that
silently reshapes the UI.

Two things also want fixing regardless of the contract. damson's typing surfaces are at 14px, which
is the iOS-Safari zoom bug the contract's 16px floor exists to prevent. And damson's 48 bare `ease`
classes emit nothing at all (see "Facts verified at write time" #4), so 48 transitions have never
used the easing they appear to specify.

## Chosen approach

**Install, reconcile, resolve the collisions by hand, sweep once, gate.** The same shape the
predecessor slice used, and for the same reason: the guardrail can only go green at the end, so the
mechanical renames want to be one reviewable commit whose review surface is the mapping table rather
than the diff.

The one structural difference is stage 3. The predecessor was a pure substitution; this slice has
**four names where damson and the contract both define the same spelling with different meanings**,
and those need judgement at ~14 call sites. Separating them from the sweep keeps the sweep
mechanical.

Alternatives considered and rejected are recorded per-decision under "Resolved questions" — the
radius one (accept the reflow vs. shift every site down a step vs. override the values locally) is
the load-bearing choice in the slice.

## Starting state, measured at `f91d654`

| | |
| --- | --- |
| Type: size primitives | `--text-body` 15/1.5, `--text-ui` 14/1.4, `--text-small` 12/1.4 in `globals.css`'s damson-owned `@theme inline` |
| Type: composed roles | 22 `@utility text-*` rules; 25 names registered in `design-system/utils/cn.ts` |
| `text-ui` usages | 103 |
| `text-small` usages | ~91 |
| `text-label` usages | 17 — 6 in app code, 11 in stories |
| Bare `text-title` / `-heading` / `-subhead` usages | 14 — 8 in app code, 6 in stories |
| Radii | `rounded-md` 54, `rounded-sm` 27, `rounded-full` 24, `rounded-lg` 7, `rounded-[3px]` 9 |
| `--radius` | `0.5rem`, deriving `sm` 4.8 / `md` 6.4 / `lg` 8 / `xl` 11.2 |
| `duration-std` usages | 44 |
| `duration-slow` usages | 1 (`compositions/Section.tsx`) |
| `duration-fast` usages | 0 — the 80ms token is declared and unused |
| Bare `ease` usages | 48, **all of them no-ops** |
| `ease-bounce` usages | 0 as a utility; `--ease-bounce` is consumed by `--animate-pop-up` |
| `shadow-elevated` usages | 13 |
| Typing surfaces | `Input`, `Textarea`, `SearchInput` — all at `text-ui`, all already on `bg-background` |

## Facts verified at write time

Each checked against the working trees, not inferred. The first two correct the 0.1.7 handover.

1. **`rounded-pill` → `rounded-full` is already done.** Zero `rounded-pill` remain, no
   `--radius-pill` is declared, and `scripts/check-tokens.mjs` already fails the build on the
   spelling. The handover's "damson currently runs both spellings" was true at `a67d367`, before the
   predecessor slice.

2. **`--fs-*` → role names is already done.** damson has never had `--fs-*`. It is on
   `--text-body` / `--text-ui` / `--text-small` plus composed `@utility` roles — which is precisely
   what makes fact 3 bite.

3. **The type collision is four names, not one.** The handover flagged `text-label`. damson also
   defines bare `@utility text-title`, `text-heading` and `text-subhead`, each of which the contract
   declares as a theme token at the same size. damson's four additionally bake in a weight, and its
   `text-label` bakes in uppercase and tracking, so all four are semantically different from the
   contract's same-named sizes. Whether Tailwind errors on the clash or silently picks one is
   **unverified** — see stage 3.

4. **The 48 bare `ease` classes emit nothing.** `--ease-ease` generates `ease-ease`, not `ease`, and
   `ease` is not a Tailwind built-in. Confirmed against the built stylesheet
   (`web/static/assets/index-*.css`, built 2026-08-15 post-`f91d654`): it contains `.duration-std`
   and **no `.ease` rule and no `.ease-*` rule at all**. So `ease` → `ease-standard` is a
   latent-bug fix — 48 transitions currently run on Tailwind's default curve and will visibly change.

5. **`@shared/type-scale` cannot replace damson's `classGroups` list.** `src/lib/type-scale.ts`
   registers exactly 8 names via `extend.theme.text`. damson's list is 25, and 17 are local composed
   roles the contract has no equivalent for — `text-mono` (59 sites), `text-caption` (48),
   `text-num`, `text-table-row`, `text-table-header`, `text-list-row`, and 12 weight variants.
   Replacing rather than merging re-introduces the exact silent-drop bug the extension exists to
   prevent, across ~120 sites. `consuming.md` says "replaces"; for damson it must be a merge.

6. **`--subtle-foreground` has no upstream value to copy.** 0.1.7 raised its own to
   `oklch(0.54 0 0)`. damson's light value is already `oklch(0.52 0.01 80)` — darker. damson's
   contrast problem comes from its warm surfaces (`--card` 0.965, `--muted` 0.945) sitting below
   upstream's white, not from the tier being too light. What transfers is the precedent — fix the
   token, don't exempt the tier — not the number. Out of scope here; see Q3.

7. **damson's typing surfaces are already on `--background`.** `Input` and `Textarea` both carry
   `bg-background`, which is what 0.1.7 settled on after finding `--background-inset` puts
   placeholder text at 4.48:1. So the field work is a font-size change only, not a surface change.

8. **`@shared/input` and damson's `Input` do not compose.** The registry's is a bare `<input>`
   carrying its own border and background. damson's is a wrapping `<div>` that carries the border and
   background, with `leadingIcon`, `trailingSlot` and `invalid` props; `SearchInput` builds on it
   with a magnifier and a `Kbd`. Nesting them produces two borders. See Q4.

## The mapping

Counts measured at `f91d654`. **Re-measure before starting.**

### Type

| damson today | 0.1.7 | Sites | Note |
| --- | --- | ---: | --- |
| `--text-body` 15/1.5 | `--text-body` 15/**1.55** | — | value change; spelling unchanged |
| `--text-ui` 14/1.4 | `--text-ui` 14/**1.45** | 103 | value change; spelling unchanged |
| `--text-small` 12/1.4 | `--text-label` 12/1.4 | ~91 | rename onto a name damson currently occupies |
| `@utility text-label` (12 + medium + uppercase + wide tracking) | `@utility text-overline`, unchanged rules | 17 | 6 app, 11 stories — frees `text-label` for the line above |
| `@utility text-title` / `-heading` / `-subhead` (size + 600) | deleted; call sites take the existing `-semibold` spelling | 14 | 8 app, 6 stories — see Q1 |
| — | `--text-display` 28/1.2 | 0 | new, unused. damson's largest is `title` at 22 |
| — | `--text-field` 16/1.4 | 3 | new; the 16px floor |
| `text-mono`, `text-caption`, `text-num`, `text-table-row`, `text-table-header`, `text-list-row`, the 12 weight variants | stay local, unchanged | ~120 | **must stay registered in `cn`** — fact 5 |

### Radius

| damson today | 0.1.7 | Sites | Note |
| --- | --- | ---: | --- |
| `rounded-sm` 4.8 | `rounded-sm` **6** | 27 | spelling unchanged |
| `rounded-md` 6.4 | `rounded-md` **12** | 54 | spelling unchanged; the largest visual change in the slice |
| `rounded-lg` 8 | `rounded-lg` **16** | 7 | spelling unchanged |
| `rounded-full` | `rounded-full` | 24 | unchanged; there is no pill token |
| `rounded-[3px]` | `rounded-xs` (4) | 9 | closes a follow-up the predecessor deferred |
| `calc(var(--radius) * 0.6)` in `primitives/MarkdownEditorView.tsx` | `var(--radius-sm)` | 1 | these are real custom properties now |
| `--radius: 0.5rem` | `--radius` becomes an alias of `--radius-lg` (16px) | — | anything reaching for `var(--radius)` by hand resolves to 16, not 8 |

### Motion

| damson today | 0.1.7 | Sites | Note |
| --- | --- | ---: | --- |
| `duration-std` 100ms | `duration-fast` 120ms | 44 | |
| `duration-slow` 120ms | `duration-fast` 120ms | 1 | same value it has today; the contract's `slow` is 260ms, which is not what this site means |
| `duration-fast` 80ms | dropped | 0 | declared and unused |
| bare `ease` (emits nothing) | `ease-standard` | 48 | fact 4 — a behaviour change, not a rename |
| `--ease-ease` | `--ease-standard` | — | |
| `--ease-bounce` | stays local | — | consumed by `--animate-pop-up`, not used as a utility |
| — | a global `prefers-reduced-motion` block | — | the CLI writes it in from the item's `css` field; damson has none today |

### Elevation

| damson today | 0.1.7 | Sites | Note |
| --- | --- | ---: | --- |
| `--elevation` → `shadow-elevated` | `--shadow-sm` / `-md` / `-lg`, per theme | 13 | overlays take `lg`; resting surfaces take `md` — see Q5 |

## Task breakdown

Roughly one commit per stage.

### 1. Re-run `add` at 0.1.7 and reconcile what the CLI wrote

`@shared/tokens` first, then `type-scale`, then re-run `button`, `dialog`, `empty-state` and
`icon-button` so all six sit on one version.

Three things to clean up after, all of which bit the predecessor slice on **every** install:

- Delete the reference `:root` / `.dark` palette the CLI appends to `globals.css`. It lands after
  `tokens.css`, unlayered, so it wins the cascade. `check-tokens.mjs` already fails on this — so end
  the stage with `task check`, not with a browser.
- Keep the generated `@theme inline` block and merge damson's non-contract entries back into its
  **second** block: the 8 z-index, 3 widths, 3 animations and their `@keyframes`, the local colours
  (`--unknown`, `--unknown-soft`, `--primary-hover`, `--swatch-foreground`), and `--ease-bounce`.
  Losing any of these is silent. The three font-size primitives and the motion durations are now the
  contract's and come out of damson's block.
- `registryDependencies` resolve against damson's own `components.json`, not against the URL
  installed from.

Verify the wrapper survives: `design-system/primitives/Button.tsx` must be byte-identical afterwards.
That is the predecessor slice's exit criterion and this is the first re-install since it was proven.

### 2. Merge `typeScale` into `cn`

`extendTailwindMerge` takes the imported extension **and** damson's 17 local composed names —
`extend.theme.text` from `@shared/type-scale` merged with damson's `extend.classGroups`. Fact 5 is
why this is a merge.

Add a `cn` unit test asserting `cn("text-mono", "text-muted-foreground")` keeps both classes, so a
future "simplification" back to the stock two-liner fails loudly instead of silently. There is no
existing test for `cn`; `design-system/primitives/Button.test.tsx` is the nearest pattern for a
plain Vitest file in that directory.

### 3. Resolve the four type collisions

The only stage in the slice that is judgement rather than substitution, which is why it is separate.

- **First, establish what Tailwind actually does** when a theme-generated `text-label` meets an
  `@utility text-label` — error, or one silently wins. That determines whether the build can be
  trusted to catch a missed rename in stage 4, and it is currently unknown.
- Rename `@utility text-label` → `@utility text-overline`, rules unchanged, and update its 17 sites.
- Delete the bare `@utility text-title`, `text-heading` and `text-subhead`. Their 14 call sites take
  the existing `-semibold` spelling (see Q1).
- Update the registered names in `cn.ts` and `design-system/tokens/Tokens.stories.tsx` to match.

### 4. The sweep — one mechanical commit

`text-small` → `text-label` · `duration-std` → `duration-fast` · `duration-slow` → `duration-fast` ·
bare `ease` → `ease-standard` · `shadow-elevated` → `shadow-lg` / `shadow-md` per Q5 ·
`rounded-[3px]` → `rounded-xs` · the `calc(var(--radius) * 0.6)` in `MarkdownEditorView.tsx` →
`var(--radius-sm)`.

Codemod-driven; the mapping tables above are the review surface. **Re-measure every count first.**

The bare-`ease` rename needs care: `ease` is a common English word, so match it only inside class
strings adjacent to another utility rather than as a bare word.

### 5. The 16px field floor

`Input`, `Textarea` and `SearchInput` move from `text-ui` to `text-field`. Per fact 7 the surface is
already correct, so this is a font-size change only.

Check the surrounding layouts by eye — a 2px bump in a dense row can push a toolbar over, and
`SearchInput` sits in the library top bar. `Input`'s `py-1.5` and `Kbd`'s sizing are the two places
to look.

### 6. Extend the guardrail

`scripts/check-tokens.mjs`. Its existing `RETIRED` list feeds a regex built from `COLOUR_PREFIXES`,
so the new names need a **second** list and regex — they are size and motion utilities, not colour
ones. Add: `text-small`, `duration-std`, `shadow-elevated`, `--elevation`, `rounded-[3px]`, and bare
`ease`.

Two constraints worth knowing before writing it:

- The retired-name checks currently run **before** the comment/fixture early-return, so a bare `ease`
  in prose or a comment would false-positive. Either move the new check after that return or scope it
  to class-attribute context.
- Add the rule 0.1.7 exists to make travel: a typing surface may not carry a size below `text-field`.
  Implement it as a **file allowlist** (`design-system/primitives/{Input,Textarea,SearchInput}.tsx`
  plus `components/ui/{input,textarea}.tsx`) rather than trying to detect a typing surface
  generally — a scoped check that works beats a general one that guesses.

Keep the existing `globals.css` `:root` / `.dark` check unchanged; it earns its place on every
install.

### 7. Storybook in both themes, and the docs

Re-render the token stories against the new scales and verify the scoped-theme case still follows —
that is what the `@theme inline` distinction buys and it is worth re-checking after the CLI
regenerates the block.

Docs: `design-system.md`'s **Typography**, **Sizing, radii, motion, z-index** and **Tailwind
mapping** sections, and its **Input** primitive entry. Record the `@shared/input` divergence (Q4) as
a deliberate local difference, next to the `Button` wrapper precedent. `architecture.md` needs no
change — it carries no radius or type claims; only its design-system pointer, already corrected.

## Reuse

- **`@shared/type-scale`** (`src/lib/type-scale.ts`) — the `tailwind-merge` extension, installed as
  source rather than hand-maintained. Merged into damson's existing `cn`, never replacing it.
- **damson's existing `cn`** (`design-system/utils/cn.ts`) — kept and extended, per the standing
  "never overwrite `cn`" rule and `components.json`'s `aliases.utils`.
- **damson's `scripts/check-tokens.mjs`** — extended, not rewritten. Its `:root`/`.dark` check and
  its walk/report structure carry over unchanged.
- **damson's `-semibold` type variants** — `text-title-semibold` and siblings already exist, are
  already registered in `cn`, and already render exactly what the bare aliases render. The collision
  fix reuses them rather than introducing `font-semibold` at 14 call sites.
- **`shared-design/docs/consuming.md`** — the migration guide, including the porting tables and the
  two clean-up-after-install warnings. Read it rather than re-deriving the mapping.
- **Searched and came up empty:** no codemod tooling in the repo; `web/scripts/` holds only
  `check-tokens.mjs`. The sweep needs a throwaway script, not a reusable one — same finding as the
  predecessor slice.

## Resolved questions

### Q1 — the four colliding type names: the contract's sizes win

damson and the contract both define `text-title`, `text-heading`, `text-subhead` and `text-label`.
The contract's are plain sizes; damson's bake in a weight, and `text-label` bakes in uppercase and
tracking. Meanwhile damson's `text-small` wants the name `text-label` that its own uppercase role
occupies.

**Decided: A1 — the contract's sizes win outright.** The four `@utility` rules go, the uppercase role
is renamed **`text-overline`**, and `text-small` → `text-label` across ~91 sites.

Two alternatives were weighed. *Keep damson's compositions and rename only what collides* was
rejected for leaving two spellings for one size, with nothing to tell a reader which is which. *Don't
take the type scale yet* was rejected because `@shared/tokens` is all-or-nothing — the four names
would have to be actively undone after every `add`, which is the two-vocabularies problem again in
the one place the guardrail cannot see it.

**One refinement on the signed-off wording.** The artifact said the bare heading aliases "become
`text-title font-semibold`". The spec instead uses the existing `text-title-semibold` spelling: same
rendered result, one class instead of two, already registered in `cn`, and already the idiom at
`primitives/Markdown.tsx`. Same decision, cheaper mechanics.

`text-overline` was chosen over `text-eyebrow` and `text-section-label` as the conventional name for
this treatment.

### Q2 — accept the radius reflow

**Decided: B1 — take the literal scale as shipped.** `rounded-sm` 4.8 → 6, `rounded-md` 6.4 → 12,
`rounded-lg` 8 → 16. damson gets visibly rounder on 88 sites, and the nine `rounded-[3px]`
arbitraries land on `rounded-xs`.

**This supersedes Q4 of the predecessor spec**, which chose `--radius: 0.5rem` to keep damson's
compact feel against a multiplicative scale. That scale no longer exists, and upstream's own guidance
is explicit: *"expect a visible reflow, and do not fight it. Nothing in these designs was signed off
in the first place."* Per the standing rule this is a new dated decision, not an edit to that spec.

Two alternatives were weighed:

- **Shift every site down one step** (`md`→`sm`, `sm`→`xs`, `lg`→`md`) lands within a pixel of
  today's look on 81 of 88 sites and overrides nothing. Rejected because registry components hardcode
  their own corners — `@shared/input` is `rounded-md`, so 12px — leaving damson's own components at
  6px beside installed ones at 12px. That is the two-vocabularies problem again, in geometry.
- **Override `--radius-sm/md/lg` locally.** Upstream calls this "legal and off the happy path".
  Rejected because it recreates the exact hazard that made `shared-design` stop shipping `theme.css`
  as a file: a consumer carrying a second, divergent scale that the next `add` regenerates over.

### Q3 — the palette a11y work stays its own slice

`--subtle-foreground` is decision (a) of the queued *"Turn the Storybook a11y gate back on"* roadmap
item, alongside a `destructive`-on-`destructive-soft` pairing at 4.42:1 and a Markdown link at
1.13:1.

**Decided: C1 — this slice stays mechanical.** It takes the four scales and the field floor; all
three palette questions stay with the a11y-gate item, which already has them scoped. 0.1.7 settled
the *principle* upstream — fix the token, don't exempt the tier — and that has been recorded against
the roadmap item. Per fact 6 there is no upstream number to copy, so damson has to measure its own
regardless, and doing that here would mix a mechanical sweep with three colour judgements. One of
them, the link affordance, is not a colour question at all.

### Q4 — take the `--text-field` token, keep damson's owned primitives

**Decided: D1.** `Input`, `Textarea` and `SearchInput` move to `text-field` and stay damson's;
`@shared/input` and `@shared/textarea` are **not** installed. Fact 8 is why: the two designs put the
border and background on different elements, so nesting them double-borders, and damson's icon and
keyboard-hint slots have no registry equivalent.

The guardrail carries the rule instead of the component (stage 6), so the 16px floor is enforced
rather than merely applied.

*Install both and rebuild the affordances as overlays* was rejected as a real rewrite of three
primitives and their stories to reach the same rendered result. *Send damson's icon-slot API upstream
first* — the path `IconButton` took, and the estate's own rule for anything primitive-shaped and not
damson-specific — is the right long-run answer but would block this slice on a `shared-design`
change. Deferred below.

### Q5 — `shadow-elevated` splits by role, not by depth

damson has one `--elevation`; the contract has three, with different values per theme.

**Decided: split the 13 sites by role, and take the contract's values as installed.** Overlays —
`Popover`, `DropdownMenu`, `Select`, `DetailPanel`, `SelectionBar`, `FileBrowser`'s selection bar,
`NotificationCentre` — take `shadow-lg`. Resting raised surfaces — the three auth-page cards, the PDF
preview canvas — take `shadow-md`. Confirm each by looking at it rather than by the list; the list
was assembled by grep.

damson's single `--elevation` was never a considered scale, so there is no damson value to preserve
for `sm` and `md`. Override `--shadow-lg` to damson's existing `--elevation` value only if the
installed one reads wrong against damson's palette — the names are the contract's, the values are
damson's wherever damson has a view.

## Deferred follow-ups

- **Contribute damson's icon-slot field API to `@shared/input`** — a leading-icon / trailing-slot
  input is primitive-shaped and not damson-specific, so by the estate's own rule it belongs in the
  registry. The same path `IconButton` took. Would let damson retire three owned primitives.
- **`--text-display` (28px) is installed and unused** — damson's largest role is `title` at 22px.
  Recorded so nobody reads an unused token as a missed mapping.
- **Extend the 16px floor to `select`, `command` and combobox search fields** — upstream names these
  as carrying the same 14px class and explicitly not yet covered by the contract. damson's `Select`
  is Radix-backed and owned, so it is damson's to fix whenever it grows a typing surface.
- **`--duration-normal` (180ms) and `--duration-slow` (260ms) are installed and unused** — damson's
  motion collapses onto `fast` once `duration-slow`'s single site moves. Whether damson wants a
  genuine slow step is a design question nobody has asked.
- **Revisit `--ease-bounce`** — it survives as a local extension consumed only by `--animate-pop-up`.
  If a second project sights a bounce curve it is a contract candidate.
