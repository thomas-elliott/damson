# Damson Design System

Damson's UI is built in `web/src/design-system/` — primitives, compositions and domain components
behind the `@/design-system` barrel — on Tailwind 4 and Radix. **Storybook is the visual reference
and the catalogue** (`just storybook`); this document holds the contract and the traps. Feature
code in `web/src/pages/` and `web/src/components/` composes the system rather than re-rolling it,
and design-system components do no data fetching: a caller passes data in.

The token vocabulary is shadcn's, extended; the values are Damson's, and every colour value lives in `web/src/design-system/tokens/tokens.css`. Light is the
base (`:root`); dark opts in through the `.dark` class.

---

## Contract

### Contract components

`Button`, `IconButton`, `Dialog` and `EmptyState` began as copies of an external component package
and now live in `primitives/` and `compositions/` like everything else. They keep its contract:
`Button` variants `primary | secondary | ghost | destructive`, sizes `sm | md | lg`, state on
`data-*`. `IconButton` is always `ghost`, at the same three sizes.

- **`Button`'s `destructive` is a soft red tint, not a fill**, because a filled red reads fine in a
  confirm dialog and shouts in a toolbar.
- **`EmptyState` sets `role="status"`** and its title is optional.
- **`ErrorState` is `EmptyState` with a Retry.** The caller gives fixed wording; the server's
  error message is never shown. Retry stays disabled until the retry it started settles.
- **`Input` is a wrapping `<div>`** that carries the border and background, with icon and
  keyboard-hint slots; it takes the `--text-field` token and the 16px floor from the contract.

### Tokens and Tailwind

There is no `tailwind.config.ts`. `web/src/styles/globals.css` carries two `@theme inline` blocks,
in order: the **contract** block, which maps the vocabulary onto `tokens.css`; then **Damson's**,
which holds the extension colours, z-indexes, widths, animations and the contract's scales restated
as literals. Tailwind merges `@theme` blocks with the later one winning, so Damson's block
overrides the first.

Colour tokens map `--color-x: var(--x)`, two different names, so their values live in
`tokens.css`. The scales (radius, type, duration) are literals in Damson's block. The font stacks
(`--sans-stack`, `--mono-stack`) and per-theme shadows (`--elevation-sm/md/lg`) are values in
`tokens.css` mapped across in Damson's block. Prefer the utility over `var(--token)` — `bg-card`,
`text-muted-foreground`, `border-border`, `rounded-md`.

Type is two-layered. The contract's role sizes (`text-display` … `text-label`) carry size and line
height only. Damson's composed `@utility` rules in `globals.css` add a weight, family or transform
on top: weight variants such as `text-title-semibold`, and roles such as `text-mono`,
`text-overline` and `text-num`. There is no bare alias: `text-title` is the contract's plain size.

Surfaces: an element that positions itself (popover, dropdown, dialog, tooltip, context menu,
selection bar) takes `--popover`; everything else takes `--card`. The two are equal in both themes
on purpose, so a misclassified element cannot render correctly in one theme and wrongly in the
other. Radii nest by subtraction (`inner = outer − padding`), not by ratio.

`just lint frontend` (and so `just check`) runs `web/scripts/check-tokens.mjs`. It fails on a
retired token name, a raw hex in a stylesheet outside `tokens.css`, an arbitrary colour utility, a
`:root` or `.dark` block in `globals.css`, and a typing surface below `text-field`. CI does not
run it.

### Stories and tests

Every component appears in a story. Stories are grouped by area in files beside the components
(`primitives/Display.stories.tsx`, `domain/Misc.stories.tsx`, …); app-level stories — wired
components rendered with their real hooks against a stubbed `window.fetch` — live under
`src/components/`. Prefer a design-system story where the component is presentational.

There are two Vitest projects. `unit` (`just test frontend`) is jsdom. `storybook`
(`just test stories`) runs every story in Playwright Chromium with axe-core, and
`a11y.test = "error"` fails it on any violation; `STORYBOOK_THEME=light pnpm run test:stories`
runs it in light. Neither `just check` nor CI runs the storybook project.

If axe flags contrast, fix the token or reach for a brighter tier before adding an exception. A
legitimate decorative exception is `parameters.a11y.test = "todo"` on the story, with the reason in
a comment.

---

## Adding a component

Start from upstream shadcn/ui where it has the component, then fit it to the contract. Write it in
the lowest tier that fits.

Set `data-*` attributes for state (`data-pressed`, `data-active`) so a story can show each state
without real interaction. Add the component to its area's stories file with every variant and
state; wrap a section in `pseudo-<state>-all` when `:hover`, `:focus-visible` or `:active` matter.
Then run `just test stories` yourself — nothing else will.

---

## Traps

- **Delete the palette `shadcn add` appends.** Every install re-appends the contract's reference
  palette to the end of `globals.css` as an unlayered `:root` / `.dark`, which beats `tokens.css`
  in the cascade and silently repaints the app. `just lint frontend` catches it; CI does not.
- **Never give a `tokens.css` value the contract's own name, and never rename `--sans-stack`,
  `--mono-stack` or `--elevation-*` to the contract's spelling.** The contract block maps
  `--radius-md: var(--radius-md)` — a self-reference, invalid at computed-value time, which
  invalidates every declaration of that name whatever the cascade layer says. Inter stops loading
  and every shadow disappears, with no error anywhere.
- **Register every new composed `text-*` utility with tailwind-merge in
  `design-system/utils/cn.ts`.** A role name fails tailwind-merge's t-shirt-size test, falls
  through to the `text-color` group, and `cn("text-mono", "text-muted-foreground")` silently
  returns the colour alone. `cn.test.ts` pins it.
- **Don't pass `size={n}` to a Lucide icon inside `IconButton`.** The size step sets the icon size
  and overrides it; pass a `size-*` class instead.
- **Keep every typing surface at `text-field` (16px).** Below it, iOS Safari zooms the viewport on
  focus and never zooms back. The guardrail checks only the files listed in `TYPING_SURFACES` in
  `check-tokens.mjs`, so add a new typing surface there.
- Always pair `border*` with a colour token; the default border colour is
  Tailwind's gray-200.
- **`--accent` is not the brand.** In the contract's vocabulary `--accent` is the hover / selected
  surface and `--primary` is the brand. Getting it backwards looks wrong rather than broken, which
  is much harder to spot.
- **Use `subtle-foreground` for anything informational.** `faint-foreground` is decorative only:
  it falls below WCAG AA in the light theme.
