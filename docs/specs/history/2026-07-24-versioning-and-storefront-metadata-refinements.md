# Versioning rework + storefront-metadata refinements

Date: 2026-07-24
**Status:** implemented — signed off 2026-07-24 (lavish alignment session), merged 2026-07-25
(PR #33, `dca99b2`).

One PR bundling four areas surfaced by dogfooding. Sizes vary from a
one-function fix to a versioning-model rework with a schema migration.

## Problem

Four issues from testing:

1. **Versioning tracks the wrong thing.** Versions behave like an internal
   edit counter (`v1 → v2 → v3`) when they should track the **external pack
   version** (e.g. `1.3`). If files are uploaded as `1.3`, that set should stay
   `1.3` forever; a new number should only ever come from uploading *another*
   pack. There's no way to remove a version, no way to add a version to an
   existing asset by uploading, and no way to merge two uploads that are really
   the same pack.

   *Reality check from the code:* metadata edits/reviews do **not** create
   versions today — `PATCH /assets/:id` and the metadata apply path never touch
   `asset_versions`. The spurious versions come from the count-based
   `NextVersionLabel` that `re-extract` / `subset` / `import retry` stamp. A
   version already carries two fields: `version_label` (`v1`/`v2`…) and
   `store_version` (`1.3`, parsed from the filename at first upload only). This
   rework collapses the identity onto `store_version`.

2. **"Find listing" match picker is too thin.** Candidate rows show only a
   40×40 cover, title, and `store · publisher · price` — not enough to choose.
   The search API's `candidateView` carries no description or images. Separately
   the Unity cover often renders broken because `coveoCover` prefers
   `ec_thumbnails`, where a **video** preview can land.

3. **Store gallery images should show in the sidebar.** (Largely already
   built — needs verification + polish; see Reuse.)

4. **Storefront tags aren't imported.** When fetched metadata carries
   tags/categories they should be attached to the asset as tags. No part of
   this exists yet.

## Chosen approach

Extend the existing storefront-metadata subsystem and rework the versioning
semantics in place — no new subsystems, mirroring current handler/pipeline
patterns. The versioning change makes `store_version` the version identity and
adds the missing lifecycle operations (delete, add-by-upload, merge); the
metadata changes are incremental additions to adapters, the fetch/apply paths,
and the detail panel.

It beat the alternatives on the two forks that were explicitly decided:

- **Version identity = store version** (over keeping the dual
  label/store-version concept). Chosen because the dual concept is exactly what
  confused the workflow; collapsing to one removes the spurious `v1/v2`.
- **Add/merge = both upload-time detect + manual merge** (over manual-only or
  detect-only). Manual-only loses the low-friction "this is a new version of an
  existing pack" path; detect-only has no recovery when detection is wrong or
  when two existing assets need folding together. Both together cover import
  and after-the-fact cleanup.

## Task breakdown

Ordered so small, isolated changes land first (fast review, de-risks the rest),
the versioning rework and its migration in the middle, frontend last. Each
stage is a coherent, independently-committable unit — roughly one commit each.

1. **Cover selection = first still image.** In `coveoCover`
   (`internal/services/metadata/unity.go`), prefer the first `ec_images` entry
   that is a real still image — filtering out video/stream URLs — before
   falling back to `ec_thumbnails`. Fold in the same fix for the **extracted
   `.unitypackage` preview**: today `UnityExtractor.Extract` takes the first
   `preview.png` in tar order regardless of which asset it belongs to; buffer
   candidate previews per-GUID and pick the first whose bundle `pathname` has an
   image extension (the extension list already exists in
   `thumbnail.go:classify`). Fixture-tested in `unity_test.go` and the
   extractor tests.

2. **`Metadata.Tags` + adapter capture.** Add `Tags []string` to
   `metadata.Metadata`; request the keyword/category field from Coveo (extend
   `coveoFields` and map it in `coveoMetadata`) and populate it from
   Sketchfab/itch where they expose categories. Fixture-tested per adapter.

3. **Attach tags on apply + auto-enrich.** Extract `postAssetTag`'s
   create-or-reuse-by-path + idempotent `AssetTag` link block (`tags.go`) into a
   shared helper; call it from `applyMetadata` (`metadata.go`, alongside the
   existing `StoreGalleryImages` call) and `enrichFillBlanks`
   (`imports/enrich.go`). Tags are additive (never removed by enrichment),
   normalised via `normaliseTagPath`, and audited like manual tagging.

4. **Gallery verify + backfill.** Confirm `StoreGalleryImages` stores
   `asset_images` rows end-to-end and that `image_urls` reach it (the stage-1
   video filter keeps the raster generator from choking on non-images). Provide
   a backfill for already-imported assets via a re-fetch. Mostly test +
   confirmation; no new model.

5. **Store version = identity; re-extract in place.** `re-extract` / `subset` /
   `retry` stop calling `NextVersionLabel`. Re-extract updates the source
   version's files **in place** rather than creating a new row. Detail view and
   version ordering key off `store_version` (via the existing
   `ParseStoreVersion` / `CompareStoreVersions`); the label field becomes
   editable and nullable. Migration relaxes the `(asset_id, version_label)`
   unique constraint (move uniqueness to `(asset_id, store_version)` where
   non-blank, allowing one blank/"unversioned" row semantics per the resolved
   question). Both the SQL migration and the GORM model tags change together.

6. **Delete a version.** New `DELETE /api/assets/:id/versions/:vid`. Repoint
   `current_version_id` to the next-highest remaining version by store-version
   order; **block with 409 if a project pins it** (unpin first); clean up files
   via the existing storage GC references; audit `asset.version.delete`.

7. **Upload → add as a version of an existing asset.** `POST /api/imports`
   accepts an optional target asset id; when set, the upload becomes a new
   version on that asset instead of a new asset. Store version comes from the
   filename (`ParseImportFilename`) or the request. Reuses the
   extract/enrich/preview pipeline unchanged.

8. **Upload-time match detection.** On upload, run the existing normalised-name
   comparison (`normaliseTitle`, shared from the enrich path) against the
   library's assets; when a confident match exists, return it so the UI can
   prompt "add as a version of &lt;asset&gt;?" — defaulting to a new asset.

9. **Merge assets.** New `POST /api/assets/:id/merge` folding a source asset's
   versions into a target, de-duplicating by store version, moving
   tags / collection / project references, then deleting the emptied source.
   Conflict resolution: **keep the target's** name / description / thumbnail /
   licence, but return what the source had so the UI can offer one-click
   adoption. Audited.

10. **Frontend.** Version list gains inline label edit + delete; the upload
    dialog surfaces the "add as a version of…" prompt on a detected match; an
    asset action opens a merge picker. "Find listing" candidate rows become
    **expandable** — on expand, lazy-fetch the full listing for that candidate
    (the existing `Fetch` already returns description + `image_urls`) and show
    description + gallery thumbnails. Gallery strip in `AssetDetailPanel` gains
    click-to-enlarge (lightbox). Hooks/queries follow `useMutations.ts` /
    `useDataQueries.ts`.

## Reuse

- **`store_version` + parsing already exist.** `AssetVersion.StoreVersion`
  (migration `014_store_version.sql`), `imports.ParseImportFilename`
  (peels `v3.4` / `4.0` from the filename), `ParseStoreVersion` /
  `CompareStoreVersions` (`internal/services/imports/version.go`), and the
  editable `PATCH /assets/:id/versions/:vid` (`patchAssetVersion`,
  accepts `store_version` + `notes`). The rework re-points identity onto fields
  that are already there.
- **Gallery is already built.** `asset_images` table + `models.AssetImage`,
  `imports.StoreGalleryImages` (downloads up to 8, dedupes by `source_url`,
  stores via the raster thumbnail generator), the `images []assetImageView`
  in `assetDetailView`, and `GalleryStrip`/`GalleryThumb` in
  `AssetDetailPanel.tsx`. Area 3 is verify + polish, not new build.
- **Tag attach logic exists.** `postAssetTag`'s transaction body
  (`tags.go`) — create-or-reuse tag by path (with ancestors) + idempotent
  `AssetTag` `FirstOrCreate` — is the block to extract into a shared helper;
  `normaliseTagPath` handles normalisation. The frontend Tags section and
  `useAddAssetTag`/`useRemoveAssetTag` already exist.
- **Metadata fetch/apply infrastructure exists.** `metadata.Registry`
  (SSRF-guarded client, per-host adapters, Coveo for Unity), `SourceRecord`
  persisted to `Asset.SourceMetadataJSON`, `applyMetadata`'s fill-blank /
  confirm-overwrite semantics, `FetchImage`. Stage 10's expand reuses the
  existing `Fetch` (description + `image_urls`) rather than adding search-side
  payload.
- **`normaliseTitle` / `exactMatchCandidate`** (`imports/enrich.go`) — the
  high-confidence name comparison, reused for upload-time match detection.
- **Extractor bundle assembly** (`extract/unitypackage.go`) already buffers a
  per-GUID `bundle` with `pathname`; the still-image preview selection hooks
  into that existing structure.
- **Audit** — `audit.Write` in the same transaction, used by all new
  mutations (delete/merge/tag-attach).
- Searched for an existing per-version delete endpoint, an "add version to
  existing asset" path, and any asset-merge code: **none exist** — stages 6, 7,
  9 are greenfield handlers wired into `handler.go`.

## Resolved questions

- **Version identity:** `store_version` is the identity; auto-incrementing
  `v1/v2` retired; re-extract updates the existing version in place; blank /
  "unversioned" allowed when no version is detectable.
- **Add / merge mechanism:** both upload-time match detection (prompt to add as
  a version of the matched asset) and a manual merge action.
- **Duplicate store versions on one asset:** block with a clear message and
  offer "replace the files of &lt;version&gt;" rather than silently creating a
  second row with the same number.
- **Merge conflict resolution:** keep the target asset's
  name / description / thumbnail / licence; surface the source's values for
  one-click adoption; de-duplicate versions by store version.
- **"Unversioned" display + sort:** label a blank-version row "unversioned",
  sort it last, keep it fully usable.
- **Candidate expand mechanism:** lazy per-candidate fetch on expand
  (store-agnostic, no bulk over-fetching) — not up-front enrichment of every
  search result.
- **Extractor preview fix:** folded into stage 1 (same "first still image"
  intent), not deferred.
- **Gallery presentation:** click-to-enlarge lightbox is sufficient for this
  round.
- **Tag noise control:** normalise via `normaliseTagPath` and import all tags —
  they're cheap to remove — rather than capping or curating a subset.
- **Scope:** all four areas ship in one PR, as requested.

## Follow-up (next session)

Two of stage 10's five frontend features are deferred. Their **backend is complete
and tested** — these are additive UI wiring only.

### 1. Upload "add as a version of…" prompt (stages 7 + 8)

The upload response already carries `suggested_match` (`{asset_id, name}`) when a
confident same-name match exists, and `POST /api/imports` already accepts an
`asset_id` form field to attach the upload as a new version. What's missing is the
UI:

- `uploadFile` (`web/src/api.ts`) hardcodes a file-only `FormData`; thread an
  optional `assetId` through it and `startUpload` (`NotificationProvider.tsx`) so a
  retry-as-version can re-post the same `File` with `asset_id` set.
- `uploadFile` already resolves with the parsed response — on success, read
  `suggested_match` and surface an actionable notification ("This looks like a new
  version of *<name>* — add as a version?"). Accepting re-uploads with `asset_id`;
  the default (a new asset) is already what happens.
- Duplicate store version on the target returns 409 with a clear message — surface
  it as the "replace the files of that version" affordance (re-extract / delete +
  re-upload).

### 2. Merge picker (stage 9)

`useMergeAssets` and `POST /api/assets/:id/merge` are in place; the endpoint returns
`adoptable` (source name/description/thumbnail) for one-click adoption. What's
missing:

- An asset action (detail-panel footer, next to "Delete asset") opening a
  source-asset picker — a name search over the library's assets (reuse the asset
  grid query) with the current asset as the merge target.
- On success, offer to adopt the returned `adoptable` fields (name/description via
  the existing `usePatchAsset`; thumbnail adoption needs a "set thumbnail from key"
  path that doesn't exist yet — scope that or drop thumbnail adoption for v1).

### Minor cleanup suggestion (not applied)

The store-version ordering comparator is duplicated between `getAssetDetail`'s
version sort (`assets_crud.go`) and `nextCurrentVersion`. They differ subtly
(detail view keeps SQL import-date order for unversioned rows; `nextCurrentVersion`
sorts unversioned by newest import), so extracting a shared comparator risks
changing display order — left as-is per "don't refactor unrelated code," flagged
here for a deliberate future consolidation.
