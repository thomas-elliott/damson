# Richer asset metadata fields from storefronts

Date: 2026-07-31
Status: signed off (lavish alignment session, 2026-07-31)

Extends `2026-07-13-storefront-metadata-enrichment.md` and
`2026-07-24-versioning-and-storefront-metadata-refinements.md`. Same subsystem,
wider field set.

## Problem

Enrichment fetches a storefront listing, normalises it into
`metadata.Metadata`, and then promotes only **five** of those fields to asset
state: name, description, licence, thumbnail, tags (plus the gallery images).

`Publisher`, `Price` and `Currency` are already parsed today and then written
into `assets.source_metadata_json` and never surfaced again — they appear
nowhere in `assetDetailView`, nowhere in the detail panel, and can't be edited,
filtered or searched. `applyMetadata` (`internal/handlers/api/metadata.go`)
writes only `name`, `description` and `source_url`; the `assets` table has no
publisher, date, version or compatibility columns at all.

The stores expose considerably more than that: who made the asset, when it was
released, when it was last updated, which engine versions it supports, how big
the download is, what it costs, how it's rated. We want those as real asset
metadata — hand-correctable and, where it earns it, filterable — and we want
the store-specific long tail (asset count, vertex count, platforms, "made
with") captured and displayed rather than silently dropped.

### Store field availability (verified live, 2026-07-31)

Queried from a dev machine against Unity Coveo, a Unity listing page, the
Sketchfab Data API and an itch.io asset page. This table is the ceiling on the
field set — anything not here can't be filled by enrichment.

| Field | Unity Coveo | Unity listing payload | Sketchfab API | itch.io page | Generic OG/JSON-LD |
| --- | --- | --- | --- | --- | --- |
| Developer / publisher | `publisher_name`, `publisher_id` | `ProductPublisher.name`, `.url`, `.supportUrl`, `.supportEmail` | `user.displayName` | Author row | `og:site_name`, brand, seller |
| Release date | `first_published_at` (epoch ms) | `firstPublishedDate` (ISO 8601) | `publishedAt`, `createdAt` | Published row (when set) | `datePublished` (rare) |
| Last updated | `last_published_at` | `ProductVersion.publishedDate` | — | Updated row | — |
| Store version | — | `ProductVersion.name` (e.g. `"1.24"`) | — | — | — |
| Compatible versions | `min_unity_version` (e.g. `202003`) | `supportedUnityVersions` (e.g. `["2020.3.0"]`) | — | Made with, Platforms rows | — |
| Price / currency | `ec_price` | `originalPrice{finalPrice, currency, isFree, discount, entitlementType}` | — | JSON-LD offers | JSON-LD offers |
| Rating | `ec_rating`, `ec_rating_count` | `reviewCount` | `likeCount`, `viewCount` | Rating row | `aggregateRating` |
| Category | `ec_category_level1..3` | `Category` entity | `categories[]` | Category, Genre rows | — |
| Tags | `ec_keywords` | `ProductTag.name` | `tags[]` | Tags row | `keywords` |
| Licence detail | — | `customLicense`, `licenseText`, `entitlementType` | `license{}` | Asset license row | JSON-LD `license` |
| Size / counts | — | `downloadSize`, `assetCount` | `vertexCount`, `faceCount`, `animationCount`, `archives` | File size row | — |
| Package type | — | `packageType` | — | — | — |
| Changelog | — | `publishNotes` (HTML) | — | — | — |
| Store id / slug | `ec_product_id`, `product_slug` | `id`, `slug`, `productId` | `uid` | — | — |

Fab (Epic) stays URL-only — still Cloudflare + hCaptcha, unchanged from the
July finding.

### Two live findings that shape the plan

**1. A much richer Unity source is already being downloaded.** The 2026-07-13
spec concluded Unity package pages are fully client-rendered with nothing to
scrape, and accepted losing per-asset licence fidelity as a result. That is
only half true. The page ships its entire normalised entity store inline as the
argument to `__component__.ReactDOMrender({…})`, reachable at `data.ENTITY`:

- `Product` — `firstPublishedDate`, `supportedUnityVersions`, `downloadSize`,
  `assetCount`, `packageType`, `publishNotes`, `originalPrice{finalPrice,
  currency, isFree, discount, entitlementType}`, `customLicense`,
  `licenseText`, `reviewCount`, `images`
- `ProductVersion` — `name` (the store version, e.g. `"1.24"`), `publishedDate`
- `ProductPublisher` — `name`, `url`, `supportUrl`, `supportEmail`
- `ProductTag` — `name` (the real tag labels)
- `Category`

`unityListingGallery` already downloads this exact page and scans it only for
the first `"images":` array, discarding the rest. Parsing the payload properly
adds **no** request volume, and it recovers the `entitlementType` /
`customLicense` / `licenseText` licence fidelity the July spec wrote off as
unobtainable.

**2. Two confirmed defects in today's Unity tag capture.**

- `coveoFields` requests `keywords`; the live index field is `ec_keywords`. So
  Unity keyword tags (`Landscape`, `Terrain`, `Grass`, `lowpoly`, …) have never
  been imported — only `ec_category` lands.
- `ec_category` arrives as `["3d", " 3d > environments", " 3d > environments >
  landscapes"]`, so an enriched Unity asset gets three tags including a leading
  space and literal `>` separators. Damson already has hierarchical tag paths;
  this should be the single path `3d/environments/landscapes`, derived from
  `ec_category_level3`.

## Chosen approach

**Option C — curated typed columns plus a spec sheet.** A small set of
first-class, nullable, hand-editable columns on `assets` for the cross-store
facts worth querying, **plus** an ordered `Attributes` list on the fetched
metadata for everything else, rendered read-only in the detail panel.

It beat the alternatives because each of them fails one half of the ask:

- **A, typed columns only** — everything queryable, but the store-specific long
  tail (asset count, vertex count, platforms, "made with") gets dropped, which
  is precisely the behaviour we're trying to stop.
- **B, an `asset_attributes` bag only** — captures everything with zero schema
  churn, but nothing is typed, so "released after 2023" and "sort by rating"
  stay out of reach and cross-store key normalisation lands on every reader.

C is the only option that captures broadly *and* makes the important fields
first-class. Promoting an attribute to a column later is a contained change,
and no data is lost in the meantime.

### First-class columns on `assets` (migration 019)

All nullable, all hand-editable via `PATCH /api/assets/:id`, all subject to
fill-blank-only on enrichment.

| Column | Type | Why a column |
| --- | --- | --- |
| `publisher` | text | filter + search |
| `publisher_url` | text | link out |
| `store` | text | filter |
| `store_product_id` | text | re-fetch key |
| `released_at` | timestamptz null | filter + sort |
| `store_updated_at` | timestamptz null | staleness |
| `latest_store_version` | text | update-available detection |
| `engine` | text | filter |
| `min_engine_version` | text | filter |
| `price`, `price_currency` | text, text | display; volatile |
| `rating`, `rating_count` | numeric, int | sort |

`publisher` joins the `search_vector` at weight C so a developer name is
findable from the search box.

### Spec-sheet attributes (no column)

`Metadata.Attributes []Attribute{Key, Label, Value}` — an **ordered list, not a
map**, so the panel renders them in a stable, store-authored order. Persisted
inside the existing `source_metadata_json` envelope (`metadata.SourceRecord`);
no new column, no new table.

- **Unity** — supported versions (full list), download size, asset count,
  package type, entitlement type, changelog
- **Sketchfab** — vertex / face / animation counts, downloadable, view + like
  counts, available archive formats
- **itch.io** — status, genre, platforms, made with, file size, average
  session, inputs — every info-table row not already mapped to a typed field

Read-only in v1: attributes are always re-derivable from a re-fetch, so there
is nothing to lose and no edit/merge semantics to design.

### Detail-panel presentation

A new **Details** section in `AssetDetailPanel`, between Overview and Licence:
typed fields first (with the same Edit-button flip the Overview section already
uses), then a "From the store listing" spec sheet of the attributes. The store
name and — from stage 8 — an `update: <version>` pill sit in the section
header, matching the SPDX-pill-in-header pattern the Licence section already
uses.

## Task breakdown

Adapters first, behind the existing normalised type, so nothing user-visible
changes until the schema lands. Each stage is a coherent, independently
committable unit.

1. **Expand `metadata.Metadata` + add `Attribute`.** New typed fields
   (publisher URL, released/updated timestamps, latest store version, engine +
   min engine version, rating + rating count, category path) and the ordered
   `Attributes []Attribute{Key, Label, Value}` list. Type-only change — every
   adapter still compiles and returns what it does today.

2. **Unity listing-payload parser.** Parse `ReactDOMrender(…)` → `data.ENTITY`
   and map `Product` / `ProductVersion` / `ProductPublisher` / `ProductTag` /
   `Category`. Replaces the crude `"images":` scan in `unityListingGallery`
   with a real parse of the same download, keeping today's best-effort
   semantics: any parse failure yields Coveo-only metadata, never an error.
   Feeds `entitlementType` / `customLicense` / `licenseText` into the existing
   licence path (`buildLicenceFromMeta`). Save the real page as testdata and
   fixture-test the mapping.

3. **Coveo fixes + new field mapping.** `keywords` → `ec_keywords`;
   hierarchical category from `ec_category_level3` into one `a/b/c` tag path
   (via `tags.NormalisePath`) instead of three malformed flat tags; map
   `first_published_at` / `last_published_at` (epoch ms → time),
   `min_unity_version` (`202003` → `2020.3`), `ec_rating`, `ec_rating_count`,
   `publisher_id`, `product_slug`. Fixture-tested in `unity_test.go`.

4. **Sketchfab, itch.io and generic capture.** Sketchfab: `publishedAt` /
   `createdAt`, geometry and engagement counts, archive formats. itch:
   generalise the existing info-table walker (`itchLicence` / `itchTags`
   already do label→value) into a full row harvest — Author, Status, Genre,
   Platforms, Made with, File size, Published/Updated, Rating — typed where it
   maps, attributes otherwise. Generic: JSON-LD `datePublished`,
   `aggregateRating`, brand URL. Fixture-tested per adapter.

5. **Schema + model + read path.** Migration 019 (all columns nullable,
   `IF NOT EXISTS` throughout, `BEGIN; … COMMIT;`), matching GORM tags on
   `models.Asset`, `publisher` added to the `assets_search_vector_update()`
   trigger's vector **and** its `UPDATE OF` column list, with a
   `SET name = name` backfill in the same migration. New fields on
   `assetDetailView` and the TS `AssetDetailView`; `assetPatchRequest` extended
   so every one is hand-correctable, with the audit diff following
   `patchAsset`'s existing shape.

6. **Apply + enrich semantics.** Extend `applyMetadata`
   (`internal/handlers/api/metadata.go`) and `enrichFillBlanks`
   (`internal/services/imports/enrich.go`) to the new columns under the
   existing rules: fill blanks silently, return anything that would overwrite
   as a `metadataConflict`, audit the diff. Attributes persist in the
   `SourceRecord` envelope. The existing conflict UI picks up the new field
   names as-is.

7. **Frontend: Details section + filters.** New **Details** section in
   `AssetDetailPanel` (typed fields, edit-flip form, then the store spec
   sheet); `publisher`, `store` and released-date filters in
   `LibraryFilterSidebar` + `lib/libraryFilters.ts` + `assetListFilters`; a
   release-date sort on the grid. Hooks follow `useDataQueries.ts` /
   `useMutations.ts`.

8. **Update-available detection.** Compare `latest_store_version` against the
   asset's `asset_versions.store_version` rows using the existing
   `imports.ParseStoreVersion` / `imports.CompareStoreVersions`; badge the
   asset in the detail panel header and the grid card, and add an "updates
   available" filter. No background polling — the comparison uses whatever the
   last user-triggered fetch stored.

## Reuse

- **`metadata` package end to end** — SSRF-guarded `Client`, host→adapter
  `Registry`, `SourceRecord` envelope persisted to `Asset.SourceMetadataJSON`,
  `FetchImage`. This slice adds fields to the existing normalised type; no new
  service package.
- **`unityListingGallery` already downloads the listing page** whose payload
  stage 2 parses — the richer Unity data costs no additional request.
- **`applyMetadata`'s fill-blank / confirm-overwrite semantics** and its
  `metadataConflict` shape — the new columns slot into the existing loop and
  the existing conflict UI, unchanged.
- **`enrichFillBlanks`** — the auto-enrich counterpart, same pattern.
- **`tags.Attach` / `tags.NormalisePath`** — create-or-reuse-by-path plus
  idempotent linking; stage 3's hierarchical category path uses them as-is.
- **`imports.ParseStoreVersion` / `CompareStoreVersions`**
  (`internal/services/imports/version.go`) — the store-version comparator
  stage 8 needs already exists and is already the version-identity comparator.
- **`assets_search_vector_update()` trigger + `SET name = name` backfill
  idiom** (migrations 006/007) — stage 5's `publisher` addition follows the
  same shape, including the `UPDATE OF` column-list gotcha 007 documents.
- **`patchAsset` + `audit.Write`** — the editable-field path and same-
  transaction audit writer both extend rather than needing new machinery.
- **Detail-panel section patterns** — the Overview section's read/edit flip and
  the Licence section's collapsed-with-pill-in-header behaviour are the two
  patterns the Details section composes.
- Searched for an existing typed metadata surface beyond name/description: none
  exists. `Publisher`/`Price`/`Currency` are parsed but never promoted, and
  there is no attribute/key-value store on any entity — stages 1 and 5 are the
  first.

## Resolved questions

- **Approach:** C — curated typed columns plus an ordered attribute spec sheet.
  A (columns only) drops the long tail; B (bag only) can't be queried.
- **Which fields earn a column:** all seven groups proposed —
  publisher + publisher_url, released_at, store_updated_at, engine +
  min_engine_version, store + store_product_id, price + price_currency,
  rating + rating_count. Price is knowingly volatile and kept anyway, stamped
  with the existing `fetched_at` provenance.
- **Attributes are read-only in v1.** They're re-derivable from a re-fetch, so
  no edit or merge semantics are needed. Promoting one to a column later is a
  contained change.
- **Attributes are an ordered list, not a map** — stable store-authored render
  order.
- **Update-available detection is in scope** as stage 8: badge plus an "updates
  available" filter, driven by `latest_store_version` vs the asset's
  `store_version` rows.
- **The two Unity tag defects: fix forward only.** Correct the mapping in this
  slice (stage 3); **do not** retroactively clean up the malformed
  `" a > b"`-shaped tags on already-enriched assets. They stay until removed
  manually or overwritten by a re-fetch, keeping this slice additive.
- **No background re-fetching.** Price, rating and `latest_store_version` are
  snapshots that rot; refreshing them stays user-triggered, as today.
- **Unity payload fragility is accepted.** It's an undocumented internal
  shape. Every field is independently optional, a parse failure degrades to
  Coveo-only metadata, and enrichment already never fails an import.
- **Out of scope:** Fab scraping (unchanged), authenticated owned-library
  import, scheduled/background metadata refresh, an `asset_attributes` table.
