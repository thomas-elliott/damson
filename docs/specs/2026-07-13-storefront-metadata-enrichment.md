# Storefront metadata enrichment

Date: 2026-07-13
Status: signed off (lavish alignment session, 2026-07-13)

## Problem

Assets in damson carry only what can be derived from the uploaded package: a
filename-derived name, an extracted thumbnail, and a locally detected licence.
When an asset came from an online storefront — Unity Asset Store, Sketchfab,
itch.io, or anywhere else — the listing holds richer metadata we currently
re-type by hand: description, publisher, store URL, licence terms, and a proper
cover image. We want damson to fetch that metadata itself, two ways:

1. **Search by name** — find candidate listings across storefronts and let the
   user pick the match.
2. **Fetch from a source URL** — when the user supplies (or picks) a listing
   URL, pull the metadata from that page/API.

Plus, per the chosen approach: **auto-enrichment on import**, filling blank
fields automatically when a high-confidence match is found.

### Storefront access (verified live, July 2026)

| Store | Access without auth | Search | Licence info |
| --- | --- | --- | --- |
| Unity Asset Store | Server-rendered pages with OG tags + JSON-LD Product; unofficial GraphQL (`POST https://assetstore.unity.com/api/graphql/batch`) returns version, publish dates, screenshots, publisher, price, download size, `customLicense`/`licenseText`, and entitlement type (`SEAT` = per-seat extension asset). CSRF bootstrap: GET any page with a cookie jar → `_csrf` cookie → send cookies + `X-Csrf-Token: <cookie value>` + `X-Requested-With: XMLHttpRequest`. Body is a JSON **array** of `{query, variables, operationName}`. | Anonymous Coveo: `GET /api/coveo/search-token?searchHub=Assetstore_Search` → JWT → `POST https://unitytechnologiesproductionmkahteav.org.coveo.com/rest/search/v2?organizationId=unitytechnologiesproductionmkahteav` with `{"q": ..., "searchHub": "Assetstore_Search"}`; results carry `ec_name`, `ec_price`, `ec_rating`, `publisher_name`, `ec_product_id`, `ec_images`. | Via GraphQL only (EULA vs extension/SEAT, custom licence text) |
| Sketchfab | Official Data API v3, unauthenticated: `GET https://api.sketchfab.com/v3/models/{uid}` (uid = last segment of the URL slug after the final `-`) → description, counts, price, and a structured `license` object (label, fullName, requirements, url, slug). Rate limit advertised in `x-request-limit-*` headers (1500/window). | Official `GET /v3/search?type=models&q=` | Structured CC licence per model |
| itch.io | Server-rendered pages: JSON-LD Product (name, description, price, seller, rating) + twitter/og meta (note: **no `og:title`** — use `twitter:title` or `<title>`), plus a plain-HTML "More information" table that includes an "Asset license" row (e.g. CC0) when the seller sets one. Behind Cloudflare but no challenge for polite GETs. | Scrape `https://itch.io/search?q=` results HTML | Info-table row, when present |
| Fab (Epic) | Blocked: Cloudflare + hCaptcha kills server-side fetching. Out of scope until Epic ships its promised public API — Fab URLs get generic handling (which will fail gracefully to URL-only). | No | n/a |
| Any other URL | Generic OG tags + JSON-LD Product scraping (verified working on Kenney, Humble Bundle); degrade to `<title>` when og:title is absent. | No | Rarely |

Authenticated sessions would only add owned-library import (purchase lists) —
**out of scope**; noted as a possible future feature (itch.io has a clean
API-key route for it).

## Chosen approach

**Approach C — layered fetchers + import auto-enrichment.** A generic
OG/JSON-LD scraper is the base; per-store adapters (Unity, Sketchfab, itch.io)
layer on top keyed by URL host, each degrading to the generic scraper on
failure. Exposed through manual search/fetch endpoints **and** an automatic
import-pipeline stage.

It beat the alternatives because: a generic-only scraper (B) loses the
search-by-name UX and all structured licence data — the fields that motivated
the feature; and deferring auto-enrichment (A) was considered but the user
explicitly wants enrichment to happen on import, gated behind a per-library
setting and a conservative match-confidence gate to control the noise and
request-volume risks.

Key semantics (all signed off):

- **Fill blanks, confirm overwrites.** Fetched values are applied
  automatically to empty fields only. Any field that would overwrite an
  existing value is returned for explicit user confirmation. Human-reviewed
  data is never silently clobbered (same posture as `rescanLicence`).
- **Cover image becomes the asset thumbnail** via the existing
  content-addressed thumb pipeline (`PutThumb` → `ThumbnailObjectKey`). Set
  automatically when no thumbnail exists; otherwise offered as an overwrite.
  Rescan can regenerate the extracted thumbnail if the user reverts.
- **Raw fetched payloads always persist to `Asset.SourceMetadataJSON`**
  (existing, currently unused jsonb field), including provenance (store,
  fetched-at, adapter used, match confidence) and stored search candidates.
