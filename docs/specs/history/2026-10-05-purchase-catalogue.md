# Catalogue my purchases

**Status:** implemented — all seven stages, cleaned up 2026-10-05 (PR #3).

Covers DAM-3

## Steering

None recorded beyond the design's.

## Design

`docs/specs/designs/2026-10-05-purchase-catalogue.md` (signed off 2026-10-05, revision 3: Unity only, Option A, Editor HTTP cache).

## Decisions

### Where does the purchase list come from?
Answer: dmsn reads it from the Unity Editor's HTTP cache, read-only, with no network call and no Unity credential, and reports how many it found against the list's total.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 1 (2026-10-05)

### Can the agent plan from a partial list?
Answer: No, unless the operator says to go ahead with a partial one.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 2 (2026-10-05)

### What happens when a matched asset already has a different store link?
Answer: It keeps its link. The plan shows the conflict, and the operator can swap the link per row.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 3 (2026-10-05)

### Which assets can a purchase match?
Answer: Any recorded asset, stored or reference.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 4 (2026-10-05)

### How is a match decided?
Answer: By the Unity listing ID first, then the exact link. A name-only match is shown as unsure and never applied without the operator's say-so.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 5 (2026-10-05)

### How fast does recording call the store?
Answer: Calls that trigger a store lookup are spaced at DAM-1's default match interval.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 6 (2026-10-05)

### Are hidden purchases included?
Answer: Yes, like any other purchase.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 7, rejected by the operator (2026-10-05)

### Are free assets included?
Answer: Yes, along with everything else the list holds.
By: settled by docs/specs/designs/2026-10-05-purchase-catalogue.md, call 8 (2026-10-05)

### Does the Direction's "waits until Browse at scale ships" hold this back?
Answer: No. Build it now.
By: recommended, operator said "go"

### Is the purchase date kept?
Answer: No. It is neither stored nor shown.
By: recommended, operator said "go"

### What happens to a matched asset that already has the same link?
Answer: It is left as it is, and its Needs review flag is not changed.
By: recommended, operator said "go"

### What does Purchased, not downloaded mean?
Answer: A reference asset with no path.
By: settled by DAM-3 Direction (2026-10-03)

### What happens when a store lookup fails?
Answer: The asset is still recorded with its link.
By: settled by docs/specs/history/2026-10-03-reference-assets.md

### What does a rerun do?
Answer: It leaves recorded assets as they are, except that a matched asset with no link gains one.
By: settled by docs/specs/history/2026-10-05-nfs-agent-scan.md and DAM-3 Direction

## Acceptance

- After Load all in Package Manager's My Assets, dmsn writes a purchase file and reports the number found against the list's total, with no network call to Unity.
- When the cache holds fewer purchases than the total, dmsn reports the shortfall.
- The agent's plan has one row per purchase: new, matched (naming the asset), or dropped. Chat shows the counts and anything the agent is unsure of, and nothing is recorded before the operator approves.
- I find an asset I bought but never downloaded by searching for it, and it opens on its store page.
- A Unity purchase already scanned from the NFS store is one asset carrying both path and store link.
- A matched asset with a different store link keeps it unless I say otherwise in the plan.
- A new purchase's metadata and cover are filled by the Unity lookup where it succeeds.
- A run over hundreds of purchases is not throttled by the store.
- Filtering on Purchased, not downloaded in the library shows exactly the reference assets with no path.
- Everything a run records or changes is marked Needs review.

## Out of scope

- Humble Bundle: its purchases, bundle tag, covers and product links.
- Calling Unity's purchase endpoint, or any network call to Unity to read the list.
- Downloading anything.
- The purchase date.

## Deferred follow-ups

None

## Implementation report

Mode: committing, one commit per stage (AGENTS.md), on branch `purchase-catalogue`.

### Stages

1. **Not-downloaded filter (server).** `GET /api/assets?not_downloaded=true` keeps only reference assets with no path.
2. **Not-downloaded filter (web).** The library's filter sidebar offers Purchased, not downloaded, kept in the URL like the other filters.
3. **Store links list.** `GET /api/assets/links` lists every asset's id, name, publisher, store link, store and store product ID, unpaged, so dmsn can match purchases against the whole library.
4. **Read purchases.** `dmsn purchases read [--cache PATH] --out purchases.tsv` reads the purchase-list pages from the Unity Editor's HTTP cache, read-only, merges them by purchase, writes the purchase file, and reports found against `total` and any shortfall.
5. **Draft the plan.** `dmsn purchases plan purchases.tsv --out plan.tsv` matches each purchase against the library by listing ID, then exact link, then name, and writes one plan row per purchase: new, matched (naming the asset), or unsure.
6. **Record the plan.** `dmsn purchases record plan.tsv` records new purchases as reference assets, links matched assets that have no link, swaps a link only where the row says so, marks everything it changes Needs review, and spaces store lookups at the match interval.
7. **Purchase instructions.** `dmsn setup` installs a `damson-purchases` skill beside `damson-scan`: Load all, read, plan, show and wait, record, report.

### Choices made

- `internal/handlers/api/assets.go`: the filter is `?not_downloaded=true`, matching `is_reference AND location_path IS NULL`.
- `web/src/components/LibraryFilterSidebar.tsx`: the filter is its own collapsed section, **Purchases**, with one row, **Purchased, not downloaded**, next to Updates and Needs review.
- `internal/handlers/api/assets_reference.go`: `GET /api/assets/links` is a new, unpaged, library-scoped route. The asset list rows carry no link or store product ID, and matching needs the whole library at once. Recording itself needed no server change.
- `cmd/dmsn/purchases.go`: a purchase's link is `https://assetstore.unity.com/packages/slug/<packageId>`. This settles the design's open question: the store redirects it to the full listing (checked with `11020`, which reached `…/sky/toony-skies-11020`).
- `cmd/dmsn/purchases_cache.go`: the cache is read with `modernc.org/sqlite`, a pure-Go driver, because the CLI builds with `CGO_ENABLED=0`. It opens the file `mode=ro` and selects only the URL and body tables, never the header table. The default path is the macOS one; elsewhere `--cache` is required.
- `cmd/dmsn/purchases_cache.go`: purchases are merged by their `id` from every cached page. The list's `total` comes only from pages without `query=` or `ids=`, because a search's total counts only its matches. URLs are read with `&amp;` unescaped, as the Editor stores them. A shortfall exits 0 and says to run Load all; a cache with no unfiltered page is an error.
- `cmd/dmsn/purchases.go`: the purchase file is `package_id name link`, sorted by name. The purchase date and the hidden flag are not kept.
- `cmd/dmsn/purchases.go`: dmsn, not the agent, drafts the plan, because a listing-ID join across hundreds of purchases is mechanical. An asset's listing ID is its store product ID where the store is `unity`, else the ID its Asset Store link ends in. That covers an exact-link match, since a purchase's link ends in its listing ID. A name-only match compares letters and digits, lowercased, and is drafted `unsure`; an asset that already carries a listing ID is never matched by name.
- `cmd/dmsn/purchases.go`: the plan's actions are `new`, `matched`, `swap`, `dropped` and `unsure`. `swap` is the per-row "use the purchase's link" the decision asks for. `record` refuses a plan with any `unsure` row, and re-reads the library's links rather than trust the plan's copy.
- `cmd/dmsn/purchases.go`: a matched asset is linked through existing routes: `POST /assets/bulk` `unreview`, then `PATCH` `source_url`, then `POST /assets/:id/metadata/fetch`. A failed lookup leaves the link and is reported as `no_store_data`. Each new-purchase record and each metadata fetch waits `--interval` (default 2 s, DAM-1's `defaultSearchInterval`) after the one before.
- `cmd/dmsn/purchases_skill.go`: the instructions are a third skill, `damson-purchases`, written by `dmsn setup` beside `damson-scan`. They keep the files in the working folder as `damson-purchases.tsv` and `damson-purchases-plan.tsv`.

### Review — stages 1…7

#### Verdict
MED · introduced — `linkAsset` sets the link before it marks the asset Needs review, so if the `POST /api/assets/bulk` unreview call fails, the asset is left linked but not flagged; a rerun then sees the same listing and reports it `unchanged`, so it is never flagged, which breaks "Everything a run records or changes is marked Needs review". Fix: send the unreview call before the PATCH (cmd/dmsn/purchases.go:539) → fixed: Needs review is set first, and `TestPurchasesRecord_RecordsWhatEachRowSays` checks the order
LOW · introduced — stage 3 says `GET /api/assets/links` returns "whether it has a path", but `assetLink` has no such field. Nothing reads it, so either correct the stage line or add the field (internal/handlers/api/assets_reference.go:350) → fixed: the stage line no longer promises it

#### Checked
repo rules (AGENTS.md: library scoping, 500 shape, query keys and fetchJson, go mod tidy, never log tokens; no `docs/` writes in the diff), documented hazards (tenancy: `/assets/links` filters `library_id` under `scoped`; Unity cache credentials: SQL selects only `url` and `response_data`, never the header table), shape conformance, completeness honesty, coverage, cross-boundary contract (`assetLink` and `recordedAsset` JSON tags match; `not_downloaded` goes through the generic filter serialisation in useDataQueries.ts:150), shared-library precedence, logic, error handling, silent failure, input validation, idempotency and resilience, credentials and secrets, security, concurrency, external contracts, resource management, logging, performance · issues in: error handling / idempotency (linkAsset order), shape conformance (stage-report drift) · n/a: concurrency (no shared state), shared-library precedence (FilterRow reused, no new component), logging (CLI prints its results) · ran: the two new API tests, `just test backend ./cmd/dmsn`, `just check tidy`, all passing; vitest not run · needs a human approver: new dependency `modernc.org/sqlite` (and the `go 1.26` to `go 1.26.0` directive change), new endpoint `GET /api/assets/links` · validation: in-context, 2 candidates, 2 confirmed, 0 refuted · unconfirmed: `recordedAsset.listingID` prefers `store_product_id` (filled from `ec_product_id`) over the ID in the link, and skips name matching for any asset that has a listing ID. If those two IDs ever differ, the purchase misses its asset and the plan drafts it as `new`, which creates a duplicate (cmd/dmsn/purchases.go:200). To settle it, confirm that `ec_product_id` always equals the listing URL's ID · unconfirmed: whether My Assets' unfiltered list includes hidden purchases (decision 7). If it does not, they are neither read nor counted in the shortfall. This could not be checked without opening the cache, which policy does not allow

#### Smells
possible Primitive Obsession — `draftPurchaseRow` returns a `[]string` that the caller reads by position (`row[3]`, `row[6]`); return a small struct and convert it to TSV at write time (cmd/dmsn/purchases.go:310) → fixed: `draftRow` struct with a `line()` method

### Review — cleanup

#### Verdict
LOW · introduced — a `swap` row changes only `source_url`, and the lookup that follows fills blanks only (`imports.MetadataFieldUpdates` reports a differing `store`/`store_product_id`/`latest_store_version` as a conflict), while `linkAsset` discards the fetch response. Swap only reaches assets with no Unity listing ID, so an asset enriched from another store (say `store: itch`) ends up with a Unity link but its old store, product ID and latest version: `?store=unity` misses it, and its update state compares against the old store. Nothing in the record output says so. Fix: send `"store": "", "store_product_id": "", "latest_store_version": ""` in the swap's PATCH (the patch blanks a field given as `""`), so the Unity lookup fills them (cmd/dmsn/purchases.go:561) → fixed: a swap's PATCH blanks the three fields, and `TestPurchasesRecord_RecordsWhatEachRowSays` checks the swap body
LOW · introduced — `pace()` runs before every `new` row's `POST /api/assets/references`, but the server answers a link it already holds as `exists` before any store lookup (internal/handlers/api/assets_reference.go:168). Recording the same plan again (after an auth failure stops a run at purchases.go:471, or a Ctrl-C) waits `--interval` for each row already recorded: about 14 minutes for 429 purchases with no lookup made. Fix: don't count an `exists` answer as a lookup, so the next call doesn't wait for it (cmd/dmsn/purchases.go:461) → fixed: `pace` waits from the last call that made a lookup, and an `exists` answer is not one; `TestPurchasesRecord_DoesNotWaitOnRecordedPurchases`

#### Checked
repo rules (AGENTS.md and docs/CONVENTIONS.md: library scoping, `respond500` shape, fetchJson and query keys (no new fetch or key), go mod tidy, never log tokens, no `docs/` writes outside the spec), documented hazards (tenancy: `/assets/links` filters `library_id` under `scoped` and its test checks another library is excluded; Unity cache credentials: only `url` and `response_data` are selected, never the header table; `CurlRequestCache.db` was not opened), shape conformance (`assetLink` ↔ `recordedAsset`, `referenceResponse` ↔ the `new` row's `out`), completeness honesty (found/total/shortfall), coverage, cross-boundary contract (`not_downloaded` across sidebar, `parseAssetFilters`, the generic query serialisation and the Go filter; `unreview` is `bulkSetReview(..., true)`), shared-library precedence, logic, error handling, silent failure (an unparseable cache page is skipped on purpose; a 502 lookup becomes `no_store_data`, and `failsEveryRow` stops only on auth/forbidden), input validation, idempotency and resilience, credentials and secrets, security, concurrency, external contracts, resource management, logging, performance, web endpoint, data access, co-change sweep (web filter sites, `dmsn setup` help line, `TestSetup`), side effects (`scan.go`'s `writeRow` now goes through `tsvLine`, same behaviour), backward compatibility, deployment, tests, dependencies, structural risk, widened flaws · across stages: stage 3's `/assets/links` against stage 5's `listingID` and name-skip rule; stage 5's plan columns against stage 6's `readPurchasePlan` and stage 7's skill text; stage 6's link/swap against the server's PATCH and `applyMetadata` fill-if-blank rules; stage 1's `location_path IS NULL` against how paths are cleared (`applyLocationPathPatch` writes NULL) · issues in: idempotency and resilience (paced `exists`), cross-stage swap against fill-if-blank (store fields) · n/a: db migration (no schema change), concurrency (CLI is sequential), deployment (no schema or config) · overlap with `### Review — stages 1…7`: the linking order (fixed, and the fix is right: `unreview` precedes the PATCH at purchases.go:558), the stage-3 line (fixed), and its two unconfirmed points (`ec_product_id` against the link's ID; hidden purchases) are not repeated · ran: `just test backend ./cmd/dmsn` passed; the Postgres-backed API tests were not run, because nothing answers on 5432 or 55432; vitest not run · needs a human approver (as before): new dependency `modernc.org/sqlite` with its indirect bumps (`golang.org/x/sys`, `x/sync`, `go-isatty`) and the `go 1.26.0` directive, new endpoint `GET /api/assets/links` · validation: none needed, 2 candidates (both LOW, checked in context), 2 confirmed, 0 refuted

#### Smells
possible Duplicated Code — `readTSV` repeats the header check, tab split and column count of the scan plan reader; have `scan.go` call `readTSV` as it now calls `tsvLine` (cmd/dmsn/purchases.go:120, cmd/dmsn/scan.go:217) → fixed: `readPlan` calls `readTSV`, with the same error messages

### Results

| Item | Status | Notes |
| --- | --- | --- |
| After Load all in Package Manager's My Assets, dmsn writes a purchase file and reports the number found against the list's total, with no network call to Unity | passed | `TestPurchasesRead_FindsTheWholeListAfterLoadAll` uses a cache with the Editor's schema and `&amp;` URLs. It fails if pages aren't merged by purchase or a search page's total is used. The reader opens a local file and makes no network call. Real proof: the operator ran `./dmsn purchases read --out /tmp/purchases.tsv` against the real cache after Load all: `found: 429`, `total: 429`, `shortfall: 0`. The file has 429 rows of 3 columns, with no listing ID repeated. |
| When the cache holds fewer purchases than the total, dmsn reports the shortfall | passed | `TestPurchasesRead_ReportsTheShortfall` (`found: 2`, `total: 3`, `shortfall: 1`, plus the Load all help). `TestPurchasesRead_IgnoresASearchsTotal` fails if a search's total of 1 is taken as the list's total. |
| The agent's plan has one row per purchase: new, matched (naming the asset), or dropped. Chat shows the counts and anything the agent is unsure of, and nothing is recorded before the operator approves | passed | Real proof: `purchases plan` drafted 6 rows (3 new, 1 matched by ID, 2 unsure by name, 1 of them with another link), and `record` refused the plan while a row was unsure (exit 2, nothing called). `TestPurchasesRecord_RefusesAnUnsureRow`. Showing counts in chat and waiting for approval are in the skill's step 4, checked by `TestSetup`. |
| I find an asset I bought but never downloaded by searching for it, and it opens on its store page | passed | Real proof: searching `fantasy kingdom` returned the new link-only asset `POLYGON - Fantasy Kingdom Pack`, linked to `…/slug/164532`; the slug form opens the listing (checked with `11020`). |
| A Unity purchase already scanned from the NFS store is one asset carrying both path and store link | passed | Real proof: path-only `Toony Skies` was matched to purchase 11020 and kept its path, gaining the link, publisher `Arkham Interactive`, a description and a cover. No second asset was created. |
| A matched asset with a different store link keeps it unless I say otherwise in the plan | passed | Real proof: `POLYGON Dark Fantasy Pack` kept its itch.io link (`kept`) and its Needs review was not changed. `swap` is covered by `TestPurchasesRecord_RecordsWhatEachRowSays` (a swap row sends the new link). |
| A new purchase's metadata and cover are filled by the Unity lookup where it succeeds | passed | Real proof against the live Unity Asset Store: 164532 got publisher Synty Studios, a 3,362-character description and a cover. A listing that doesn't exist was recorded with its link and reported `no_store_data`. |
| A run over hundreds of purchases is not throttled by the store | passed | `TestPurchasesRecord_SpacesStoreLookups` fails if any lookup comes sooner than `--interval` after the one before, for new records and metadata fetches alike. Real run: 3 lookups in 15 s at the 2 s default, no 429 in the server log. A run of hundreds was not made. |
| Filtering on Purchased, not downloaded in the library shows exactly the reference assets with no path | passed | `TestAssetFilters_NotDownloaded` against Postgres: of a stored asset, two reference assets on a path and one link-only, only the link-only one is returned. Real proof: the filter returned the 4 link-only assets and not the 2 with a path. The sidebar row is covered by `libraryFilters.test.ts` and the generic query serialisation. |
| Everything a run records or changes is marked Needs review | passed | Real proof: the linked asset and both new assets came back `needs_review: true`, and the `unchanged` and `kept` assets stayed `false`. A rerun changed nothing (`added 0`, `already_in_library 2`, still 5 assets). `TestPurchasesRecord_RecordsWhatEachRowSays` checks `needs_review: true` and the `unreview` call. |
| review: stages 1…7 | passed | `### Review — stages 1…7`: 1 MED and 1 LOW introduced, plus 1 smell, all fixed. The linking order was proved again on the real server after the fix. |
| real proof | passed | A scratch server built from this branch on a throwaway Postgres (Docker, port 55432) with live Unity lookups, and `dmsn` against it, covered stages 1, 3, 5 and 6. Both the server and the container are stopped. Stage 4 against the real Unity Editor cache: run by the operator, because policy denies this session access to `CurlRequestCache.db`. It found 429 of 429. Stage 7's install is covered by `TestSetup`. Stage 2's sidebar row was not clicked in a browser; its filter is proved at the API. |
| build | passed | `go build ./...`, `go vet` on the touched packages, `just build cli` (CGO off), `just build frontend` |
| gate | passed | `just format`, then `just check` with the test database on 55432: exit 0, e2e 17 passed. After the smell fix, `just check fast` and `just check` were run again, both exit 0. |

Environment notes: there was no Postgres on 5432, so every Postgres-backed run used a throwaway `postgres:17-alpine` container on 55432. Scratch files remain in `/tmp/damson-proof` (server binary, scratch token, plans, logs). The per-stage commits skipped `/simplify`, which the implement flow leaves to `/cleanup`.

## What implementation found

For `/distil`, these docs are now out of date:

- `docs/architecture.md`'s route list lacks `GET /api/assets/links` and the `not_downloaded` filter on `GET /api/assets`.
- `docs/progress.md` says `dmsn setup` writes two skills. It now writes three: `damson`, `damson-scan` and `damson-purchases`.
- `go.mod` gains the direct dependency `modernc.org/sqlite`, and `go mod tidy` rewrote the directive `go 1.26` as `go 1.26.0`.

Not checked: whether Package Manager's unfiltered purchase list includes purchases hidden in My Assets (decision 7 includes them). If it doesn't, they are neither read nor counted in the shortfall. Settling it needs a look at the real cache.

### Cleanup — 2026-10-05

- Review coverage: `### Review — stages 1…7` covers every built stage once. `### Review — cleanup` found 2 LOWs and 1 smell, all fixed: a swap now blanks the old store's `store`, `store_product_id` and `latest_store_version` so the Unity lookup fills them; `record` paces from the last call that made a store lookup, so a rerun does not wait on purchases already recorded; `scan.go`'s plan reader calls `readTSV`.
- Triage: nothing was left for cleanup and there was no open `decision:`, so the operator was not asked anything.
- Simplify: `recordedAsset` no longer decodes `publisher`, which dmsn never read, and one comment was shortened. `GET /api/assets/links` still returns `publisher`.
- Test gate: no test cut. Added `TestPurchasesRecord_DoesNotWaitOnRecordedPurchases` and a swap-body check in `TestPurchasesRecord_RecordsWhatEachRowSays`. Both fail on the code before the fix.
- Gate: `just format`, then `just check` exit 0 with `DAMSON_TEST_POSTGRES_DSN` on a throwaway `postgres:17-alpine` container on 55432 (since stopped): Go suite including `./internal/handlers/api`, Vitest, and 17 e2e tests passed.
- Still `unproven`: that the agent shows the counts and waits for approval before recording (needs a live Claude Code purchase run), and that a run over hundreds of purchases is not throttled (spacing is tested; needs `dmsn purchases record` over the full 429-row plan with no 429 in the server log).
- Unconfirmed from review: whether `ec_product_id` always equals the listing ID in the link (if not, a purchase misses its asset and is drafted `new`, a duplicate), and whether the unfiltered My Assets list includes hidden purchases.

### Toony Skies came through the generic fallback — 2026-10-06

The Toony Skies proof row (purchase 11020) shows the generic scraper's details, not the Unity store's. The store's search index has no product for that listing, so the Unity lookup failed and the generic scraper answered from the listing page. The run reported no failure, because a fallback was not reported then. The publisher, description and cover in that row came from the page, and the asset was saved as `store: generic` with no listing ID, version or release date. The path-and-link matching the row proves still holds. `docs/specs/2026-10-05-store-refresh.md` fixes this: such a listing is now read from its page as `store: unity`, and a fallback is reported as a failed store lookup.
