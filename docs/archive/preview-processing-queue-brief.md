# Preview Processing Queue & Requeue — Context Brief

**Status:** pre-spec context for a future `spec` session. *Not* a spec or a design.
**Purpose:** capture the problem, the current architecture, the settled requirements, and
the open design questions so a fresh (compacted) spec session can start with grounding.

Origin: while validating the file-previews feature (`docs/specs/history/2026-06-18-file-previews.md`),
three issues surfaced. One was a contained bug (fixed); the other two are this feature.

---

## The three observations

1. **Library grid size was wrong — FIXED** (commit `851484c`). The grid summed sizes with
   `SUM(DISTINCT size_bytes)` (dedup-by-value), so packages with many identically-sized
   files reported a fraction of the truth (39.8 MB vs the real 522.6 MB). Replaced with
   correlated subqueries in `internal/handlers/api/assets.go`. **Not part of this feature** —
   listed only for completeness.
2. **No way to requeue preview generation** (asset- or file-level). *(This feature.)*
3. **No visibility into preview processing** — "we have the import queue for review, we
   should be able to see the processing queue." *(This feature.)*

\#2 and \#3 share one root cause and should be specced together.

---

## Why they're missing: previewing is an invisible inline sub-stage of import

Today, backend preview derivatives are generated **inside the import job**, not as their own
tracked unit of work:

- `internal/services/imports/pipeline.go` → `Run()` calls `generatePreviews(ctx, deps, jc)`
  synchronously, as a `StagePreviewing` step between `thumbnailing` and `done`.
- `generatePreviews` iterates the version's `asset_files`, calls `preview.Generate()` per
  supported file, and writes `preview_object_key` / `preview_content_type` on success.
- **Failures are silent** — every error path is a `slog.Warn` and the file is skipped.
- **There is no preview status column.** `asset_files` has only `preview_object_key`
  (set on success). So "pending", "processing", "failed", "unsupported", and "skipped
  (toggle off)" are all indistinguishable from the outside — a missing key could mean any
  of them.
- Once `asset_imports.stage = 'done'`, previewing has already run and is **unobservable and
  un-rerunnable**.

So both asks need the same foundation: **preview state must become first-class** (a status
per file, aggregatable per asset) and **decoupled from the import job** so it can be re-run
and watched independently — mirroring the existing import/review queue.

---

## Settled requirements (decided before speccing)

- **Requeue granularity: BOTH asset-level and file-level.** Regenerate all previews for an
  asset's current version, *and* regenerate a single file from the file-browser detail panel.
- **Processing-queue visibility** should mirror the existing import/review queue UX (a queue
  view with live updates), not just a status buried in the asset panel.

---

## Open design questions for the spec

1. **Decouple vs. inline+status.** The likely shape is a *separate* preview queue (its own
   job kind / table, enqueued after `asset_files` are written, processed by a worker,
   tracked with status), giving a clean "Processing" queue view and requeue. An alternative
   is keeping previewing inline but adding a status column + an asset-level requeue endpoint
   that re-runs `generatePreviews`. The decoupled path matches the "see the processing
   queue" ask better but is more work. **Spec should choose and justify.**
2. **Reuse the import worker pool or a separate pool?** `internal/services/imports` already
   has a bounded `Pool` draining a `Job{ImportID, LibraryID}` queue with resume-on-startup.
   A preview job is a different unit (asset/version/file scoped, not import-row scoped) —
   decide whether to extend `Job` with a kind, add a sibling pool, or generalise.
3. **Status model.** What states (`queued`/`processing`/`done`/`failed`/`unsupported`/
   `skipped`?), at what grain (per file, with a per-asset rollup), and where stored (a column
   on `asset_files` vs a `preview_jobs` table). Note "unsupported" is a legitimate terminal
   state, distinct from "failed" — most files in a package have no backend preview.
4. **Live updates.** The import queue streams via SSE (`getImportEvents` + a `Broadcaster`).
   Decide whether previews reuse that broadcaster/stream or get their own.
5. **Requeue semantics.** Does an asset-level requeue clear existing `preview_object_key`s
   first, or only (re)try the ones without a successful preview? Does it respect
   `library_settings.previews_enabled`? Should it be idempotent (content-addressed keys
   already make regeneration cheap on unchanged files).
6. **Interaction with import.** If previewing decouples, the import pipeline should enqueue
   preview work after `asset_files` commit instead of running it inline — and the
   `StagePreviewing` stage either goes away or becomes "previews enqueued".

---

## Integration points (files the spec session should read)

**Backend — current previewing**
- `internal/services/imports/pipeline.go` — `Run()`, `generatePreviews()`, `StagePreviewing`.
- `internal/services/imports/worker.go` — the `Pool` + `Job` queue, sync mode, resume.
- `internal/services/imports/events.go` — stage constants, `Event`, the `Broadcaster`.
- `internal/services/imports/resume.go` — resume-on-startup pattern.
- `internal/services/preview/preview.go` — `Generator`, `Supports()`, `Generate()`.
- `internal/models/asset.go` — `AssetFile` (`PreviewObjectKey`/`PreviewContentType`; no status).
- `internal/models/settings.go` — `LibrarySettings.PreviewsEnabled`.
- `internal/handlers/api/files.go` — `getPreview` serving endpoint.
- `internal/handlers/api/imports.go` — `getImports`, `getImportEvents` (the queue API to mirror).

**Frontend — import queue to mirror**
- `web/src/pages/QueuePage.tsx` — the import/review queue view.
- `web/src/hooks/useImportEvents.ts` — SSE subscription.
- `web/src/design-system/domain/QueueItem.tsx` — queue row.
- `web/src/components/filebrowser/` — where a file-level "regenerate preview" action lands;
  `FileDetailPanel` / `FilePreviewPane` for the detail-panel button.

---

## Deferred follow-up: derive asset thumbnails from file previews

The library grid only shows an asset thumbnail when one was generated at import —
loose raster/audio/font files, or Unity packages with an embedded `preview.png`.
Unity/zip packages without one get no `assets.thumbnail_object_key`. Now that per-file
preview derivatives exist (PSD/TIFF/BMP/SVG → web image), a future step can pick a
representative previewable file per thumbnail-less package and generate an asset
thumbnail from it. **Deferred to its own spec** (decided 2026-06-18). The frontend
display fix — rendering the existing `thumbnail_object_key` in the grid — shipped first.

## Constraints (from AGENTS.md / CONVENTIONS.md)

- **Migration parity:** any schema change lands in BOTH a `NNN_*.sql` migration (wrapped in
  `BEGIN;…COMMIT;`, `IF NOT EXISTS`) AND the GORM struct tags (SQLite `AutoMigrate` in tests).
  Next migration number is `010`.
- **Library scoping:** every query filters on `library_id` at the top level; 403 (never 404)
  on membership gaps. Requeue endpoints must be library-scoped + membership-checked.
- **Handlers:** receiver methods on the shared `handler`; `respond*` helpers; generic
  `"internal error"` for 500; `slog.Warn` diagnostic detail before a 4xx.
- **Frontend API discipline:** no raw `fetch` in components — go through `src/api.ts`; query
  keys in `lib/queryKeys.ts`; data hooks in `hooks/useDataQueries.ts`.
- **Worker reuse:** previewing already runs on the import pool; preview derivatives are
  content-addressed via `storage.PutThumb` (regeneration is cheap/idempotent).
