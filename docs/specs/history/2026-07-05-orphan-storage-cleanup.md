# Manual orphan storage cleanup

**Status:** implemented — signed off and merged 2026-07-05 (PR #26, `c1114ef`, branch
`feat/large-uploads-storage`).

## Problem

Deleting an asset removes its DB rows but never touches the store — `deleteAsset`
(`internal/handlers/api/assets_crud.go:350`) records *"Blob storage is untouched —
content-addressed, GC is a later job."* Failed imports, replaced thumbnails, and
purge misfires leave the same dangling objects. Nothing reclaims them today.

The storage interface can't even enumerate what's there: it has `DeleteOriginal`
but no `List`, no `DeleteBlob`, no `DeleteThumb` (`internal/services/storage/storage.go:27`).
The object metadata added in the storage overhaul (`storage.go:17`) was designed
*"as a fallback for DB-loss recovery and orphan sweeps"* — this is the sweep it
anticipated.

This change adds a **manual, sysadmin-triggered** scan → review → delete flow. It is
deliberately manual for v1: an operator invokes it from the UI, reviews the candidate
objects (with any metadata) before anything is deleted, and confirms. Automation comes
later, once we trust it.

**Out of scope:** automatic/scheduled sweeps (the `audit.Purger` ticker at
`internal/services/audit/purge.go` is the template when we're confident); DB-side GC of
soft-deleted rows; reconciling *missing* objects (a DB row whose object is absent). This
change finds and removes objects the DB no longer references — nothing more.

## Chosen approach

A pair of sysadmin-only endpoints on the existing admin surface, a small
orphan-detection service that generalises the reference-count logic already in
`maybePurgeOriginal` (`internal/services/imports/pipeline.go:223`), and a review panel
in the existing `ServerAdminSection`. Each deletion is audited server-level via the
existing `audit.Write`. Unlike the storage overhaul, this change **touches the
frontend** — the review UI is inherent to "manual in the UI".

### What counts as an orphan (D2·A — prefix-accurate, purged eligible)

An object is orphaned when its content-hash key is referenced by **no** live DB row.
Because storage is content-addressed and deduped, "referenced" means the union across
every column that can point at a key, mapped per prefix:

| Prefix | Live references (the "known set") |
| --- | --- |
| `blobs/` | `asset_files.object_key` |
| `thumbs/` | `assets.thumbnail_object_key` ∪ `asset_files.preview_object_key` |
| `originals/` | `asset_versions.original_package_object_key` **WHERE `original_package_purged = false`** |

Purged originals are excluded from the known set on purpose: the DB already says that
object should be gone, so a leftover from a failed purge becomes a legitimate cleanup
target. If the same content hash is still referenced by a non-purged version, the union
keeps it safe. Loose uploads' `original_package_object_key` points into `blobs/` and is
already covered by `asset_files.object_key`, so it needs no separate handling. Thumbs
carry no object metadata (written via `PutThumb`), so their candidate rows simply show
none.

### The flow

1. **Scan** — list all three prefixes, diff against the known set, drop anything newer
   than the min-age threshold → candidate list.
2. **Review** — the sysadmin sees each candidate: prefix, key, size, age, and metadata
   (filename / owning IDs) where present.
3. **Delete** — on confirmation of a selection, each key is *re-verified* (still
   unreferenced + still old enough + still exists) immediately before `Delete`, then
   audited. Anything that changed is skipped with a reason.

### Safety — the one dangerous failure mode

Deleting in-flight uploads. Objects are written to the store **before** their DB rows
commit (the D1 upload-ordering from the storage overhaul; extraction also writes blobs
before `asset_files` exist). A scan running mid-import would see freshly-written,
not-yet-referenced objects and flag them. Two independent guards prevent any deletion of
live-but-uncommitted data:

- **Minimum-age filter (D3·A)** — objects whose last-modified is newer than
  `DAMSON_ORPHAN_MIN_AGE` (**default 7 days**) are never candidates. Covers the entire
  write→commit window with enormous margin. Mirrors the `DAMSON_AUDIT_PURGE_INTERVAL`
  config precedent (`internal/config/config.go:73`) — a Go duration parsed with a
  fallback default; no settings-form or migration churn.
- **Re-verify at delete time** — between scan and confirm, dedup means a brand-new
  upload could reference an "orphan" hash. Each key is re-checked against a freshly-read
  known set (and re-`Stat`'d for age + existence) immediately before deletion; a key that
  became referenced, too new, or already gone is skipped and reported, never deleted.
  This is a non-negotiable part of the design, not an option.

### Scan execution model (D1·A — synchronous)

The scan runs inside the request and returns the candidate list as JSON; the result
lives in React state for review. No new table, no SSE — simplest, and fine for a
deliberate, infrequent, sysadmin-only action. Known tradeoff: a very large bucket makes
the request slow. If that becomes real, the async DB-backed job + SSE variant (cloning
the preview/import queue) is the upgrade path — explicitly deferred.

### New storage primitives

Added to `storage.Service` and both implementations (+ in-memory test fakes):

- `List(ctx, prefix) ([]ObjectInfo, error)` — `ObjectInfo{Key, Size, LastModified}`.
  Cheap: one paginated `ListObjectsV2` (S3) / one directory walk (LocalFS) per prefix.
  Returns the bare content-hash key (un-sharded), matching what the DB stores.
- `StatMeta(ctx, prefix, key) (ObjectMeta, error)` — reverses `metaToS3`
  (`x-amz-meta-*` → `ObjectMeta`, URL-decoding the filename) via `HeadObject`; LocalFS
  reads the `.meta.json` sidecar. Called **only for candidates**, not the whole bucket,
  because metadata needs a HEAD per object.
- `Delete(ctx, prefix, key) error` — generic delete. `DeleteOriginal` becomes a thin
  wrapper delegating to `Delete(ctx, "originals", key)`, so there's one deletion path.

The prefix-parameterised signatures are a slight, deliberate departure from the
method-per-prefix interface convention (`PutBlob`/`PutThumb`/`PutOriginal`) because the
cleanup caller is inherently generic over the three prefixes; the private implementation
helpers are already prefix-parameterised (`putHashed(prefix, …)`, `get`, `head`).

## Task breakdown

Ordered stages, roughly one commit each.

1. **Storage enumeration + deletion primitives.** Add `List`, `StatMeta`, and generic
   `Delete` to `storage.Service`. S3 (`s3.go`): `ListObjectsV2` pagination (strip the
   `prefix/hh/` shard to recover the bare key), `HeadObject` + a `metaFromS3` reverse of
   `metaToS3`, `DeleteObject`. LocalFS (`localfs.go`): walk the prefix dir, read the
   `.meta.json` sidecar (reuse the existing suffix constant), `os.Remove` (object +
   sidecar). Refactor `DeleteOriginal` to delegate to `Delete`. Update the in-memory test
   fakes. No caller behaviour change yet.
2. **Orphan-detection service.** New `internal/services/storage/cleanup` package:
   `FindOrphans(ctx, store, db, minAge, now) ([]Candidate, error)` builds the
   prefix-accurate known sets from the DB, lists each prefix, drops anything newer than
   `minAge`, and stat-fetches metadata for the survivors. `DeleteOrphans(ctx, store, db,
   keys, minAge, now) (Result, error)` re-verifies each key against a freshly-read known
   set then deletes + audits, returning deleted / skipped(+reason) / freed-bytes. Unit
   tests against the in-memory fake + sqlite cover: a true orphan found; a referenced key
   spared; a too-new object spared; a purged-original leftover found; a key that becomes
   referenced between scan and delete skipped.
3. **Admin endpoints + config knob.** `DAMSON_ORPHAN_MIN_AGE` in `config.go` (Go
   duration, default 7 days, parsed with a fallback like `parseAuditPurgeInterval`).
   `POST /admin/storage/cleanup/scan` and `POST /admin/storage/cleanup/delete` in a new
   `internal/handlers/api/storage_cleanup.go`, registered near `handler.go:126`, both
   gated by `requireSysadmin`. The delete body carries an explicit `[{prefix, key}]` list
   so the operator deletes exactly what they reviewed.
4. **Frontend review-then-delete panel.** A "Storage cleanup" sub-section in
   `web/src/components/settings/ServerAdminSection.tsx`: a *Scan* button → candidate
   table (prefix, key, size, age, filename/IDs), select-all + per-row checkboxes, a
   freed-bytes summary, and *Delete selected* behind a `ConfirmDialog`. New `api.ts`
   calls + query/mutation hooks (`useMutations.ts`, `useDataQueries.ts`, `queryKeys.ts`).
   No SSE.

## Reuse

- **`maybePurgeOriginal`** (`internal/services/imports/pipeline.go:223`) — the
  count-live-referrers-before-delete logic is the exact orphan-detection shape,
  generalised across all prefixes.
- **Admin surface** — `requireSysadmin` (`internal/handlers/api/audit.go:305`), route
  tier at `handler.go:126`, server-level `audit.Write` with nullable `LibraryID`
  (`internal/services/audit/audit.go`).
- **Config precedent** — `parseAuditPurgeInterval` / `DAMSON_AUDIT_PURGE_INTERVAL`
  (`internal/config/config.go:73,205`) for the duration env var.
- **`metaToS3`** (`internal/services/storage/s3.go:179`) — `metaFromS3` is its inverse;
  the `.meta.json` sidecar suffix constant already exists in `localfs.go`.
- **Frontend** — `ServerAdminSection.tsx` (sysadmin block + `Dialog` confirm pattern),
  `QueuePage.tsx` (review-table with per-row actions), `ConfirmDialog` composition,
  `api.ts` client, `useMutations.ts` / `useDataQueries.ts` / `queryKeys.ts`. The
  `useDeleteLibrary` mutation is the destructive-action precedent.

## Resolved questions

- **D1 — scan execution model:** synchronous endpoint returning the candidate list.
  (Rejected for v1: async DB-backed job + SSE — deferred as the upgrade path if buckets
  get large.)
- **D2 — known-set precision & purged originals:** prefix-accurate per-column mapping;
  purged originals (`original_package_purged = true`) excluded from the known set so
  failed purges are reclaimed. (Rejected: global-union over-safe set that leaves failed
  purges behind forever.)
- **D3 — minimum-age safety knob:** config env var `DAMSON_ORPHAN_MIN_AGE`, **default 7
  days** (revised up from 24h during sign-off — the more conservative default). (Rejected:
  a `ServerSettings` admin-editable field — more surface than v1 needs.)
- **Re-verify before delete:** non-negotiable. Each candidate is re-checked against a
  freshly-read known set + re-stat'd for age/existence immediately before deletion.
- **Automation / scheduled sweep:** explicitly out of scope; manual-only for v1.
