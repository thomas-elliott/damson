# Roadmap

> Maybes and shaped work, personal-scale. Items graduate: Ideas → Shaped → Next.
> An item bigger than a screen is ready to be a spec. Shipped items are pruned — what shipped
> lives in [`progress.md`](progress.md) and [`specs/history/`](specs/history/README.md).

**Rejected work is deliberately not here.** Decisions to *not* build something, with the reason,
stay in [`progress.md`](progress.md)'s "Known gaps" table. This file holds only what you might
actually pick up.

Created 2026-08-15 from the specs, at which point all 14 were implemented and nothing was in flight.

---

## Next  (ordered — the queue for sitting-down time or an agent)

### Turn the Storybook a11y gate back on

Intent: apply the one-line fix that makes the `storybook` Vitest project emit Tailwind, then clear
what it finds.

Why now: the gate has been green while checking unstyled components since it was added, so
`a11y.test = "error"` currently proves nothing about colour. The config half is a known one-liner,
already proven in `shared-design` (`4135476`); the work is the palette decisions behind it.

Decisions so far: none — that is the point. Measured 2026-08-15, the fix yields 9 failures over 4
story files and three questions. **(a)** `--subtle-foreground` is used for 10–12px labels on
`--card` at 3.96–4.34:1, and the tier is documented as deliberately sub-AA decorative — so either
those labels move to `--muted-foreground`, or the tier is exempted where it is genuinely decorative,
per `shared-design`'s `TextTiers` precedent. **(b)** a `destructive`-on-`destructive-soft` pairing
at 4.42:1. **(c)** a Markdown link at 1.13:1 against surrounding body text, which is a link-affordance
question, not just a colour one. Note `shared-design` darkened its own `--destructive` for (b)-shaped
reasons; damson overrides that token, so it does not inherit the fix.

**(a) now has an upstream answer.** `shared-design` 0.1.7 raised its own `--subtle-foreground` to
pass AA (`8c7cee3`) rather than exempting the tier. damson overrides the value so it does not inherit
it, but the estate has taken the decision once — which makes this a question of following or
diverging, not of deciding from scratch. Pairs with the 0.1.7 uptake below.

Spec-ready: no — (c) still needs a decision, and (a) needs confirming rather than taking.

*Nothing else queued — the 0.1.7 uptake moved to `In flight` on 2026-08-15. Promote something from
`Shaped` when it lands.*

---

## In flight  (spec written — pointer only, the spec owns the detail)

- **Take up `shared-design` 0.1.7** → [`specs/2026-08-15-shared-design-0-1-7.md`](specs/2026-08-15-shared-design-0-1-7.md).
  Signed off 2026-08-15. Seven stages; frontend only. Absorbs the four scales 0.1.7 added past
  colour — literal radius, eight-role type, motion, elevation — plus the 16px field floor and
  `@shared/type-scale`. **Supersedes Q4 of the slice below**, whose radius anchor answered a
  multiplicative scale that no longer exists.

