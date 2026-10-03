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
