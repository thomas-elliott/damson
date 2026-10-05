# Refresh store details from the server

**Status:** signed off 2026-10-05.

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

- decision: whether to add a filter for deprecated assets, and whether Updates available should skip them — operator.
- decision: whether to correct the purchase-catalogue spec's proof row for Toony Skies, which went through the generic fallback, with a `## What implementation found` note — operator.

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
