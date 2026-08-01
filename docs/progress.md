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

### Unity local Asset Store cache import — 2026-08-01

Scoped by
[`specs/2026-08-01-unity-local-cache-import.md`](specs/2026-08-01-unity-local-cache-import.md),
built on the CLI slice below.

- **`internal/unitycache`.** A leaf package, so the server and the CLI share one
  implementation of the cache layout: `ParseCachePath` / `SplitCategory` (the
  lowercase→uppercase category split, checked against the 40 samples in the spec) and
  `Scan` (symlinks and dotfiles excluded, depth bounded, an over-full tree an error rather
  than a truncated list). `NormaliseRelativePath` is the shared validator for what is, on
  the server side, untrusted input that gets persisted.
- **Source columns.** `asset_versions.source_kind` / `.source_relative_path` /
  `.source_size_bytes` (migration 021), nullable throughout. `POST /api/imports` takes the
  first two as form fields; the size is written from what storage returns, so a client can
  ask the idempotency question but never answers it.
- **`store_version` stays blank, and that took work.** The cache holds no version anywhere,
  but `ParseImportFilename` reads `v2` out of `DOTween HOTween v2` — and `store_version` is
  a per-asset uniqueness key, so that phantom would block the real `2.1.0` forever.
  `source_kind = unity_cache` suppresses the filename version parse.
- **Preflight gained precision.** `already_imported` and `updated` join the shipped
  `name_match` / `new`, keyed on `(source_kind, source_relative_path)` + size, scoped through
  `assets.library_id`, newest version winning on a repeated path. Additive: an entry with no
  source hint answers exactly as before, which the pre-existing preflight tests confirm.
- **`ApplySourceHints`** runs before enrich so category tags land whether or not
  auto-enrich is on, and fills `assets.publisher` blanks-only. That publisher then
  disambiguates `exactMatchCandidate` — breaking ties among title matches, and unlocking a
  whole-word-prefix near match on an exact publisher agreement, recorded at confidence `0.8`
  rather than a claimed `1.0`.
- **`dmsn sync scan|push`**, with `--cache-root --publisher --dry-run --adopt-versions
  --concurrency --wait`. Re-runnable by design. `--adopt-versions` governs `name_match`
  only: off, skipped, because a name match cannot tell a duplicate from a newer version.
- **`internal/importsource`** joins the leaf-package set, holding the source kinds and the
  four preflight statuses. Reaching into `internal/services/imports` for the constant had
  pulled GORM and the AWS S3 SDK into the CLI binary; `go list -deps ./cmd/dmsn` is the check.
- The shipped upload path became per-item — `importOptions` on `item`, replacing the
  positional `asset_id` / `store_version` on `PostImport` — because a cache sync targets a
  different asset per package. The shipped `upload` tests pass unmodified across it.

Out of scope and still open: any SPA import surface (a `webkitdirectory` picker is
buildable against these same endpoints), UPM `.tgz` packages, resumable uploads, and
deleting from damson when a package leaves the cache.

### `dmsn` CLI + personal access tokens — 2026-08-01

Scoped by [`specs/2026-08-01-dmsn-cli.md`](specs/2026-08-01-dmsn-cli.md), split out of the
Unity cache-import spec because a local CLI is its own piece of work.

- **Personal access tokens.** `api_tokens` (migration 020), secret
  `damson_pat_<32 random bytes>` stored as a plain SHA-256 hex digest — not bcrypt, because
  the secret is already high-entropy and a fast digest allows the indexed lookup every
  request needs. `auth.Middleware` takes a nil-able `TokenAuthenticator` and branches on the
  prefix ahead of the mode authenticator, so PATs behave identically in local and OIDC
  deployments. Scope is `read` | `write`; `read` is confined to GET. Token auth is refused on
  `/api/me/tokens` and `/api/admin/*` — a token can neither mint another nor reach sysadmin
  surface. Managed from Settings → Access tokens in the SPA, which is the only way to create one.
