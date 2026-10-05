# Refresh store details from the server

**Status:** implemented — all nine stages, cleaned up 2026-10-06 (PR pending).

Covers DAM-7

## Why are we doing this

The 2026-10-05 purchase run saved 208 deprecated Unity listings with only generic details, and the run reported no missing store data. The operator needs to refresh store details without fixing assets by hand, and recording shouldn't have to wait on the store.

## Steering

- The server, not the agent or dmsn, runs and throttles store lookups. Nobody fixes assets by hand, one page at a time. — operator, 2026-10-05
- dmsn gets a way to refresh an asset's store details. — operator, 2026-10-05
- Fix whatever else the purchase run hit that needs a code change. — operator, 2026-10-05
- Recording no longer waits for the store. This is important. — operator, 2026-10-05
- A refresh may replace what the last lookup wrote, but only when the refresh was asked for explicitly. — operator, 2026-10-05
- Show a listing's deprecated state. — operator, 2026-10-05

## Design

none — the operator's steering above, refined by the decisions below.

## Decisions

### Who runs the lookups for a refresh?
Answer: The server, in the background, not inside the HTTP request.
By: operator — "the server can manage it"

### Does dmsn get a refresh command?
Answer: Yes. `dmsn asset refresh <id>` for one asset. `dmsn bulk refresh` for many, taking `--assets <ids>` or the same filters as `dmsn assets` (for example `--store generic --source-host assetstore.unity.com`).
By: operator asked for it; command shape recommended, operator said "go"

### How are store lookups throttled?
Answer: In the server, for every caller.
- One store lookup job runs at a time.
- Lookups to one store are at least `DAMSON_STORE_LOOKUP_INTERVAL` apart. This is a new server setting, default 2 s, the gap dmsn uses today.
- The existing 0.5 s gap between requests to one host stays for every request, images included.
- A 429 or 503 pauses that host for every caller. The pause lasts for `Retry-After`, or 60 s if the store gives none, and never more than 10 minutes. The job that hit it waits, then tries again.
By: recommended, operator said "go"

### Where do deprecated Unity listings get their details?
Answer: From the listing page, when the store's search index has no product for them. The asset is saved as `store: unity` with its listing ID, version, dates, publisher, rating, description and images. Price stays blank.
By: recommended, operator said "go"

### Is a listing's deprecated state shown?
Answer: Yes, as a stored flag on the asset. Any Unity lookup that reads a deprecated listing page sets it, and one that finds the listing live clears it. The asset's details section says "Deprecated on the store". `dmsn asset` shows `store_deprecated`, and so does `dmsn assets --fields`.
By: operator — "we might as well show it"; flag and placement recommended, operator said "go"

### How is a lookup that fell back reported?
Answer: As a failed store lookup. This is when a link's store lookup fails and the generic scraper answers. The scraped fields are still kept, but the response says the store lookup failed, and dmsn counts it under `no_store_data`.
By: recommended, operator said "go"

### What does a refresh overwrite?
Answer: An explicit refresh fills blank fields. It also replaces a field a person hasn't changed. An explicit refresh is `dmsn asset refresh`, `dmsn bulk refresh`, or the server endpoint behind them. A person has changed a field if it no longer matches the asset's stored source record. If the asset has no source record, a person has changed the field if the audit log shows a person editing it. A field a person changed is kept and reported as a conflict. Every other lookup keeps today's rule and fills blanks only: recording, imports, and the web's Fetch metadata.
By: operator — "replace only on an explicit refresh"; source-record comparison recommended, operator said "go"; audit log for assets with no source record chosen by operator from options

### Does recording save a source record?
Answer: Yes, from now on. Recording a store link saves the lookup's source record, as Fetch metadata and imports already do, so later refreshes can compare against it.
By: recommended, operator said "go"

### Does a refresh mark the asset Needs review?
Answer: Only when it changes a field.
By: recommended, operator said "go"

### Does recording purchases still look the store up inside the request?
Answer: No. `dmsn purchases record` asks the server to queue each lookup. Each row takes one quick request, and the server paces the lookups. dmsn's `--interval` flag goes. The web's add-by-location dialog and Fetch metadata button still look up straight away, because a person is waiting for the result. They go through the same throttle.
By: operator — "yes, this is important"; the web keeping its immediate lookup recommended, operator said "go"

### How does dmsn show progress?
Answer: `record` prints a line to stderr for each row as it goes. By default, `bulk refresh` waits and prints the counts as they change. With `--no-wait` it returns a batch ID straight away, and that ID can be checked later. Stopping dmsn leaves the server's jobs running.
By: recommended, operator said "go"

### What does a timed-out record row say?
Answer: That the row may have been recorded, and to rerun the plan. A rerun answers `exists` for anything already recorded.
By: recommended, operator said "go"

### Can `dmsn assets` show the store?
Answer: Yes. `store`, `source_url` and `store_deprecated` become list fields.
By: recommended, operator said "go"

### Should there be a filter for deprecated assets, and should Updates available skip them?
Answer: Yes to both. `GET /api/assets` and `dmsn assets` gain a `store_deprecated` filter, and Updates available leaves deprecated listings out. This replaces the first line of Out of scope.
By: operator, in cleanup, 2026-10-05