- **Licence data feeds the existing licence subsystem** (`PUT
  /assets/:id/licence` semantics): Unity entitlement type maps to the existing
  `unity-eula` knowledge in `internal/services/licence`; Sketchfab/itch CC
  slugs map to SPDX ids; custom licence text lands in `OriginalText`.
- **Auto-enrich on import** runs as a new pipeline stage after thumbnailing:
  search by the derived asset name → apply the top hit **only on a
  high-confidence normalised-title match** (otherwise store candidates, apply
  nothing) → fill blanks only → flag `NeedsReview`. Enrichment failure never
  fails the import. Gated on a new per-library `LibrarySettings` toggle
  (default on); manual search/fetch works regardless of the toggle.
- **Unity data source:** unofficial GraphQL primary, OG/JSON-LD page-scrape
  fallback. ToS posture: undocumented endpoints, so keep volume low —
  user-triggered or confidence-gated, per-host rate limiting, responses cached
  in `SourceMetadataJSON`, automatic degrade to page scrape on breakage.

### Security requirements (must be in stage 1, not bolted on)

The server fetches user-supplied URLs, so the fetcher must:

- Block private/loopback/link-local IPs after DNS resolution (SSRF), including
  on redirects; cap redirect count.
- Enforce timeouts, cap response body size, and restrict schemes to http(s)
  (`validateSourceURL` in `assets_crud.go` covers scheme checks for stored
  URLs; the fetcher needs its own guard since it follows redirects).
- Validate content types before storing cover images (decode as image, re-emit
  known-safe format if in doubt).
- Per-host rate limiting shared between manual endpoints and the pipeline
  stage.

## Task breakdown

One coherent, independently-committable stage per item.

1. **Fetcher core + generic scraper.** New package
   `internal/services/metadata/`: SSRF-guarded HTTP client (timeouts, size
   caps, redirect policy, private-IP blocking, per-host rate limiter),
   OG/JSON-LD Product parser (`golang.org/x/net/html`, already an indirect
   dep), a normalised `Metadata` struct (title, description, publisher, price,
   currency, licence hint, cover URL, image URLs, store id, listing URL, raw
   payload), and a host→adapter registry that falls back to the generic
   scraper. Unit tests on fixture HTML (Unity/itch/Sketchfab/Kenney pages
   saved as testdata).
2. **Store adapters.** Unity (GraphQL batch with CSRF bootstrap, page-scrape
   fallback), Sketchfab (Data API v3), itch.io (JSON-LD + meta tags + the
   info-table licence row). Each degrades to the generic scraper on any
   failure. Fixture-based tests per adapter; keep the GraphQL query minimal
   (only fields we consume) to reduce breakage surface.
3. **Fetch + apply endpoints.** `POST /api/assets/:id/metadata/fetch` —
   resolves the adapter from the asset's source URL (or a URL in the request
   body, which also sets `SourceURL`), fetches, persists the raw blob +
   provenance to `SourceMetadataJSON`, fills blank fields (description,
   licence via the licence service, thumbnail-if-none via `PutThumb`), and
   returns applied fields plus would-overwrite candidates. A companion apply
   endpoint persists user-confirmed overwrites, reusing the patch/licence/
   thumbnail paths, writing audit entries (follow `rescanLicence`'s pattern of
   respecting reviewed data). Expose `source_metadata` (or a trimmed view of
   it) in `assetDetailView` and the TS `AssetDetailView`.
4. **Name search endpoint.** `GET /api/metadata/search?q=` fanning out
   concurrently (short per-store timeout) to Unity Coveo, Sketchfab API, and
   itch.io search-results HTML; returns a merged candidate list with store
   provenance (title, publisher, price, cover URL, listing URL, store).
5. **Frontend.** In `AssetDetailPanel`: a "Fetch" action on the existing
   Source URL field and a "Find listing" action that opens a match-picker
   dialog (new composition from the existing `Dialog` primitives: search
   input pre-filled with the asset name, candidate rows with cover/title/
   publisher/store/price). Picking a candidate sets the source URL and runs
   the fetch; an overwrite-confirm view lists only conflicting fields. Query/
   mutation hooks follow `useDataQueries.ts` / `useMutations.ts` patterns.
6. **Import pipeline auto-enrichment.** New `LibrarySettings` boolean
   (default true) + migration + settings UI checkbox alongside the existing
   library toggles. New pipeline stage after thumbnailing (skipped when the
   toggle is off): name search → high-confidence gate (normalised-title
   comparison between the cleaned asset name and the top hit) → fill blanks →
   flag `NeedsReview`; store candidates in `SourceMetadataJSON` when the gate
   abstains. SSE stage events like the existing stages; enrichment failure
   logs and continues — never fails the import.
7. **Review surface polish.** Show enrichment provenance in the detail panel
   (which store, when, matched vs abstained) and surface stored candidates for
   one-click correction when the gate picked wrong or abstained.

## Reuse

- `Asset.SourceURL` — full-stack support already exists (model, API view,
  PATCH, `validateSourceURL`, editable detail-panel field, `source_host`
  search filter).
- `Asset.SourceMetadataJSON` — existing unused jsonb field; becomes the home
  for raw payloads, provenance, and stored candidates. No new table needed.
