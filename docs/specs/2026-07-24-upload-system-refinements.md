# Upload system refinements

## Problem

Four related problems in the upload path:

1. **502s on multi-file upload.** Dropping many files fires one
   `POST /api/imports` per file, all at once, with no cap
   (`NotificationProvider.startUpload` does `files.forEach(runUpload)`).
   Each request synchronously spools the whole file to a temp file and
   uploads it to S3 *before* returning 202, so N files means N
   simultaneous long-lived requests. The upstream reverse proxy times
   out or saturates and returns 502/504. Go's own server timeouts are
   30 min, so the 502 originates at the proxy, not in-process. Backend
   error logging in the import path is also thin — the pipeline's
   `fail()` writes the reason to the DB + SSE but never `slog`s it; the
   S3 layer logs nothing and returns context-free errors; the upload
   handler's 500 path doesn't log at all. So real failures leave no
   actionable server log line.

2. **A dropped folder becomes ~600 separate assets.** There is no
   folder or multi-file concept anywhere in the stack. react-dropzone
   flattens a folder to a flat `File[]`; each loose file gets its own
   find-or-create asset, version, and single file row. The only
   mechanism that groups many files into one asset is the
   `.unitypackage` extractor. `.zip` is currently classified `loose`
   and never extracted — a latent gap, since `package_kind = "zip"` is
   already threaded through the rest of the codebase (`isArchiveKind`,
   cleanup, file-serving) but nothing populates it and no zip extractor
   exists. The use case is uploading a large folder of loose files as
   **one coherent asset**.

3. **No bulk actions; can't cancel/clear all uploads.** The asset grid
   is single-select only — no bulk edit or delete. The upload centre
   can cancel one upload at a time; there is no "cancel all" and no way
   to clear the upload list.

4. **Ambiguous drops need a disambiguation prompt.** When the intent
   isn't clear from the drop alone — a folder to bundle into one asset
   vs. several unrelated files to upload separately — the user should
   be asked how to handle it via a modal.

**Context note:** the import pipeline is *already* async (River-backed
workers, SSE progress via `imports.Broadcaster`, resume-on-boot). The
`docs/` still describe it as synchronous/deferred — they are stale. The
502 is therefore not a pipeline problem; it is the synchronous upload
*HTTP* leg being fired N-at-once with no client-side limit.

## Chosen approach

**Client-zip the bundle, reuse the archive-extraction pipeline, and
queue every upload.**

When the user chooses to bundle a folder (or several loose files) into
one asset, the browser zips them (store method — no compression,
streamed) into a single `.zip` and uploads it as **one** request. The
backend gains a real **zip extractor** mirroring the unitypackage one,
fanning the archive into N `asset_files` under a single asset. This
reuses the entire mature archive path — one original blob → extract →
files → preview/thumbnail → retain/purge — and lights up the `"zip"`
`package_kind` that is already plumbed everywhere but never populated.

The other three fixes are shared regardless of the bundling choice and
are largely determined: a client-side upload concurrency queue (the
direct 502 fix), a backend error-logging pass, bulk asset actions +
cancel/clear-all for uploads, and the intent modal.

**Why this beat the alternatives.** Two alternatives were considered
and rejected:

- **B — server multi-file endpoint** (many parts + relative paths in
  one request, grouped server-side). Rejected: one giant multipart
  request can still hit proxy limits for large folders; it adds a whole
  new upload path parallel to the existing one; it doesn't fix the
  `.zip`-never-extracted gap and drops the single-original retention
  model.
- **C — store the zip unextracted** (one opaque blob, one asset, one
  file). Rejected: the inner files aren't individually
  addressable/searchable, no thumbnails or licence detection run inside
  the bundle, and it fights the app's `asset_files` model.

Approach A collapses the 600-request folder case into a single request
(killing that 502 path outright), fixes the latent `.zip` gap for free,
and keeps assets in the content-addressed, per-file-browsable model.
The one real cost — zipping in the browser — is acceptable with a
store-only streaming zip and is mitigated by warning on very large
bundles rather than blocking.

## Task breakdown

Ordered so each stage is a coherent, independently-committable unit.

### Stage 1 — Backend error-logging pass

Add structured `slog.Error` at the three gaps, carrying actionable
fields (`import_id`, `asset_id`, `object_key`, `library_id`, and for S3
`bucket`/`status` where available):

- `internal/services/imports/pipeline.go` — `fail()` currently persists
  the reason and broadcasts it but never logs. Log the failure reason
  with context.
- `internal/services/storage/s3.go` — the layer returns raw,
  context-free errors. Wrap S3 operation errors (`putHashed`, HEAD,
  upload) with the object key, bucket, and any AWS error code/status
  before returning; log at the boundary.