### Does the purchase-catalogue spec's Toony Skies proof get a note?
Answer: Yes. Append a `## What implementation found` note to the shipped purchase-catalogue spec saying the Toony Skies lookup went through the generic fallback, so its proof row shows the scraper's details, not the store's.
By: operator, in cleanup, 2026-10-05

## Acceptance

- A deprecated Unity listing looked up by its link is saved as `store: unity` with its listing ID, release date and version. Its details section says "Deprecated on the store".
- A lookup that isn't an explicit refresh never replaces a field that already has a value.
- A lookup where the store adapter failed and the generic scraper answered is counted under `no_store_data`, by record and refresh alike.
- `dmsn bulk refresh --store generic --source-host assetstore.unity.com` over prod's 208 assets leaves each one `store: unity` with a release date, or reports why not. None is changed where a person edited the field.
- A refresh over hundreds of assets runs one store lookup at a time, at least the interval apart, with no 429 in the server log.
- When the store answers 429, every later request to that host waits out `Retry-After`, and the job that hit it finishes afterwards. This is tested against a fake store.
- Recording 429 purchases takes minutes, not most of an hour. No row is reported failed because a lookup was slow.
- A newly recorded asset with a store link has a source record.
- dmsn shows progress during `record` and `bulk refresh`.
- `dmsn assets --fields id,name,store,source_url,store_deprecated` works.

## Out of scope

- Filtering on the deprecated state, or excluding deprecated assets from Updates available.
- Refreshing on a schedule. A refresh runs when asked.
- A listing-page fallback for stores other than Unity. The throttle and fallback reporting apply to every store.
- Raising `dmsn assets`'s 200-row limit. Refresh takes the filters itself, so the agent doesn't need to page.
- The two failed imports of "POLYGON - Fantasy Kingdom Pack" from 31 July and 1 August, which the run surfaced but didn't cause.

## Start here

- internal/services/metadata/registry.go
- internal/services/metadata/unity.go
- internal/handlers/api/assets_reference.go
- cmd/dmsn/purchases.go

## Deferred follow-ups

None.

## Implementation report

Mode: committing — branch `store-refresh`, one commit per stage (AGENTS.md: "/implement commits per stage in this repo").

### Stages

1. **Deprecated Unity listings.** When the search index has no product, the Unity adapter reads the listing page alone and saves `store: unity` with its listing ID, dates, version, publisher, rating, description and images, price blank. Any lookup that reads the page sets or clears a new `assets.store_deprecated` flag. The API, the details section ("Deprecated on the store") and `dmsn asset` show it.
2. **Fallback reported as a failed lookup.** When an adapter fails and the generic scraper answers, the metadata says so. Recording reports `metadata_failed`, Fetch metadata reports `store_lookup_failed`, and `dmsn purchases record` counts both under `no_store_data`.
3. **Server throttle.** Lookups to one store at least `DAMSON_STORE_LOOKUP_INTERVAL` apart (default 2 s). A 429 or 503 pauses the host for every caller for `Retry-After`, or 60 s, never more than 10 minutes. A background lookup waits the pause out and tries again; a web lookup is told to try later. Tested against a fake store.
4. **Background store lookups.** A `store_lookup` River queue with one worker, lookup and batch tables, `POST /api/store-lookups`, `GET /api/store-lookups/batches/:id`, and recording that queues its lookup in a batch instead of making it in the request. Queued lookups fill blanks only and save the source record.
5. **Explicit refresh overwrite rule.** A refresh fills blanks and replaces fields a person hasn't changed: judged against the stored source record, or the audit log when there is none. A person's field is kept and reported as a conflict. Needs review is set only when a field changes.
6. **dmsn refresh and list fields.** `dmsn asset refresh <id>` and `dmsn bulk refresh` (`--assets` or the `dmsn assets` filters, `--no-wait`, `--batch`), with progress on stderr. `store`, `source_url` and `store_deprecated` become `dmsn assets` list fields.
7. **Recording queues its lookups.** `dmsn purchases record` drops `--interval`, queues each row's lookup in one batch, prints a stderr line per row, says a timed-out row may have been recorded, and ends with the batch's counts and how to follow it.
8. **Deprecated filter and Updates available.** A `store_deprecated` filter on `GET /api/assets` and `dmsn assets`, and Updates available leaves deprecated listings out. From the operator's cleanup decision.
9. **Toony Skies note.** A `## What implementation found` note on `docs/specs/history/2026-10-05-purchase-catalogue.md`: its Toony Skies proof came through the generic fallback. From the operator's cleanup decision.

### Choices made

