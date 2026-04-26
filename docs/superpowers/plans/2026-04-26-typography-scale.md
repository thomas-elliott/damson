# Typography Scale Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the web design system from a 10-token font-size scale to a two-layer system: 3 Layer-1 size primitives (`text-body` 15, `text-ui` 14, `text-small` 12) plus 19 Layer-2 composed semantic tokens (3 heading roles × 4 weights = 12, plus 4 generic and 3 view-specific role tokens).

**Architecture:** Add new tokens in Tailwind config + `globals.css` `@layer components` classes, leaving old tokens in place so the build keeps working. Sweep call sites by directory, committing each directory as a reviewable chunk. Remove old tokens at the end. Body default changes 13→15 in Phase 1 so visual regressions surface during the sweep, not at the end.

**Tech Stack:** Tailwind CSS 3.4, Vite, TypeScript, Vitest (unit + storybook), Storybook 10, React 18.

**Spec:** `docs/superpowers/specs/2026-04-26-typography-scale-design.md`

---

## File Structure

**Configuration files (touched once in Phase 1):**
- `web/tailwind.config.ts` — `fontSize` block (Layer 1 only).
- `web/src/styles/globals.css` — body default + Layer 2 `@layer components` classes.
- `web/src/design-system/utils/cn.ts` — tailwind-merge `font-size` group.

**Visual reference (Phase 2):**
- `web/src/design-system/tokens/Tokens.stories.tsx` — rewritten typography section.

**Migration sweep (Phases 3–7):** ~210 call sites across:
- `web/src/design-system/primitives/` (≈50 sites; also `Glyph.tsx`, `Badge.tsx`, `Kbd.tsx`, `Tabs.tsx` for height/icon-size adjustments).
- `web/src/design-system/compositions/` (≈27 sites; also `NavRail.tsx` for the notification dot).
- `web/src/design-system/domain/` (≈39 sites).
- `web/src/components/` (≈39 sites).
- `web/src/pages/` (≈38 sites).

**Cleanup (Phase 8):** old tokens removed from `tailwind.config.ts` and `cn.ts`.

**Docs (Phase 9):**
- `docs/design-system.md` — typography table.
- `docs/ui-design.md` — inline scale list (line 36).

---

## Token reference (Layer 2 CSS to write in Task 3)

This block is referenced by Task 3 verbatim. Do not paraphrase or re-derive.

```css
/* Layer 2 — composed semantic tokens. Add to globals.css inside
 * @layer components { ... } below the @tailwind directives. */
@layer components {
  /* Headings — title (22), heading (20), subhead (18). Each has
   * 4 weight variants and a bare alias = -semibold. */
  .text-title-reg       { font-size: 22px; line-height: 1.25; font-weight: 400; }
  .text-title-medium    { font-size: 22px; line-height: 1.25; font-weight: 500; }
  .text-title-semibold  { font-size: 22px; line-height: 1.25; font-weight: 600; }
  .text-title-bold      { font-size: 22px; line-height: 1.25; font-weight: 700; }
  .text-title           { font-size: 22px; line-height: 1.25; font-weight: 600; }

  .text-heading-reg       { font-size: 20px; line-height: 1.3; font-weight: 400; }
  .text-heading-medium    { font-size: 20px; line-height: 1.3; font-weight: 500; }
  .text-heading-semibold  { font-size: 20px; line-height: 1.3; font-weight: 600; }
  .text-heading-bold      { font-size: 20px; line-height: 1.3; font-weight: 700; }
  .text-heading           { font-size: 20px; line-height: 1.3; font-weight: 600; }

  .text-subhead-reg       { font-size: 18px; line-height: 1.35; font-weight: 400; }
  .text-subhead-medium    { font-size: 18px; line-height: 1.35; font-weight: 500; }
  .text-subhead-semibold  { font-size: 18px; line-height: 1.35; font-weight: 600; }
  .text-subhead-bold      { font-size: 18px; line-height: 1.35; font-weight: 700; }
  .text-subhead           { font-size: 18px; line-height: 1.35; font-weight: 600; }

  /* Generic role tokens. */
  .text-label   { @apply text-small font-medium uppercase tracking-wide; }
  .text-mono    { @apply font-mono text-small; }
  .text-caption { @apply text-small text-text-3; }
  .text-num     { @apply text-ui font-mono; font-feature-settings: 'tnum'; }

  /* View-specific role tokens. */
  .text-table-row    { @apply text-ui; }
  .text-table-header { @apply text-ui font-medium; }
  .text-list-row     { @apply text-ui; }
}
```