- Licence subsystem (`internal/services/licence/`) — detect-review-persist
  flow, Unity EULA fingerprint/attributes, SPDX matching, `PUT
  /assets/:id/licence`.
- Thumbnail pipeline (`internal/services/thumbnail/`, `setThumbnail` in
  `assets_rescan.go`, `GET /api/thumbs/:objectKey`) — covers download-and-
  store for cover images unchanged.
- Detect-then-confirm pattern (`rescanLicence`, licence detect endpoints) —
  the interaction and data-safety model for apply semantics.
- Import pipeline + SSE (`internal/services/imports/`) — stage machinery,
  worker pool, broadcaster for the auto-enrich stage.
- Frontend: `Dialog` primitives + `ConfirmDialog` composition, `fetchJson`
  wrapper, TanStack Query hook patterns.
- `golang.org/x/net/html` (already an indirect dependency) for HTML parsing.
- Searched for existing outbound HTTP/scraping/API-client code: **none exists**
  (grep for `http.Client`/`http.Get`/`goquery`/`colly` across `internal/`
  returns only inbound handlers and S3 SDK internals) — stage 1 introduces the
  first, as its own service package.

## Resolved questions

- **Approach:** C — layered fetchers (generic scraper + store adapters) with
  import auto-enrichment included now, not deferred.
- **V1 store adapters:** Unity Asset Store (GraphQL + Coveo search), Sketchfab
  (official API), itch.io page scrape **and** itch.io name search. Fab is
  URL-only until Epic ships a public API.
- **Unity data source:** unofficial GraphQL primary with OG/JSON-LD page-scrape
  fallback, accepting the documented ToS/stability posture and mitigations.
- **Apply semantics:** fill blank fields automatically; explicit confirmation
  only for overwrites.
- **Cover image:** the fetched cover becomes the asset thumbnail through the
  existing thumb pipeline; no separate cover field.
- **Auto-enrich matching:** high-confidence normalised-title match only;
  otherwise store candidates for review and apply nothing.
- **Auto-enrich toggle:** per-library `LibrarySettings` boolean, default on;
  manual fetch/search unaffected by the toggle.
- **Out of scope:** authenticated owned-library import (Unity session cookie /
  itch API key), Fab scraping, multi-image gallery model (screenshot URLs are
  kept in `SourceMetadataJSON` only).

## Deferred comments

- `internal/services/metadata/client.go` (guardedDialContext) — dials the
  validated IP directly rather than the hostname so a second DNS resolution
  can't swap in an internal address between check and connect (DNS-rebinding
  TOCTOU). TLS still verifies against the URL host, not the dial address.
- `internal/services/metadata/client.go` (hostLimiter.wait) — reserves the
  next slot under lock *before* sleeping so concurrent callers to one host
  queue at increasing offsets instead of all waking at once.
- `internal/services/metadata/client.go` (Do, `maxBody+1`) — reads one byte
  past the cap so an at-limit body is distinguishable from an over-limit one.
- `internal/services/metadata/scrape.go` (GenericFetcher.Fetch, status>=400) —
  errors on 4xx/5xx so an error page's markup isn't parsed into junk metadata;
  as the fallback fetcher, erroring is the right degrade.
- `internal/services/metadata/unity.go` (findMapWithKeys, name+description) —
  requires *both* keys so an error/publisher node isn't mistaken for the
  product; a schema change that renames `name` makes GraphQL "fail" and
  degrade to page-scrape by design, rather than return empty metadata.
- `internal/services/metadata/unity.go` (fetchUnityPage) — defaults LicenceHint
  to `unity-eula` when the page yields none: Asset Store assets carry the
  standard EULA unless the publisher set a customLicense.
- `internal/services/metadata/sketchfab.go` (sketchfabUID) — the uid is the
  last 32-hex token in the URL (Sketchfab appends it to the slug and also
  exposes the bare /models/<uid> form).
- `internal/handlers/api/metadata.go` (applyThumbnailFromMeta) — routes the
  fetched cover through the raster thumbnail pipeline, which decodes and
  re-encodes; that *is* the content-type-validation SSRF requirement (a
  non-image body classifies to nothing and is silently skipped).
- `internal/handlers/api/metadata.go` (buildLicenceFromMeta, hint path) —
  synthesises a short original_text because the Licence model requires it
  non-blank, while the real signal is the detected_type + known attributes.
- `internal/handlers/api/metadata.go` (applyLicenceFromMeta) — same-type
  existing licence is a no-op (not a conflict); fill-blanks never touches an
  existing licence, so a reviewed one is only ever replaced on explicit apply.
- `web/src/components/detail/MetadataActions.tsx` (candidate cover `<img>`) —
  a plain `<img src>` is used because candidate covers are external storefront
  CDN URLs, directly renderable, unlike our own file endpoint which forces
  octet-stream + nosniff and needs the fetchBlob dance.
- `web/src/components/detail/MetadataActions.tsx` (match picker `q`) — the
  search box seeds from the asset name at mount and the "Find listing" button
  re-searches that name on each open; `q` isn't force-reset on reopen (avoids
  a setState-in-effect), so a previously-typed query persists — acceptable.
