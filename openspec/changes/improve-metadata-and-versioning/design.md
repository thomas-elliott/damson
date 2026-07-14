# Design — Storefront metadata & versioning improvements

## Context

Enrichment already exists (`internal/services/metadata`, `internal/services/imports/enrich.go`,
`internal/handlers/api/metadata.go`). This change refines the inputs (search
terms, store preference), widens the outputs (gallery images, Markdown
descriptions, asset rename), and adds a parsed, editable store version. The four
shaping decisions were confirmed with the user up front and are recorded as D2,
D3, D5, D6 below.

## Decisions

### D1 — One shared import filename parser
`assetNameFromFilename` (assets.go) is replaced by `parseImportFilename(filename)
→ (name string, storeVersion string)`, the single source of truth for both the
metadata search terms and the parsed version.

- Strip the extension.
- Match a trailing version token with `[._\s-]v?(\d+(?:\.\d+)*)\s*$` (case-
  insensitive); the captured `\d+(\.\d+)*` becomes `storeVersion` and the token
  is removed from the name.
- Replace `_`, `-`, and interior `.` separators with spaces; collapse runs of
  whitespace; trim.
- Preserve original casing and do **not** split camelCase — `AnimSet` stays
  `AnimSet` (per the user's example, `Goblin_Warrior_AnimSet_v1.1` →
  `Goblin Warrior AnimSet` + `1.1`).

The cleaned `name` becomes the import-time default asset name **and** the search
query used by auto-enrich and the picker's default. `storeVersion` is `""` when
no token is present (unit tests cover both).

### D2 — Version model: internal sequence + separate store version *(user choice)*
`AssetVersion.VersionLabel` (`v1`, `v2`… by count) stays as the immutable
internal sequence. A new nullable `AssetVersion.StoreVersion string` column holds
the store-facing version. On import the first version's `StoreVersion` is set
from `parseImportFilename`; later versions parse their own upload filename. The
internal label is never conflated with the store version.

- Migration: `ALTER TABLE asset_versions ADD COLUMN store_version text`.
- The existing `(asset_id, version_label)` unique index is untouched;
  `store_version` is free-form and non-unique (two uploads can legitimately share
  a store version).

### D3 — Semantic version ordering *(user choice)*
Versions are ordered newest-highest by `store_version`, parsed as dot-separated
integers and compared component-wise (`1.10 > 1.9 > 1.1`). A version with no
parseable `store_version` sorts by `import_date` as today. Because SQL can't
express the semver compare portably, the version list is ordered in Go after the
`getAssetDetail` scan:

1. Versions with a parseable store version, descending by numeric components.
2. Then versions without one, descending by `import_date` (current behaviour).

`CurrentVersionID` is **not** changed by this ordering — "current" remains the
latest-imported version (non-goal to change). The comparator lives in a small
`semver`-lite helper with unit tests.

### D4 — Editable store version
New `PATCH /assets/:id/versions/:vid` accepting `{ store_version?, notes? }`.
Mirrors `patchAsset`: library-scoped lookup, partial update, audit entry
(`asset.version.update`) with a before/after diff. `store_version` is validated
against the same `\d+(\.\d+)*` shape (empty string clears it). Frontend adds an
inline edit affordance on `VersionItem`.

### D5 — Preferred store, per-library setting, default Unity *(user choice)*
`LibrarySettings` gains `PreferredStore string` (default `"unity"`), exposed in
the settings API + UI. Ordering is applied by the **caller** of `Registry.Search`
(the handler and `enrichAsset`), not inside the registry, because the registry is
library-agnostic:

- `sortByPreferredStore(cands, preferred)` — stable sort that moves the preferred
  store's candidates to the front while preserving each store's internal
  relevance order.
- Auto-match: `exactMatchCandidate` prefers an exact-title match from the
  preferred store when more than one store matches, before falling back to the
  first exact match in merged order.

Default `"unity"` matches how the library is actually sourced; the setting keeps
it changeable.

### D6 — Gallery images: fetch, store, and display *(user choice)*
New `asset_images` table and a gallery strip in the detail panel.

