# Roadmap

> Maybes and shaped work, personal-scale. Items graduate: Ideas → Shaped → Next.
> An item bigger than a screen is ready to be a spec. Shipped items are pruned — what shipped
> lives in [`specs/history/`](specs/history/README.md).

**Rejected work is deliberately not here.** Decisions to *not* build something, with the reason,
stay in [`progress.md`](progress.md)'s "Known gaps" table. This file holds only what you might
actually pick up.

---

## Next  (ordered — the queue for sitting-down time or an agent)

### Turn the Storybook a11y gate back on

Intent: apply the one-line fix that makes the `storybook` Vitest project emit Tailwind, then clear
what it finds.

Why now: the gate has been green while checking unstyled components since it was added, so
`a11y.test = "error"` currently proves nothing about colour. The config half is a known one-liner,
already proven in `shared-design` (`4135476`); the work is the palette decisions behind it.
`web/vitest.config.ts` replaces `vite.config.ts` rather than extending it, so the `storybook`
project gets no `@tailwindcss/vite`; the fix is `tailwindcss()` in that project's `plugins`.

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

### 2. Contribute damson's icon-slot field API to `@shared/input`

A leading-icon / trailing-slot input is primitive-shaped and not damson-specific, so by the estate's
own rule it belongs in the registry. The path `IconButton` took. Deferred out of the 0.1.7 uptake
because it would have blocked that slice on a `shared-design` change; taking it would let damson
retire three owned primitives (`Input`, `SearchInput`, `Textarea`).

Blocked on nothing but sequencing. Spec-ready: nearly — the API is already designed, because damson
is running it.

### 3. Tell `shared-design` its `@theme inline` self-mapping is a cycle

The 0.1.7 uptake found that the CLI generates `--radius-md: var(--radius-md)` — the same name on
both sides — which is invalid at computed-value time and destroys any other declaration of that
name. Every consumer that deletes the appended reference `:root`, which `consuming.md` instructs
them to do, silently loses the whole non-colour scale. damson has a local workaround
(`docs/design-system.md`, **Tailwind mapping**); the fix belongs in `gen-tokens-item.mjs`.

Not damson work, but damson found it and nobody else has hit it yet. Spec-ready: n/a — it is an
upstream issue, not a slice.

*Nothing else queued. Promote something from `Shaped` when these land.*

---

## In flight  (spec written — pointer only, the spec owns the detail)

*Nothing in flight.*

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

### e2e coverage

1. **Visual regression** — still unowned. Decide whether it belongs here or
   in the Storybook tier.
2. **Coverage for post-MVP surfaces** — previews / processing queue, the
   notification centre's upload ownership across navigation, storefront
   enrichment, and bulk edit have no e2e flow yet. The upload-survives-
   navigation case is the highest-value of these: it's exactly the
   multi-system, hard-to-reproduce-by-hand class this suite exists for.
3. **Runtime budget** — keep the full suite under 3 minutes in CI.

### `cmd/worker` — the multi-container split

A worker-only binary: same River client, no HTTP. `internal/queue` was deliberately structured so
this is additive rather than a rewrite.

**Blocked on a decision, not on code:** which processing actually moves off-box. That question is
the spec.

Source: [`specs/history/2026-07-19-river-queue-migration.md`](specs/history/2026-07-19-river-queue-migration.md),
deferred there explicitly.

### `docs/index.md` — the per-file router

The load-bearing file: a table naming every doc, what it is, and when to load it. `AGENTS.md` currently routes to folders, which means
either reading everything in `docs/` or guessing.

Cheap — one file. Pairs naturally with the spec-history split, which is the other half of making
built-vs-not-built answerable by location rather than by reading.

---

## Ideas  (unshaped maybes — one to three lines each)

- **Adopt the package's `Chip`, `Tag` and `SegmentedControl`** — `@telliott/shared-ui` exports all
  three, Damson hand-writes its own, and nothing records why. Move onto the package, or find the
  divergence and record it beside Input/Textarea in `design-system.md`. Unscoped.
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
- **`docs/probes/` for storefront metadata** — damson's storefront work qualifies: keep the raw evidence of how an external service actually behaved separate
  from the contract derived from it, so a wrong contract is debuggable. Unscoped.