- `internal/handlers/api/assets.go` — `postImport`'s 500 storage-put
  path (`respond500` at the `storage put` site) doesn't log. Add an
  `slog.Error` there.

Independent, low-risk, and immediately useful for diagnosing the 502 in
the field. Land it first.

### Stage 2 — Frontend upload concurrency queue + cancel/clear-all

- Add a small client-side concurrency limiter (**3** uploads in flight;
  the rest queue) wrapping the upload dispatch in
  `web/src/components/notifications/NotificationProvider.tsx`. Queued
  (not-yet-started) uploads must be cancellable — `uploadFile` already
  accepts an `AbortSignal`, so a queued item that's cancelled simply
  never starts.
- Add **"Cancel all"** (abort every in-flight + queued upload) and a
  **"Clear"** affordance to the uploads tab in
  `web/src/components/notifications/NotificationCentre.tsx`.

This is the direct fix for the many-separate-files 502.

### Stage 3 — Backend zip extractor + `.zip` classification

- New `internal/services/extract/zip.go` mirroring
  `unitypackage.go`: walk zip entries, stream each to `Storage.PutBlob`,
  record `(relative_path, size, hash, object_key)`, harvest a cover
  image if present. Reuse the unitypackage safety posture: reject `..`
  / absolute paths (zip-slip), cap per-entry size, total uncompressed
  size, and entry count.
- Classify `.zip` uploads as `package_kind = "zip"` in `postImport`
  (`internal/handlers/api/assets.go`) and route them to `PutOriginal`
  (archives go to the `originals/` prefix), same as unitypackage.
- Generalise the pipeline's `isUnity` gate to an `isArchive` concept in
  `internal/services/imports/pipeline.go` (`loadJobContext` +the
  extract branch in `Run`) so zip archives run extraction. `OpenOriginal`
  and `isArchiveKind` already handle `"zip"`.
- Tests: extractor happy path, zip-slip rejection, caps; handler
  classification; pipeline produces N `asset_files` for one asset.

### Stage 4 — Frontend bundle path + intent modal

- Detect ambiguous drops in `web/src/components/DropZone.tsx`: a folder
  (entries with directory structure) or multiple loose files. Single
  files and archive files (`.unitypackage`, `.zip`) bypass the modal —
  archives already extract, a single loose file is unambiguous.
- New intent dialog (built on the Radix `Dialog` primitive /
  `useConfirm()` pattern) offering **Bundle into one asset** vs.
  **Upload separately**.
- On "Bundle": client-zip the files (store method, streamed) into one
  `.zip` named from the folder, and enqueue it as a single upload
  through the Stage 2 queue. On "Upload separately": the existing
  per-file path, now queued.
- Very-large-folder handling: **warn and let the user proceed** (no
  hard block, no per-file fallback for now).

### Stage 5 — Bulk asset select + bulk edit/delete

- Multi-select state on the asset grid (`web/src/components/AssetGrid.tsx`
  / `LibraryPage.tsx`), following the `FileBrowser` `Set<>`-selection
  pattern, driving the existing `SelectionBar` composition.
- Bulk actions: **delete**, **add/remove tags**, **mark
  reviewed / needs-review**, **add to collection**, **set licence**.
- A bulk backend endpoint in `internal/handlers/api/assets.go` so a
  bulk operation over many assets is one audited request, not N
  client-side calls (which would recreate the same overload problem).
  Mirror the existing single-id mutation semantics and audit each
  affected asset.
- New mutation hook(s) in `web/src/hooks/useMutations.ts` + query-key
  invalidation.

## Reuse

- **Archive-extraction pipeline** — the zip extractor reuses the whole
  unitypackage flow: single original blob in `originals/`,
  `OpenOriginal`, `WriteAssetFiles`, preview/thumbnail handoff,
  retain/purge-original logic. `package_kind = "zip"` is already
  recognised by `isArchiveKind` (`pipeline.go`), cleanup
  (`storage/cleanup/cleanup.go`), and file-serving (`handlers/api/files.go`).
- **Upload plumbing** — `uploadFile` in `web/src/api.ts` already
  supports `AbortSignal` and an optional `asset_id`; the queue and
  cancel-all layer on top without touching it. The async import
  pipeline (River workers, SSE broadcaster, resume-on-boot) already
  exists and needs no change for these fixes.
- **Frontend UI primitives** — `SelectionBar` (floating action pill),
  the `FileBrowser` `Set<number>` multi-select pattern, the Radix
  `Dialog` primitive and `useConfirm()` imperative-dialog helper, and
  the existing single-id `useDeleteAsset` / `usePatchAsset` /
  `useReviewAsset` / tag / collection / licence mutations as the
  behavioural reference for the bulk endpoint.