- `internal/services/metadata/unity_listing.go`: deprecated is read from the listing page's `Product.state` (`deprecated` or `published`, checked against the real page of listing 32351); any other state leaves the flag unknown.
- `internal/services/imports/metadata_fields.go`: the deprecated flag is written by every lookup that could tell, whatever it held, because it is the store's state rather than a field a person writes.
- `web/src/components/detail/DetailsSection.tsx`: "Deprecated on the store" is a row labelled Listing, shown only when the flag is set.
- `internal/services/metadata/registry.go`: lookups are spaced per adapter host (`assetstore.unity.com`, `sketchfab.com`, `itch.io` with all its subdomains), or per URL host for the generic scraper. Only `Fetch` is spaced; name searches are not lookups.
- `internal/services/metadata/client.go`: during a pause a request to the host is refused without being sent, rather than held. Background lookups (`FetchWaiting`) sit the pause out and try again, up to 3 pauses. A web lookup gets a 503 with `Retry-After`, so a person isn't left waiting up to 10 minutes. A 429 from the listing page is passed up, not swallowed, so a lookup is never saved with the page's details missing.
- `internal/handlers/api/store_lookups.go`: `POST /api/store-lookups` takes `asset_ids`, or the `GET /api/assets` filters in its query string, and refuses a request with neither (a sort alone is not a filter), so an empty request can't look up the whole library. Assets with no store link are skipped and counted, not queued.
- `internal/handlers/api/store_lookups.go`: the batch report lists as problems the lookups that failed, that only the generic scraper answered, or that kept a field against the store, each with its reason; the rest are counts.
- `internal/handlers/api/store_lookups.go`: a lookup the store can't answer is recorded as failed and not retried; only a shutdown, a timeout or a database error is left for River to retry.
- `internal/handlers/api/assets_reference.go`: recording queues its lookup with `queue_lookup`, joining `lookup_batch_id` or a new batch. A named batch is checked before anything is recorded. Without `queue_lookup` the web's add dialog keeps its immediate lookup.
- `cmd/server/main.go`: the queue now starts after the handler is built, because store lookup jobs run through it; queued lookups left from a previous run are re-enqueued at start.
- `internal/services/imports/metadata_fields.go`: against a source record, a field holding a value the last lookup did not give counts as a person's, so it is kept; ratings are compared at the column's two decimal places.
- `internal/handlers/api/store_refresh.go`: a thumbnail counts as a person's unless a lookup set it (`thumbnail_from_store`), because the record keeps only the cover's URL. A licence counts as a person's when it is not the one the last lookup would give, or a person has reviewed it.
- `internal/handlers/api/store_refresh.go`: with no source record, a person's edits are the columns of `asset.update` audit rows with a user, and any `asset.licence.*` row for the licence. Fetch metadata's own `asset.metadata.apply` rows are a lookup's writes, not a person's.
- `internal/handlers/api/store_refresh.go`: a refresh skips a cover that is the one the last lookup gave when the asset already has a thumbnail, so a refresh that finds nothing new changes nothing and doesn't mark the asset Needs review.
- `cmd/dmsn/assets.go`: `bulk refresh` takes every `dmsn assets` filter except paging and sort; bulk's own `--tag` doubles as the tag filter, since both name a tag path. Giving `--assets` and filters together is refused.
- `cmd/dmsn/refresh.go`: `asset refresh` always waits; it is one lookup. `bulk refresh --batch <id>` follows a batch queued with `--no-wait`, waiting unless `--no-wait` is given again. Progress is one stderr line each time the counts move, polled every 2 s.
- `cmd/dmsn/purchases.go`: record does not wait for its lookups, which can run for an hour or more over 429 purchases with galleries. It ends with the batch's counts so far (`no_store_data_so_far` and the rest) and `dmsn bulk refresh --batch <id>` to follow it, where `no_store_data` is counted once the lookups have run. The skill tells the agent to follow the batch before reporting.
- `cmd/dmsn/purchases.go`: a row times out when the request does (`--timeout`, default 30 s), and only then says it may have been recorded.
- `internal/handlers/api/assets.go`: the update badge, in the list and the detail view, leaves deprecated listings out too, so it still agrees with the Updates available filter and the overview count.
- `cmd/dmsn/assets.go`: `--store-deprecated` only selects deprecated assets, as `--needs-review` only selects unreviewed ones; `GET /api/assets` also takes `store_deprecated=false`. `POST /api/store-lookups` refuses a `store_deprecated` it can't read, as it does the other filters.
- `docs/specs/history/2026-10-05-purchase-catalogue.md`: it already had a `## What implementation found` section, so the Toony Skies note is a dated subsection at its end rather than a second heading.

### Review — stages 1…7

#### Verdict
MED · introduced — when an explicit refresh's store adapter fails and the generic scraper answers, `RunStoreLookup` still applies the result with the refresh confirm set. A passing Unity outage therefore turns `store: unity` into `generic`, replaces the name and description with the scraped page title and text, marks the asset Needs review, and overwrites the good source record. Fix: when `meta.StoreLookupFailed`, apply with `confirm` nil and keep the existing record (internal/handlers/api/store_lookups.go:321) → fixed: a generic answer only fills blanks, even in a refresh, and never replaces a record already stored; `TestStoreRefresh_GenericFallbackOnlyFillsBlanks`
MED · introduced — `MetadataFieldUpdates` compares the raw rating with the `numeric(3,2)` column, and Coveo's `ec_rating` carries more precision. Every refresh of a rated live Unity asset therefore applies `rating`, counts it `changed` and sets Needs review. This breaks "Needs review only when it changes a field". Fix: round to two places before comparing, as `ChangedSince` already does (internal/services/imports/metadata_fields.go:100) → fixed: both compare through `roundRating`; `TestStoreRefresh_NoChangeLeavesNeedsReview` now sends 4.8456 against a stored 4.85
LOW · introduced — a queued record creates its lookup in a second transaction after the asset commits. If that transaction fails, the row answers 500 with the asset already recorded. The rerun the message advises answers `exists` and queues nothing, so the asset never gets a lookup. Fix: create the lookup inside the first transaction (internal/handlers/api/assets_reference.go:305) → fixed: the record and its queued lookup commit in one transaction, and a bad batch rolls the record back; `TestRecordReference_QueuedLookup`
LOW · introduced — fill-blanks batches, which recording uses, count every conflict as `kept_a_persons_field`. A conflict there is any differing existing value, such as a plan-supplied name that differs from the listing title, and not a field a person changed. Fix: label the count by batch kind (cmd/dmsn/refresh.go:97) → fixed: the server counts and lists kept fields only in refresh batches, so `kept_a_persons_field` means what it says; `TestStoreLookups_QueueThenFillBlanksOnly`

