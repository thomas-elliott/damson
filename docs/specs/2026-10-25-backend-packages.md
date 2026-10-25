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
