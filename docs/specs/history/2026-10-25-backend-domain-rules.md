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

### Review — cleanup

#### Verdict
MED · introduced — `assets.Merge` does not move the source's `asset_imports` rows, so deleting the source cascades them away (`asset_imports.asset_id … ON DELETE CASCADE`, `database/migrations/001_initial.sql:127`). `ChooseCurrent` then counts a moved version with no import rows as finished. Trigger: a source version whose import failed (e.g. at extract) or is still running, with a `store_version` (set at upload, `import_create.go:170`) above every target version, is merged into the target. Failure: that version, which has no files, becomes the target's current version, breaking "a version is never current before its files are". An in-flight import for it can no longer finish because its row is gone. Before this branch, merge never changed `current_version_id`. Fix: before deleting the source, `UPDATE asset_imports SET asset_id = target WHERE asset_version_id IN (moved ids)`, or refuse to merge a source that has an unfinished import (`internal/services/assets/lifecycle.go:321-324`) → fixed: the moved versions' imports move to the target; `TestPostAssetMerge_MovesARunningImportWithItsVersion`
LOW · introduced — `ChooseCurrent` reads the versions and writes the asset without locking the asset row first, and finalize commits files and current before `transition(StageEnriching)` runs as a separate statement (`pipeline.go:233-243`). Trigger: another import of the same asset finalizes, or a version delete, subset, merge or re-extract runs, in the gap between that commit and that statement, or while another `ChooseCurrent` sits between its read and its write. Failure: the other transaction still sees the newer version at `thumbnailing`, skips it, and writes an older version as current last. Nothing chooses again afterwards. The window is milliseconds, but a batch of versions of one asset running on parallel import workers can reach it. Fix: lock the asset row (`SELECT … FOR UPDATE`) at the start of `ChooseCurrent`, and write `StageEnriching` inside the finalize transaction (`internal/services/imports/version.go:95-117`) → fixed: `ChooseCurrent` locks the asset row; the gap after finalize closed with the `files_pending` fix below, which finalize clears in its own transaction
LOW · introduced — `ChooseCurrent` decides whether a version's files are committed from the stage of its latest import, and two paths with committed files leave that stage `failed`. (a) A failure after finalize: `fail("finalize done")` (`pipeline.go:254-255`), or `FailImport` on the last attempt after a job timed out during enrichment, marks a version failed even though it is already current with its files. (b) `Reextract` passes `finishing=0` (`reextract.go:66`), so re-extracting a version whose import failed rewrites its files but never makes it eligible. In (a) the next version delete, subset, merge, re-extract or import on the asset moves current away for good. The stage review's MED named the "failed `done` write" case, but its fix (count `enriching` as finished) does not reach it, because `failJob` writes `failed`. Fix: mark files committed in the transaction that writes them (e.g. an `asset_versions` column set by finalize and `Reextract`), and filter on that instead of the import stage (`internal/services/imports/version.go:97-101`) → left for cleanup: the fix adds an `asset_versions` column (migration and GORM struct) and changes which failures count against a version, which is the operator's call → fixed: `asset_versions.files_pending` (`030_version_files_pending.sql`), set by import create and cleared by finalize and re-extract with the files; `ChooseCurrent` filters on it and loses its `finishing` argument; `TestPostReExtract_VersionFailedAfterFinalizeStaysCurrent`, `TestPostReExtract_VersionFailedBeforeFinalizeIsPassedOver`