---

## Migration mapping (used by Tasks 6–10)

This block is referenced by every directory-migration task. Each task applies this mapping to its directory.

| Old class | New class | Notes |
|---|---|---|
| `text-2xl` | `text-title` | bare alias = semibold (matches existing `font-semibold` siblings, which can be deleted from the same `className`) |
| `text-xl` | `text-heading` | same — drop adjacent `font-semibold`/`font-medium` if it matches the bare alias; otherwise pick `-medium`/`-bold` etc explicitly |
| `text-lg` (heading site) | `text-subhead` | drop adjacent `font-semibold`/`font-medium` per the rule above |
| `text-lg` (Glyph size=`lg`) | `text-[16px]` | icon size, not typography |
| `text-md` (heading site, e.g. `Dialog.tsx`, `LoginPage.tsx`, `BootstrapPage.tsx`, `EmptyState.tsx`, `DetailPanel.tsx`) | `text-subhead` | drop adjacent weight class per rule above |
| `text-md` (Glyph size=`md`) | `text-[14px]` | icon size |
| `text-md` (other non-heading) | `text-ui` | weight stays explicit |
| `text-base` | _delete_ | redundant with new body default (15px) |
| `text-sm-plus` (inside `<table>` body cell) | `text-table-row` |  |
| `text-sm-plus` (inside `AssetListRow`/`ProjectListRow`/`ActivityRow`/`QueueItem`/`VersionItem` row body) | `text-list-row` |  |
| `text-sm-plus` (anywhere else) | `text-ui` |  |
| `text-sm` | `text-ui` |  |
| `text-xs-plus` + `font-mono` on same element | `text-mono` (drop the `font-mono`) |  |
| `text-xs-plus` (other) | `text-small` |  |
| `text-xs` (next to muted helper text — typically paired with `text-text-3` under an input/field label) | `text-caption` (drop the `text-text-3`) |  |
| `text-xs` (other) | `text-small` |  |
| `text-2xs` + `uppercase` + `tracking-*` (label pattern) | `text-label` (drop `uppercase` and `tracking-*`; keep `text-text-3` etc separately if present) |  |
| `text-2xs` + `font-mono` | `text-mono` (drop `font-mono`) |  |
| `text-2xs` (other) | `text-small` |  |
| `<thead>` cell with `text-2xs uppercase tracking-wide font-medium text-text-3` (or close variant) | `text-table-header` (drop the bundled classes; keep `text-text-3` if needed) |  |

**Per-site judgment cases:** the `text-xs` → `text-small` vs `text-caption` decision and the `text-sm-plus` → `text-table-row`/`text-list-row`/`text-ui` decision are not pure regex. When in doubt:
- `text-caption` if the site is *under another element* and serves as an explanation/helper for it (e.g. helper text under an input).
- `text-table-row` only if the element is a `<td>` or inside `<tbody>`; not for "table-like" lists.
- `text-list-row` for the body cell of a list-row component (`AssetListRow`, `ProjectListRow`, etc.).
- Default to `text-small` / `text-ui` if unclear.

---

## Task 1: Update Tailwind config — Layer 1 size primitives

**Files:**
- Modify: `web/tailwind.config.ts:64-75`

- [ ] **Step 1: Add the 3 Layer 1 primitives alongside the existing tokens**

