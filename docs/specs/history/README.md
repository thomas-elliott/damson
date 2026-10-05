# Shipped specs

Every spec in this folder has been implemented and merged. They are **never rewritten** — each
records what was decided *at the time*, not what is true now. For the live shape of the system read
[`../../architecture.md`](../../architecture.md); read a spec here only for the *why*.

The top level of [`../`](../) is what is **in flight**. Nothing being there means nothing is in
flight — see [`../../roadmap.md`](../../roadmap.md) for what is queued.

| Date | Spec | What it did | Merged |
| --- | --- | --- | --- |
| 2026-06-18 | [file-previews](2026-06-18-file-previews.md) | Per-file preview derivatives for the scoped file browser. Format coverage beyond pure-Go raster is the staged remainder. | PR #24 `3fca0ae` |
| 2026-06-18 | [preview-processing-queue](2026-06-18-preview-processing-queue.md) | The `preview_status` state model and requeue path. Still live; its goroutine-pool transport was later replaced by River. | PR #24 `3fca0ae` |
| 2026-07-04 | [notification-centre](2026-07-04-notification-centre.md) | Lifted upload state out of `DropZone` into persistent notifications, so uploads survive navigation. | PR #25 `ce150ca` |
| 2026-07-05 | [large-uploads-storage](2026-07-05-large-uploads-storage.md) | The S3/Garage storage backend. Superseded on the transport side by the 2026-08-01 upload-reliability slice. | PR #26 `c1114ef` |
| 2026-07-05 | [orphan-storage-cleanup](2026-07-05-orphan-storage-cleanup.md) | The two-step sysadmin-driven orphan sweep — scan, re-verify, delete, audit. | PR #26 `c1114ef` |
| 2026-07-13 | [storefront-metadata-enrichment](2026-07-13-storefront-metadata-enrichment.md) | First storefront metadata pass. Extended and partly corrected by the 2026-07-31 slice. | PRs #27, #28 |
| 2026-07-19 | [river-queue-migration](2026-07-19-river-queue-migration.md) | Replaced the hand-rolled queues with River on Postgres, keeping the domain state model on top. | PR #32 `c7a618e` |
| 2026-07-24 | [upload-system-refinements](2026-07-24-upload-system-refinements.md) | Fixed 502s on multi-file upload and three related upload-path problems. | PR #33 `dca99b2` |
| 2026-07-24 | [versioning-and-storefront-metadata-refinements](2026-07-24-versioning-and-storefront-metadata-refinements.md) | Versioning rework — a version's identity is its `store_version` — plus metadata fixes. Both of its deferred follow-ups have since shipped. | PR #33 `dca99b2` |
| 2026-07-25 | [library-topbar-file-manager-browser](2026-07-25-library-topbar-file-manager-browser.md) | Library top bar, rail account menu, file-manager browser. Frontend only. | PR #34 `d2453f8` |
| 2026-07-31 | [asset-metadata-fields](2026-07-31-asset-metadata-fields.md) | Corrected the Unity field mapping and widened the promoted field set. | PR #36 `ab8db02` |
| 2026-08-01 | [dmsn-cli](2026-08-01-dmsn-cli.md) | The `dmsn` CLI and personal access tokens. Split out of the Unity slice, taking its stages 1 and 4 wholesale. | PR #37 `05faeae` |
| 2026-08-01 | [unity-local-cache-import](2026-08-01-unity-local-cache-import.md) | Importing from the local Unity Asset Store cache, with `(source_kind, source_relative_path)` preflight precision. | PR #38 `531ea5b` |
| 2026-08-01 | [upload-reliability-observability](2026-08-01-upload-reliability-observability.md) | Removed the double spool, added the chunked resumable path, and made upload failures visible in logs. | PR #39 `510b9ff` |
| 2026-08-15 | [shared-design-system](2026-08-15-shared-design-system.md) | Moved the frontend off 57 `--ds-*` tokens onto the shared `@shared` registry and shadcn's vocabulary, in one sweep behind a token gate. Frontend only — no Go, no SQL. | PR #40 `f91d654` |
| 2026-08-15 | [shared-design-0-1-7](2026-08-15-shared-design-0-1-7.md) | Took the contract past colour — literal radii, eight role sizes, motion, per-theme elevation, the 16px field floor. Found that the CLI's `@theme inline` self-mapping is a dependency cycle that had been silently killing tokens since before the slice. | PR #41 |
| 2026-10-03 | [reference-assets](2026-10-03-reference-assets.md) | Record an asset by its location — a path, a store link, or both — with nothing imported: search on path words, version-less project pins, an Add by location dialog and `dmsn asset record`. | PR #1 |
| 2026-10-05 | [nfs-agent-scan](2026-10-05-nfs-agent-scan.md) | Catalogue a folder of packs by agent scan: record with publisher and folder tags, list recorded paths to find new and gone packs, one paced store search to a matches file, record an approved plan file in one call, and a `damson-scan` skill from `dmsn setup`. | PR #2 |
| 2026-10-05 | [purchase-catalogue](2026-10-05-purchase-catalogue.md) | Catalogue Unity purchases: `dmsn purchases read` takes the list from the Unity Editor's HTTP cache, `plan` matches by listing ID then name, `record` adds or links them as Needs review at a paced rate, plus a Purchased, not downloaded filter and a `damson-purchases` skill. | PR pending |

