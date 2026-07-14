# Improve storefront metadata & asset versioning

## Why

Dogfooding the storefront-enrichment feature surfaced a cluster of rough edges
that make imported assets harder to identify, browse, and track:

- **Search is fed raw filenames.** Auto-enrich and the "Find listing" picker
  search on `asset.Name`, which is the upload filename minus its extension —
  `Goblin_Warrior_AnimSet_v1.1`. Underscores, separators, and version suffixes
  are noise to a storefront search; the query the user actually wants is
  `Goblin Warrior AnimSet`.
- **No store preference.** `Registry.Search` merges every store's hits in
  goroutine-completion order, so the Unity Asset Store — where most of the
  library was actually purchased — has no priority in the candidate list or in
  the conservative auto-match.
- **Listings carry more than we keep.** Unity's Coveo record exposes a gallery
  of screenshots (`ec_images`); we take a single cover and drop the rest. Extra
  preview images materially help identify the right asset when browsing.
- **Enrichment never renames.** A confident store match fills description,
  licence, and thumbnail but leaves the asset named after its filename. The
  human-readable store title is right there.
- **Descriptions lose all formatting.** Unity descriptions are HTML; `stripTags`
  flattens them to a single unstructured blob, discarding paragraphs and lists.
- **The Files section dumps the manifest.** The detail panel's "Files" section
  lists up to 50 individual files inline — redundant now that the scoped file
  browser exists. It should summarise (count + total size) and offer the
  browse/rescan actions, not re-list.
- **Versions are count-labelled and uneditable.** Every version is `v1`, `v2`…
  assigned by counting existing rows. The store's real version (`v1.1`, parseable
  straight out of `Goblin_Warrior_AnimSet_v1.1`) is neither captured nor
  editable, so versions can't be compared against the store.

## What Changes

Two capability areas, sharing one import-time filename parser.

### Storefront metadata

- **Clean search terms from the filename.** A shared parser strips the
  extension, a trailing version token, and turns separators into spaces before
  the name is used as a search query — for both auto-enrich and the manual
  picker's default query. The import-time default asset name is cleaned the same
  way (`Goblin_Warrior_AnimSet_v1.1` → `Goblin Warrior AnimSet`).
- **Preferred store, default Unity.** A new per-library `preferred_store` setting
  (default `unity`) sorts the preferred store's candidates to the top of search
  results and biases the conservative auto-match toward the preferred store when
  multiple stores return an exact title match.
- **Gallery preview images.** The Unity adapter populates `ImageURLs` from the
  full `ec_images` set. On fetch/enrich, gallery images are downloaded, stored
  (capped, deduped by source URL), and surfaced in a gallery strip in the detail
  panel.
- **Rename from the store title.** A confident match renames the asset to the
  store title: unconditionally on auto-enrich (the name is still the import
  default at that point), and as a confirmable conflict field on manual fetch.
- **Preserve description formatting as Markdown.** Unity's HTML description is
  converted to Markdown rather than stripped to plain text. No new display is
  required now; the raw Markdown is stored and shown in the existing textarea.
- **Summarise the Files section.** The detail panel's Files section shows the
  current version's file count and total size plus the browse/rescan/regenerate
  actions, and no longer lists individual files.

### Asset versioning

- **Parse the store version from the filename.** The shared parser extracts a
  trailing `v1.1`-style token into a new `store_version` field on the version at
  import.
- **Editable store version.** A new `PATCH /assets/:id/versions/:vid` endpoint
  lets the store version (and notes) be edited after import, audited like other
  metadata edits.
- **Semantic version ordering.** Versions sort by their parsed `store_version`
  numerically (newest-highest), falling back to import date when a version has no
  parseable store version. The detail panel shows the store version alongside the
  internal label.

## Impact

- Affected specs: new `storefront-metadata` capability; new `asset-versioning`
  capability.
- Affected code:
  - **Backend — new:** shared `parseImportFilename` helper (clean name +
    store version); `preferred_store` library setting + migration; `asset_images`
    model + migration; gallery download/store on fetch+enrich; HTML→Markdown
    description conversion; `PATCH /assets/:id/versions/:vid`; semver comparator
    for version ordering.
  - **Backend — modified:** `metadata/search.go` (preferred-store ordering),
    `metadata/unity.go` (`ImageURLs` from `ec_images`, Markdown description),
    `imports/enrich.go` + `handlers/api/metadata.go` (rename field, gallery
    apply, preferred-store match), `assets.go` (`assetNameFromFilename` →
    `parseImportFilename`, set `store_version`), `assets_crud.go`
    (version + gallery in the detail view, semver sort), `files.go` (serve
    gallery images via `/thumbs`).
  - **Frontend — modified:** `AssetDetailPanel` (Files summary, gallery strip,
    version store-version display + edit), `MetadataActions` (rename conflict,
    cleaned default query), `useDataQueries`/`useMutations` (image list, version
    patch), settings UI for `preferred_store`.
- Non-goals: no reordering of which version is "current" (stays latest-imported);
  no rich Markdown *renderer* in the detail panel (raw Markdown for now); no
  gallery images for non-Unity stores beyond whatever `ImageURLs` a generic/JSON-LD
  listing already yields; no camelCase word-splitting of filenames
  (`AnimSet` stays `AnimSet`).

## Deferred comments

Candidate *why* notes parked during implementation (no explanatory comments were
written inline beyond idiomatic Go doc comments matching the codebase). `cleanup`
should evaluate each and keep only what the code can't convey:

- `internal/services/imports/filename.go` `ParseImportFilename` — the `isAllDigits`
  guard on the extension exists so a no-extension `Goblin…_v1.1` isn't stripped to
  `.1` by `filepath.Ext`; only real extensions are removed.
- `internal/services/imports/filename.go` `importVersionRE` — the token must be
  `v`-prefixed or dotted so a bare trailing integer (`Building_7`) stays in the name.
- `internal/services/imports/version.go` `CompareStoreVersions` — missing trailing
  components are treated as zero so `1.0 == 1`.
- `internal/handlers/api/assets_crud.go` `getAssetDetail` sort — semver ordering is
  done in Go (not SQL) because a portable component-wise compare isn't expressible
  in the query; `CurrentVersionID` is deliberately not reordered.
- `internal/handlers/api/metadata.go` `applyMetadata` — name is a conflict (never a
  fill) because an asset name is never blank; auto-enrich renames unconditionally
  since the name is still the import default at that point.
- `internal/handlers/api/files.go` `authoriseThumbKey` — the gallery lookup must keep
  the `assets.library_id` scope so gallery blobs stay tenant-isolated.
- `internal/services/imports/gallery.go` `StoreGalleryImages` — best-effort by design;
  failures are logged and skipped so enrichment/fetch never fails on a bad image.