- Model: `AssetImage { ID, AssetID (index), ObjectKey, ContentType, SourceURL,
  SortOrder, CreatedAt }`; unique on `(asset_id, source_url)` for idempotent
  re-fetch.
- Source: `UnityAdapter` populates `Metadata.ImageURLs` from the full `ec_images`
  array (currently only the cover is taken). The generic scraper keeps whatever
  `ImageURLs` it already yields.
- Store: on fetch (`applyMetadata`) and auto-enrich, up to `maxGalleryImages`
  (8) images are downloaded via the existing `FetchImage` (SSRF-guarded) and run
  through the raster thumbnail generator — the same decode/re-encode that already
  guards the cover path and rejects non-images. Rows are written skipping any
  `source_url` already present. The cover is not duplicated into the gallery.
- Serve: gallery blobs live in the `thumbs/` namespace and are served by the
  existing `GET /thumbs/:objectKey`, whose authorisation is extended to also
  match `asset_images.object_key` (today it only matches
  `assets.thumbnail_object_key`).
- Display: `getAssetDetail` returns `images: [{ id, object_key, content_type }]`
  ordered by `sort_order`. The detail panel renders a horizontal gallery strip
  using the existing authenticated-blob pattern (`useBlobUrl("/api/thumbs/…")`),
  the same mechanism `AssetGrid` already uses for the cover thumbnail.

### D7 — Rename asset from the store title
Renaming reuses the fill-vs-conflict machinery in `applyMetadata`:

- **Auto-enrich** (post-import, confident match): rename unconditionally. Safe
  because enrichment runs inside the import pipeline before the user can touch the
  name — the current name is still the parsed import default.
- **Manual fetch/apply**: `name` becomes a conflict field (the name is never
  empty, so it never "fills"); the user confirms the rename in the existing
  `ConflictDialog`. `title`/`name` is added to the conflict + audit paths.

### D8 — HTML → Markdown descriptions
`coveoMetadata` converts the HTML `ec_description` to Markdown instead of calling
`stripTags`. Introduce `github.com/JohannesKaufmann/html-to-markdown` (v2)
behind a small `htmlToMarkdown(s string) string` wrapper so the dependency is
swappable and the fallback (on converter error) is the current `stripTags`
output. The generic scraper's `og:description` is already plain text and is left
as-is. Stored Markdown renders acceptably as raw text in the existing textarea;
a Markdown *renderer* is a non-goal.

> Confirm the exact library + version against Context7 before wiring; the wrapper
> keeps the blast radius to one function if a different converter is preferred.

### D9 — Files section is a summary, not a list
`AssetDetailPanel`'s Files section drops the `<ul>` of individual files. It shows
the current version's `file_count` and total `size_bytes` (already on the
`assetVersionView` for the current version) and retains the existing actions:
"Browse all N files" (browsable only), "Rescan package", "Regenerate previews".
The scoped file browser is now the only place that lists individual files.

## Risks / open questions

- **Version-token false positives.** A name legitimately ending in a number
  (`Building 7`) would be misparsed as a version. Mitigation: require the `v`
  prefix *or* a dotted number (`1.1`) to treat a token as a version; a bare
  trailing integer with no `v` and no dot stays part of the name. Covered by
  unit tests; revisit if real filenames disagree.
- **Gallery fetch cost.** Up to 8 image downloads per confident enrich, best-
  effort and off the import's critical path (enrich is already best-effort and
  swallows failures). Cap + dedupe bound it; failures are logged, not fatal.
- **`/thumbs` authorisation widening.** Extending `getThumb` to a second table
  must keep the library scope check (`library_id`) so gallery blobs stay
  tenant-isolated.
- **Markdown dependency.** New third-party dep; the wrapper + `stripTags`
  fallback contain the risk. Verify licence compatibility.
- **Existing `v1` labels.** Historic versions keep `version_label = v1`; their
  `store_version` is null until edited. Ordering falls back to import date for
  them, so behaviour is unchanged for un-parsed libraries.