## What they deferred

Harvested into [`../../roadmap.md`](../../roadmap.md) on 2026-08-15. Each spec's own deferred
section stays intact as the historical record — this is a copy, not a move.

- **`cmd/worker`** — the worker-only binary proving the multi-container split, from the River
  migration. Blocked on deciding which processing actually moves off-box.
- **`ephemeral-storage` request/limit on the pod** — gitops-side, from the upload-reliability
  slice. Nothing large spools any more, but a future accidental spool should evict with a clear
  reason rather than being OOM-killed silently.
- **Preview format coverage** — PSD/EXR/TGA, SVG rasterisation, PDF posters, staged behind the
  shipped pure-Go raster path.
- **Drop `library_settings.theme_default`** — from the design-system slice. The theme became a
  browser concern; the column, its API field and its GORM tag survive unread, because removing them
  is itself a database change.
- **`IconButton`'s `variant="destructive"` is filled, not soft** — the soft-red override lives in
  `Button.tsx`'s wrapper and `IconButton` is a pure re-export, so the two disagree. No call site uses
  it, so it is latent rather than live.
- **Dark-mode a11y coverage for damson's own palette** — the design-system slice covered the story
  set; a sweep across composed pages was not in scope.
- **Contribute damson's icon-slot field API to `@shared/input`** — from the 0.1.7 slice. A
  leading-icon / trailing-slot input is primitive-shaped and not damson-specific, so by the estate's
  own rule it belongs in the registry. Would let damson retire three owned primitives.
- **Tell `shared-design` its `@theme inline` self-mapping is a cycle** — from the 0.1.7 slice, and
  the one with reach beyond damson. Every consumer that deletes the appended reference `:root`, which
  `consuming.md` instructs them to do, silently loses the entire non-colour scale.
- **Extend the 16px field floor to `select`, `command` and combobox search fields** — from the 0.1.7
  slice. Upstream names these as carrying the same 14px class and not yet covered by the contract.
- **A genuine slow motion step** — from the 0.1.7 slice. `--duration-normal` (180ms) and
  `--duration-slow` (260ms) are installed and unused; damson's motion collapsed onto `fast`. Whether
  it wants a slower step is a design question nobody has asked.

Three of the design-system slice's deferrals were **answered upstream by `shared-design` 0.1.7** and
taken up by the slice below it: the radius-scale-shape question, the nine `rounded-[3px]` arbitraries
(now `rounded-xs` at 4px), and the dissolved third sighting of `--ds-surface-2`. None of them reached
the roadmap.

The versioning slice's two follow-ups — the *add as a version of…* upload prompt and the merge
picker — **both shipped** (`NotificationProvider.tsx`, `MergeDialog.tsx`) and are not carried
forward.