- **`cmd/dmsn`.** A single binary on stdlib `flag`, no new dependency, built to the
  [AXI principles](https://axi.md/): TOON output by default (`internal/toon`), 3–4 default
  columns widened by `--fields`, truncation with a length note, pre-computed aggregates,
  definitive empty states, structured errors on *stdout* with exit codes 0/1/2, and a
  content-first home view. The whole contract lives in `internal/axi` so no command
  reimplements it.
- **`GET /api/overview`.** One round trip for the home view — five counts, an updates-available
  count reusing `updatesAvailableSQL()`, and a capped recent-import tail. Composing it from the
  existing list routes would be five requests, three of which discard their payloads.
- **`POST /api/imports/preflight`.** The name-only half: "do you already hold something by this
  name?" for a whole batch, against one read of the library. Backs `upload --skip-existing`.
  The Unity slice above widened it with path-and-size precision.
- **Folder upload with no server change.** `--as asset` zips a directory client-side and posts
  it as `.zip`, which is already a first-class package kind, so `extract.ZipExtractor` writes
  the files with relative paths preserved. `--as assets` applies the same rule per child. A
  directory with no `--as` exits 2 carrying both concrete commands rather than guessing.
- **`dmsn setup`** writes a Claude Code skill (and an optional SessionStart hook script)
  describing the output contract and command surface, so an agent gets the tool's conventions
  without being told them.
- `max_upload_bytes` joined `GET /api/settings` read-only, so a client can size an upload
  instead of learning the cap from a 413 after transferring it.
- `GET /api/me` now describes the token a request authenticated with (prefix, label, scope,
  expiry) so `dmsn whoami` can report its own scope without reaching `/api/me/tokens`.
  `GET /api/health` reports the build identifier.

Build with `task build:cli`. Deliberately **no** cross-compile matrix, Woodpecker step or
release artefacts — the binary exists where someone builds it.

### Library top bar, rail account menu, file-manager browser — 2026-07-25

Scoped by
[`specs/2026-07-25-library-topbar-file-manager-browser.md`](specs/2026-07-25-library-topbar-file-manager-browser.md).
Front-end only — no API, schema or handler change.

- **The library toolbar is the page's identity row.** Library switcher (doubling
  as the title) + asset count, a `Filter` toggle, search, then Upload. The
  `Library · N assets` breadcrumb is gone — it repeated the switcher.
- **Filters are a toggleable sidebar, closed by default** (persisted per browser
  under `damson.library.filters`), so the grid gets the 248px back. No new
  composition: `AppShell` already collapses its sidebar column when no sidebar is
  passed. The toggle badges the active-filter count so nothing filters invisibly.
- `project_id` now actually filters. The sidebar wrote the search param but
  `LibraryPage` never parsed it, so clicking a project highlighted it and
  filtered nothing. Filter parsing moved to `lib/libraryFilters.ts`.
- **Account controls consolidated** into a rail-foot avatar menu (`UserMenu`):
  identity, Light/Dark, Settings, Sign out. The standalone theme rail item and
  the two sign-out buttons it replaced (filter-sidebar footer, `SignedOutSidebar`)
  are gone.
- **The scoped file browser behaves like a file manager.** Its root view is the
  root *folder* (folders first, then that folder's files) rather than a flat
  listing of the whole pack; the flat listing is now only for search and
  file-type results, where the 300-result cap was always load-bearing. The tree
  is folders-only and expands to depth 3 on open.
- Two behaviour changes that follow from the above: the file detail panel's
  prev/next stepper walks the current folder rather than the whole pack, and a
  flat "every file" listing is reachable only via a search term or type filter.
  A first-class flat mode would be a third option on the grid/list control.

### Asset sidebar refinements — 2026-07-25

No spec — UI polish on the library detail panel.

- The asset name is no longer repeated: it lives in the panel header only, and
  renaming moved into the Overview edit form (with description + source URL),
  replacing the click-the-title-to-rename affordance and the footer Save.
- **Overview reads as rendered markdown.** Descriptions are markdown (storefront
  enrichment writes them that way) but were shown in a permanent textarea, so
  most users only ever saw raw syntax. Read mode renders the description and
  shows the source URL as a link; an Edit button in the section header flips to
  the form. An asset with no description opens straight into the form.
- New `Markdown` + `MarkdownEditor` primitives (CodeMirror 6 +
  `@lezer/markdown`) back that — see [`design-system.md`](design-system.md).
- The rendered description is clamped to five lines with a Read more / Show less
  toggle (store descriptions run to hundreds of lines and buried every section
  below Overview), and the editor caps at 320px with internal scrolling.
- Read mode puts the source-URL link and the Fetch / Find listing buttons on one
  line; enrichment provenance (`Metadata from unity · date · listing`, plus the
  stored candidates for correcting a bad match) is housekeeping rather than
  content, so it only shows in edit mode. `EnrichmentReview` is now its own
  exported component owning its fetch cycle, and `MetadataActions` is just the
  two buttons.
- Licence section starts collapsed and carries the SPDX identifier as a pill in
  its header while collapsed — the one fact people scan for, without opening the
  section. Licence chips dropped from the library grid cards and the sidebar hero
  (still on project pins, where they earn the space).
- Storybook now globs `src/components/**` too, and the sidebar has real stories
  (`App/AssetDetailPanel`) driven by a stubbed `window.fetch`.

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
off Docker-out-of-Docker to buildkit-rootless on the k8s backend.
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
| No CLI distribution | `task build:cli` only — no cross-compile matrix, no release artefacts. `dmsn setup` does the discovery work with no install story behind it. Deliberate; revisit if the CLI gets external users. |
| Name-only preflight | `--skip-existing` matches on normalised title alone, so it cannot tell a duplicate from a genuine new version and skips both. The Unity slice adds path-and-size precision. |
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
