# Preview Processing Queue & Requeue

**Status:** signed off (2026-06-18) — ready for `implement`.
**Origin brief:** `docs/archive/preview-processing-queue-brief.md`.
**Builds on:** `docs/specs/2026-06-18-file-previews.md` (the previewing feature whose
validation surfaced these gaps).

## Problem

Backend preview derivatives are generated **inside the import job** as an invisible
inline sub-stage, not as their own tracked work. That single design choice produces three
user-visible gaps:

- **No status.** `asset_files` carries only `preview_object_key` (set on success). So
  "pending", "processing", "failed", "unsupported", and "skipped (toggle off)" are all
  indistinguishable from an empty key. Failures are silent `slog.Warn`s
  (`imports/pipeline.go:236`).
- **No queue.** The import queue (`GET /api/imports/events` → `QueuePage`) shows uploads;
  there is no equivalent surface for previewing. Once an import reaches `done`, previewing
  has already run and is unobservable.
- **No requeue.** A bad or missing preview can't be regenerated short of re-importing the
  whole asset — there's no endpoint and no UI.

Underlying cause: `generatePreviews()` runs synchronously in `pipeline.Run()` between
`thumbnailing` and `done` (`imports/pipeline.go:187-190`), so the import blocks on slow
renders and previews can't be watched or re-run independently.

Both asks (requeue at asset **and** file level; visibility into the processing queue) need
the same foundation: **preview state becomes first-class** (a per-file status, aggregatable
per asset) and **decoupled from the import job** so it can be watched and re-run on its own,
mirroring the existing import/review queue.

## Chosen approach

**Status column + sibling preview pool — no new job table.** A per-file
`asset_files.preview_status` column is the durable state; orchestration mirrors the proven
import machinery (`imports.Pool` / `imports.Broadcaster`) inside the existing
`internal/services/preview` package, without entangling the two pipelines. The per-file
status column *is* the queue — queue snapshot and resume are aggregate queries over
`asset_files`, so no `preview_jobs` table is introduced.

Why it beat the alternatives:

- **vs. a first-class `preview_jobs` table (Option B):** that table would be the closest
  1:1 mirror of `asset_imports` (stable job id, trivial snapshot/resume/history), but the
  file-level UI still needs per-file status, so we'd maintain *both* a table and the
  columns plus a second state machine to keep in sync. Over-built given regeneration is
  already cheap and idempotent (content-addressed keys via `storage.PutThumb`).
- **vs. keeping it inline + status only (Option C):** least work, but it doesn't deliver a
  real processing queue mirroring imports (the headline ask), requeue would contend with
  import worker semantics, and file-level requeue is awkward.

Option A delivers both asks with the least schema and clean reuse, and leaves the import
pipeline *simpler* than today (no inline render).

### Resolved questions

Settled during alignment (lavish, 2026-06-18):

1. **Architecture** — Option A: per-file `preview_status` column + sibling preview pool, no
   `preview_jobs` table.
2. **Requeue vs `previews_enabled` toggle** — requeue **respects** the toggle: return
   **409** when previews are disabled for the library. The toggle gates previewing whether
   automatic (on import) or manual (requeue); an admin who turned previews off is not
   overridden piecemeal.
3. **Asset-level requeue scope** — regenerate **all supported files** in the asset's current
   version (not just missing/failed), since a requeue is typically triggered to replace a
   stale-but-present preview.
4. **Queue surface** — a **dedicated Processing queue page + nav entry**, mirroring the
   import queue (not a tab inside the existing Queue page).

### Status model

`asset_files.preview_status` (text, `NOT NULL`), one of:

| Status        | Meaning                                                           | Terminal |
| ------------- | ---------------------------------------------------------------- | -------- |
| `unsupported` | `preview.Supports()` is false — no backend renderer (common case) | yes (until requeue changes nothing — stays unsupported) |
| `queued`      | supported, awaiting a worker                                     | no       |
| `processing`  | a worker is rendering it                                         | no       |
| `done`        | `preview_object_key` set                                        | yes (retryable via requeue) |
| `failed`      | render error                                                    | yes (retryable via requeue) |

