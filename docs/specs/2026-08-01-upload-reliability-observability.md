# Large-upload reliability & upload observability

Status: signed off 2026-08-01. Ready for `implement`.

## Problem

Uploads of very large archives (20 GB `.zip` / `.unitypackage`) fail unreliably,
and the failures are invisible: the client sees a 502/503 while the backend logs
show nothing, and VictoriaLogs reports `missing _msg field` on the lines it does
receive. Two causes are in the code, not inferred.

### F1 — every upload is spooled to disk twice

`postImport` reads the body with `c.FormFile("file")`
(`internal/handlers/api/assets.go:415`). That calls Go's
`ParseMultipartForm(32 MiB)`, which writes any part larger than 32 MiB to a temp
file in `os.TempDir()`. The handler then opens that temp file and hands the
reader to `putHashed` (`internal/services/storage/s3.go:174-190`), which streams
it to *a second* temp file while hashing before multipart-uploading it.

A 20 GB upload therefore needs 40 GB of scratch and three full passes over the
bytes. A killed process writes no log line, and the proxy reports 502/503 with
nothing behind it.

**Confirmed during implementation (2026-08-01):** the failing pods do show
`OOMKilled`, and the k8s deploy mounts nothing at `TMPDIR` — so the mechanism is
not the tmpfs the 2026-07-05 spec assumed. `/tmp` is the container's overlay
filesystem, and 40 GB written through it becomes dirty page cache charged to the
pod's memory cgroup; writeback cannot keep up with the ingest rate, so
`memory.current` reaches the limit and the kernel kills the pod. Different
mechanism, same conclusion: F1 is *the* cause of the unlogged 502/503s, and
stage 3 fixes it by not writing to scratch at all. Moving `TMPDIR` to a real
volume is therefore unnecessary rather than merely optional.

The streaming fix in the 2026-07-05 spec (thread 1) addressed the S3 layer only.
It did not know that Gin had already spooled the whole body before the storage
layer was reached, so the double write survived it.

### F2 — the upload handler swallows the cause of every transport failure

Any `c.FormFile` error that isn't a `*http.MaxBytesError` becomes a flat
`400 {"error": "missing 'file' form field"}` with the underlying error discarded
and never attached to `c.Errors` — so `requestLogger` has no `err` field to
print. A client disconnect mid-transfer, a full scratch volume
(`no space left on device`), and a malformed multipart body all report
identically and unhelpfully. The same pattern appears at `fh.Open()`
(`assets.go:427`) and `invalid asset_id` (`assets.go:453`).

### F3 — slog emits `msg`, VictoriaLogs wants `_msg`

`newLogger()` (`cmd/server/main.go:415-433`) uses the stock `slog` JSON handler,
which writes `{"time":…,"level":…,"msg":…}`. VictoriaLogs treats `_msg` as the
message field unless the ingest URL sets `_msg_field=msg`.
`docs/observability.md` asserts the cluster's ingest URL does; it evidently
doesn't, and the `k8s-monitoring` chart is not ours to change.

### F4 — every request line carries the same message

`requestLogger` hard-codes `msg="request"`, so even once mapped onto `_msg` the
field carries no information and VL's default view is a wall of identical rows.

### F5 — nothing is logged until a request *ends*

`requestLogger` logs after `c.Next()` returns. A 40-minute upload is invisible
for 40 minutes, and if it never returns — proxy timeout, pod kill, client
hang-up — there is no line at all. There is also no `bytes_in`, so a transfer
that died at 19 GB is indistinguishable from one that never started, and no way
to tell a client abort from a server fault.

### F6 — one request, no resume, no dedupe-before-transfer

The whole transfer is a single HTTP request, so every hop — ingress body cap,
proxy read timeout, LB idle timeout — must be tuned to survive a multi-hour
request, and any blip at 19 GB restarts from byte 0. Re-uploading a file damson
already holds still moves all 20 GB: content-addressing dedupes *after* the
bytes arrive.

**Also in scope:** `ServerSettings.MaxUploadBytes` still defaults to 2 GiB
(`internal/models/settings.go:7`), so a 20 GB upload is rejected today
regardless of the above.

## Chosen approach

**Chunked resumable uploads (approach A), with the single-request streaming fix
(approach C) as an early stage.**

The client hashes the file, opens an upload session, and sends it as a series of
bounded chunks that the server streams straight into an S3 multipart upload. No
request is larger than one chunk or longer than a few seconds, so proxy timeouts
and body caps stop being part of the reliability story; a dropped chunk is
retried rather than restarting the transfer; and nothing is ever spooled to
scratch.

