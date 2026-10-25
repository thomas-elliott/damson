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