- A file is "in the preview queue" when its status is `queued` or `processing`.
- **Per-asset rollup is derived**, not stored: counts of files by status for the current
  version. The queue/SSE event carries the rollup `{total, done, failed, unsupported,
  processing, queued}` plus an overall version status (`processing` if any file is
  processing, else `queued` while any remain, else terminal).
- `unsupported` is set at enqueue time (so we never fetch a blob we can't render) — it is a
  legitimate terminal state, distinct from `failed`, and the common case for most files in a
  package.
- The `previews_enabled` toggle gates whether work is enqueued at all; a disabled library
  simply produces no `queued` rows (and requeue 409s — see resolved Q2).

## Task breakdown

Ordered stages; `implement` commits roughly one stage per commit. Each obeys the **migration
parity** rule (SQL migration + GORM tags) and **library scoping** (filter `library_id` at the
top; 403 never 404 on membership gaps).

### Stage 1 — Schema & model: per-file preview status

- Migration `010_asset_file_preview_status.sql` (wrapped `BEGIN;…COMMIT;`, `IF NOT EXISTS`):
  add `asset_files.preview_status text NOT NULL DEFAULT 'unsupported'`.
- Backfill existing rows: `preview_status = 'done'` where `preview_object_key IS NOT NULL`,
  else leave `'unsupported'`. (Historical rows that were supported-but-silently-failed read
  as `unsupported` until a requeue corrects them — acceptable, documented here.)
- `models/asset.go`: add `AssetFile.PreviewStatus string` with matching GORM tag (so SQLite
  `AutoMigrate` in tests carries the column).
- Status constants live with the renderer package (Stage 2 home), referenced here only by
  value in SQL.

### Stage 2 — Preview orchestration core

In `internal/services/preview`:

- Status constants (`StatusUnsupported`, `StatusQueued`, `StatusProcessing`, `StatusDone`,
  `StatusFailed`).
- Refactor the import package's `generatePreviews` into a status-driving
  `ProcessVersion(ctx, deps, versionID uint, fileID uint)`:
  - `fileID == 0` → process every supported file in the version; otherwise just that file.
  - Per file: set `processing` → render via the existing `Generator` → set `done`
    (+ `preview_object_key`/`preview_content_type`) or `failed`. Files where
    `Supports()` is false are marked `unsupported` and skipped (no blob fetch).
  - Respects `previews_enabled` (no-op when disabled).
- Queue-snapshot + resume aggregate queries over `asset_files` (group by version, joined to
  assets for library scoping and name): the rows currently having any `queued`/`processing`
  file, with per-status counts.
- Unit tests: a version with a mix of supported/unsupported/failing files lands the right
  terminal statuses; the snapshot query rolls up correctly.

### Stage 3 — Preview worker pool + broadcaster

In `internal/services/preview` (mirroring `imports/worker.go` + `imports/events.go`):

- `Pool` draining `previewJob{VersionID, LibraryID, FileID}` with `New(deps, size)` and a
  `NewSync(deps)` for deterministic tests; `Enqueue` / `Shutdown`.
- `Broadcaster` — per-library SSE fan-out of the version-rollup event shape.
- Resume-on-startup: re-enqueue versions with `queued`/`processing` files (reuse the Stage 2
  snapshot query). Mirror `imports.ResumeInFlight`.
- The pool worker calls `ProcessVersion` and publishes a rollup event per transition.

### Stage 4 — Wire into import pipeline + bootstrap

- `imports/pipeline.go`: replace the inline `generatePreviews` call with an **enqueue** of
  the version onto the preview pool *after* the `asset_files` commit. Drop `StagePreviewing`
  from the import state machine (import goes `thumbnailing` → `done`); remove the constant
  from `events.go`. To avoid an import cycle (`imports` already imports `preview`), the
  preview pool is injected into `imports.Deps` via a small interface
  (`PreviewEnqueuer interface { EnqueueVersion(libraryID, versionID uint) }`) that
  `preview.Pool` satisfies structurally.
- `cmd/server/main.go`: construct the preview `Pool` + `Broadcaster` alongside the import
  ones; call the preview `ResumeInFlight` at startup; pass both into `imports.Deps` and the
  API `Handler`.
- `handler.go`: add `previewPool` / `previewBroadcaster` fields + `Deps`.
- Update import tests that assert the `previewing` stage / inline preview side effects
  (e.g. `TestPostImport_*`): import now reaches `done` without previewing; previews are
  asserted via the (sync) preview pool instead.

### Stage 5 — Requeue + queue API

New `internal/handlers/api/previews.go`, all library-scoped + membership-checked (403 not
404), registered in `handler.go`'s `scoped` group:

- `POST /assets/:id/previews/requeue` — asset/version level. Resets every **supported** file
  in the asset's current version to `queued` and enqueues the version. **409** if
  `previews_enabled` is false. (Keeps existing `preview_object_key` until overwritten so the
  UI doesn't flash to "no preview".)
- `POST /assets/:id/files/:fileId/previews/requeue` — file level. Resets one file (must be
  supported, must belong to the asset's current version in this library) and enqueues a
  single-file job. Same 409 rule.
- `GET /previews` — queue snapshot (the in-flight version rollups for the current library),
  shaped like `getImports`.
- `GET /previews/events` — SSE stream mirroring `getImportEvents` (subscribe-then-replay,
  heartbeat, library-gated).
- Handler tests: requeue resets status + enqueues (sync pool); 409 when disabled; 403 on
  cross-library asset/file; snapshot lists only in-flight rows for the current library.

### Stage 6 — Frontend: queue page + requeue actions

- `hooks/useDataQueries.ts`: `usePreviewQueue` (GET `/api/previews`) + the
  `PreviewQueueRow`/rollup types; extend `AssetFileView` consumption with `preview_status`.
- `hooks/usePreviewEvents.ts`: SSE subscription mirroring `useImportEvents`, folding rollups
  into the `usePreviewQueue` cache.
- `hooks/useMutations.ts`: `useRequeueAssetPreviews` + `useRequeueFilePreview`, invalidating
  the preview queue + the asset detail query.
- `lib/queryKeys.ts`: `previews` key.
- `pages/PreviewQueuePage.tsx`: a Processing queue page mirroring `QueuePage` (rollup row:
  asset name + `done/total`, failed count, an overall status badge). Add the route + nav
  entry next to the import queue.
- Requeue buttons: asset-level in the asset detail panel; file-level in the file detail
  panel (`web/src/components/filebrowser/`), each calling its mutation, disabled while
  pending and when `previews_enabled` is off.
- No raw `fetch` in components — go through `src/api.ts`; keys via `lib/queryKeys.ts`.

## Reuse

- **Import queue machinery as the template** — `imports.Pool`/`Job` (`worker.go`),
  `imports.Broadcaster`/`Event` (`events.go`), `ReplaySnapshot`/`ResumeInFlight`/`LoadEvent`
  (`resume.go`), `getImports`/`getImportEvents`/`streamImportEvents` (`imports.go`),
  `postImportRetry` (the retry/enqueue pattern). The preview equivalents are siblings, not
  shared code, to keep the two pipelines decoupled.
- **`preview.Generator`** (`internal/services/preview/preview.go`) — the renderer is reused
  unchanged; only orchestration (status + pool) is added around it.
- **Content-addressed storage** (`storage.PutThumb`/`GetThumb`) — regeneration is idempotent
  and cheap, which is what makes "regenerate all supported on requeue" safe.
- **Frontend SSE + cache-fold pattern** — `useImportEvents` + `QueuePage` + `QueueItem`
  (`web/src/design-system/domain/QueueItem.tsx`) are mirrored directly.
- **Existing `previews_enabled`** (`LibrarySettings`) and `getPreview` serving endpoint
  (`files.go`) — unchanged; this feature adds status/queue/requeue around them.

## Risks & migration concerns

- **Import-test churn** (Stage 4): removing `StagePreviewing` touches every test that asserts
  the stage sequence or inline preview side effects. Mitigated by the sync preview pool —
  tests assert previews via it deterministically.
- **Backfill imprecision** (Stage 1): historical supported-but-failed files read as
  `unsupported` until requeued. Acceptable and documented; a requeue corrects them.
- **No import cycle**: `imports` must not import the preview *pool* type directly — inject via
  the `PreviewEnqueuer` interface defined in `imports`.
- **Resume ordering**: both import and preview resume run at startup; an import resumed mid-flight
  re-enqueues previews on completion, and the preview resume re-enqueues any already-`queued`
  versions — both paths are idempotent, so double-enqueue is harmless.
