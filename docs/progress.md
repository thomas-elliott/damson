# Implementation progress

What is **actually in the tree**, so a new slice doesn't assume a foundation
that isn't there.

**Last audited: 2026-07-25** (against `b1cbcec`).

Two eras, and they're tracked differently:

- **M0–M7, the MVP ladder** — scoped up-front by
  `docs/archive/technical-design.md` §21. Complete. Summarised in one table
  below; the detail is in the code and in `docs/architecture.md`.
- **Post-MVP** — spec-driven. Each slice gets a signed-off spec in
  `docs/specs/` before implementation, and the spec stays as the record of
  *why*. Listed below newest-first.

For the current shape of the system read
[`docs/architecture.md`](architecture.md); this doc is about *status*, not
design.

---

## The MVP ladder (M0–M7) — complete

| Milestone | Scope | Landed as |
|---|---|---|
| **M0** Skeleton | Gin + GORM + structured logging, config loader with startup-panic semantics, pluggable auth (OIDC + local) with `BootstrapFirstAdmin`, health endpoint, SPA shell + `AuthGuard`, Dockerfile, SQL migration runner with advisory lock, root `//go:embed` | `cmd/server`, `internal/{config,auth,db}`, `damson.go` |
| **M1** Storage primitives | Content-addressed `storage.Service` with LocalFS + S3 impls, core asset schema, upload → blob → asset/version/file, library-scoped list, `requireLibrary` 403-never-404 middleware, library switching | `internal/services/storage`, `internal/handlers/api/{assets,library,me,middleware_library}.go` |
| **M2** Unity packages | `.unitypackage` tar extractor (per-GUID bundling, traversal + size caps), original archive retention, `unity_guid` manifests, preview harvest, re-extract | `internal/services/extract` |
| **M2.5** Design system | Tokens, primitives, shell compositions, domain components — inserted before feature work to avoid a later visual-rework cycle | `web/src/design-system/`, `docs/design-system.md` |
| **M3** Metadata + licences | Tags tree + `asset_tags`, licence model + SPDX/EULA detection + editor UI, asset CRUD and review | `internal/services/licence`, `internal/handlers/api/{tags,licences,assets_crud}.go` |
| **M4** Projects and pins | Projects, version pinning, logged upgrade path | `internal/handlers/api/projects.go` |
| **M5** Collections, search, filters | Collections, `search_vector` + GIN + triggers, composable filters, `/api/search` | `internal/handlers/api/{collections,assets}.go` |
| **M6** Thumbnails | Raster / audio waveform / font workers, per-library toggles, icon fallback | `internal/services/thumbnail` |
| **M7** Audit, queue, polish | Same-transaction audit writer, audit viewers at every scope, import queue + retry, settings editors, members management, keyboard shortcuts | `internal/services/audit`, `internal/handlers/api/{audit,imports,settings,members,admin}.go` |

Everything the tech design listed as a gap has since been built. The three
long-standing ones — SSE import events, `POST /api/libraries`, and a
real-Postgres test tier — all closed during the post-MVP work below, as did
the one stretch goal (a zip-format extractor).

---

## Post-MVP slices

Newest first. Each heading links the spec that scoped it.

### Versioning rework + storefront-metadata refinements — 2026-07-24 → 07-25

Spec: [`specs/2026-07-24-versioning-and-storefront-metadata-refinements.md`](specs/2026-07-24-versioning-and-storefront-metadata-refinements.md)

- **`store_version` is now the version identity** (migration 018). An
  `AssetVersion` is identified by the external pack version parsed from the
  filename, not by an incrementing label. Re-extraction rewrites a version's
  files **in place**; a new version only comes from uploading another pack.
  A partial unique index allows unversioned (`''`) rows to coexist.
- Version edit + delete endpoints (`PATCH` / `DELETE
  /api/assets/:id/versions/:vid`) and the matching UI.
- Upload can be adopted as a **new version of an existing asset**; the import
  response carries a `suggested_match` and the SPA prompts.
- **`POST /api/assets/:id/merge`** folds one asset into another, moving tags,
  collection references and project pins (remapping dropped pins).
- Storefront gallery: video URLs filtered, first still image picked as cover,
  `thumbnail_from_store` stops extraction clobbering a store cover
  (migration 017), gallery backfill endpoint for assets enriched earlier,
  storefront tags attached on apply and auto-enrich, lightbox UI.