The shared design system slice itself shipped 2026-08-15 (PR #40, `f91d654`) — see
[`specs/history/2026-08-15-shared-design-system.md`](specs/history/2026-08-15-shared-design-system.md)
and [`progress.md`](progress.md). What it deferred is carried in
[`specs/history/README.md`](specs/history/README.md).

<details>
<summary>What the roadmap banked before that spec (2026-08-15) — kept for the record</summary>

**Intent:** install `@shared` registry items, replace the `--ds-*` vocabulary with shadcn's token
names, and prove that copy-in ownership survives re-running `shadcn add`.

**Why now:** damson is consumer **2 of 2** in the estate proof and the brownfield half of it —
dialogue-db tested a clean install (**shipped 2026-08-15**, PR #39 `5e6b4ba`), damson tests
installing into an existing vocabulary of 57 tokens and 27 primitives. damson also carries the one
exit criterion nobody else can: a deliberate soft-red `destructive` kept as a **wrapper** around
the registry `Button` rather than an edit to the installed file. That is the crux of copy-in
ownership, and the only place it gets tested.

**Brief:** [`shared-design-adoption.md`](shared-design-adoption.md) — estate context, the
measurements, and the seven hazards. Read it before running `/spec`.

**Scale:** 832 token-utility usages across 116 `.ts`/`.tsx` files; only 69 raw `var(--ds-*)`. This
is a **class-name rename, not a `var()` rename** — the opposite shape from dialogue-db, which had
~1,050 `var()` calls and 46 utilities.

**Settled elsewhere** (`personal-practices/decisions/2026-08-15-design-system-vocabulary.md`, do
not reopen): shadcn's vocabulary is primary, not aliased · drop the `--ds-` prefix · `destructive`,
not `danger` · default `Button` variant is `primary` · upstream direct for anything shadcn has,
`@shared` only for what it lacks · never overwrite `cn` · the old `--accent` is the brand and
becomes `--primary`.

**Decisions taken 2026-08-15** — banked so `/spec` confirms rather than decides:

| | Decision | Reasoning |
| --- | --- | --- |
| **Theme selector** | **Invert to match the contract.** Light values at `:root`, dark on `.dark`; the toggle sets/clears `.dark`. **The initial theme follows `prefers-color-scheme`** rather than being hard-defaulted to dark. | The registry `tokens` item carries only `light`/`dark` `cssVars`, which the CLI writes as `:root` and `.dark` — so the "neither app has to change its toggle" reassurance never actually reaches a consumer. dialogue-db shipped exactly this (`tokens.css:5`/`:88`, `app/theme.ts:11`), so the estate converges on one mechanism. Dark-by-default was a *palette* choice and survives the inversion. Costs `.storybook/preview.tsx:12` and `design-system/tokens/index.ts:7`. |
| **`cn` location** | **Point `aliases.utils` at `@/design-system/utils`.** Do not create `src/lib/utils.ts`. | Nothing moves, installed files pick up the extended `cn`, and the reason it is extended (24 registered `text-*` class names for the font-size system) stays next to it. A re-export invites someone to later "simplify" it back to the stock two-liner, which deduplicates `text-mono` against a colour class and breaks damson silently. |
| **Installed file location** | **Keep installed and owned primitives apart.** `components/ui/` is the CLI's; `design-system/primitives/` stays damson's. Say so in `design-system.md`. | "Which files does re-running `add` overwrite?" is the question copy-in ownership makes people ask, and a directory answers it for free. |
| **`--ds-surface-2`** | **Ask `shared-design` to name the second upward step.** Adopt `--muted` as the interim if the answer is slow. Treat as a stage-0 dependency. | The contract's rule is that a name enters on its **second** sighting; this one has three (damson, taskmanager, mangapack) and `shared-design/src/styles/tokens.css:92` records the gap deliberately. That same file already took a **one**-sighting exception for `-sunken`/`-inset`, which makes a three-sighting ask very hard to refuse. |
| **`--card` vs `--popover`** | **Steal both dialogue-db's rule and its equal-values trick.** Floating things — `Popover`, `DropdownMenu`, `Dialog`, `Tooltip`, context menus, selection bars — take `--popover`; everything else takes `--card`. Set the two equal in *both* themes. | The two are the same value in light and different in dark in the reference palette, so a wrong classification **passes every check and only shows when someone toggles the theme**. Equal values mean the classification cannot render wrongly, leaving them free to diverge later. Verified against dialogue-db's merged code, not just its spec (`web/src/styles/tokens.css:8-11`, `:92-93`). |
| **`Button` default flip** | **Grep first.** A mechanical codemod adds `variant="secondary"` to every prop-less `<Button>` *before* the swap, so the flip is a visual no-op; new code gets the new default. | Every `<Button>` with no `variant` prop changes appearance, and damson uses that call shape widely. Reviewable as one mechanical commit rather than an eyeball audit in Storybook. Sizes also move `h-6/h-7` → `h-7/h-8/h-9`. |
| **`IconButton`** | **Publish it as `@shared/icon-button`**, with damson's as the reference implementation. | It was not published for dialogue-db, which turned out not to use it, so damson's copy is the only remaining evidence for publishing it at all — this slice effectively decides it. Contributing it back is also the estate rule: primitive-shaped and not damson-specific goes to the registry. |
| **`docs/mockup/`** | **Demote it.** Moves to `archive/` under damson's standing rule — keeps the old names, consulted only for the *why*. Storybook becomes the visual reference. `SpriteThumb`'s byte-identical provenance becomes a note on the component. | Code becomes the source of truth. Same fork dialogue-db hit, same call. |
| **Guardrail and staging** | **Add the token gate, sweep in one commit.** Copy `personal-practices/resources/check-tokens.mjs` into `web/scripts/`, wire into `task check`. The 832 utility renames land together; everything else stages finely around it. | A half-swapped vocabulary is the thing to avoid, and the gate makes "no half-swapped vocabulary" enforced rather than hoped for. It fails on any surviving old name and on raw hex outside `tokens.css`, so it can only go green at the end — which is itself the argument for the single commit. |
| **Which primitives retire** | `Button`, `Dialog`, `EmptyState` — **and grow the rest during adoption.** Enumerated properly once `shadcn init`'s diff has been read. | damson has 27 primitives, ten Radix-backed; the registry has four items. The estate rule is that the inventory grows when a consumer needs it. |

**Stage 0, decided:** switch `globals.css:16` from `@theme` to `@theme inline` **before** adoption,
then verify the theme toggle both at root and in a scoped wrapper. This works today only because
the switch is at `html`, which *is* `:root`; plain `@theme` bakes the value into the utility at
build time, so the moment a theme is scoped below root — Storybook's `addon-themes`, any scoped
preview — it stops following. A migration step, not a live bug.

**Four of these were corrected by the spec**, which checked them against the working trees:
`B4` (the `--ds-surface-2` ask) was dropped — its twelve sites decompose into three roles that
already have names; the guardrail source is dialogue-db's React script, not the Svelte one in
`personal-practices`; the CLI generates the `@theme inline` block itself, so stage 0 is a merge
rather than a hand edit; and the contract's radius scale turned out to be multiplicative, so no
anchor reproduces damson's `4 / 6 / 10`.

</details>

---

## Shaped  (scoped, key decisions taken, not yet queued)

### Preview formats, tier 2

PSD / EXR / TGA decoding, SVG rasterisation, and PDF posters — staged behind the shipped pure-Go
raster path. The generator, the content-addressed `thumbs/` keys and the frontend dispatcher all
exist, so each format is an additive decoder rather than new machinery.

Source: [`specs/history/2026-06-18-file-previews.md`](specs/history/2026-06-18-file-previews.md).
Deliberately rejected there and still rejected: `.blend`, Unity scene/prefab visual reconstruction,
AIFF transcoding, rtf/doc conversion — each needs a heavyweight external binary. Re-open only if a
library turns out to be dominated by one of them.

### e2e tiers 2 and 3

Tier 1 is implemented; tiers 2 and 3 are already enumerated in
[`e2e-flows.md`](e2e-flows.md), so the scoping work is done and this is execution.

Chunked uploads stay excluded — triggering the path needs a 256 MiB file and generating one per run
blows the suite's runtime budget. Covered by unit and handler tests instead.

### `cmd/worker` — the multi-container split

A worker-only binary: same River client, no HTTP. `internal/queue` was deliberately structured so
this is additive rather than a rewrite.

**Blocked on a decision, not on code:** which processing actually moves off-box. That question is
the spec.

Source: [`specs/history/2026-07-19-river-queue-migration.md`](specs/history/2026-07-19-river-queue-migration.md),
deferred there explicitly.

### `docs/index.md` — the per-file router

`personal-practices/standards/project-docs.md` calls this "the load-bearing file": a table naming
every doc, what it is, and when to load it. `AGENTS.md` currently routes to folders, which means
either reading everything in `docs/` or guessing.

Cheap — one file. Pairs naturally with the spec-history split, which is the other half of making
built-vs-not-built answerable by location rather than by reading.

---

## Ideas  (unshaped maybes — one to three lines each)

- **Local auth hardening** — in-memory rate limiting only (no shared store across replicas), no MFA,
  a minimum-length password floor and nothing else. Fine for dev and small installs, which is what
  it is documented as; a real multi-replica deployment changes that. Unscoped.
- **Server-level audit retention** — `library_settings.audit_retention_days` purges per-library
  rows, but `library_id IS NULL` events are never purged because `server_settings` has no retention
  column. Unscoped.
- **Grow the CLI surface** — projects, pins, licences, previews, merge and subset are reachable via
  the API but not wrapped by `dmsn`. The CLI spec deliberately left them until the render layer
  existed; it now does. Unscoped.
- **An MCP server for damson** — explicitly out of scope for the CLI slice. The personal-access-token
  and typed-client layers it would need already exist, so the marginal cost is lower than it looks.
  Unscoped.
- **Subset from a non-current version** — the scoped file browser only loads an asset's *current*
  version, so a subset can only ever be cut from that. Unscoped.
- **Orphan sweep: incomplete multiparts** — `FindOrphans` lists objects only, so a multipart upload
  orphaned outside damson's knowledge is invisible to the janitor. Unscoped.
- **Preflight without a source hint** — an entry carrying no `(source_kind, source_relative_path)`
  falls back to `name_match` on the normalised title alone, which cannot tell a duplicate from a
  genuine new version. The Unity slice closed this for cache imports only. Unscoped.
- **Remove the dead `LIKE` search branch** — `applyAssetFilters` dispatches on
  `db.Dialector.Name()`; the non-Postgres branch has been unreachable since SQLite was dropped.
  Marked "remove on next touch". Unscoped, and small.
- **`ephemeral-storage` request/limit on the pod** — gitops-side, not this repo. Nothing large
  spools after the upload rework, so this is purely so a *future* accidental spool is evicted with a
  clear reason rather than OOM-killed silently. From the upload-reliability slice. Unscoped.
- **Zitadel OIDC as the default IdP** — estate decision, 2026-08-15: Zitadel is the default, authz
  stays local to each app. damson's OIDC mode already fits the shape, so this is deployment wiring
  rather than code. Unscoped.
- **Dev services in-cluster** — estate decision: dev and test Postgres + S3 run in k8s rather than
  local containers, and CI deliberately does not share them. damson currently assumes local Postgres
  on 5432 (`DAMSON_TEST_POSTGRES_DSN`). Unscoped.
- **`docs/probes/` for storefront metadata** — `standards/project-docs.md` names damson's storefront
  work as qualifying: keep the raw evidence of how an external service actually behaved separate
  from the contract derived from it, so a wrong contract is debuggable. Unscoped.

---

## Direction notes  (standing intent that outlives any one item)

- **React + shadcn/ui is the estate's frontend standard.** shadcn *is* the primitive layer; the
  `@shared` registry carries only what upstream lacks, and its inventory grows when a consumer needs
  it. Anything primitive-shaped and not damson-specific goes to the registry rather than staying
  here. Components are copy-in source you own — the CLI is the provenance, not a version range.
- **The library is the tenancy boundary, never the user.** Every library-scoped entity carries
  `library_id` and queries filter on it at the top level, never inferring via parent FKs.
  `created_by_user_id` and `pinned_by_user_id` are attribution, not ownership.
- **Domain columns are the truth; SSE is notification only.** `asset_imports.stage` and
  `asset_files.preview_status` are the durable user-visible state. A dropped event must always be
  recoverable by refetching — that is what keeps River layered *under* the state model rather than
  replacing it.
- **Blobs are never deleted on the write path.** Reclaiming storage stays a deliberate,
  sysadmin-driven, re-verified two-step. Deleting an asset leaves its objects behind on purpose.
- **Specs are dated records, never rewritten.** A changed decision is a new dated spec; the old one
  stays as history. Shipped specs move to `specs/history/` so built-vs-not-built is a question of
  location rather than of reading.
- **Task is the only entry point.** No direct `go`, `npm` or `docker compose` for build, test or
  lint. The canonical verbs are the contract.
- **Errors never leak internals.** `500` is always the generic string; the diagnostic detail goes to
  `slog` at the failure site instead.
