# Large uploads & storage overhaul (S3/Garage backend)

**Status:** implemented — signed off and merged 2026-07-05 (PR #26, `c1114ef`). Superseded on
the transport side by
[`2026-08-01-upload-reliability-observability.md`](2026-08-01-upload-reliability-observability.md),
which removed the double spool and added the chunked resumable path.

## Problem

Uploads above ~500 MB fail on the S3/Garage backend. The root cause is that
the S3 storage implementation buffers the whole file in memory: `putHashed`
does `io.ReadAll(r)` into a single `[]byte` and holds it for the `PutObject`
body (`internal/services/storage/s3.go:104,128`). A 500 MB upload is 500 MB+
resident, and `io.ReadAll`'s slice-doubling spikes higher; in a memory-capped
container this OOMs. There is no multipart/streaming path — the comment at
`s3.go:100` records that streaming was deliberately deferred. LocalFS already
streams to a temp file through a hash `MultiWriter` (`localfs.go:88`), so the
fix is S3-side.

Bundled with the upload fix are three storage improvements that share the same
code (`storage.Service` and its two implementations):

- **Object metadata** — blobs today carry no self-describing metadata; all
  filename info lives only in Postgres. Write the plaintext original filename
  plus owning IDs onto each object as a fallback for DB-loss recovery and for
  eyeballing a future orphan sweep. The DB remains the source of truth; this is
  a fallback only.
- **`originals/` prefix** — the uploaded archive currently lands in the shared
  `blobs/` pool. Give true archives their own prefix so retain/purge and future
  sweeps operate on them independently.
- **Enforce `RetainOriginalPackages`** — the per-library flag exists (default
  `true`) but is never enforced. When `false`, purge the original archive after
  a successful extraction. Requires a `Delete` method on the storage interface
  (there is none today).

Also in scope: wire up `MaxUploadBytes` (currently dead code — editable in
admin, never read in the upload path) since we are in the upload path anyway.

**Out of scope:** the orphan-GC / sweep job itself. The metadata and the
`Delete`/`DeleteOriginal` method lay groundwork for it but the sweep is a later
change.

**Not a concern:** backwards compatibility and migration of existing uploads.
Only test data has been uploaded so far — new behaviour applies to new uploads;
no dual-prefix fallback or data fix-up is needed for pre-existing objects.

## Chosen approach

Four threads on the storage layer, shipped together because they all touch the
same interface and implementations.

### Thread 1 — stream large uploads (constant memory)

Rewrite the S3 `putHashed` to mirror LocalFS: stream the reader to a temp file
through a `sha256` `MultiWriter` (preserving content-addressing), then upload
the seekable temp file with the AWS SDK v2 `feature/s3/manager` `Uploader`,
which chunks into multipart parts automatically. Memory stays flat regardless
of size. The HEAD-skip dedup check is unchanged (hash → HEAD → upload only if
absent). Temp files live in `os.TempDir()` (deploy mounts a ~20 GB tmpfs there
via `TMPDIR`). Extraction entries (up to 2 GiB each, `unitypackage.go:179`) go
through the same `PutBlob` and get the fix for free.

### Thread 2 — object metadata (DB-loss / sweep fallback)

Attach plaintext original filename + owning IDs to each object.

- **S3/Garage:** `PutObject` `Metadata` map (`x-amz-meta-*`) — filename
  (URL-encoded, since header values must be ASCII), `library_id`, `asset_id`,
  `version_id`, `kind`.
- **LocalFS:** a sidecar `<path>.meta.json` with the same fields, keeping the
  backends at parity.

Accepted tradeoffs (confirmed): metadata is write-once — content-addressed
dedup HEAD-skips the re-upload, so a blob shared across libraries records the
*first* uploader's filename/IDs only. This is fine because it is a fallback, not
the sweep mechanism.

**ID capture (decision D1·A):** `PutBlob` runs before the DB rows exist and
extraction stores content before `asset_files` is written, so IDs are not known
at first write. Rather than a two-phase backfill, we take a filename-forward
approach and reorder only the archive upload to capture IDs cheaply:

- Upload handler (`assets.go` `postImport`): stream+hash the upload to a temp
  file → create asset/version/import rows → upload from the temp file with full
  metadata `{filename, library_id, asset_id, version_id, kind}`.
- Content blobs (loose uploads and extracted Unity entries) get filename-only
  best-effort metadata — IDs omitted where not cheaply available at write time.

### Thread 3 — `originals/` prefix (decision D2·A)

Give true archives (unitypackage now; zip later) their own `originals/` prefix.
Loose uploads keep going to `blobs/` (their content and their "original" are the
same object). Extracted files still go to `blobs/`. Thumbs unchanged.

Add parallel methods mirroring the existing `PutBlob`/`PutThumb` pattern:
`PutOriginal`, `GetOriginal`, `HeadOriginal`, `DeleteOriginal`. Retrieval of an
original branches by `package_kind` (loose → `blobs/`, archive → `originals/`)
via a single `openOriginal(version)` helper so the read sites stay clean.

Six sites read `OriginalPackageObjectKey` today and route through the helper:
pipeline extract (`pipeline.go:91`), pipeline thumbnail-from-original
(`pipeline.go:128`), re-extract (`assets.go:447`), reimport (`imports.go`),
download-original (`files.go:70`), resume (`resume.go`).

### Thread 4 — enforce `RetainOriginalPackages` (decision D3·B)

When a library has `RetainOriginalPackages=false`, purge the original archive
after a *successful* Unity extraction (after `asset_files` commit). Only true
archives are purged — a loose upload's original is its content blob and is never
purged. On extraction failure the archive is kept for retry/debug.

Purged-key handling (D3·B): keep `OriginalPackageObjectKey` on the row and add a
new `original_package_purged bool` column (migration + GORM model field).
`has_original` (`assets_crud.go:129`) and the retrieval/re-extract/download
guards consult the flag. This preserves the historical key for forensics while
still reporting the original as unavailable.

Downstream when purged: re-extract and download-original return "no retained
package" — the intended meaning of retain=off. Confirm the UI degrades
gracefully (it already gates on `has_original`).

### MaxUploadBytes enforcement (decision D4·A)

Wrap the upload request body in `http.MaxBytesReader` sized to
`ServerSettings.MaxUploadBytes` + headroom for multipart overhead, so parsing
aborts with a clean 413 before an oversized file spills to tmpfs. Load the
setting once per request.

## Task breakdown

Ordered stages; roughly one commit each.

1. **Storage interface + metadata plumbing.** Add an `ObjectMeta` struct and
   thread it through the `Put*` methods; add `PutOriginal`/`GetOriginal`/
   `HeadOriginal`/`DeleteOriginal` to `storage.Service`. Implement for S3
   (`x-amz-meta-*`), LocalFS (sidecar JSON), and the in-memory test fakes
   (`localfs_test.go`, `unitypackage_test.go`). No caller behaviour change yet.
2. **Streaming S3 upload via `manager.Uploader`.** Replace `io.ReadAll` in
   `putHashed` with stream-to-temp-file + hash + multipart upload. Add the
   `github.com/aws/aws-sdk-go-v2/feature/s3/manager` dependency. Verify a
   >500 MB upload against Garage keeps memory flat.
3. **`originals/` routing + retrieval helper.** Route archive uploads to
   `PutOriginal`; add `openOriginal(version)` and switch the six read sites.
   Loose path unchanged.
4. **Metadata at call sites (filename + IDs).** Reorder `postImport` per D1·A so
   the archive upload carries full metadata; pass filename metadata on
   content-blob writes (loose + extracted entries).
5. **Enforce RetainOriginalPackages purge.** Add the `original_package_purged`
   column (migration + model). On Unity extraction success with retain=off,
   `DeleteOriginal` + set the flag. Update `has_original` and the retrieval
   guards to consult it. Keep the archive on failure.
6. **Enforce MaxUploadBytes.** Wire the existing setting into the upload route
   with `http.MaxBytesReader` and a clean 413.

## Reuse

- **LocalFS streaming pattern** (`localfs.go:77-121`) — the temp-file + hash
  `MultiWriter` + rename flow is exactly what the S3 rewrite mirrors.
- **AWS SDK v2 `feature/s3/manager` `Uploader`** — standard multipart streaming
  uploader; new dependency but same SDK family already in `go.mod`
  (`aws-sdk-go-v2/service/s3`).
- **Existing method-per-prefix convention** (`PutBlob`/`GetBlob`/`PutThumb`/
  `GetThumb`, `s3.go:59-98`) — the `*Original` methods follow it.
- **`RetainOriginalPackages` setting** already plumbed through the settings API
  (`settings.go`) and defaults (`pipeline.go:346`); this change only adds
  enforcement.
- **`MaxUploadBytes` setting** already exists on `ServerSettings`
  (`models/settings.go:7`) and admin API; this change only reads/enforces it.

## Resolved questions

- **D1 — metadata ID capture:** filename-forward; reorder the archive upload to
  capture IDs. Content blobs get filename-only best-effort metadata. (Rejected:
  two-phase CopyObject backfill on every object; object tags.)
- **D2 — `originals/` representation:** parallel `*Original` storage methods
  with retrieval branching by `package_kind`. (Rejected: no prefix split /
  metadata-only marker.)
- **D3 — purged-key handling:** keep the key, add an `original_package_purged`
  bool column. (Chosen over blanking the key, to preserve the historical key.)
- **D4 — MaxUploadBytes enforcement:** `http.MaxBytesReader` on the request body
  (hard cap, prevents tmpfs spill). (Over post-parse `fh.Size` check.)
- **Backwards compatibility / migration:** not a concern — only test data
  exists. New behaviour applies to new uploads; no fix-up of existing objects
  and no dual-prefix retrieval fallback needed.
- **Orphan-GC sweep:** explicitly out of scope; this change only lays the
  groundwork (metadata + `Delete`).