- Store images proxied through `GET /api/metadata/image` so the CSP doesn't
  need to allow store CDNs.
- Bulk asset select + bulk edit/delete (`POST /api/assets/bulk`).

### Upload system refinements — 2026-07-24

Spec: [`specs/2026-07-24-upload-system-refinements.md`](specs/2026-07-24-upload-system-refinements.md)

- Upload concurrency capped, with per-upload cancel and clear-all — fixes
  502s when a multi-file drop fired N unbounded simultaneous requests.
- **Zip extractor** (`internal/services/extract/zip.go`) and `.zip` as a
  first-class `package_kind`. This closes the one stretch goal.
- Intent modal on multi-file drop: separate assets, or one asset bundled from
  a folder. Bundling is client-side via `fflate` (`web/src/lib/bundleUpload.ts`).
- Structured error logging through the upload/import path.

### River queue migration — 2026-07-19

Spec: [`specs/2026-07-19-river-queue-migration.md`](specs/2026-07-19-river-queue-migration.md)

Replaced both hand-rolled goroutine-pool-over-channel queues with
[River](https://riverqueue.com), Postgres-backed. The pipelines themselves
are unchanged — River wraps them. See `architecture.md` §12.

- `internal/queue` — River client, `import` and `preview` queues, workers,
  its own `pgxpool` separate from GORM's.
- Failure / retry / timeout semantics; resume-in-flight on boot (idempotent,
  and it covered the one-time cutover).
- **SQLite dropped entirely** (stage 0). Go tests and e2e now run against real
  Postgres via `internal/testdb`; `DAMSON_TEST_POSTGRES_DSN` is required and
  the suite fails fast without it. This closes the long-standing
  real-Postgres-tier gap — tsvector, JSONB operators and advisory locks are
  exercised directly.
- Queue sizing and timeouts configurable per queue (§15.1).

> One leftover: `applyAssetFilters` still carries a non-Postgres `LIKE`
> fallback branch dispatched on `db.Dialector.Name()`. It's unreachable —
> dead code awaiting removal.

### Dependency sweep — 2026-07-15/16

No spec (housekeeping). React Router 7→8, react-dropzone 14→17, TypeScript
side-by-side TS6/TS7 install, Go + web minor/patch sweep, `x/image` and
`quic-go` vulnerability fixes.

### Storefront metadata enrichment — 2026-07-13 → 07-14

Spec: [`specs/2026-07-13-storefront-metadata-enrichment.md`](specs/2026-07-13-storefront-metadata-enrichment.md)

- SSRF-guarded fetcher core + generic OG/JSON-LD scraper
  (`internal/services/metadata`).
- Unity Asset Store (via Coveo), Sketchfab and itch.io adapters; name-search
  fan-out; per-library `preferred_store` ordering (migration 015).
- `POST /api/assets/:id/metadata/{fetch,apply}`, `GET
  /api/metadata/{preview,search,image}`.
- Auto-enrich on import behind `library_settings.auto_enrich_enabled`
  (migration 013), surfacing as the `enriching` import stage.
- Asset name + store version parsed from the upload filename.
- Gallery images stored in `asset_images` (migration 016); markdown
  descriptions; enrichment provenance and candidate correction in the SPA.

### Large uploads + orphan storage cleanup — 2026-07-05

Specs: [`specs/2026-07-05-large-uploads-storage.md`](specs/2026-07-05-large-uploads-storage.md),
[`specs/2026-07-05-orphan-storage-cleanup.md`](specs/2026-07-05-orphan-storage-cleanup.md)

- **S3 uploads stream** via `manager.Uploader` instead of `io.ReadAll` into
  memory — this is what unblocked uploads above ~500 MB.
- New `originals/` storage prefix for uploaded archives, separate from the
  extracted-content blob pool; `ObjectMeta` stamping (filename, owning ids)
  with `StatMeta` readback; `List` / `StatMeta` / generic `Delete` primitives.
- `RetainOriginalPackages` actually enforced — archives purged after
  extraction, tracked by `original_package_purged` (migration 011).
- `MaxUploadBytes` enforced with a hard body cap.
- Orphan cleanup service (`internal/services/storage/cleanup`) plus
  `POST /api/admin/storage/cleanup/{scan,delete}` and a sysadmin review panel.
  Age-gated by `DAMSON_ORPHAN_MIN_AGE`, re-verified immediately before delete,
  every deletion audited.
- Observability pass: request correlation, `err` on 4xx/5xx, Alloy →
  VictoriaLogs shipping ([`observability.md`](observability.md)).

### Notification centre — 2026-07-04/05

Spec: [`specs/2026-07-04-notification-centre.md`](specs/2026-07-04-notification-centre.md)

Client-side only — no table, no endpoint. `NotificationProvider` mounts above
the router so **it owns the upload queue**: uploads no longer die when
navigation unmounts the page. Adds the nav-rail bell, the panel, upload
cancellation in `uploadFile`, and mutation-error surfacing.

### File previews + processing queue — 2026-06-18/19

Specs: [`specs/2026-06-18-file-previews.md`](specs/2026-06-18-file-previews.md),
[`specs/2026-06-18-preview-processing-queue.md`](specs/2026-06-18-preview-processing-queue.md)

- Per-file preview derivatives (`internal/services/preview`), stored under
  `thumbs/`, currently pure-Go raster (TIFF/BMP and friends).
- **`asset_files.preview_status` as first-class state** (migrations 009, 010,
  012) — `unsupported` / `queued` / `processing` / `done` / `failed`. This is
  what makes preview work observable and retryable rather than an invisible
  sub-stage of import.
- Previewing decoupled from import via a pool handoff; broadcaster and
  resume-on-startup (later replaced by River, above — the state model was
  kept, only the transport changed).
- `GET /api/previews`, `/api/previews/events`, `/api/previews/:objectKey`,
  requeue per asset and per file; `/processing` page in the SPA.
- Per-file thumbnails in the file-browser grid; server thumbnails in the
  library grid.

### CI/CD migration — 2026-06-19/20

No spec. Moved off GitHub Actions to **Woodpecker** (`.woodpecker.yml`), and
off Docker-out-of-Docker to buildkit-rootless on the kerbol k8s backend.
Deploys are ArgoCD-reconciled kustomize tag bumps: `k8s/test/damson` on every
`main` build, `k8s/prod/damson` on a release tag.

---

## Known gaps

Deliberate, not forgotten.

| Gap | Note |
|---|---|
| Dead `LIKE` search branch | `applyAssetFilters` dispatches on `db.Dialector.Name()`; the non-Postgres branch is unreachable now SQLite is gone. Remove on next touch. |
| Server-level audit retention | `library_settings.audit_retention_days` purges per-library rows; `library_id IS NULL` events are never purged — `server_settings` has no retention column. |
| Subset source version | The scoped file browser only loads the asset's *current* version, so a subset can only be cut from that. |
| No `useInfiniteQuery` | Lists are `limit`/`offset` server-side and the grid virtualises a single page. Add paging when a list actually needs it. |
| Preview format coverage | Pure-Go raster only so far. PSD/EXR/TGA, SVG rasterisation and PDF posters are staged behind it — see the file-previews spec. |
| e2e tiers 2 and 3 | Tier 1 is implemented; see [`e2e-flows.md`](e2e-flows.md). |
| Local auth hardening | In-memory rate limiting only (no shared store across replicas), no MFA, minimum-length password floor only. Documented as dev / small-install only. |

---

## Decisions in code that diverge from the archived draft docs

- **Local auth mode exists alongside OIDC.** The archived tech and product
  docs describe OIDC only. `DAMSON_AUTH_MODE=local` is a real, tested path;
  the two are mutually exclusive at startup.
- **External identities live in `user_identities`, not `users.subject`.**
  Migration 008 dropped `users.subject`. Identities are keyed
  `(issuer, subject)` and a verified email is the cross-IdP join key, so one
  user can sign in through several IdPs. No data backfill — an existing OIDC
  user is re-adopted by email on next login.
- **A version's identity is its `store_version`,** not a sequence number.
  Re-extraction rewrites in place instead of spawning a version.
- **`importResponse` reports `file_count` + `package_kind`,** not a
  single-file `file_id`. The M1 shape assumed one file per import, which M2
  made wrong. Read the handler, not the draft §16.
- **Background work is a Postgres-backed queue,** not in-process goroutines.
  The DB stays the source of truth for *state*; River tracks *work*.