C is not a competing option so much as the subset of A that can land first: the
`r.MultipartReader()` change removes the OOM on its own and A builds directly on
top of it. Stages 1–3 are therefore independently shippable and worth merging
before the chunking work exists.

**Why this beat the alternatives.**

- **B — presigned direct-to-S3 multipart from the browser.** Fastest, and the
  bytes never touch Go or the proxy at all. Rejected: it requires Garage to be
  reachable from users' browsers with CORS configured, it is dead on the
  `LocalFS` backend so feature parity breaks, the server never sees the bytes so
  integrity needs a follow-up read, and it moves reliability into deployment
  topology rather than code.
- **C alone — stream the single request and stop there.** Rejected as a
  destination (though adopted as stage 3): it still leaves one multi-hour
  request, so proxy tuning is still required, a blip at 19 GB still restarts
  from zero, and there is no dedupe-before-transfer.

### Upload session API

New endpoints on the library-scoped group, alongside the existing
`POST /api/imports`:

| Endpoint | Body / returns | Behaviour |
| --- | --- | --- |
| `POST /api/imports/uploads` | `{filename, size_bytes, sha256, asset_id?, source_kind?, source_relative_path?}` → `{upload_id, chunk_size, received_bytes, deduped}` | `sha256` is **required**. HEAD that key first: already present ⇒ `deduped:true`, the import runs immediately and not a byte transfers. Otherwise open an S3 multipart upload targeting the final content-addressed key. |
| `PUT /api/imports/uploads/:id/chunks/:n` | raw octet-stream body → `{received_bytes, next_chunk}` | Streams straight to `UploadPart`. Idempotent per index — a retried chunk overwrites the same part. |
| `GET /api/imports/uploads/:id` | → `{received_bytes, next_chunk, status, expires_at}` | Resume point after a reload or network drop. |
| `POST /api/imports/uploads/:id/complete` | → `202 {import_id, asset_id, suggested_match?}` | Completes the multipart, checks the server's running hash against the declared one (mismatch ⇒ abort the multipart, 400), then runs the transaction `postImport` runs today and enqueues the pipeline. |
| `DELETE /api/imports/uploads/:id` | → `204` | Aborts the multipart and drops the session. |

Validation, source-hint handling (`imports.ValidateSource`), duplicate-version
conflict checks, and the asset/version/import row creation are all reused
verbatim from `postImport` at the complete step — the session endpoints add
transport, not import semantics.

### Storage interface addition

One new method on `storage.Service`, following the existing prefix-method
convention:

```go
BeginMultipart(ctx context.Context, prefix string, key string, meta ObjectMeta) (MultipartUpload, error)

type MultipartUpload interface {
    ID() string
    UploadPart(ctx context.Context, n int, size int64, r io.Reader) (etag string, err error)
    Complete(ctx context.Context, parts []Part) (objectKey string, err error)
    Abort(ctx context.Context) error
}
```

S3 maps 1:1 onto `CreateMultipartUpload` / `UploadPart` /
`CompleteMultipartUpload` / `AbortMultipartUpload`. LocalFS appends to a temp
file and renames on complete — effectively what it already does in `localfs.go`.
The in-memory test fakes gain the same.

### Verifying the declared hash across requests

The client declares the hash, but the server never trusts it — it re-hashes what
actually lands. Chunks arrive as separate HTTP requests, possibly on different
pods, so the running SHA-256 cannot live in memory. `crypto/sha256`'s digest
implements `encoding.BinaryMarshaler`, so the hash state is marshalled into
`upload_sessions.hash_state` after each chunk and restored before the next.

This makes chunks strictly sequential — the trade for verifying without a second
full read. It is the right default at these sizes (a 32 MiB chunk over a typical
link is ~2–10s). Because the declared hash fixes the key up front there is no
staging key and no copy-on-complete: parts are written straight into
`originals/<hh>/<hash>`.

### Logging

- **`_msg` / `_time`** — `slog.HandlerOptions.ReplaceAttr` in `newLogger()`
  renames `msg` → `_msg` and `time` → `_time` for the JSON handler only; the
  text handler stays human-readable for local dev. Collector-independent, so it
  works whatever the cluster chart does.
- **Composed messages** — `requestLogger` builds
  `"POST /api/imports → 202 in 431ms"`. Structured fields are unchanged, so
  every LogsQL query in `docs/observability.md` keeps working.
- **Real causes** — every `AbortWithStatusJSON` in the upload/import path goes
  through `respond400` / `respond500` (which call `c.Error`), and the upload
  handler distinguishes client disconnect, disk-full, and malformed body instead
  of blaming a missing form field.