- **Search that came up empty** — no client-side upload concurrency
  limiter exists anywhere (`p-limit`/semaphore/batch search found
  nothing in the upload path); no zip extractor exists
  (`internal/services/extract/` holds only `unitypackage.go`); no
  asset-level multi-select or bulk endpoint exists. A JS zip dependency
  will be added for the client-side bundling (a streaming, store-capable
  zip lib) — none is currently present.

## Resolved questions

- **Folder-bundling approach:** A — client-zip + backend zip extractor
  (over B server multi-file endpoint and C store-unextracted).
- **Upload concurrency limit:** 3 in flight.
- **Bulk-edit scope:** delete plus all four edits — add/remove tags,
  mark reviewed/needs-review, add to collection, set licence.
- **Very large folder handling:** warn and let the user proceed; no
  per-file fallback, no hard threshold block for now.
- **Client-side zip mode:** store method (no compression), streamed —
  fast, low-memory, and most inner assets are already-compressed bytes.
- **Bundled-asset naming/version:** asset name from the folder name;
  `store_version` stays blank (unversioned), consistent with existing
  loose-upload behaviour.
- **Bulk delete & GC:** bulk delete removes rows + audits only;
  unreferenced blobs are handled by the existing deferred/separate GC
  sweep — no new blob-deletion path.
- **Duplicate detection on bundles:** the zip's content hash dedupes
  the archive (HEAD-before-PUT); individual inner files still dedupe by
  their own hashes on extract.

## Deferred comments

Parked why-notes from `/implement` (comment-free build). `/cleanup` evaluates
each with the diff in view, drops any the code already conveys, and writes the
survivors as terse comments — then removes this section.

- `internal/services/storage/s3.go` `wrapS3` — why errors are wrapped with
  bucket/object-key/AWS-code and logged at the boundary: makes the multi-file
  502 diagnosable in the field. The double log (here + the handler's 500 path)
  is intentional — different context, correlated by object key.
- `internal/handlers/api/assets.go` postImport 500 path — the slog carries the
  full asset/version/import context for the same reason.
- `web/src/components/notifications/NotificationProvider.tsx` `cancelUpload` —
  a still-queued upload has no in-flight XHR to fire an abort event, so it must
  be pulled from the queue + visible list explicitly.
- NotificationProvider `MAX_CONCURRENT_UPLOADS = 3` — the client-side cap that
  is the direct 502 fix.
- NotificationProvider `clearUnread` — deletes action handlers inside the
  setState updater; idempotent so StrictMode double-invoke is safe, but it
  diverges from `dismiss`'s delete-outside-the-updater pattern.
- `internal/services/extract/unitypackage.go` `cleanRelPath` — shared zip-slip
  safety core behind both the unitypackage and zip extractors.
- `internal/services/extract/zip.go` — the cover is the first bundled image
  (≤ maxZipCoverBytes) stored raw, not a downscaled thumbnail; mirrors the
  unitypackage raw-`preview.png` precedent. The total-size cap sums declared
  central-directory sizes (a lying directory is a residual zip-bomb vector
  equal to the existing unitypackage posture); per-entry actual bytes are hard
  capped via LimitReader + post-write check.
- `internal/services/imports/pipeline.go` thumbnail gate
  `!isArchive || (isUnity && ThumbnailsUnityPreview)` — zip must not attempt to
  thumbnail its raw archive bytes; it relies on the extractor-harvested cover.
- `web/src/lib/bundleUpload.ts` `pushFile` — streams via File.stream() in the
  browser (bounded memory for large files); falls back to arrayBuffer() where
  stream() is absent (jsdom/older envs). Output chunks still accumulate fully
  in memory before the File is built — the large-bundle dialog warning is the
  mitigation (store method, warn-and-proceed).
- `internal/handlers/api/assets_bulk.go` `postBulkAssets` — one audited
  transaction so a bulk edit is a single request rather than N client calls
  (which would recreate the upload overload). The up-front scope count makes a
  stray cross-library id fail the whole batch atomically (404).
- `internal/handlers/api/assets_crud.go` `deleteAssetTx` — extracted so the
  single-asset delete and the bulk endpoint purge the same children + audit
  identically.
- `web/src/pages/LibraryPage.tsx` selection reset (`scopeKey`/`prevScopeKey`) —
  clears the selection when the search/filter scope changes, using the
  render-time reset pattern rather than an effect (satisfies
  react-hooks/set-state-in-effect).