Open `web/tailwind.config.ts`. Replace the `fontSize` block:

```ts
fontSize: {
  // Layer 1 — size primitives (new system; final state).
  body: ["15px", "1.5"],
  ui: ["14px", "1.4"],
  small: ["12px", "1.4"],

  // OLD tokens — kept temporarily so existing call sites compile during
  // the migration. Removed in Task 12.
  "2xs": ["10.5px", "1.3"],
  xs: ["11px", "1.35"],
  "xs-plus": ["11.5px", "1.4"],
  sm: ["12px", "1.4"],
  "sm-plus": ["12.5px", "1.45"],
  base: ["13px", "1.45"],
  md: ["14px", "1.45"],
  lg: ["16px", "1.4"],
  xl: ["20px", "1.3"],
  "2xl": ["22px", "1.25"],
},
```

- [ ] **Step 2: Verify build succeeds**

```bash
cd web && npm run build
```

Expected: type-check + Vite build pass. New utilities `text-body`/`text-ui`/`text-small` should now exist in the compiled CSS but not yet be used.

- [ ] **Step 3: Commit**

```bash
git add web/tailwind.config.ts
git commit -m "typography: add Layer 1 size primitives (body/ui/small)"
```

---

## Task 2: Teach tailwind-merge about the new tokens

**Files:**
- Modify: `web/src/design-system/utils/cn.ts:8-29`

- [ ] **Step 1: Extend the font-size group**

Replace the `extendTailwindMerge` call:

```ts
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        {
          text: [
            // Layer 1 — size primitives (new system).
            "body",
            "ui",
            "small",

            // Layer 2 — composed semantic tokens (new system).
            "title",
            "title-reg",
            "title-medium",
            "title-semibold",
            "title-bold",
            "heading",
            "heading-reg",
            "heading-medium",
            "heading-semibold",
            "heading-bold",
            "subhead",
            "subhead-reg",
            "subhead-medium",
            "subhead-semibold",
            "subhead-bold",
            "label",
            "mono",
            "caption",
            "num",
            "table-row",
            "table-header",
            "list-row",

            // OLD tokens — kept until Task 12.
            "2xs",
            "xs",
            "xs-plus",
            "sm",
            "sm-plus",
            "base",
            "md",
            "lg",
            "xl",
            "2xl",
          ],
        },
      ],
    },
  },
});
```

- [ ] **Step 2: Verify build**

```bash
cd web && npm run build
```

