# Move domain rules out of the HTTP handlers

Covers DAM-60

## Why are we doing this

Contributors find each domain rule in one place, so a rule changes once and the copies stop
drifting. Users stop seeing the effects of drifted copies, such as an asset's current version
depending on which action last touched it.

## Steering

Session, 2026-10-25, operator: "Split it into however many specs they need for each phase", then
chose "Fewer, larger specs": one spec per phase.

## Design

none. Direct decision: Phase 2 of the backend architecture review, as scoped on DAM-60.

## Decisions

### Which current-version rule wins?
Answer: one rule for every path: the highest parseable store version, then the newest import
date. This is today's version-delete rule. Import, version delete, subset, re-extract and merge
all use it. Importing v1.0 after v2.0 leaves v2.0 current. Re-extracting an old version no longer
makes it current. A merge moves current to a newer moved version. A subset becomes current when
its store version ties, because it has the newest import date.
By: recommended, operator said "go" (2026-10-25)

### Can a licence write replace a reviewed licence?
Answer: automatic paths (import enrichment, rescan, store refresh) never replace a reviewed
licence. A person's manual write still replaces it and clears the review, as today.
By: recommended, operator said "go" (2026-10-25)

### How does enqueue join the creating transaction?
Answer: an insert-only River client on `riverdatabasesql` over GORM's `sql.DB`, so `InsertTx`
takes GORM's transaction. The worker client stays on pgx.
By: recommended, operator said "go" (2026-10-25)

### Where does store lookup and enrichment live?
Answer: `internal/services/enrich`. DAM-61 moves it to `internal/enrich`.
By: recommended, operator said "go" (2026-10-25)

### What is left of the library step?
Answer: the last-admin check already runs inside the transaction with a row lock. The library
step is now: delete relies on `ON DELETE CASCADE` instead of walking child tables by hand, and one
settings loader replaces the two copies of `loadOrCreateLibrarySettings`.
By: settled by `internal/handlers/api/members.go` and the migrations (2026-10-25)

## Approach

Each step ships alone, in this order:

1. Licence: status, status SQL and one `Replace` writer in `licence`. Replaces five write paths.
2. Preview hand-off: `preview.Queue(...)` replaces the copies of mark-queued, settings check,
   enqueue.
3. Store lookup and enrichment: `RunStoreLookup`, `store_refresh.go` and `applyMetadata` move to
   `enrich`. `Apply` runs in one transaction and returns errors. Import enrichment uses it, which
   deletes the twins in `enrich.go`. The queue no longer runs on `*Handler`.
4. Job registry: each job kind registers its worker, resume sweep and final-failure handler.
   Enqueue moves into the creating transaction.
5. Library: delete through the cascade; one settings loader.
6. Asset lifecycle: one `ChooseCurrent`, plus delete, delete version, merge, subset and
   re-extract. Bulk calls the same functions.

## Acceptance

- Each step deletes the duplicates it replaces.
- `docs/architecture.md` loses the traps each step removes.
- After import, version delete, subset, re-extract or merge, the current version is the one with
  the highest parseable store version, then the newest import date.
- An import, rescan or store refresh leaves a reviewed licence unchanged.
- A metadata apply that fails part-way writes nothing and reports the failure instead of
  "applied".
- A failed request that would have enqueued a job leaves no job behind.
- `cmd/server` no longer passes the HTTP handler to the queue.

## Out of scope

- Store refresh's audit-log fallback for assets with no source record stays as it is.
- Package moves and renames beyond `enrich` (DAM-61).

## Start here

- `docs/architecture.md`
- `internal/handlers/api/assets_crud.go`
- `internal/queue/queue.go`

## Deferred follow-ups

None

## Implementation report

Mode: committing, one commit per stage, on branch `dam-60-domain-rules`.

### Stages

1. Licence: status, status SQL and one `Replace` writer in `licence`; automatic paths never replace a reviewed licence.
2. Preview hand-off: `preview.Queue(...)` replaces the mark-queued, settings check and enqueue copies.
3. Store lookup and enrichment move to `internal/services/enrich`; `Apply` runs in one transaction and returns errors; import enrichment uses it; the queue no longer runs on `*Handler`.
4. Job registry: each job kind registers its worker, resume sweep and final-failure handler; enqueue joins the creating transaction through an insert-only `riverdatabasesql` client.
5. Library: delete through `ON DELETE CASCADE`; one library settings loader.
6. Asset lifecycle: one `ChooseCurrent` used by import, version delete, subset, re-extract and merge; delete, delete version, merge, subset and re-extract as domain functions that bulk calls.

### Choices made