#### Checked
repo rules and documented hazards (library scoping on new tables and queries, sentinel errors in Transaction, 500 text, migration shape and model tags, enqueue after commit), mechanical (shape, completeness, coverage, client/server contract for store-lookups and record), logic, error handling, silent failure, input validation, idempotency and resilience, credentials, security (new endpoints sit under scoped routes with library_id filters), concurrency (hostLimiter pause/wait, single worker), external contracts (Retry-After seconds and date forms, 429/503), resource management, logging, performance, smells · issues in: logic, error handling, idempotency and resilience · n/a: none · validation: in-context, 4 candidates, 4 confirmed, 0 refuted · tests run: `just test backend ./internal/handlers/api -run StoreLookup` and `./internal/services/metadata` pass · unconfirmed: `editedInAudit` relies on `audit_log`, which the retention purge removes per `library_settings.audit_retention_days`. On an asset with no source record, a refresh could overwrite a person's edit older than the retention period. Not sure what the default retention is. → checked: `audit_retention_days` is nullable with no default, so nothing is purged unless a library sets a retention; with one set, the audit rule forgets edits older than it, which follows from the decision to use the audit log.

### Review — cleanup

#### Verdict
MED · introduced — a fill-blanks lookup saves the store's full metadata as the source record, including the fields it kept as conflicts and did not write. The refresh rule then reads those kept values as a person's. The trigger is in this branch's own flow: `purchases record` swaps an asset that has a record from its old link, blanks only store, product ID and version, then queues a fill-blanks lookup of the new link. The old listing's publisher, description, dates, rating, engine and price stay on the asset, and the new listing's values go into the record. From then on, every `bulk refresh` keeps those old values and reports them as `kept_a_persons_field`, though no person touched them. Fix: when a lookup applied with `confirm` nil saves its record, copy each conflicting field's value from the previous record's metadata into the record it saves. A person's edit still differs from the old record and is kept, and a lookup's old value matches it and is replaced (internal/handlers/api/store_lookups.go:347) → fixed: a lookup's record keeps the last record's value for each field it kept, through `copyRecordFields`; `TestStoreRefresh_ReplacesWhatAnEarlierLookupKept`
LOW · introduced — far side of the first stage-review fix: when a generic answer fills a blank on an asset whose store record is kept, the filled value does not match the kept record. The next refresh therefore reads it as a person's and keeps it for good. `TestStoreRefresh_GenericFallbackOnlyFillsBlanks` builds exactly this state: publisher "From The Page" against a kept Unity record with no publisher. The same happens to the `store: generic` a swap's blanked store gets when the new link falls back. Fix: when `keepRecord`, write the fields this answer applied into the kept record's metadata, so they read as a lookup's (internal/handlers/api/store_lookups.go:329) → fixed: a generic answer writes the blanks it filled into the kept record; `TestStoreRefresh_GenericFallbackOnlyFillsBlanks` now refreshes again and sees the scraper's publisher replaced
LOW · introduced — `storeLookupWorker.Work` doesn't mark the row failed on River's last attempt, as `importWorker.Work` does (internal/queue/queue.go:119). A lookup whose job is discarded after 5 errors, such as a database outage of a few minutes, stays `queued` until the next server start. Meanwhile `dmsn asset refresh` and `bulk refresh` poll it forever, because `waitForBatch` returns only when `Queued == 0`. Fix: on `job.Attempt >= job.MaxAttempts`, record the lookup failed with the error before returning it (internal/queue/queue.go:89) → fixed: the worker calls `FailStoreLookup` on River's last attempt; `TestStoreLookupWorker_LastAttemptMarksTheLookupFailed`
LOW · introduced — on a matched or swap row, `linkAsset` sets the link and then queues the lookup in a separate request. If that last request times out or fails, the row says to rerun the plan. But the rerun finds the asset already on the row's link, answers `unchanged` and queues nothing, so the asset is linked and never looked up. This is the far side of the third stage-review fix, which closed the gap for new rows only. Fix: when the lookup request fails after the link is set, report the row with `dmsn bulk refresh --assets <id>` rather than the rerun advice (cmd/dmsn/purchases.go:578) → fixed: the row says `linked, but its store lookup was not queued …; run dmsn bulk refresh --assets <id>`; `TestPurchasesRecord_LinkedRowWithoutLookupSaysToRefresh`
LOW · widened — `parseAssetFilters` silently drops a `released_after` it can't parse. dmsn passes `--released-after` through unchecked, so `bulk refresh --store unity --released-after 2024/01/01` refreshes every Unity asset rather than refusing. `GET /api/assets` has always done this, but there it only widened a list, and now it widens a write that can set Needs review. Fix: refuse an unparseable `released_after` in `postStoreLookups` with a 400, or check the date format in `selectionQuery` (internal/handlers/api/store_lookups.go:89) → fixed: `postStoreLookups` refuses an unreadable `released_after`, `tag_id`, `project_id` or `collection_id` with a 400; `TestStoreLookups_FiltersSelectTheAssets`
LOW · introduced — `DAMSON_STORE_LOOKUP_INTERVAL` is a new server setting, but it is missing from `.env.example`, which AGENTS.md names as "the full set" and which lists its sibling `DAMSON_UPLOAD_JANITOR_INTERVAL`. Fix: add a commented `# DAMSON_STORE_LOOKUP_INTERVAL=2s` line beside it (internal/config/config.go:137) → left for cleanup: a hook blocks the agent from reading `.env` files, `.env.example` included, so the line needs the operator → fix now: the operator adds the line, since the hook blocks the agent; asked again 2026-10-06, still fix now → fixed: the operator added it beside `DAMSON_UPLOAD_JANITOR_INTERVAL`