Expected: pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/design-system/utils/cn.ts
git commit -m "typography: register Layer 1 + Layer 2 tokens with tailwind-merge"
```

---

## Task 3: Add Layer 2 composed token classes + body default

**Files:**
- Modify: `web/src/styles/globals.css:25-35` (body block) and append `@layer components` block.

- [ ] **Step 1: Bump body default to 15 / 1.5**

Edit the `body` rule (currently lines 25-35):

```css
body {
  margin: 0;
  font-family: var(--ds-font-sans);
  font-size: 15px;
  line-height: 1.5;
  color: var(--ds-text);
  background: var(--ds-bg);
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
  font-feature-settings: "cv11", "ss01";
}
```

- [ ] **Step 2: Append the Layer 2 component classes**

After the existing `@layer base { ... }` block, append the full Layer 2 block from the **Token reference** section above. Copy verbatim — do not paraphrase.

- [ ] **Step 3: Verify build + dev server boot**

```bash
cd web && npm run build
```

Expected: pass.

```bash
cd web && npm run dev &
sleep 3
curl -fsS http://localhost:5173/ > /dev/null && echo "OK"
kill %1
```

Expected: `OK`. (Pages will look slightly off because body is bigger and call sites still use old tokens — that's fine; it's the intended in-progress state.)

- [ ] **Step 4: Commit**

```bash
git add web/src/styles/globals.css
git commit -m "typography: add Layer 2 component classes; bump body to 15/1.5"
```

---

## Task 4: Rewrite Tokens.stories.tsx typography section

**Files:**
- Modify: `web/src/design-system/tokens/Tokens.stories.tsx:88-110` (the typography story; line numbers approximate — find the existing `<p className="text-2xl">22px — Page heading</p>` block).

- [ ] **Step 1: Replace the typography demo**

Find the existing typography story block (the `<div>` containing the lines `22px — Page heading`, `20px — Section heading`, etc., currently around lines 88-110). Replace its body with:

```tsx
<div className="flex min-w-[420px] flex-col gap-6">
  <section className="flex flex-col gap-2">
    <p className="text-label text-text-3">Layer 1 — size primitives</p>
    <p className="text-body">text-body — 15px (default running text)</p>
    <p className="text-ui">text-ui — 14px (UI controls, list rows)</p>
    <p className="text-small">text-small — 12px (fine print floor)</p>
  </section>

  <section className="flex flex-col gap-2">
    <p className="text-label text-text-3">Layer 2 — headings (paired with HTML h1/h2/h3)</p>
    <p className="text-title">text-title — 22px / semibold</p>
    <p className="text-title-bold">text-title-bold — 22px / bold</p>
    <p className="text-title-medium">text-title-medium — 22px / medium</p>
    <p className="text-title-reg">text-title-reg — 22px / regular</p>
    <p className="text-heading">text-heading — 20px / semibold</p>
    <p className="text-subhead">text-subhead — 18px / semibold</p>
  </section>

  <section className="flex flex-col gap-2">
    <p className="text-label text-text-3">Layer 2 — role tokens</p>
    <p className="text-label">text-label — section labels</p>
    <p className="text-mono">text-mono — sha256:ab3c1d…</p>
    <p className="text-caption">text-caption — helper text under inputs</p>
    <p className="text-num">text-num — 1,234,567 (tabular numerals)</p>
    <p className="text-table-row">text-table-row — body of a table cell</p>
    <p className="text-table-header">text-table-header — column header</p>
    <p className="text-list-row">text-list-row — body of a list-view row</p>
  </section>
</div>
```

- [ ] **Step 2: Verify story renders**

```bash
cd web && npm run test:stories -- --run
```

Expected: pass (the Tokens story is rendered headlessly; failure means a class name doesn't exist).

- [ ] **Step 3: Eyeball in Storybook**

```bash
cd web && npm run storybook &
```

Visit `http://localhost:6006` → Tokens → typography story. Verify all 22 tokens render at their expected sizes/weights. Stop the storybook process.

- [ ] **Step 4: Commit**

```bash
git add web/src/design-system/tokens/Tokens.stories.tsx
git commit -m "typography: rewrite Tokens story for two-layer system"
```

---

## Task 5: Migrate `web/src/design-system/primitives/` (incl. height bumps + Glyph)

This is the largest single migration task: ≈50 token call sites plus 4 primitives that need height/icon-size adjustments. Apply the **Migration mapping** from the top of this plan.

**Files modified (call-site sweep):**
- `Badge.tsx`, `Button.tsx`, `Chip.tsx`, `Dialog.tsx`, `DropdownMenu.tsx`, `Glyph.tsx`, `IconButton.tsx`, `Input.tsx`, `Kbd.tsx`, `Pill.tsx`, `Popover.tsx`, `ProgressBar.tsx`, `ScrollArea.tsx`, `SearchInput.tsx`, `SegmentedControl.tsx`, `Select.tsx`, `Separator.tsx`, `Slider.tsx`, `StatusDot.tsx`, `Tabs.tsx`, `Tag.tsx`, `Textarea.tsx`, `Tooltip.tsx`.

**Files modified (height bumps):**
- `Badge.tsx` size=`sm` variant: `h-[18px]` → `h-[20px]`.
- `Kbd.tsx`: `h-[18px]` → `h-[20px]`.
- `Tabs.tsx` count badge: `h-[18px]` → `h-[20px]`.