- **In-flight visibility** — a start line on upload routes, plus `bytes_in`
  (bytes actually read, not `Content-Length`) and a `client_aborted` flag from
  `errors.Is(c.Request.Context().Err(), context.Canceled)`. With chunking, each
  chunk is its own short request, so progress becomes inherently visible.
- **Storage-side** — extend the existing `wrapS3` treatment to multipart part
  failures, and log spool errors (`ENOSPC`) distinctly from S3 errors.

## Task breakdown

Ordered stages; roughly one commit each. Stages 1–3 stand alone and can merge
before 4–8 exist.

1. **Log plumbing.** `_msg`/`_time` mapping via `ReplaceAttr`, composed request
   message, `bytes_in`, `client_aborted`, start-line on upload routes. Correct
   and extend `docs/observability.md`.
   — `cmd/server/main.go`, `docs/observability.md`
2. **Stop swallowing upload errors.** Fix F2: real causes through
   `respond400`/`respond500`, disconnect vs disk-full vs malformed-body
   distinguished rather than collapsed into "missing 'file' form field". Sweep
   the sibling abort sites in the import path.
   — `internal/handlers/api/assets.go`, `imports.go`
3. **Stream the one-shot upload — kill the double spool.** Replace `c.FormFile`
   with `r.MultipartReader()`; stream the part into the uploader without a
   second temp file. Enforce `MaxUploadBytes` against the streamed count, and
   raise its default from 2 GiB to 64 GiB. This alone removes the OOM.
   — `internal/handlers/api/assets.go`, `internal/services/storage/s3.go`,
   `internal/models/settings.go`, admin defaults in `admin.go`
4. **`storage.Service` multipart abstraction.** `BeginMultipart` +
   `MultipartUpload` for S3, LocalFS and the test fakes. No caller behaviour
   change yet.
   — `internal/services/storage/*.go`
5. **`upload_sessions` table + session endpoints.** Migration + GORM model
   (library_id, user_id, filename, declared size, declared sha256, chunk size,
   received bytes, next index, backend upload id, part ETags as JSONB,
   `hash_state bytea`, expires_at, status), the five endpoints, and an expiry
   janitor that calls `Abort` on stale sessions.
   — `database/migrations/NNN_upload_sessions.sql`, `internal/models`,
   `internal/handlers/api/uploads.go`, `internal/handlers/api/handler.go`
6. **SPA chunked uploader.** SHA-256 in a Web Worker with a visible "hashing"
   phase, chunk loop with backoff, resume on reload, progress and retry
   surfacing in the upload centre. The existing 3-way concurrency limiter still
   applies, now per-file rather than per-request.
   — `web/src/api.ts`, `web/src/components/notifications/NotificationProvider.tsx`,
   `NotificationCentre.tsx`
7. **`dmsn` CLI chunked uploader.** The same loop in Go, plus `--resume`.
   Hashing up front is cheap here, so cache re-syncs dedupe without
   transferring.
   — `cmd/dmsn/upload.go`
8. **Docs + e2e.** `docs/architecture.md` upload section, `docs/e2e-flows.md`
   coverage, `docs/progress.md`.

## Reuse

- **`postImport`'s import semantics** — filename parsing
  (`imports.ParseImportFilename`), source validation (`imports.ValidateSource`),
  the duplicate-store-version conflict check, and the asset/version/import
  creation transaction are reused verbatim at the complete step. The session
  endpoints add transport only.
- **`respond400` / `respond500` / `respondGORM`** — the existing error-and-log
  convention (`docs/CONVENTIONS.md`); stage 2 is about routing through it, not
  inventing anything.
- **`wrapS3`** (`s3.go:222`) — already logs op/key/bucket/AWS code well; the
  multipart methods reuse it.
- **`manager.Uploader`** — stays as the one-shot path's uploader after stage 3;
  the chunked path uses the lower-level multipart API directly.
- **LocalFS temp-file + rename flow** (`localfs.go:77-121`) — what the LocalFS
  `MultipartUpload` implementation extends.
- **`http.MaxBytesReader`** — already wired for `MaxUploadBytes`
  (`assets.go:411`) and for JSON bodies (`main.go:311`); the chunk endpoint gets
  the same treatment at chunk size.
- **`/api/imports/preflight`** — already answers "does the library hold this?"
  by source path and size; the CLI's computed hash can strengthen it later
  (not in this spec).
- **`imports.Broadcaster` / River pipeline** — unchanged. Sessions hand off to
  the existing import queue exactly as `postImport` does.