#### Checked
repo rules and documented hazards (library scoping on store_lookup_batches, store_lookups and every new query, enqueue after commit, sentinel errors in Transaction, 500 text, migration shape and model tags, `.env.example` as the full set), mechanical (shape and coverage of the batch response, client/server contract for `/api/store-lookups`, the batch route and record's `queue_lookup`/`lookup_batch_id`, completeness), logic, error handling, silent failure, input validation, idempotency and resilience, credentials, security (new routes under `scoped`, batch reads and joins filtered by library, cross-library test present), concurrency (registry and host limiters, single worker), external contracts (Retry-After forms, 429/503 passed up rather than fallen back from), resource management, logging, performance, data-access block, web-endpoint block, db-migration block (025 adds a constant-default column, no rewrite), co-change sweep, side effects (`applyMetadata` and `setThumbnail` moved from gin.Context to context.Context, all callers updated), backward compatibility (`--interval` removed from dmsn, `metadata_failed` kept), deployment (queue now starts after the handler and import reconcile enqueues before Start, which River allows), tests, structural risk, cross-stage (stage 4's fill-blanks records against stage 5's refresh rule, stage 2's fallback marking against the stage-review keepRecord fix, stage 7's rerun advice against stage 4's two-request link-then-queue) · issues in: logic, idempotency and resilience, input validation, co-change sweep, cross-stage · n/a: credentials — no secrets touched · validation: independent, 1 candidate, 1 confirmed, 0 refuted; LOW candidates checked in context · overlap: the four stage-review findings are fixed as described; the second finding here is the far side of the first fix, the fourth the far side of the third · tests: not run — local Postgres on 5432 refused connections, so `just test backend ./internal/handlers/api -run TestStore` could not start; findings rest on reading the code and the test fixtures · unconfirmed: web Fetch metadata saves a record of conflicts the person chose not to apply, so a later refresh keeps those values as a person's. That may be the intended reading of a person declining the change, so it is not listed as a finding.

#### Smells
possible Duplicated Code — unmarshalling the source record and checking `rec.Metadata != nil` appears in `changedByPerson`, `withoutUnchangedCover` and twice in metadata.go; one `sourceRecordOf(asset)` helper would serve all four (internal/handlers/api/store_refresh.go:52) → fixed for the branch's code: `sourceRecordOf` serves `changedByPerson`, `withoutUnchangedCover` and `RunStoreLookup`; the two metadata.go sites predate the branch and give their own 409s, so they stay

### Review — stages 8…9

#### Verdict
No blockers

