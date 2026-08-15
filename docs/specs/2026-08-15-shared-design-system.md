# Adopt the shared design system

**Status:** signed off 2026-08-15 — not yet implemented.
**Alignment artifact:** `.lavish/spec-shared-design.html`.
**Input brief:** [`docs/shared-design-adoption.md`](../shared-design-adoption.md) — estate context,
the original measurements, the seven hazards. This spec supersedes it wherever the two disagree.
**Roadmap item:** `Next` #1 in [`docs/roadmap.md`](../roadmap.md), which banked ten decisions on
2026-08-15. Those are carried below as settled, not re-decided.

**Assumed base:** damson at `a67d367`; `shared-design` at `11db3c7`. All counts in this file were
measured at those commits and **must be re-measured at build time** — they size the work, they are
not targets. Nothing else is assumed to have landed.

## Problem

damson's frontend carries 57 hand-rolled `--ds-*` design tokens and 27 primitives assembled by hand
from 10 Radix packages. The estate has since settled on shadcn/ui as its primitive layer, with a
shared custom registry (`@shared`, `https://design.smol.cloud/r/<name>.json`) carrying only what
upstream lacks. The token *names* are the contract; palette values stay each project's own.

damson is consumer **two of two** in the proof and the brownfield half of it. dialogue-db tested a
clean install (shipped 2026-08-15, PR #39 `5e6b4ba`); damson tests installing into an existing
vocabulary. damson also carries **one exit criterion nobody else can**: a deliberate local
difference — its soft-red `destructive` — kept as a wrapper around the registry `Button` rather than
an edit to the installed file. That is the crux of copy-in ownership, and the only place it gets
tested.

Left alone, every component arriving from the registry expects names damson does not have, and the
translation cost is permanent.

## Chosen approach

**Install first, sweep once, gate last.** Scaffold `components.json`, install `@shared/tokens`,
rewrite `tokens.css` into the contract's vocabulary with damson's own values, then flip all 845
utility usages in **one mechanical commit**, and add the token guardrail immediately after it goes
green. Registry components arrive afterwards, onto the vocabulary they already expect.

It beat two alternatives:

- **Components first behind a shim** (install `Button`/`Dialog` early, alias contract names onto
  `--ds-*`, swap later) — introduces a third vocabulary that must later be deleted, and makes the
  half-swapped state a deliberate condition lasting weeks. A half-swapped vocabulary is the specific
  thing the estate set out to avoid.
- **Dual names, drained incrementally** (emit old and new utilities, migrate file by file) — smaller
  commits, but every file is reviewed twice for an identical end state, the emitted utility surface
  doubles while it runs, and there is no forcing function, so the last few sites never get drained.

The single sweep commit is mechanical, so the review surface is the mapping table below rather than
845 diffs. The guardrail can only go green at the end, which is itself the argument for doing it in
one go.

## Starting state, measured at `a67d367`

| | |
| --- | --- |
| Stack | React 19, Tailwind 4.3, Vite, Storybook 10.5 + `addon-a11y` |
| shadcn stack, assembled by hand | 10 Radix packages, `cva`, `clsx`, `tailwind-merge`, `lucide-react` |
| `components.json` | absent |
| Token declarations | 57 `--ds-*` in `web/src/design-system/tokens/tokens.css` |
| Token *utility* usages | **845** across **117** `.ts`/`.tsx`/`.css` files |
| Raw `var(--ds-*)` | 69 — but **38 are inside `globals.css`'s own `@theme` block**, which is being rewritten anyway. **31 real sites across 5 files**, 20 of them in `MarkdownEditorView.tsx` |
| Primitives | 27 under `design-system/primitives/`; `EmptyState` is a *composition*, not a primitive |
| Stories | 21 `*.stories.tsx`, running at `a11y.test = "error"` |
| Radii | `rounded-md` 51, `rounded-sm` 29, `rounded-full` 13, `rounded-pill` 11, `rounded-lg` 6, plus 9 `rounded-[3px]` |
| `<Button>` occurrences | 95 |

**This is a class-name rename, not a `var()` rename** — the opposite shape from dialogue-db, which
had ~1,050 `var()` calls and 46 utilities.

## Facts verified at write time

These correct the input brief and the roadmap item. Each was checked against the working tree, not
inferred.

1. **The guardrail script must come from dialogue-db, not from `personal-practices`.**
   `personal-practices/resources/check-tokens.mjs` is the **Svelte** variant: it scans `<style>`
   blocks in `.svelte` files for hex and px font-sizes, has **no retired-name check at all**, and its
   own header says React projects need a different variant. The roadmap credited it with the
   retired-name check. The real source is `dialogue-db/web/scripts/check-tokens.mjs` — React, walks
   `.css/.ts/.tsx`, carries a `RETIRED` list, and adds a check nobody wrote down: that `globals.css`
   declares no `:root`/`.dark` block, because `shadcn add` appends the reference palette there and it
   silently wins the cascade over `tokens.css`.

2. **The CLI generates the `@theme inline` block itself.** `@shared/tokens` is a `registry:theme`
   item with cssVars and no files. Per `shared-design/docs/consuming.md`, verified there against a
   scratch consumer, the CLI merges the cssVars into the consumer's CSS as `:root`/`.dark`, then
   generates the `@theme inline` mapping — including the `--radius-*` scale and a
   `@custom-variant dark`. So the roadmap's stage 0 ("switch `globals.css:16` from `@theme` to
   `@theme inline`") is not a hand edit; the work is *rescuing* damson's non-colour `@theme` entries
   into what the CLI writes.

3. **The contract's radius scale is multiplicative, not additive.** `shared-design/src/styles/theme.css`
   uses `sm = 0.6r`, `md = 0.8r`, `lg = r`, `xl = 1.4r`, `2xl = 1.8r`, `3xl = 2.2r`, `4xl = 2.6r`, and
   its own comment records the switch away from shadcn's stock `calc(var(--radius) - 4px)` form.
   Confirmed in dialogue-db's generated `globals.css`. **No choice of `--radius` reproduces damson's
   4 / 6 / 10** — the ratios are fixed and only the anchor moves.

4. **`--background-sunken` / `--background-inset` landed upstream** (`96eacda`), but both are
   *downward* steps and damson has neither. The gap that matters to damson — a name for the second
   *upward* step — is still open, and `shared-design/src/styles/tokens.css` still records it
   deliberately as thrice-sighted and unnamed.

5. **The registry `Button` and `Dialog` grew.** `Button` confirms `h-7 / h-8 / h-9` and adds
   `asChild`, `loading`, `fullWidth`, and `outline`/`default` compatibility aliases; it needs
   `@radix-ui/react-slot`. `Dialog` now takes `size="default | wide | full"` and has a scrolling body,
   and needs `tw-animate-css`; damson's takes a numeric `width` prop, so this is a real API change
   across its call sites. There is still **no `icon-button` item**.

6. **`EmptyState` is already API-compatible.** It lives in `design-system/compositions/`, not
   `primitives/`, and its props are `{icon, title, description, action}` — identical to the
   registry's. Two deltas: damson's `title` is optional where the registry's is required, and damson
   sets `role="status"`, which the registry does not.

7. **damson's font-size layer uses Tailwind 4's `@utility` directive**, not `@layer components` as
   the brief states.

8. **`library_settings.theme_default` is a real column** — `text NOT NULL DEFAULT 'dark'` from
   `database/migrations/005_settings.sql`, present in `internal/models/settings.go` and
   `internal/handlers/api/settings.go`, and editable from the Settings page. See Q1 below.

## The mapping

Counts are utility usages measured at `a67d367`.

| damson | Contract | Sites | Note |
| --- | --- | ---: | --- |
| `--ds-bg` | `--background` | 37 | straight |
| `--ds-panel` | `--card` | 34 | straight |
| `--ds-surface` | `--card` / `--popover` / `--muted` | 48 | judgement — see Q2's rule |
| `--ds-surface-2` | `--accent` / `--popover` / `--muted` | 12 | see Q2 |
| `--ds-line` | `--border` | 103 | 95 `border-` + 7 `bg-` + 1 `divide-` |
| `--ds-line-2` | `--border-strong` | 24 | extension, already in the contract |
| `--ds-text` | `--foreground` | 108 | straight |
| `--ds-text-2` | `--muted-foreground` | 72 | straight |
| `--ds-text-3` | `--subtle-foreground` | 169 | extension — the single largest rename |
| `--ds-text-4` | `--faint-foreground` | 57 | extension; sub-AA by design in both projects |
| `--ds-accent` | `--primary` | 45 | **trap 1** — the brand, *not* shadcn's `--accent` |
| `--ds-accent-2` | `--primary/90`, token dropped | 9 | the registry `Button` uses an opacity step; no contract name exists |
| `--ds-accent-tint` | `--primary-soft` | 10 | straight |
| `--ds-accent-ring` | `--ring` | 27 | straight |
| `--ds-accent-fg` | `--primary-foreground` | 11 | straight |
| `--ds-ok` / `-tint` | `--success` / `--success-soft` | 8 | straight |
| `--ds-warn` / `-tint` | `--warning` / `--warning-soft` | 17 | straight |
| `--ds-err` / `-tint` | `--destructive` / `--destructive-soft` | 42 | unifies licence-error, error UI and the danger `Button` under one name |
| `--ds-unk` / `-tint` | `--unknown` / `--unknown-soft` — **local** | 5 | see Q3 |
| `--ds-r-sm/md/lg/pill` | `--radius` + derived scale | 110 | see Q4 |
| `--ds-font-sans` / `-mono` | `--font-sans` / `--font-mono` | — | straight; Inter / JetBrains Mono values stay |
| `--ds-shadow` | stays local | — | no contract name; keeps the `shadow-elevated` utility |
| `--ds-sidebar-w` / `-rail-w` / `-detail-w` | stays local | — | layout, not palette |
| `--ds-transition-*` / `--ds-ease*` | stays local | — | motion; **must survive the CLI rewriting `@theme`** |
| `--ds-z-*` (8) | stays local | — | stacking; same survival note |
| `text-title` / `-heading` / … (19) | stays local, untouched | — | the contract has no type scale past `--font-sans`/`--font-mono`. This is why `cn` must not be overwritten |

**Installed but unused, and that is fine:** `--background-sunken`, `--background-inset`,
`--background-hover`, `--background-active`, `--secondary`, `--info`, `--input`, `--row-h`. The
`tokens` item is all-or-nothing; these simply sit unused until something wants them. Recorded so
nobody reads an unused token as a missed mapping.

## Task breakdown

Roughly one commit per stage.

### 1. Scaffold `components.json`; read the `init` diff before accepting it

`aliases.utils` → `@/design-system/utils`, `aliases.ui` → `@/components/ui`, register the `@shared`
namespace. Verify `init` creates no `src/lib/utils.ts` and does not touch the extended `cn`. The
`@/*` → `./src/*` path already resolves (`web/tsconfig.json`).

### 2. Install `@shared/tokens`; reconcile what the CLI wrote

The CLI appends `:root`/`.dark` to `globals.css` and generates the `@theme inline` block. Delete the
appended palette — `tokens.css` owns values — keep the generated mapping, and **merge damson's
non-colour `@theme` entries back into it**: 8 z-index, 3 transition durations, 2 easings, 3 widths,
3 animations and their `@keyframes`, the elevated shadow, and the three font-size primitives. Losing
any of these is silent.

### 3. Rewrite `tokens.css`: contract names, damson values, inverted selectors

Light values at `:root`, dark on `.dark`; the toggle sets and clears the class. `--card` and
`--popover` set **equal in both themes**. Update `design-system/tokens/index.ts` and
`.storybook/preview.tsx` to set/clear the class rather than the `data-theme` attribute. Resolves Q1,
Q3 and Q4.

### 4. The sweep — one mechanical commit

845 utility renames across 117 files, plus the 31 real `var(--ds-*)` sites in 5 files. Codemod-driven;
the mapping table above is the review surface. Includes `rounded-pill` → `rounded-full` and the Q2
role split. **Re-measure before starting.**

### 5. The guardrail

`web/scripts/check-tokens.mjs`, ported from `dialogue-db/web/scripts/check-tokens.mjs` with damson's
own `RETIRED` list, wired into `lint:web` so both `task check` and CI run it. Two adaptations: point
its hardcoded `src/styles/tokens.css` at `src/design-system/tokens/tokens.css`, and keep the
`globals.css` `:root`/`.dark` check — that is the trap that renders wrong and never fails. Do **not**
list `--unknown` / `--unknown-soft` as retired.

### 6. Codemod the prop-less `<Button>` call sites

95 `<Button>` occurrences measured at write time — **re-measure** — and add `variant="secondary"` to
every one with no `variant` prop *before* the default flips, so the flip is a visual no-op and new
code gets the new default.

### 7. Install `@shared/button`; wrap it for the soft-red destructive

Retire `design-system/primitives/Button.tsx`. Sizes move `h-6`/`h-7` → `h-7`/`h-8`/`h-9`. The wrapper
is the exit criterion — see Q5. Adds `@radix-ui/react-slot`.

### 8. Install `@shared/dialog` and `@shared/empty-state`

`Dialog`'s numeric `width` prop becomes `size="default | wide | full"` — the only genuine API change
in the slice. `EmptyState` is a near drop-in; carry `role="status"` across on the wrapper or record
dropping it deliberately. Adds `tw-animate-css`.

### 9. Publish `@shared/icon-button` from damson's implementation

Lands in `shared-design` first, then installs back here. damson's `IconButton` is the only remaining
evidence for publishing it at all, so this slice effectively decides it. Its `h-6` / `h-[34px]` sizes
need the same look-at-them-side-by-side treatment the `Button` heights are getting.

### 10. Storybook and a11y in both themes

21 stories at `a11y.test = "error"`. damson overrides every contract value, so `shared-design`'s a11y
gate has checked *its* palette, not damson's, in either theme. This stage is also where the inverted
selector is verified **at root and in a scoped wrapper** — the scoped case is the whole reason the
`@theme inline` distinction matters, since plain `@theme` bakes the value in at build time and stops
following a theme scoped below `:root`.

### 11. Docs

Rewrite `docs/design-system.md`'s Tokens section, the `Button`/`IconButton` entries, and the
"Contributing a new component" tiering; state the installed-vs-owned directory rule and the local
extension tokens. Move `docs/mockup/` to `docs/archive/` under damson's standing rule — Storybook
becomes the visual reference, and `SpriteThumb`'s byte-identical provenance from
`docs/mockup/src/thumbnails.jsx` becomes a note on the component. Correct `docs/architecture.md`'s
"Radix headless primitives — no shadcn/ui" line. Check `AGENTS.md`'s frontend bullets.

## Reuse

- **`@shared` registry** — `tokens`, `button`, `dialog`, `empty-state` install via the ordinary shadcn
  CLI. Copy-in source; the CLI is the provenance, not a version range.
- **`dialogue-db/web/scripts/check-tokens.mjs`** — the React token guardrail, ported rather than
  written. Also the reference for the whole shape: dialogue-db shipped this migration on 2026-08-15
  and its `web/src/styles/tokens.css` is a more reliable reading of what it settled on than its spec.
- **damson's existing `cn`** (`design-system/utils/cn.ts`) — extended with 24 registered `text-*`
  class names for the font-size system. Reused as-is via `aliases.utils`; **never overwritten**.
- **damson's `IconButton`** — becomes the registry item's reference implementation rather than being
  rewritten.
- **Searched and came up empty:** no codemod tool in the repo for class-name renames, and no existing
  script under `web/scripts/` at all — the directory does not yet exist. The sweep needs a
  throwaway script, not a reusable one.

## Resolved questions

### Inherited from the roadmap — settled 2026-08-15, not reopened

Recorded in `personal-practices/decisions/2026-08-15-design-system-vocabulary.md` and the roadmap's
decision table:

- shadcn's vocabulary is **primary, not aliased**; drop the `--ds-` prefix; `destructive` not
  `danger`; the default `Button` variant is `primary`; upstream direct for anything shadcn has and
  `@shared` only for what it lacks; **never overwrite `cn`**; the old `--accent` is the brand and
  becomes `--primary`.
- **B1** — invert the theme selector: light at `:root`, dark on `.dark`.
- **B2** — point `aliases.utils` at `@/design-system/utils`; create no `src/lib/utils.ts`. A
  re-export invites someone to later "simplify" it back to the stock two-liner, which deduplicates
  `text-mono` against a colour class and breaks damson silently.
- **B3** — installed (`components/ui/`) and owned (`design-system/primitives/`) stay apart, so
  "which files does re-running `add` overwrite?" is answered by a directory.
- **B5** — `--card` / `--popover` split by "the element positions itself", set **equal in both
  themes** so a wrong classification cannot render wrongly.
- **B6** — grep prop-less `<Button>` *before* the default flips, not after.
- **B7** publish `@shared/icon-button` · **B8** demote `docs/mockup/` · **B9** token gate plus a
  single sweep commit.
- Retire `Button`, `Dialog`, `EmptyState`; the other primitives stay, and the registry inventory
  grows when a consumer needs it.

### Q1 — the theme default leaves the database entirely

`library_settings.theme_default` is a real, admin-editable, server-persisted column, and precedence
today is `stored → library default → 'dark'`. Because the column defaults to `'dark'`, every existing
library shadows any system-preference fallback — so changing only the final fallback would have
changed nothing for anybody.

**Decided: the theme becomes purely a browser concern.**

- `resolveInitialTheme()` becomes `stored → prefers-color-scheme`. `theme_default` is **no longer
  consulted at all**.
- While nothing is stored, damson **follows the OS live** via a `matchMedia` change listener; the
  first explicit toggle stops it following.
- The theme control comes out of the Settings page — a setting that no longer does anything is worse
  than no setting.
- **No migration in this slice.** The column stays where it is, unread. This slice touches **no Go
  and no SQL**.

### Q2 — `--ds-surface-2` needs no contract name

The roadmap (B4) decided to ask `shared-design` to name the second upward step and treat it as a
stage-0 dependency. Reading all twelve `bg-surface-2` sites dissolved the premise: six are the
hover/selected surface (`Button` hover/active/pressed, `SegmentedControl` selected, `Chip` close
hover, `PdfPreview` nav hover), three are floating (`Tooltip`, `SelectionBar`, `FileBrowser`'s
selection bar), and three are muted fills (`AssetDetailPanel` skeleton, `UserMenu` avatar). Nothing is
left over.

**Decided: split by role — `--accent`, `--popover`, `--muted` respectively.** damson takes no new
contract names and the cross-repo blocker disappears. The rule matches the contract's own porting
note that "shadcn splits elevation by role rather than by depth", and its "your old `--bg-hover`
becomes `--accent`".

**And still file the ask upstream** — not as a request for a name, but to tell `shared-design` that
damson's third sighting dissolved, so taskmanager and mangapack are re-examined before a name is
minted for what may be the same illusion.

### Q3 — the fourth licence status stays local

`LICENCE_STATUSES` is `["ok","warn","err","unk"]`. Three map cleanly; `unk` has no contract
equivalent. Mapping it to `--muted-foreground` was proposed and **rejected**: the values happen to be
close, but it mislabels a fill as a foreground, and the licence axis genuinely is four-valued.

**Decided: `--unknown` and `--unknown-soft` are damson's declared local extensions.** They go in
`tokens.css` under an explicit "local, not contract" heading, are excluded from the guardrail's
`RETIRED` list, and are recorded in `design-system.md` as a contract candidate if a second project
ever sights one. `--info` is the wrong answer — it is blue, and "unknown" is not informational.

### Q4 — `--radius: 0.5rem`, and the scale shape is an upstream problem

Given the fixed multipliers, the anchor is the only lever and none reproduces damson's 4 / 6 / 10.

**Decided: `--radius: 0.5rem`** → `sm` 4.8, `md` 6.4, `lg` 8, `xl` 11.2. The sub-pixel drift on ~80
sites is imperceptible; only the six `rounded-lg` sites move a visible 2px. damson keeps its tight
feel and the one-knob property stays true. `rounded-pill` → `rounded-full` on 11 sites.

Rejected: `0.625rem`, which gives clean integers and an exact `lg` but adds 2px to 80 sites — a
visible roundness increase on a deliberately compact UI, and the wrong direction given the scale is
too tight at the *bottom*, not the top. Also rejected: overriding `--radius-sm/md/lg` to 4/6/10
explicitly, which re-creates the exact hazard that made `shared-design` delete `theme.css` as a
shipped file — a consumer carrying a second, divergent radius scale that the next `add` regenerates
over.

The real finding goes upstream. damson's scale is 4 / 6 / 10 — ratios `0.4 / 0.6 / 1.0`. The
alternative sketched during alignment is 4 / 6 / 12 / 16 / 24 — ratios `0.25 / 0.375 / 0.75 / 1.0 /
1.5`. The contract's bottom end is `0.6`. **Both want a steeper bottom than a single multiplier can
produce**, which is a property of the scale's shape rather than of its anchor.

### Q5 — the soft-red destructive stays as a wrapper

The registry's `destructive` is filled; damson's `danger` is a soft tint. `shared-design`'s
`button.tsx` now names damson in a comment and floats a `destructive-subtle` variant.

**Decided: damson keeps a thin wrapper composing the registry `Button`, and `shared-design` holds
`destructive-subtle` for now.** The wrapper *is* the exit criterion — proving copy-in ownership needs
something to own. Shipping the upstream variant first would satisfy damson's visual need while
deleting the only test of the mechanism.

## Deferred follow-ups

- **Drop `library_settings.theme_default`** — the column, its API field and its GORM tag survive this
  slice unread, because removing them is itself a database change and the point of the decision was
  to take the theme out of the database. A later slice can drop it with a migration.
- **Revisit the shared contract's radius scale shape** — the multiplicative derivation
  (`sm = 0.6r … 4xl = 2.6r`) cannot express the steeper bottom end that both damson's current scale
  and the sketched alternative want. This is a `shared-design` decision affecting every consumer, and
  it wants looking at once a second repo has real corners in it.
- **Report the dissolved third sighting to `shared-design`** — damson's `--ds-surface-2` decomposed
  into three existing roles rather than needing a name. taskmanager's and mangapack's second upward
  steps should be re-examined the same way before a name is minted.
- **`Button` and `IconButton` heights, looked at side by side** — the registry's `h-7/8/9` is a
  midpoint between damson's compact `h-6/h-7` and dialogue-db's 30px, and nobody has yet compared
  them in one place. Adopting the registry `Button` here is what finally puts them side by side.
- **Migrate the nine `rounded-[3px]` arbitraries** onto a scale value — they predate the token system
  and the guardrail does not flag non-colour arbitraries.
- **Dark-mode a11y coverage for damson's own palette** — `shared-design`'s gate has only ever checked
  its reference palette. Stage 10 covers the story set; a broader sweep across composed pages is not
  in scope here.

## Deferred comments

Transient scratch for `/cleanup` — one bullet per candidate. Not comment text; a prompt to
reason about later.

- `web/components.json` — `baseColor: "neutral"` is inert (only `init` and base-color installs read
  it) but reads as if damson uses shadcn's neutral palette, which it emphatically does not. JSON
  cannot carry a comment, so if this is worth saying it belongs in `docs/design-system.md`.
- `web/components.json` — why `aliases.utils` points at `@/design-system/utils` rather than the
  conventional `@/lib/utils` (B2: the extended `cn` and its 24 registered `text-*` groups live
  there, and a re-export invites someone to "simplify" it back to the stock two-liner). Same
  constraint: belongs in `design-system.md`, not the file.
- `web/src/styles/globals.css` — why there are three `@theme` blocks and why the legacy `--ds-*` one
  is deliberately **last**: it must keep winning the cascade so stages 2-3 render identically to
  today, because `--color-accent` collides (legacy brand yellow vs the contract's hover surface).
  Stage 4 deletes that block in the same commit as the call-site sweep, so the flip is atomic.
- `web/src/styles/globals.css` — the first block is CLI-owned (`shadcn add @shared/tokens`
  regenerates it); damson's own theme entries are in a separate block on purpose so re-running the
  CLI cannot clobber them.
- `web/src/design-system/tokens/tokens.css` (stage 3) — `--row-h` must be defined or the generated
  `--row-h: var(--row-h)` mapping stays a self-referential no-op.
- `web/index.html` — why a blocking inline script exists at all: the B1 inversion makes light the
  default, so without it every dark user gets a flash of light before React mounts. The `catch`
  defaults to dark deliberately (a thrown `matchMedia`/`localStorage` in privacy mode should not
  mean "light").
- `web/src/hooks/useTheme.ts` — why the first effect calls `applyTheme` and not `applyAndStore`:
  storing on mount would make a stored value always exist, and "follow the OS until the user picks"
  would never fire. The `[mode]` dependency on the watcher effect is deliberate — it re-checks
  `readStoredTheme()` so an explicit toggle unsubscribes.
- `web/src/design-system/tokens/tokens.css` — `--primary-hover` is a deviation from the spec, which
  said to drop `--ds-accent-2` in favour of `--primary/90`. Four of its sites are a gradient
  (`from-accent to-accent-2`) and opacity is not lightness, so the token survives as a local
  extension. Worth a line in `design-system.md`.
- `web/src/design-system/primitives/MarkdownEditorView.tsx` — why the border radius is a raw
  `calc(var(--radius) * 0.6)` rather than `var(--radius-sm)`: `@theme inline` emits no custom
  property, so `--radius-sm` does not exist at runtime. Only the utility does.
- `web/src/styles/globals.css` — the three `--color-*` mappings for `--unknown`, `--unknown-soft`
  and `--primary-hover` are what make those utilities exist at all. Without them Tailwind silently
  emits nothing, which is the characteristic failure of this whole migration.
- `web/scripts/check-tokens.mjs` — why `accent` is deliberately absent from `RETIRED`: it is a live
  contract name meaning the hover surface, so listing it would flag every correct `bg-accent`.
- `web/src/design-system/tokens/tokens.css` — `--code-canvas` is GitHub Dark's canvas and must track
  `HIGHLIGHT_THEME` in `previews/highlight.ts`, not damson's palette. It is here because tokens.css
  is the only place a raw value may live, not because it is a semantic token.