- **Searches that came up empty** — there is no resumable/chunked upload
  machinery anywhere in the tree (no tus, no session table, no multipart
  abstraction on `storage.Service`); no JS hashing dependency is present, so a
  streaming WASM SHA-256 will be added for the SPA; `requestLogger` has no
  request-start hook and no `ReplaceAttr` today.

## Resolved questions

- **Upload approach:** A (chunked resumable), with C (single-request streaming)
  as stage 3. Rejected B (presigned direct-to-S3) — needs Garage reachable from
  browsers, kills LocalFS parity, and the server never sees the bytes. Rejected
  C-as-destination — leaves proxy tuning and no-resume in place.
- **Spec scope:** one spec covering all eight stages, rather than splitting
  observability and chunking into two. Stages 1–3 remain independently
  shippable.
- **Client-declared SHA-256:** **required** for chunked uploads. The final key
  is known before the first chunk, so there is no staging key and no
  copy-on-complete, and dedupe-before-transfer is always available. The cost is
  a full client-side read before the upload starts, so the SPA needs a visible
  hashing phase.
- **`_msg` content:** composed (`POST /api/imports → 202 in 431ms`) over the
  static `"request"`. Structured fields unchanged.
- **Chunk size:** 32 MiB, advertised by the server at session creation so it can
  be tuned without a client release.
- **Session TTL:** 24h, with a janitor that calls `AbortMultipartUpload` on
  expiry so abandoned parts don't accumulate in the bucket.
- **One-shot endpoint:** `POST /api/imports` is retained for small files;
  clients chunk only above ~256 MiB. Existing tests, the current CLI, and any
  scripted use keep working — chunking is additive.
- **`MaxUploadBytes`:** default raised from 2 GiB to 64 GiB. Existing
  deployments keep whatever is in their `server_settings` row; only the default
  changes.
  - **Deviation taken during implementation.** Read literally this would leave
    every existing deployment — including the one that prompted the spec — still
    capped at 2 GiB, because its row holds the old default verbatim and nothing
    distinguishes "never touched" from "deliberately set to 2 GiB". Migration
    022 therefore also raises rows whose value is *exactly* 2147483648, and
    leaves any other value alone. Anyone who deliberately chose a cap keeps it;
    anyone who never chose one gets the new default.
- **Multipart field ordering (new constraint).** Streaming the file part means
  the server reads text fields as it passes them, so `asset_id`,
  `store_version`, `source_kind` and `source_relative_path` must be sent
  *before* the `file` part. The CLI and test helpers already did; the SPA was
  changed. A recognised field arriving after the file is a 400 naming it,
  rather than being silently dropped.

## Known risks

- **Sequential chunks cap throughput.** Server-side incremental verification
  forces ordered chunks, so a fat pipe can't be parallelised. Fine at 32 MiB; if
  it bites, verify by re-reading at complete instead and parallelise.
- **Multi-replica + LocalFS.** Chunks may land on different pods. Fine for S3
  (state is in the bucket + DB); LocalFS chunked upload only works
  single-replica. Acceptable — LocalFS is the dev/small-deploy backend — but it
  must be documented.
- **Garage multipart behaviour.** Garage implements S3 multipart, but part-size
  minimums should be smoke-tested against the real cluster before stage 5 lands.
- **Two sessions, same content.** Content-addressing means concurrent uploads of
  identical bytes target the same key with two multipart uploads. Last
  completion wins and the bytes are identical, so it is harmless — but the loser
  must not delete the winner's object on abort.
- **Orphaned multipart uploads.** Beyond the janitor, the existing orphan sweep
  (`internal/services/storage/cleanup`) should learn about incomplete multipart
  uploads.

## Operational follow-up (outside this repo)

Both original questions are now answered — see the note under F1. `OOMKilled` is
confirmed and there is no `TMPDIR` mount, so the spool went to the container
overlay and died on page-cache pressure. Nothing here blocks the work.

- **No `TMPDIR` volume needed.** After stage 3 nothing large is spooled, so
  adding one would be dead weight.
- **Worth setting anyway:** a modest `ephemeral-storage` request/limit on the
  pod, so any future accidental spool is evicted with a clear reason rather
  than OOM-killed silently.

## Deferred comments

- `upload_errors.go` (uploadFailure.transport) — the flag means "the transfer
  is what failed", which is a different question from the status: a 400 can be
  either a bad body from the client (transport) or a storage-layer fault
  (not). It is what lets one classifier serve both the body-read site and the
  storage-put site.
- `upload_errors.go` (io.EOF vs io.ErrUnexpectedEOF) — deliberately two cases
  with the same status and body. From a body read both mean truncation; from a
  storage backend a bare io.EOF is far more likely an internal bug, so only
  ErrUnexpectedEOF is trusted as a transport failure.