- Stage 1, `internal/handlers/api/metadata.go`: a metadata fetch, a confirmed apply and recording a reference with a listing write as `licence.ByPerson`; only store lookups write as `licence.Automatic`. A refused automatic write is reported as a `licence` conflict.
- Stage 1, `internal/handlers/api/assets_rescan.go`: rescan no longer checks the review itself before reading the licence file; `licence.Replace` refuses, and the response carries the kept licence's type.
- Stage 2, `internal/services/library/settings.go`: the one library settings loader lands here, in stage 2, because `preview.Queue` needs it and `preview` cannot import `api`. Stage 5 keeps only the cascade delete. A concurrent first read now reads the other request's row instead of failing on the primary key.
- Stage 2, `internal/services/preview/process.go`: `preview.Queue` marks the files even with no enqueuer, so the boot resume sweep picks them up; it always enqueues the job, as the import hand-off did, so a version with nothing to render still gets its final rollup. Subset keeps its own enqueue: it copies the source's preview state rather than re-marking.
- Stage 3, `internal/services/enrich/enrich.go`: an apply splits into `Prepare` (reads, cover and gallery downloads) and `Plan.Write` (every write, in the caller's transaction), so a caller can save the source record, Needs review and the lookup's outcome in the same transaction. A cover or gallery image that fails to download is skipped and not reported applied, as before; only a failed write fails the apply.
- Stage 3, `internal/services/enrich/enrich.go`: an apply with no user writes no `asset.metadata.apply` audit row; import enrichment keeps auditing its whole run as one system `asset.update`. A store lookup whose batch creator is gone now attaches tags with no user instead of failing on user 0.
- Stage 3, `internal/handlers/api/assets_reference.go`: recording a reference whose listing fails to apply keeps the recorded asset and answers `metadata_failed`.
- Stage 3, `internal/services/imports/version.go`: `StoreUpdateAvailable` stays in `imports` beside the version parser; the rest of `metadata_fields.go` and `gallery.go` moved to `enrich`.
- Stage 4, `internal/queue/queue.go`: the registry is the unexported `kind` per job type, built in `queue.New` from the domain packages, and `Queue.Resume` replaces the four boot sweeps in `cmd/server`. A queue built without `StoreLookups` (the e2e server) has no `store_lookup` kind. A sweep's enqueue error now stops that sweep and is logged; the next boot runs it again.
- Stage 4, `internal/services/preview/process.go`: previews gain a final-failure handler, `preview.FailJob`, which marks the files a given-up job left queued or processing failed. Before, they stayed queued until the next boot.
- Stage 4, `internal/services/imports/importstest` and `internal/services/preview/previewtest`: the synchronous test pools run the job inside the enqueuing transaction, as if it ran once that transaction committed.
- Stage 5, `internal/handlers/api/library.go`: library delete deletes the libraries row and lets the cascades remove the rest; `users.current_library_id ON DELETE SET NULL` clears the members' current library, so only the token watermark is bumped by hand.
- Stage 6, `internal/services/imports/version.go`: `ChooseCurrent` is in `imports`, the package that parses store versions. It breaks a final tie by the newest row and passes over a version whose latest import failed or has not yet committed its files (any stage before `enriching`), except the import committing them in that transaction. A version is never current before its files are.
- Stage 6, `internal/services/imports/reextract.go`: a re-extract's harvested cover replaces the asset's only when the re-extracted version is current.
- Stage 6, `internal/services/assets/lifecycle.go`: the domain functions refuse with `assets.ErrConflict` and `assets.ErrBadRequest`, which `respondGORM` maps like the handler sentinels. Subset now reads the request body before the asset, and a missing asset or version both answer "asset or version not found".

### Handoff (session 1, paused by the operator)

Session 1 built and committed stages 1–5 (`1e68abf`, `6f91bb4`, `b4db1f3`, `c5f6c64`, `e0e3794`). Session 2 finished stage 6 (`8cef43e`) and ran the closing steps over all six stages.

### Review — stages 1…6

#### Verdict
MED · introduced — `ChooseCurrent` passes over any version whose latest import stage is not `done`, but the import pipeline makes its version current at finalize (`pipeline.go:233`) and only writes `done` after `enriching` (store search, listing fetch, cover and gallery downloads; `pipeline.go:243-254`); a version delete, merge, subset or re-extract on the same asset in that window re-chooses with `finishing=0`, moves current back to the older version, and nothing re-chooses once the import reaches `done`. A failed `done` write leaves a version with committed files excluded the same way. Fix: treat `enriching` as finished, e.g. `i.stage NOT IN (done, enriching)` (`internal/services/imports/version.go:97`) → fixed

#### Checked
repo rules (AGENTS.md, docs/architecture.md, docs/CONVENTIONS.md), documented hazards (library_id scoping on `assets.load`/child rows, 500 hides internals via `respondGORM`, migration + GORM struct (no schema change), `default:` tag zero-value on `library.Settings` create (all defaults true, passes), `go.mod` drift (riverdatabasesql + lib/pq indirect present)), mechanical: shape conformance, completeness honesty, coverage, cross-boundary contract (queue args/kinds vs enqueuers), shared-library precedence; correctness: logic, error handling, silent failure, input validation, idempotency & resilience, credentials & secrets, security, concurrency, resource management, logging, performance · issues in: concurrency (current-version window) · n/a: external contracts (no new provider calls), shared-library precedence (no UI added) · validation: in-context, 1 candidate, 1 confirmed, 0 refuted · checked and passed: licence writers (`Replace` locks the asset row; only rescan and store lookup/refresh and import enrichment write `Automatic`, people's writes `ByPerson`), `respondGORM` maps `assets.ErrConflict`/`ErrBadRequest` and strips the sentinel prefix, enqueue joins GORM's `*sql.Tx` (no `PrepareStmt` configured, so `ConnPool` is `*sql.Tx` inside `Transaction`), library delete relies on cascades and `SET NULL` self-refs (`029_schema_scoped_constraints.sql:14`, `002_tags_and_licences.sql:51`), `library.Settings` concurrent first read via `ON CONFLICT DO NOTHING`, subset's `previews != nil` (field is the `preview.Enqueuer` interface), removed helpers have no remaining callers · the handoff text above says stage 6 is uncommitted; it is committed as `8cef43e`

#### Smells
possible Duplicated Code — `moveAssetTags` and `moveCollectionRefs` are the same pluck-then-move-except shape over two join tables; one helper taking the model and key column would serve both (`internal/services/assets/lifecycle.go:192-216`) → fixed

### Results

| Item | Status | Notes |
| --- | --- | --- |
| Each step deletes the duplicates it replaces | passed | Stage 6 deleted `deleteAssetTx`, `nextCurrentVersion`, `versionDisplayName`, the merge helpers and the body of `subset.go` and `postReExtract`; `go build ./...` passes with no callers left |
| `docs/architecture.md` loses the traps each step removes | passed | Stages 1–5 removed theirs in their commits. Stage 6 removed no trap because the doc listed none for the current version. It adds `assets` to the repository map |
| Current version is the highest parseable store version, then newest import, after import, version delete, subset, re-extract, merge | passed | `TestChooseCurrent` (fails if the unfinished-import filter is dropped, confirmed); `TestPostImport_OlderStoreVersionLeavesNewerCurrent`, `TestPostReExtract_OlderVersionDoesNotBecomeCurrent` and `TestPostAssetVersionSubset_LeavesAStoreVersionCurrent` fail when the ranking falls back to import date only (confirmed); `TestPostAssetMerge_FoldsSourceIntoTarget` fails without `ChooseCurrent` in `Merge` (confirmed); `TestDeleteAssetVersion_RepointsCurrentAndBlocks`; all against Postgres |
| An import, rescan or store refresh leaves a reviewed licence unchanged | passed | `TestEnrichAsset_KeepsAReviewedLicence` (added this session; fails if import enrichment confirms the licence and writes as a person, confirmed), `TestLicence_RescanKeepsAReviewedLicenceAndAPersonReplacesIt`, `TestStoreRefresh_KeepsAReviewedLicence` |
| A metadata apply that fails part-way writes nothing and reports the failure | passed | `TestApply_FailingPartWayWritesNothing` (Postgres) |
| A failed request that would have enqueued a job leaves no job behind | passed | `TestEnqueue_JoinsTheCreatingTransaction`, against real River and Postgres |
| `cmd/server` no longer passes the HTTP handler to the queue | passed | Stage 3 (`b4db1f3`). `internal/queue` takes `imports.Deps`, `preview.Deps` and a `StoreLookupRunner`, which `*enrich.Lookups` implements |
| real proof: stages 1, 4, 5, 6 | passed | Discharged by the Postgres-backed tests above and `TestDeleteLibrary_CascadesAndAudits`; stage 4 by `TestEnqueue_JoinsTheCreatingTransaction` on real River |
| real proof: stages 2, 3 | passed | `just dev`. Recorded a reference asset linked to a real Unity Asset Store listing: one apply filled publisher, description, cover and a `unity-eula` licence. A queued refresh ran through River against the live store and reported a `name` conflict rather than overwriting the name a person gave. Imported a PNG: the import reached `done`, became current, and its `preview` River job completed. Both assets deleted (204), with no rows left |
| review: stages 1…6 | passed | `### Review — stages 1…6`: 1 MED and 1 smell, both fixed. Re-ran `TestChooseCurrent`, `TestRun_*` and the merge tests |
| gate | passed | `just check` EXIT=0: lint (0 errors; 22 frontend warnings in untouched code), format, tidy, 23 Go packages ok, Vitest 339 and 94 passed, e2e 20 passed |

