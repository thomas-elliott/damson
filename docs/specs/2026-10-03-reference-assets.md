# Reference assets: record an asset by its location

Covers DAM-2

**Status:** signed off 2026-10-03

## Steering

- Allow having assets added that don't have their data imported, they are just the links to where they exist on the file system. — Initiative, 2026-10-03
- Turning a reference asset into a stored one (importing its data) is a follow-up — Task, 2026-10-03

## Design

Board Task DAM-2: its `## Direction` and `## Design documents` sections (the Add by location dialog; the detail panel's Location block and "Reference · not stored" badge).

## Decisions

### What is the name when no store title is available?
Answer: The last segment of the path, minus any file extension (`…/Synty/POLYGON_Fantasy_Kingdom.unitypackage` → `POLYGON_Fantasy_Kingdom`). It can be edited like any other name.
By: recommended, operator said "go" (2026-10-03)

### Is the path searchable?
Answer: Yes. Words in the path (folder and file names) match a library search, ranked below name and description.
By: recommended, operator said "go" (2026-10-03)

### When are two paths the same?
Answer: When they match exactly after trimming surrounding whitespace and one trailing slash. The comparison is case-sensitive.
By: recommended, operator said "go" (2026-10-03)

### What does recording a known path again do?
Answer: It updates that asset. A store link given now is set, replacing any earlier one. The store lookup fills only fields that are still blank, so nothing the user has edited is overwritten. The dialog and the CLI say the asset was updated, not added.
By: recommended, operator said "go" (2026-10-03)

### What if the store link is already in the library?
Answer: A record with a store link and no path, where that link matches the store link of any existing asset (stored or reference), creates nothing new. The dialog shows "Already in your library" with that asset, and the CLI names it.
By: recommended, operator said "go" (2026-10-03)

### What if the store lookup can't read the link?
Answer: The asset is still recorded with the link. Its name comes from the path, or else from the page's title. Only when neither gives a name does the dialog ask for one, and the CLI require one.
By: recommended, operator said "go" (2026-10-03)

### Which fields does the store link fill?
Answer: Everything the existing store lookup fills on an imported asset: name, description, cover, gallery, publisher, tags, licence, price, rating and release details.
By: recommended, operator said "go" (2026-10-03)

### Does a path with no link get a store match?
Answer: No. A path recorded without a link gets no store search or automatic match.
By: recommended, operator said "go" (2026-10-03)

### Can the location be changed later?
Answer: Yes. Path and store link can be edited or cleared from the detail panel's Location block and from `dmsn asset edit`, but a reference asset always keeps at least one of them. Changing the path to one already recorded is refused, and the refusal names the other asset.
By: recommended, operator said "go" (2026-10-03)

### Is a recorded asset marked Needs review?
Answer: Not when recorded by hand in the web app. The CLI can record an asset with Needs review set, for DAM-1's scan to use.
By: recommended, operator said "go" (2026-10-03)

### Who can record one?
Answer: Any library member, as with uploading.
By: settled by `internal/handlers/api/handler.go` (asset routes are member-level) (2026-10-03)

### Can an uploaded asset carry a path?
Answer: No. Only reference assets carry a path.
By: settled by DAM-2 Direction ("a reference asset … carries a location and no files") (2026-10-03)

### How does a reference asset look in the grid?
Answer: It shows a Reference mark in place of the size and file count. It never shows "0 B".
By: settled by DAM-2 Direction ("Marked as a reference asset in the grid") (2026-10-03)

### Can files be added to a reference asset?
Answer: No. Uploading into a reference asset, or adding a version to it, is not offered.
By: settled by DAM-2 Steering (conversion is a follow-up) (2026-10-03)

### What does deleting one do?
Answer: The same as any asset delete. Nothing at the path is touched.
By: settled by DAM-2 Direction ("Damson never reads the path") (2026-10-03)

### Which stores does the lookup know?
Answer: The existing ones: Unity Asset Store, Sketchfab and itch.io, plus the generic page scraper for any other link.
By: settled by `internal/services/metadata/registry.go` (2026-10-03)

## Acceptance

- I record a pack that is on the NFS store, then find it in the library by search, with its path shown.
- I find a path-only asset by searching for a word that appears only in its folder path.
- I record an asset with only a store link; its name, cover and description come from the listing.
- A path-only record is named after its last path segment, without the extension.
- Recording the same path twice leaves one asset, and the second time reports it updated. A name I edited in between is kept.
- Recording a store link already on an asset creates nothing, and I'm shown the existing asset.
- A store link the lookup can't read is still recorded with the link.
- A reference asset offers no file browser, versions, previews or upload.
- A reference asset shows a Reference mark in the grid and the "Reference · not stored" badge and Location block (path with Copy, store link with Open) in the detail panel.
- A reference asset can be tagged, put in a collection and pinned to a project.
- I can edit a reference asset's path and store link but can't clear both, and can't change its path to one already recorded.
- I can record by location from `dmsn`, with or without Needs review set.

## Out of scope

- Importing, extracting, thumbnailing or previewing the asset's data.
- Turning a reference asset into a stored one.
- Bulk scanning of the NFS store and its agent instructions (DAM-1).
- Recording purchases, and the Purchased-not-downloaded filter (DAM-3).
- A grid or sidebar filter for reference assets.
- Checking, opening or mounting the path from the Damson server.
- Store search or automatic matching for path-only records.

## Start here

- `docs/architecture.md`
- `internal/handlers/api/import_create.go`
- `internal/handlers/api/metadata.go`
- `web/src/components/detail/AssetDetailPanel.tsx`
- `cmd/dmsn/upload.go`

## Deferred follow-ups

None

## Implementation report

### Stages

1. **Schema.** Migration 024: `assets.is_reference` and `assets.location_path`, a check that only reference assets carry a path, a check that a reference asset keeps a path or a store link, a per-library unique path index, the path's words in the search vector at weight D, and a nullable `project_asset_pins.pinned_version_id`. Plus the model tags.
2. **Record endpoint.** `POST /api/assets/references` records by path and/or store link. It covers name derivation, re-record updates, a store link already in the library, the store lookup filling blanks, a link the lookup can't read, and `needs_review`.
3. **Reference rules on the existing surface.** The grid row and detail view expose `is_reference` and `location_path`. PATCH can edit and clear the path and link, keeps at least one of them, and refuses a path that is already recorded. Upload, chunked upload, merge and the version-match suggestion refuse or skip reference assets.
4. **Pinning without a version.** A reference asset pins to a project with no version, through pin create, the project view, the asset detail and merge.
5. **Web: grid and detail panel.** The Reference mark in the grid, the "Reference · not stored" badge, and the Location block (path with Copy, store link with Open, both editable). Files, versions, previews and upload are hidden. Reference pins render on the project page.
6. **Web: Add by location.** A dialog opened from a button next to Upload, with path and store-link fields and a store-match preview shown before adding. It handles the created, updated and already-in-library outcomes and asks for a name when one is needed.
7. **CLI.** `dmsn asset record`, with or without `--needs-review`; `--location-path` on `asset edit`; and the location shown by `dmsn asset <id>`.

### Choices made

Written at close rather than stage by stage, so read these as a reconstruction.

- `internal/handlers/api/assets_reference.go`: a new asset's name comes from the given name first, then a store title, then the path's last segment, then a page title. A title from the generic scraper (`store = generic`) counts as a page title, not a store title.
- `internal/handlers/api/assets_reference.go`: a file extension is dropped only when it contains a letter, so a folder like `Pack v1.2` keeps its version.
- `internal/handlers/api/assets_reference.go`: a given name applies only on create. Re-recording a known path never renames it.
- `internal/handlers/api/assets_reference.go`: re-recording with `needs_review: true` sets the flag. `false` never clears it.
- `internal/handlers/api/assets_reference.go`: "link already in the library" is an exact match on the cleaned `source_url` across every asset, stored or reference, and it is checked before the store lookup is spent.
- `internal/handlers/api/assets_reference.go`: a record that yields no name answers 422. `internal/axi/error.go` maps 422 to the usage code (exit 2).
- `internal/handlers/api/merge.go`: a merge with a reference asset on either side is refused with 409. Merging moves versions, and turning a reference asset into a stored one is out of scope.
- `internal/handlers/api/import_create.go`: an upload's version-match suggestion never offers a reference asset, since accepting it would be refused.
- `internal/handlers/api/projects.go`: a reference asset pins with `pinned_version_id` NULL, and a `version_id` given for one is refused with 400.
- `web/src/components/detail/AssetDetailPanel.tsx`: for a reference asset the Overview form drops its Source URL field (the link is edited in Location, which guards against clearing both), and Merge is hidden.
- `web/src/components/AddByLocationDialog.tsx`: the store link is looked up 500 ms after it stops changing. Changing the path or link drops an earlier "name required".

### Results

| Item | Status | Notes |
| --- | --- | --- |
| I record a pack that is on the NFS store, then find it in the library by search, with its path shown | passed | `TestRecordReference_PathOnlyIsNamedAfterLastSegmentAndSearchable` (real Postgres) records a path and finds it via `q=Synty`. The real-SPA drive showed the path in the Location block. Fails if 024 stops splitting the path into words. |
| I find a path-only asset by searching for a word that appears only in its folder path | passed | `TestSearch_TsvectorMatchesLocationPathWords` (`synty`, `nfs`, `POLYGON`). psql showed the unsplit path is one token, so the test fails without the `regexp_replace`. |
| I record an asset with only a store link; its name, cover and description come from the listing | passed | `TestRecordReference_LinkOnlyTakesNameCoverAndDescriptionFromListing` (fake lookup). Real proof against the live Unity Asset Store: named "POLYGON - Fantasy Kingdom Pack - Art by Synty", with publisher, description, cover, 8 gallery images and 16 tags. Fails if `applyMetadata` is dropped from the record path. |
| A path-only record is named after its last path segment, without the extension | passed | `TestNameFromLocationPath` and the path-only test (`POLYGON_Fantasy_Kingdom`). Fails if `fileExtension` stops stripping. |
| Recording the same path twice leaves one asset, and the second time reports it updated. A name I edited in between is kept | passed | `TestRecordReference_SamePathTwiceUpdatesAndKeepsEditedName`, plus the dialog test "says a known path was updated" and the CLI `TestAssetRecord_SendsLocationAndReportsTheOutcome`. Fails if the path lookup or the one-trailing-slash trim is removed. |
| Recording a store link already on an asset creates nothing, and I'm shown the existing asset | passed | `TestRecordReference_KnownStoreLinkCreatesNothing` (an imported asset holds the link; no lookup is spent) and the dialog test "Already in your library". The real SPA drive showed the same. |
| A store link the lookup can't read is still recorded with the link | passed | `TestRecordReference_UnreadableLinkIsStillRecorded` records with a given name and creates nothing on the 422. `TestRecordReference_SamePathTwice…` records a path with a failing lookup. Fails if a fetch error aborts the record. |
| A reference asset offers no file browser, versions, previews or upload | passed | `TestReferenceAsset_RefusesFilesAndMerge` (one-shot and chunked upload give 409, no version created). The `Reference` story asserts no Versions or Files sections and no Merge. The real SPA drive found 0 Versions sections. Fails if `refuseReferenceTarget` is removed. |
| Reference mark in the grid; "Reference · not stored" badge and Location block (path with Copy, store link with Open) | passed | `AssetGrid` story `WithReferenceAsset` (Chromium: Reference shown, no "0 B"), the `AssetDetailPanel` `Reference` story, and `LocationSection.test.tsx` (Copy writes the path; Open links to the store). The real SPA drive showed 5 cards marked Reference and 0 showing 0 B. |
| A reference asset can be tagged, put in a collection and pinned to a project | passed | `TestReferenceAsset_TagsCollectsAndPinsWithoutAVersion` (real Postgres; the pin shows `is_reference` with no version and no update). Fails while `pinned_version_id` is NOT NULL, or if the project query inner-joins the version. |
| I can edit a reference asset's path and store link but can't clear both, and can't change its path to one already recorded | passed | `TestReferenceAsset_PatchLocation` (409 names "Forest"; clearing both gives 400; a path on a stored asset gives 400) and `LocationSection.test.tsx` (Save disabled when both are blank). The real SPA drive showed the refusal naming `"POLYGON_Fantasy_Characters" (asset 2)`. |
| I can record by location from `dmsn`, with or without Needs review set | passed | `TestAssetRecord_*` and `TestAsset_DetailShowsAReferenceAssetsLocation`. Real proof: `dmsn asset record --location-path /mnt/nfs/Kenney/Platformer --needs-review` against the running server gave `outcome: added`, and a nameless link exited 2 pointing at `--name`. |
| review: stages 1…7 | passed | `### Review — stages 1…7`: 1 MED and 1 LOW introduced, both fixed, plus the smell and one unconfirmed item fixed. One unconfirmed race is left for cleanup (below). |
| real proof | passed | A scratch server on a throwaway embedded Postgres, with the live Unity store lookup, `dmsn` against it, and the built SPA driven by a scratch Playwright script. The server is stopped, and its database lives in the throwaway Postgres. |
| build | passed | `go build ./...`, `go vet ./...`, `just build frontend`, `just build cli`, all exit 0 |
| format / lint / tidy | passed | `just format`, then `just check fast` exit 0. The lint warnings are all in files predating the branch (`AssetGrid.tsx:50` is the existing `useVirtualizer` call). |
| Go suite | passed | `go test -count=1 ./...` against Postgres: 19 packages ok. Run without `-race`, because `just test backend` needs cgo and this machine has no C compiler. |
| web unit | passed | `vitest run --project=unit`: 39 files, 206 tests passed |
| stories | passed | `vitest run --project=storybook`: 24 files, 92 tests passed. Chromium came from `PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu24.04-x64` because Playwright doesn't support Ubuntu 26.04. `Editing Overview` failed an axe check on `.cm-scroller` twice earlier, once with this branch's changes stashed, and passed in the full run, so it is intermittent and not introduced here. |
| e2e | passed | `just test e2e` with the same override and DSN: 17 passed |

Environment notes: no local Postgres on 5432 and Docker is closed to this user, so every Postgres-backed run used a throwaway `fergusstrange/embedded-postgres` on 55432, started from the scratchpad (not a repo dependency).

### Review — stages 1…7

#### Verdict
MED · introduced — a 9.8 MB compiled `dmsn` binary is committed at `cmd/dmsn/dmsn` (stage 7, bdbbc79); `.gitignore` ignores only the root `/dmsn`, so a squash merge puts it in `main`'s history for good — `git rm --cached cmd/dmsn/dmsn` and widen the ignore to `dmsn` or `/cmd/dmsn/dmsn` (cmd/dmsn/dmsn:1) → fixed
LOW · introduced — the detail panel's Projects section renders `p.version_label` for each pin, which is `''` for a reference asset's version-less pin, so a pinned reference asset shows its project with a blank value; the project page says "Reference" for the same pin — show "Reference" when `asset.is_reference` (web/src/components/detail/AssetDetailPanel.tsx:371) → fixed

#### Checked
repo rules (AGENTS.md error shapes, migration shape BEGIN/COMMIT + IF NOT EXISTS, schema in both SQL and model tags, fetchJson-only API calls, no forked shared-ui components), documented hazards (library tenancy — every new query filters `library_id`: record lookup, link-exists check, path-conflict check, pin; trigger UPDATE OF list grown with the vector's inputs per 019), mechanical: shape conformance, completeness honesty, coverage, cross-boundary contract (is_reference / location_path / outcome / nullable pinned_version_id agree across Go handler, TS types and CLI structs), shared-library precedence (Dialog, FieldRow, Input, Pill, Section, Button all from `@/design-system`), logic, error handling, silent failure, input validation, idempotency & resilience, credentials & secrets, security, concurrency, external contracts, resource management, logging, performance · issues in: repo hygiene (committed binary), logic (blank pin label) · n/a: none · validation: in-context (quick depth, no agent), 2 candidates, 2 confirmed, 0 refuted · tests run: `go test ./internal/handlers/api ./cmd/dmsn` ok; vitest AddByLocationDialog + LocationSection 11/11 · no prior `### Review` sections to overlap · needs a human approver: schema change (migration 024 — new CHECK constraints, unique path index, `project_asset_pins.pinned_version_id` made nullable), public API change (`POST /api/assets/references`, `pinned_version_id` now nullable in project and asset-detail responses, new 422 status mapped to CodeUsage in `internal/axi`) · unconfirmed: two concurrent link-only records of the same new link both pass the "already in library" check and create two assets (no unique on `source_url`; the dialog disables Add while pending, so only a double CLI run reaches it) → left for cleanup: closing it needs a decision on whether `source_url` becomes unique per library, which would also constrain stored assets; after a 422 the dialog keeps requiring a name even if the path is then changed to one that yields a name (web/src/components/AddByLocationDialog.tsx:94) → fixed

#### Smells
possible Speculative Generality — `--needs-review` is registered on the whole `asset` command and silently ignored by every verb but `record`; reject it outside `record` (cmd/dmsn/mutate.go:100) → fixed
