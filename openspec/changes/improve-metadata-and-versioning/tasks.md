# Tasks — Storefront metadata & versioning improvements

## Phase 1 — Shared filename parser (foundation)

- [ ] 1.1 Add `parseImportFilename(filename) → (name, storeVersion)` (strip ext, extract `v`-prefixed/dotted trailing token, separators→spaces, collapse; no camelCase split). Unit tests: dotted `v1.1`, bare `v2`, no token, trailing integer `Building_7`, mixed separators.
- [ ] 1.2 Replace `assetNameFromFilename` (assets.go) with the new parser for the import-time default name; keep behaviour for names with no token.

## Phase 2 — Asset versioning

- [ ] 2.1 Add `AssetVersion.StoreVersion` column + migration (`ALTER TABLE asset_versions ADD COLUMN store_version text`).
- [ ] 2.2 Set `store_version` from `parseImportFilename` on the first version at upload (assets.go) and on subsequent version imports (imports.go / subset paths that create versions).
- [ ] 2.3 Add a dot-separated-integer comparator (`semver`-lite) with unit tests (`1.10 > 1.9 > 1.1`; unparseable → not comparable).
- [ ] 2.4 Order versions in `getAssetDetail` in Go after scan: parseable store versions descending, then unversioned by import date; `CurrentVersionID` unchanged. Include `store_version` in `assetVersionView`.
- [ ] 2.5 Add `PATCH /assets/:id/versions/:vid` (`{store_version?, notes?}`): library-scoped lookup, validation, partial update, `asset.version.update` audit entry. Handler + route + tests.
- [ ] 2.6 Frontend: `VersionItem` shows the store version and offers inline edit; `useMutations` gains `usePatchVersion`; `useDataQueries` version type gains `store_version`.

## Phase 3 — Search inputs & store preference

- [ ] 3.1 Feed the cleaned name (Phase 1) as the search query in `enrichAsset` and as the picker's default query (`MetadataActions`).
- [ ] 3.2 Add `LibrarySettings.PreferredStore` (default `unity`) + migration; expose in the settings API + settings UI.
- [ ] 3.3 Add `sortByPreferredStore(cands, preferred)` (stable, preserves within-store order); apply in `getMetadataSearch` and `enrichAsset`.
- [ ] 3.4 Bias `exactMatchCandidate` toward the preferred store when multiple stores match. Unit tests for ordering + match preference.

## Phase 4 — Richer listing data

- [ ] 4.1 Unity adapter: populate `Metadata.ImageURLs` from the full `ec_images` set (dedupe, exclude the cover). Adapter tests against a Coveo fixture.
- [ ] 4.2 HTML→Markdown: add the converter dependency (confirm via Context7) behind `htmlToMarkdown(s)` with `stripTags` fallback; use it for Unity `ec_description`. Tests: paragraphs + list preserved; fallback on error.
- [ ] 4.3 Add rename-from-title: unconditional on auto-enrich (`enrichFillBlanks`), conflict field on manual fetch/apply (`applyMetadata` + `ConflictDialog`). Tests for both paths.

## Phase 5 — Gallery images

- [ ] 5.1 Add `AssetImage` model + migration (`asset_images`, unique `(asset_id, source_url)`).
- [ ] 5.2 On fetch (`applyMetadata`) and auto-enrich: download up to `maxGalleryImages` via `FetchImage`, raster-normalise, write `asset_images` rows skipping existing `source_url`; non-fatal on failure. Tests.
- [ ] 5.3 Extend `getThumb` authorisation to also match `asset_images.object_key` within the library scope.
- [ ] 5.4 Include `images[]` in `getAssetDetail`; frontend gallery strip in `AssetDetailPanel` via `useBlobUrl("/api/thumbs/…")`.

## Phase 6 — Files section summary

- [ ] 6.1 Replace the Files-section file list in `AssetDetailPanel` with the current version's file count + total size; keep browse/rescan/regenerate actions.

## Cross-cutting

- [ ] C.1 Backend unit + handler tests for the parser, semver, version patch, preferred-store ordering, gallery persistence, rename, and Markdown conversion.
- [ ] C.2 Frontend tests/stories for the version edit affordance, gallery strip, and Files summary.
- [ ] C.3 Update `docs/architecture.md` (data model: `store_version`, `asset_images`, `preferred_store`; API: version PATCH) and `docs/CONVENTIONS.md` if handler patterns change.
- [ ] C.4 Run migrations against a scratch DB; verify a real import end-to-end (`Goblin_Warrior_AnimSet_v1.1`-style) parses name + version, enriches, and shows the gallery.