**Files modified (icon-size):**
- `Glyph.tsx`: `text-sm-plus` → `text-[14px]` (sm), `text-md` → `text-[14px]` (md), `text-lg` → `text-[16px]` (lg). Note `sm` was 12.5px and is rounded to 14px — this is a small but intentional bump.

- [ ] **Step 1: Inventory the directory**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/design-system/primitives | grep -v stories
```

Read every match. For each, classify against the migration mapping. Record per-site decisions before editing — particularly any `text-xs` that should become `text-caption` and any uppercase-label patterns.

- [ ] **Step 2: Apply the mapping file by file**

Edit each file in turn. For each match in Step 1:
1. Replace the old class with the new class.
2. If the new class is a Layer 2 composed token, drop any classes the composed token already provides (e.g. drop `font-mono` when introducing `text-mono`; drop `uppercase tracking-wide font-medium` when introducing `text-label`).
3. If the old class was `text-base`, delete it (no replacement).

- [ ] **Step 3: Glyph icon-size adjustment**

Edit `Glyph.tsx:9-13`:

```ts
size: {
  sm: "h-7 w-7 text-[14px]",
  md: "h-9 w-9 text-[14px]",
  lg: "h-12 w-12 text-[16px]",
},
```

- [ ] **Step 4: Height bumps**

In `Badge.tsx`, find the size variant `sm: "h-[18px] px-1.5 text-2xs"` and change to `sm: "h-[20px] px-1.5 text-small"` (after the small-token swap from Step 2).

In `Kbd.tsx`, find `"... h-[18px] ... text-2xs ..."` and change height to `h-[20px]`.

In `Tabs.tsx:47`, the count badge — find `"... h-[18px] ... font-mono text-2xs ..."`, change height to `h-[20px]` and class to `text-mono` (drop `font-mono`).

- [ ] **Step 5: Verify nothing in this directory still references old tokens**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/design-system/primitives | grep -v stories
```

Expected: no output (story files are migrated in their own pass during Task 9 cleanup).

- [ ] **Step 6: Run typecheck + tests**

```bash
cd web && npm run build && npm run test:all -- --run
```

Expected: pass.

- [ ] **Step 7: Eyeball primitive stories in Storybook**

```bash
cd web && npm run storybook &
```

Visit pages: `Primitives/Badge`, `Primitives/Kbd`, `Primitives/Tabs`, `Primitives/Dialog`, `Primitives/Chip`, `Primitives/Tag`. Confirm each renders without obvious squashing or broken alignment. Pay particular attention to Dialog — its title is now `text-subhead` (18px, was 14px), check the close-button alignment in the header. Stop storybook.

- [ ] **Step 8: Commit**

```bash
git add web/src/design-system/primitives
git commit -m "typography: migrate primitives to Layer 2 tokens; bump Badge/Kbd/Tabs heights"
```

---

## Task 6: Migrate `web/src/design-system/compositions/`

≈27 token call sites plus the NavRail notification dot.

**Files modified:**
- `AppShell.tsx`, `Breadcrumb.tsx`, `DetailPanel.tsx`, `DropOverlay.tsx`, `EmptyState.tsx`, `FilterSidebar.tsx`, `FullPageView.tsx`, `NavRail.tsx`, `ResizeHandle.tsx`, `Section.tsx`, `SelectionBar.tsx`, `Toolbar.tsx`.

