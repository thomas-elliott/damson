# Restructure backend packages and narrow interfaces

Covers DAM-61

## Why are we doing this

A contributor can tell from a package's name and interface what it does, and rules such as
"downloadable original" and the object-key list are written once.

## Steering

Session, 2026-10-25, operator: "Split it into however many specs they need for each phase", then
chose "Fewer, larger specs": one spec per phase.

## Design

none. Direct decision: Phase 3 of the backend architecture review, as scoped on DAM-61.

## Decisions

### Is one generic broadcaster hub still needed?
Answer: no. DAM-58 already left one generic `broadcast.Broadcaster[T]`, so the item drops out.
By: settled by `internal/broadcast/broadcast.go` (2026-10-25)

### Which "downloadable original" rule wins?
Answer: the one in `docs/architecture.md`: the key is set, `original_package_purged` is false and
`package_kind` is an archive. All six copies use one `models` predicate. The asset detail query
starts checking `package_kind`.
By: recommended, operator said "go" (2026-10-25)

## Approach

- Split `imports.Run` into a list of stages that loads library settings once. Export `Reextract`
  from the same stages so re-extract stops copying them. Rename the package `ingest`.
- Narrow `storage.Service` to a typed `Prefix` and about eight methods. Move
  `storage.New(config)` to `cmd/server`.
- Give `models` the predicates and constants: `PackageKind`, `HasDownloadableOriginal`, and an
  `ObjectKeyColumns` list that cleanup reads, with a test that every `*_object_key` column is in
  it.
- Flatten `internal/services/*` to `internal/*` last, including `enrich` from DAM-60.

## Acceptance

- The repository map in `docs/architecture.md` matches the new layout.
- The `knownKeys` and "downloadable original" traps are gone from the doc.
- A new `*_object_key` column missing from `ObjectKeyColumns` fails a test.
- Re-extract runs the pipeline's own stages rather than a copy of them.

## Out of scope

- A repository layer over GORM, or a split by layer.
- Replacing the broadcasters (already done).

## Start here

- `docs/architecture.md`
- `internal/services/imports/pipeline.go`
- `internal/services/storage/storage.go`

## Deferred follow-ups

- blocked: DAM-60 should land first; it shrinks what has to move here.

## Implementation report

Mode: committing, one commit per stage, on branch `dam-61-backend-packages`.

### Stages

1. `models`: `PackageKind` constants, one `HasDownloadableOriginal` predicate (Go and SQL) used by every copy of the rule, and `ObjectKeyColumns`, read by cleanup, with a test that every `*_object_key` column is listed.
2. Import pipeline: `Run` becomes a list of stages that loads library settings once; `Reextract` runs the same stages; the package is renamed `ingest`.
3. Storage: `Service` narrows to a typed `Prefix` and about eight methods; `storage.New(config)` moves to `cmd/server`.
4. Flatten `internal/services/*` to `internal/*` and bring the repository map in `docs/architecture.md` up to date.

### Choices made
- Stage 1, `internal/models/package.go`: the package kinds are untyped string constants beside `AssetVersion.PackageKind`, which stays a `string`, so JSON, GORM and the handlers' filters are unchanged. `HasDownloadableOriginal` has a SQL twin, `DownloadableOriginalSQL(table)`, because the file download, asset detail, purge and sweep all decide in a query.
- Stage 1, `internal/services/imports/pipeline.go`: the purge's "another live version still uses this original" check now uses the same rule, so a loose version whose blob key equals an archive's hash no longer keeps the archive.
- Stage 1, `internal/models/package.go`: `ObjectKeyColumns` carries each column's storage prefix as a string, because `models` imports nothing from `storage`; the cleanup test checks each prefix is one the sweep lists.
- Stage 2, `internal/services/ingest/pipeline.go`: a stage is `{announce, run, reextract}` over one `jobContext`. A step fails the import with its error, or returns a `retryError` to leave it in flight for the queue; the fail reasons are unchanged (`extract: …`, `finalize: …`, `source blob missing from storage`). `Reextract` runs the steps marked `reextract` and adds its audit row and preview hand-off through `jobContext.inCommit`, inside the write transaction as before.
- Stage 2, `internal/services/ingest/pipeline.go`: `Run` loads library settings once into `jobContext.settings`; enrichment and the purge read it, and a failed load keeps each step's default (Unity cover on, no loose thumbnail, no enrichment, keep the original), as before. `preview.Queue` still reads the previews setting itself, as it does for every caller.
- Stage 2, `internal/services/ingest/reextract.go`: because re-extract now runs the import's cover step, it follows the library's Unity preview setting, and it tags extracted blobs with the version id. A storage error while re-extracting now answers 500 rather than 400.
- Stage 2, `internal/services/ingest`: log messages keep their `imports:` prefix, which `docs/observability.md` queries on.
- Stage 3, `internal/services/storage/storage.go`: `Service` is nine methods: `Put`, `Get`, `Stat`, `StatMeta`, `Touch`, `Delete`, `List` and the two multipart ones, each taking a `Prefix` (`Blobs`, `Originals`, `Thumbs`). `Stat` replaces `HeadBlob`/`HeadOriginal`. A thumbnail is now written through the same content-addressed `Put` as a blob, which yields the key its three writers computed themselves, so they stop hashing.
- Stage 3, `internal/models/package.go`: `ObjectKeyColumn.Prefix` stays a string and cleanup converts it, because `models` importing `storage` would pull the S3 SDK into every importer of `models`. `upload_sessions.object_prefix` likewise stays text and is converted where it is read.
- Stage 3, `cmd/server/storage.go`: `newStorage(ctx, cfg)` replaces `storage.New`; `storage` no longer imports `config`.