#### Checked
repo rules (AGENTS.md, docs/architecture.md, docs/CONVENTIONS.md), documented hazards (library_id scoping in `assets.load`/`Lookups.Run`, 500 hides internals (`respondGORM`, `ErrBadPackage` text matches the pre-change 400), "pass the creating tx to `Enqueue`" (every handler enqueue is in its creating tx; the pipeline's preview hand-off runs after the finalize commit, but `ResumeInFlight` re-runs an import left at `enriching`, so files are not stranded `unsupported`), "`thumbnail_from_store` before writing a thumbnail" (`chooseCurrentAndCover` checks it), no schema change), mechanical: shape conformance, completeness honesty, coverage, cross-boundary contract (`Kind()` strings, queue names and worker registration agree across `queue.go`/`enqueuer.go`; the e2e server registers no `store_lookup` kind and passes no `StoreLookups` to the handler, so those endpoints return 503 and nothing is enqueued to a kind with no worker), shared-library precedence; correctness: logic, error handling, silent failure, input validation, idempotency & resilience, credentials & secrets, security, concurrency, external contracts (River v0.40 `InsertTx` over `riverdatabasesql`; insert notifications go out through `NotifyMany` in the tx), resource management, logging, performance; change types: data-access, web-endpoint; risk and omissions: co-change sweep, side effects, backward compatibility, deployment and rollback, tests, dependencies, structural risk, widened flaws; operator intent (no `docs/intent/`; the spec quotes the operator's steering and decisions) · cross-stage, by name: licence writers (stage 1) against enrich (3) and import enrichment (3/6): every writer goes through `licence.Replace`, automatic paths are lookup/refresh, rescan and import enrichment, and `Plan.writeLicence` turns a refused write into a conflict · queue registry (4) against enqueuers (2/3/6): every enqueuer targets a registered kind; `Resume` now runs before `Start`, which is safe because inserts use the insert-only client · `ChooseCurrent` (6) against every caller: pipeline finalize, `Reextract`, `DeleteVersion`, `Merge`, `Subset`; bulk only deletes whole assets through `assets.Delete`; nothing else writes `current_version_id` (grep) · transactions spanning enqueue (4) against preview hand-off (2) and lifecycle (6): `preview.Queue` nests as a savepoint inside `Reextract`'s tx, `Subset` enqueues in the request tx, `Lookups.Resume` and `ResumeInFlight` insert outside a tx on purpose · overlap with `### Review — stages 1…6`: the `enriching` window MED is not reported again; the third LOW above is the part of that finding its fix did not reach · issues in: logic (merge cascade, failed-after-finalize), concurrency (unlocked `ChooseCurrent`) · n/a: db-migration (no migration), dependencies beyond the River driver (covered by the stage review), shared-library precedence (no UI) · validation: independent, 1 candidate, 1 confirmed, 0 refuted · unconfirmed: a version-wide preview job that River gives up on runs `preview.FailJob` and marks failed a file that a separate single-file requeue (different args, not deduped) queued, so that file stays failed if its own job skips non-queued files (`internal/services/preview/process.go:72-79`)

#### Smells
possible Duplicated Code — the rule for "this licence is reviewed" is `ReviewedAt != nil` in `licence.Replace` but `ReviewedByUserID != nil` in `licenceChangedSince`; the second is cleared when the reviewer is deleted (`ON DELETE SET NULL`), so a refresh confirms a licence that `Replace` then refuses and reports as a conflict. One `licence.Reviewed(l)` used by both would fix it (`internal/services/enrich/refresh.go:79`, `internal/services/licence/replace.go:43`) → fixed: `licenceChangedSince` tests `ReviewedAt`, as `Replace` does

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


### Cleanup

| Acceptance | Status | Evidence |
| --- | --- | --- |
| Each step deletes the duplicates it replaces | met | No callers of the removed helpers remain (`loadOrCreateLibrarySettings`, `nextCurrentVersion`, `deleteAssetTx`); `go build ./...` |
| `docs/architecture.md` loses the traps each step removes | met | Stages 1–5 removed theirs. Cleanup added the `files_pending` sentence to Imports |
| Current version is the highest parseable store version, then newest import | met | `TestChooseCurrent`, plus one handler test per path (import, re-extract, subset, merge, version delete). Two cleanup tests cover an import that failed before and after finalize |
| An import, rescan or store refresh leaves a reviewed licence unchanged | met | `TestEnrichAsset_KeepsAReviewedLicence`, `TestLicence_RescanKeepsAReviewedLicenceAndAPersonReplacesIt`, `TestStoreRefresh_KeepsAReviewedLicence`; refresh now tests `ReviewedAt` like `Replace` |
| A metadata apply that fails part-way writes nothing and reports it | met | `TestApply_FailingPartWayWritesNothing` |
| A failed request that would have enqueued a job leaves no job behind | met | `TestEnqueue_JoinsTheCreatingTransaction` |
| `cmd/server` no longer passes the HTTP handler to the queue | met | `cmd/server/main.go:140` passes `imports.Deps`, `preview.Deps` and `*enrich.Lookups` |

- Review coverage: `### Review — stages 1…6` covers every stage once. `### Review — cleanup`: 1 MED, 2 LOW and 1 smell, all fixed. The operator answered "Fix now" for the files-committed LOW.
- Triage: one finding (the files-committed LOW), answered Fix now. No open `decision:`.
- Simplify: `maps.Copy` in `enrich.Prepare` and `Plan.Write`, `SourceRecordOf` in `RepairGallery`, and an unneeded filter dropped from `commitFiles`.
- Test gate: no test cut. `TestPostAssetMerge_KeepsAnUnfinishedVersionFromBecomingCurrent` renamed to `TestPostAssetMerge_MovesARunningImportWithItsVersion`, the guarantee it alone holds once `files_pending` exists.
- Gate: `just check` EXIT=0 after the last edit: lint (0 errors; 22 frontend warnings in untouched code), format, tidy, 23 Go packages ok, Vitest 339 and 94, e2e 20.
- Open, unconfirmed: when River gives up on a version-wide preview job, `preview.FailJob` may mark failed a file that a separate single-file requeue had just queued (`internal/services/preview/process.go:72-79`).