**NavRail notification dot adjustment:**
- `NavRail.tsx:77` currently uses `h-[15px] min-w-[15px] ... text-[9px]` (an arbitrary 9px font-size that's not on the scale). Change to `h-[18px] min-w-[18px] ... text-mono` (drop the `font-mono` already on the line; the `text-mono` token brings it).

- [ ] **Step 1: Inventory**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/design-system/compositions | grep -v stories
```

Classify each match per the mapping. The `EmptyState.tsx` heading uses `text-md font-medium text-text-2` — that's a heading site → `text-subhead-medium text-text-2`. The `DetailPanel.tsx` heading uses `text-md font-semibold text-text` → bare alias `text-subhead text-text`.

- [ ] **Step 2: Apply mapping file by file**

Edit each file per the mapping. For composed-token replacements, drop redundant classes per the mapping rules.

- [ ] **Step 3: NavRail notification dot**

Edit `NavRail.tsx:77` — replace `h-[15px] min-w-[15px] ... px-1 font-mono text-[9px] font-semibold text-accent-fg` with `h-[18px] min-w-[18px] ... px-1 text-mono font-semibold text-accent-fg`.

- [ ] **Step 4: Verify clean**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/design-system/compositions | grep -v stories
```

Expected: no output.

- [ ] **Step 5: Build + test**

```bash
cd web && npm run build && npm run test:all -- --run
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add web/src/design-system/compositions
git commit -m "typography: migrate compositions to Layer 2 tokens"
```

---

## Task 7: Migrate `web/src/design-system/domain/`

≈39 token call sites. The list-row components live here — `text-sm-plus` inside `AssetListRow`, `ProjectListRow`, `ActivityRow`, `QueueItem`, `VersionItem` becomes `text-list-row`.

**Files modified:**
- `ActivityRow.tsx`, `AssetCard.tsx`, `AssetListRow.tsx`, `AssetThumb.tsx`, `CollectionCard.tsx`, `FieldRow.tsx`, `KeyValueList.tsx`, `LicenceBar.tsx`, `LicencePill.tsx`, `ProjectCard.tsx`, `ProjectGlyph.tsx`, `ProjectListRow.tsx`, `QueueItem.tsx`, `VersionItem.tsx`.

- [ ] **Step 1: Inventory**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/design-system/domain | grep -v stories | grep -v _fixtures
```

For each `text-sm-plus` match, decide whether the surrounding component is a list-row body cell (→ `text-list-row`) or generic UI text (→ `text-ui`). The 5 list-row files listed above are the ones that should pull `text-list-row` for their primary text.

- [ ] **Step 2: Apply mapping**

Edit file by file. Pay attention to:
- Hash/ID strings (likely `font-mono text-xs-plus` or similar) → `text-mono`.
- Sub-text under primary labels (likely `text-xs text-text-3`) → `text-caption`.
- Labels (likely `text-2xs uppercase tracking-wide`) → `text-label`.

- [ ] **Step 3: Verify clean**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/design-system/domain | grep -v stories | grep -v _fixtures
```

Expected: no output.

- [ ] **Step 4: Build + test**

```bash
cd web && npm run build && npm run test:all -- --run
```

Expected: pass.

- [ ] **Step 5: Eyeball the list-row stories**

```bash
cd web && npm run storybook &
```

Visit `Domain/Asset`, `Domain/Project`, `Domain/Misc`. Confirm rows/cards render cleanly. Stop storybook.

- [ ] **Step 6: Commit**

```bash
git add web/src/design-system/domain
git commit -m "typography: migrate domain components to Layer 2 tokens"
```

---

## Task 8: Migrate `web/src/components/`

≈39 token call sites. Subdirs `detail/` and `settings/` contain the most.

**Files modified:**
- `AssetGrid.tsx`, `AuditList.tsx`, `AuthGuard.tsx`, `DropZone.tsx`, `LibraryFilterSidebar.tsx`, `Shell.tsx`, plus everything under `detail/` and `settings/`.

- [ ] **Step 1: Inventory**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/components
```

`AssetDetailPanel.tsx:115` is the asset-detail header (`text-xl font-medium text-text`) → `text-heading-medium text-text`. Note the size bump: 20px stays 20px, but you should still eyeball the truncation behavior.

- [ ] **Step 2: Apply mapping**

Edit file by file. Watch for:
- `<thead>` rows (in audit/asset tables) — column-header pattern `text-2xs uppercase tracking-wide font-medium text-text-3` → `text-table-header` (drop the bundled classes; keep `text-text-3` if needed, though `text-table-header` doesn't dim by default).
- `<td>` content with `text-sm-plus` → `text-table-row`.

- [ ] **Step 3: Verify clean**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/components
```

Expected: no output.

- [ ] **Step 4: Build + test**

```bash
cd web && npm run build && npm run test:all -- --run
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/components
git commit -m "typography: migrate components to Layer 2 tokens"
```

---

## Task 9: Migrate `web/src/pages/` (and any remaining stories)

≈38 token call sites. Pages have the heading sites — `LoginPage`, `BootstrapPage`, `SettingsPage`, `ProjectDetailPage`.

**Files modified:**
- All `.tsx` files in `web/src/pages/` plus any `*.stories.tsx` files anywhere under `web/src` that still use old tokens.

- [ ] **Step 1: Inventory pages**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src/pages
```

Heading replacements: `<h1 className="text-xl font-semibold text-text">` → `<h1 className="text-heading text-text">`. `<h1 className="text-md font-semibold text-text">` (in `LoginPage`/`BootstrapPage`) → `<h1 className="text-subhead text-text">`. Note these `<h1>` elements with `text-subhead` are slightly off semantically (the spec pairs `text-subhead` with `<h3>`); leave the JSX `<h1>` element alone — it's an existing inconsistency, not something this migration introduces.

- [ ] **Step 2: Inventory remaining stories**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src --include="*.stories.tsx"
```

If any matches, apply the mapping.

- [ ] **Step 3: Apply mapping across pages and stories**

- [ ] **Step 4: Verify clean across all of `src/`**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src
```

Expected: no output. **This is the global cleanliness gate.** If any matches remain, fix them before moving on.

- [ ] **Step 5: Build + test (full)**

```bash
cd web && npm run build && npm run test:all -- --run
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add web/src
git commit -m "typography: migrate pages and remaining stories to Layer 2 tokens"
```

---

## Task 10: Eyeball pass — heading bumps and dialog header

The dialog title goes 14 → 18 (28% larger). The asset detail title goes 16 → 18.

- [ ] **Step 1: Run the dev server**

```bash
cd web && npm run dev &
sleep 3
```

Open `http://localhost:5173/`.

- [ ] **Step 2: Trigger any dialog (e.g. settings → "delete account" or any modal)**

Verify the dialog title is comfortably sized, the close button is vertically aligned, and there's no awkward whitespace. If the close button looks high or low, the fix lives in `web/src/design-system/primitives/Dialog.tsx` — adjust the header padding (currently uses default flex alignment around the title text; reduce by 2px if needed).

- [ ] **Step 3: Open an asset detail panel**

Navigate to a library, open an asset. Verify the asset detail title (now 18px) sits well in the header.

- [ ] **Step 4: Visual smoke of running text**

Browse pages with prose: settings page, login, audit page. Confirm 15px body reads cleanly and isn't crowding adjacent UI. Stop dev server (`kill %1`).

- [ ] **Step 5: Commit any tweaks**

If any padding/alignment tweaks were made:

```bash
git add web/src
git commit -m "typography: adjust spacing for new heading sizes"
```

If no tweaks: skip this step.

---

## Task 11: Remove old tokens — Tailwind config + tailwind-merge

**Files:**
- Modify: `web/tailwind.config.ts:64-75`
- Modify: `web/src/design-system/utils/cn.ts:8-29`

- [ ] **Step 1: Final scan for old tokens**

```bash
cd web && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" src
```

Expected: no output. If any matches: STOP, complete that migration before continuing this task.

- [ ] **Step 2: Trim tailwind.config.ts**

Replace the `fontSize` block with the final state — only Layer 1:

```ts
fontSize: {
  body: ["15px", "1.5"],
  ui: ["14px", "1.4"],
  small: ["12px", "1.4"],
},
```

- [ ] **Step 3: Trim cn.ts**

Remove the OLD entries from the `text:` array. Final state:

```ts
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        {
          text: [
            "body",
            "ui",
            "small",
            "title",
            "title-reg",
            "title-medium",
            "title-semibold",
            "title-bold",
            "heading",
            "heading-reg",
            "heading-medium",
            "heading-semibold",
            "heading-bold",
            "subhead",
            "subhead-reg",
            "subhead-medium",
            "subhead-semibold",
            "subhead-bold",
            "label",
            "mono",
            "caption",
            "num",
            "table-row",
            "table-header",
            "list-row",
          ],
        },
      ],
    },
  },
});
```

Update the comment block above to match (the current comment refers to "2xs, xs-plus, sm-plus, base, md" — replace with a comment describing the two-layer system).

- [ ] **Step 4: Build + test**

```bash
cd web && npm run build && npm run test:all -- --run
```

Expected: pass. Failure here means something still references an old token — find it via the Step 1 grep and migrate.

- [ ] **Step 5: Commit**

```bash
git add web/tailwind.config.ts web/src/design-system/utils/cn.ts
git commit -m "typography: remove legacy size tokens"
```

---

## Task 12: Update docs

**Files:**
- Modify: `docs/design-system.md:99-119` (Typography section + Tailwind table)
- Modify: `docs/ui-design.md:36`

- [ ] **Step 1: Replace the typography section in `docs/design-system.md`**

Find the `### Typography` section. Replace the body and the Tailwind table with:

```markdown
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
```

- [ ] **Step 2: Update `docs/ui-design.md:36`**

Replace the line:

```
- Body is 13px / 1.45 line-height. The full scale is 10.5/11/11.5/12/12.5/13/14/16/20/22 — dense, information-first, with weight doing most of the hierarchy work.
```

with:

```
- Body is 15px / 1.5 line-height. The font-size scale is 12/14/15/18/20/22 — three size primitives (`text-small`/`text-ui`/`text-body`) plus role-named semantic tokens (`text-title`/`text-heading`/`text-subhead`/`text-label`/`text-mono`/`text-caption`/`text-num`/`text-table-row`/`text-table-header`/`text-list-row`).
```

- [ ] **Step 3: Commit**

```bash
git add docs/design-system.md docs/ui-design.md
git commit -m "docs: update typography section for two-layer system"
```

---

## Task 13: Final smoke test

- [ ] **Step 1: Build everything from scratch**

```bash
cd web && rm -rf node_modules/.vite && npm run build
```

Expected: pass.

- [ ] **Step 2: Run full test suite**

```bash
cd web && npm run test:all -- --run
```

Expected: pass.

- [ ] **Step 3: Build storybook**

```bash
cd web && npm run build-storybook
```

Expected: pass.

- [ ] **Step 4: Dev-server smoke walkthrough**

```bash
cd web && npm run dev &
sleep 3
```

Open `http://localhost:5173/` and walk through:
1. Login page (h1 = `text-subhead`)
2. Library list (list rows = `text-list-row`)
3. Asset detail panel open (title = `text-subhead`)
4. Settings page (h1 = `text-heading`)
5. Audit page (table = `text-table-header` + `text-table-row`)
6. Any dialog (title = `text-subhead`)

For each surface, confirm:
- Text isn't crowding adjacent UI.
- No obviously wrong font weights.
- No leftover 13px-looking running text (suggests an old token snuck through).

Stop dev server (`kill %1`).

- [ ] **Step 5: Final cleanliness scan**

```bash
cd /Users/thomaselliott/Projects/damson && grep -rn "text-2xl\|text-xl\|text-lg\|text-md\|text-base\|text-sm-plus\|text-sm\|text-xs-plus\|text-xs\|text-2xs" web/src docs
```

Expected: no output. If anything appears, migrate it; the migration isn't done.

- [ ] **Step 6: Done**

No commit needed — verification only.