#### Checked
repo rules and documented hazards (library scoping: the new filter rides `applyAssetFilters`, which is already scoped by `library_id`; no migration, `store_deprecated` is `NOT NULL DEFAULT false` from 025, so `NOT a.store_deprecated` never turns NULL; a spec in `history/` is only appended to under `## What implementation found`, as AGENTS.md allows), mechanical (client/server contract: `--store-deprecated` sends `store_deprecated=true`, which the server reads; `/api/search` shares `getAssets`, so it gets the filter and the badge rule too), logic (filter SQL, overview count and list badge all go through `updatesAvailableSQL`/`storeUpdateAvailable`; the detail badge uses `storeUpdateAvailable`; project pins' `HasUpdate` is a pinned-version comparison, not a store one, so it is rightly untouched), error handling, silent failure (an unreadable `store_deprecated` is ignored by `GET /api/assets`, as `needs_review` is, but refused by `POST /api/store-lookups`), input validation, smells, Toony Skies note against the purchase-catalogue proof row and the store-refresh results row for 11020 · issues in: none · n/a: concurrency — no shared state; credentials — none touched; performance — one boolean predicate on an already scoped query · validation: in-context, 0 candidates, 0 confirmed, 0 refuted · tests run on the 55432 Postgres: `TestAssetFilters_StoreDeprecated`, `TestUpdatesAvailableFilterAgreesWithTheBadge`, `TestStoreLookups_FiltersSelectTheAssets` and `./cmd/dmsn -run '^TestAssets_'` pass

#### Smells
possible inconsistent name — the filter field is `Deprecated`, while the column, the model, the query parameter and the dmsn flag all say store deprecated; rename it `StoreDeprecated`, as `NeedsReview` and `SourceHost` match theirs (internal/handlers/api/assets.go:74) → fixed: renamed `StoreDeprecated`

### Review — cleanup 2026-10-06

#### Verdict
LOW · introduced — far side of the first stage-review fix, which guards only `RunStoreLookup`. The web's Fetch metadata, and recording without `queue_lookup`, still save a generic-scraper answer as the source record, replacing a store record already there. The trigger is a person pressing Fetch metadata on a Unity asset while the Unity adapter fails (not a 429, which is passed up rather than fallen back from). The record then says `store: generic`, the page title and the page description. The next explicit refresh reads `store`, `name`, `store_product_id` and every other filled field as a person's, keeps them, and reports them as `kept_a_persons_field`. Because `copyRecordFields` carries a kept field's record value forward, those fields stay "a person's" on every later refresh, so the asset can no longer be refreshed. Fix: when `meta.StoreLookupFailed` and the asset already has a record, keep that record and write into it only the blanks this answer filled, as `RunStoreLookup` does (internal/handlers/api/metadata.go:243, internal/handlers/api/assets_reference.go:331) → fixed: `RunStoreLookup`'s record step is now `saveLookupRecord`, which Fetch metadata and recording also call; Fetch metadata passes no kept fields, so its record still holds the conflicts a person can apply; `TestMetadataFetch_GenericFallbackKeepsTheStoreRecord`
LOW · introduced — the cleanup fix for unreadable filters, extended in stage 8 to `store_deprecated`, still leaves out `needs_review`, `updates_available` and `not_downloaded`. `parseAssetFilters` drops an unreadable value for any of these. So `POST /api/store-lookups?store=unity&updates_available=yes` with `refresh: true` refreshes every Unity asset rather than refusing, and the refresh can set Needs review. Only a direct API caller can trigger this: dmsn only ever sends `true`. Fix: add the three to the refusal list beside `store_deprecated` (internal/handlers/api/store_lookups.go:98) → fixed: the three are refused like the others; `TestStoreLookups_FiltersSelectTheAssets` sends `updates_available=yes`

#### Checked
repo rules and documented hazards (library scoping: the stage 8 filter rides `applyAssetFilters`, which is scoped by `library_id`; the overview's `updatesAvailableSQL` runs on `assets AS a` filtered by library; no new migration in stages 8–9; 025 is `NOT NULL DEFAULT false`, so `NOT a.store_deprecated` is never NULL; the model tag matches; the stage 9 note is appended under the history spec's `## What implementation found`, as AGENTS.md allows; 500s stay generic; enqueue happens after commit in both `postStoreLookups` and record), mechanical (shape: `store_deprecated` is on the grid row, the detail view, dmsn's row and detail structs, and the TS `AssetDetailView`; client/server contract: `--store-deprecated` → `store_deprecated=true`, which is read by `GET /api/assets`, `/api/search` (shared `getAssets`) and `POST /api/store-lookups`; `bulk refresh` inherits the flag through `registerSelection`, and a lone `store_deprecated=true` counts as a filter, because the struct is no longer zero; completeness: batch status is uncapped, and the list keeps its `total`), logic (filter SQL, overview count, list badge and detail badge all exclude deprecated listings through `updatesAvailableSQL`/`storeUpdateAvailable`; `NOT … AND … > (…)` binds as intended; project pins' `updates_available` is a pinned-version comparison, rightly untouched; the web shows the server's `update_available` and computes nothing itself), error handling, silent failure, input validation, idempotency and resilience, credentials, security (no new route in stages 8–9; the new filter is a bound parameter), concurrency, external contracts, resource management, logging, performance (one boolean predicate on a scoped query), data-access block, web-endpoint block, db-migration block (no change since cleanup), co-change sweep (dmsn usage, skill text, describeFilters, web types), side effects, backward compatibility (`store_deprecated` added to responses only; Updates available shrinks by decision), deployment, tests (a revert of `NOT a.store_deprecated` fails `TestUpdatesAvailableFilterAgreesWithTheBadge`; a revert of the filter fails `TestAssetFilters_StoreDeprecated`), structural risk, smells, cross-stage (stage 8's badge rule against stage 1's flag-setting, which never clears the flag on a lookup that couldn't tell; stage 8's filter against stage 6's `bulk refresh` selection and the cleanup-era unreadable-filter guard; stage 2's fallback marking against every path that saves a source record, not only `RunStoreLookup`; stage 5's Needs-review rule against stage 1's flag flip, where a refresh that only flips `store_deprecated` counts as changed, which follows the decision; stage 9's note against the 11020 results row) · issues in: input validation, cross-stage · n/a: none · validation: in-context, 0 HIGH/MED candidates, LOW candidates re-read at the cited lines; the generic scraper's `Store` is `generic` (internal/services/metadata/scrape.go:35), and `postAssetMetadataFetch` saves `rec` unconditionally · overlap: the four stage 1…7 fixes, the five cleanup fixes and the stage 8…9 smell fix hold as described, and the whole-package tests below cover them; the cleanup review's unconfirmed note (Fetch metadata saving a record of conflicts the person declined) is a different trigger from the first finding here and is not repeated; the `.env.example` line is still the operator's · tests run on the 55432 Postgres: `just test backend` on `./internal/handlers/api`, `./internal/services/metadata`, `./internal/queue`, `./internal/services/imports` and `./cmd/dmsn`, all ok · unconfirmed: the spec's `## Out of scope` still lists deprecated filtering and the Updates-available exclusion, though the stage 8 decision says it replaces that line; the decision is the record, so this reads as stale rather than wrong

#### Smells
possible Duplicated Code — the `true|1` / `false|0` tri-state parse for `store_deprecated` repeats the one for `needs_review` line for line; one `optionalBool(c.Query(...))` helper would serve both (internal/handlers/api/assets.go:182) → fixed: `optionalBool` reads both, and the refusal list uses it for `updates_available` and `not_downloaded`

### Results

| Item | Status | Notes |
| --- | --- | --- |
| A deprecated Unity listing looked up by its link is saved as `store: unity` with its listing ID, release date and version; its details section says "Deprecated on the store" | passed | `TestUnityFetch_DeprecatedListingReadFromPage` (fixture cut from the real page of 32351) fails if the listing-only path is removed, because the lookup falls over to the generic scraper. The DetailsSection test fails if the Listing row goes. Real proof: on the scratch server, listing 32351 came back `store: unity`, ID 32351, 1.1.6, released 2015-03-16, `store_deprecated: true` (`dmsn asset 1`). |
| A lookup that isn't an explicit refresh never replaces a field that already has a value | passed | `TestStoreRefresh_ReplacesOnlyWhatAPersonHasNotChanged/a_lookup_that_is_not_a_refresh` fails if a fill-blanks batch is applied with the refresh confirm set; `TestStoreLookups_QueueThenFillBlanksOnly` covers recording's queued lookups. Imports keep today's rename from the store title (`enrichFillBlanks` in internal/services/imports/enrich.go, unchanged by this branch), as the decision's "keeps today's rule" says. |
| A fallback is counted under `no_store_data`, by record and refresh alike | passed | `TestRegistryFetch_MarksGenericFallback` fails if the registry stops marking the answer; `TestStoreLookups_ReportFailuresAndGenericFallbacks` and `TestStoreRefresh_GenericFallbackOnlyFillsBlanks` count it in the batch both kinds share. Record shows it through its batch (`no_store_data_so_far`, then `dmsn bulk refresh --batch`), checked by `TestPurchasesRecord_RecordsWhatEachRowSays`. No live fallback occurred in the real proof. |
| `dmsn bulk refresh --store generic --source-host assetstore.unity.com` over prod's 208 assets leaves each `store: unity` with a release date, or reports why not; none changed where a person edited | unproven | Needs a run against prod. Real proof on a reproduction: `main`'s server recorded 32351 and 11020 as `generic` (no failure reported, as in the purchase run); this branch's `dmsn bulk refresh --store generic --source-host assetstore.unity.com` made both `store: unity` with release dates and versions, and kept and reported the plan-given name and a hand-edited description. |
| A refresh over hundreds runs one lookup at a time, at least the interval apart, with no 429 in the server log | unproven | `TestStoreLookups_RunOneAtATime` (real River on Postgres) fails if the queue gets more than one worker; `TestRegistryFetch_SpacesLookupsToOneStore` fails if the per-store gap is removed. Real: 2 lookups, no 429 or pause in the scratch server's log. A run of hundreds was not made. |
| On a 429, every later request to that host waits out `Retry-After`, and the job that hit it finishes afterwards; tested against a fake store | passed | `TestRegistryFetchWaiting_WaitsOutA429ThenFinishes` fails if a request reaches the store during the pause or the job gives up; `TestClient_429PausesTheHostForEveryCaller` fails if a paused request is sent. No 429 was provoked from the real store. |
| Recording 429 purchases takes minutes; no row is reported failed because a lookup was slow | unproven | A row no longer waits on a lookup: real `dmsn purchases record` of 2 rows took 0.083 s (`time`), with both lookups queued. `TestPurchasesRecord_TimedOutRowSaysToRerun` covers a slow row. A 429-row run was not made. |
| A newly recorded asset with a store link has a source record | passed | `TestRecordReference_QueuedLookup` fails if the lookup job stops saving the record. Real: asset 1's source record is `unity` with the listing's full URL (`GET /api/assets/1`). |
| dmsn shows progress during `record` and `bulk refresh` | passed | `TestPurchasesRecord_PrintsEachRowAsItGoes` and `TestBulkRefresh_FiltersWaitAndReportProgress` fail if the lines go to stdout or stop. Real: stderr showed `record 1 of 2 …` and `store lookups: 1 of 2 run …`. |
| `dmsn assets --fields id,name,store,source_url,store_deprecated` works | passed | `TestListAssets_CarriesStoreFields` (Postgres) fails if the list query drops the columns; `TestAssets_StoreFieldsCanBeShown` if dmsn doesn't allow them. Real: the command listed both assets with `unity` and `true`. |
| build | passed | `just build`, exit 0 |
| gate | passed | `just check`, exit 0: 19 Go packages ok, vitest 209 passed in 39 files, Playwright 17 passed |
| review: stages 1…7 | passed | `### Review — stages 1…7`: 2 MED and 2 LOW, all fixed |
| real proof | passed | Scratch server from this branch on a throwaway Docker Postgres (port 55432), live Unity lookups, dmsn against it; `main`'s server built in a temporary worktree for the reproduction. Server stopped, worktree removed. Stages 1, 4, 5, 6 and 7 reached the real store and database; stage 2's fallback and stage 3's 429 were not triggered live. |

### Results — stages 8…9

| Item | Status | Notes |
| --- | --- | --- |
| decision: `GET /api/assets` and `dmsn assets` gain a `store_deprecated` filter | passed | `TestAssetFilters_StoreDeprecated` (Postgres) fails if the filter is dropped from `applyAssetFilters`; `TestAssets_FiltersMapToQueryParams` fails if `--store-deprecated` stops sending `store_deprecated=true`. `TestStoreLookups_FiltersSelectTheAssets` fails if a refresh accepts `store_deprecated=yes`. |
| decision: Updates available leaves deprecated listings out | passed | `TestUpdatesAvailableFilterAgreesWithTheBadge` (Postgres) now has an outdated deprecated asset and checks the filter, the list badge, the detail badge and the overview count. Taking `NOT a.store_deprecated` out of `updatesAvailableSQL` makes it fail (checked: the filter matched the deprecated asset). |
| decision: Toony Skies note on the purchase-catalogue spec | passed | Dated subsection at the end of that spec's `## What implementation found`. No test warranted: it is a note. |
| build | passed | `just build`, exit 0 |
| review: stages 8…9 | passed | `### Review — stages 8…9`: no defects, 1 smell, fixed |
| real proof | passed | Stage 8: the two Postgres tests above run against the real database. Stage 9 has no dependency to reach. No other stage was built this session. |
| gate | passed | `just format`, then `just check`, exit 0, against a throwaway Postgres on 55432 (5432 was down): 19 Go packages ok, Vitest 209 passed in 39 files, Playwright 17 passed |

### Cleanup — 2026-10-05

| Item | Status | Notes |
| --- | --- | --- |
| acceptance: deprecated listing saved as `store: unity`, details say "Deprecated on the store" | met | as Results |
| acceptance: a lookup that isn't a refresh never replaces a value | changed | met for recording, Fetch metadata and queued lookups. Imports still rename an asset from the store title (`enrichFillBlanks`, unchanged by this branch), which the decision's "keeps today's rule" allows but this line does not |
| acceptance: fallback counted under `no_store_data` | met | not triggered live |
| acceptance: prod's 208 generic Unity assets refreshed | unproven | check: run `dmsn bulk refresh --store generic --source-host assetstore.unity.com` against prod and confirm each is `store: unity` with a release date or listed as a problem |
| acceptance: hundreds refreshed one at a time, no 429 | unproven | check: the same prod run, then the server log for 429 or pause lines |
| acceptance: 429 waited out, job finishes | met | fake store |
| acceptance: recording 429 purchases takes minutes | unproven | check: time a real `dmsn purchases record` over the full plan |
| acceptance: new recorded asset has a source record | met | |
| acceptance: dmsn shows progress | met | |
| acceptance: `dmsn assets --fields …store_deprecated` | met | |
| review coverage | passed | `### Review — stages 1…7` covers every built stage once; `### Review — cleanup` added, 1 MED and 4 LOW fixed, the smell fixed for the branch's code; the `.env.example` LOW is the operator's to add |
| test gate | passed | 33 tests added; none cut, none reworked |
| gate | passed | `just check`, exit 0, against a throwaway Postgres on 55432 (the shared one on 5432 was down): 19 Go packages ok, vitest 209 passed in 39 files, Playwright 17 passed |
| outcome | handed back | the operator answered both open decisions, adding stages 8 and 9 |

### Cleanup — 2026-10-06

| Item | Status | Notes |
| --- | --- | --- |
| acceptance: deprecated listing saved as `store: unity`, details say "Deprecated on the store" | met | as Results |
| acceptance: a lookup that isn't a refresh never replaces a value | changed | as the 2026-10-05 cleanup: imports still rename from the store title, which the decision allows |
| acceptance: fallback counted under `no_store_data` | met | not triggered live; Fetch metadata and recording now also keep a store record on a fallback |
| acceptance: prod's 208 generic Unity assets refreshed | unproven | check: run `dmsn bulk refresh --store generic --source-host assetstore.unity.com` against prod and confirm each is `store: unity` with a release date or listed as a problem |
| acceptance: hundreds refreshed one at a time, no 429 | unproven | check: the same prod run, then the server log for 429 or pause lines |
| acceptance: 429 waited out, job finishes | met | fake store |
| acceptance: recording 429 purchases takes minutes | unproven | check: time a real `dmsn purchases record` over the full plan |
| acceptance: new recorded asset has a source record | met | |
| acceptance: dmsn shows progress | met | |
| acceptance: `dmsn assets --fields …store_deprecated` | met | |
| review coverage | passed | stages 1…7 and 8…9 each covered once; `### Review — cleanup 2026-10-06` added, 2 LOW and 1 smell, all fixed |
| triage | done | the `.env.example` LOW: fixed, the operator added the line |
| simplify | done | `boolPtr` and `optionalBool`'s pointer pairs replaced by `new(true)`/`new(false)` |
| test gate | passed | stage 8's `TestAssetFilters_StoreDeprecated` and the extended `TestUpdatesAvailableFilterAgreesWithTheBadge`, `TestAssets_FiltersMapToQueryParams`, `TestStoreLookups_FiltersSelectTheAssets`, and the new `TestMetadataFetch_GenericFallbackKeepsTheStoreRecord` all kept; none cut |
| gate | passed | `just build` and `just check`, exit 0, against a throwaway Postgres on 55432 (5432 was down): 19 Go packages ok, Vitest 209 passed in 39 files, Playwright 17 passed |
| board | not moved | DAM-7 stays open: three acceptance lines are unproven until the prod run |
| outcome | retired | no acceptance line unmet, no HIGH, every stage built, nothing handed back |
