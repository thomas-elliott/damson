# Implementation progress

Source of truth for milestone scope is `docs/technical-design.md` §21; this
doc tracks what's **actually in the tree** against that plan and calls out
what's been skipped or deferred so later slices don't assume foundations
that aren't there.

Last audited: 2026-04-26 — M3, M4, and M5 landed as a single pass. The
tree now has tags + licences + projects + collections + search + library
settings, with a shared audit writer, files/thumbs streaming, and a
detail-panel + projects/collections pages on the frontend built against the
design system. See the new M3/M4/M5 sections below.

2026-04-25 — M2.5 (design system) landed. Frontend now has the full primitive + composition + domain library under `web/src/design-system/`, with Storybook + Vitest+Playwright + axe-core a11y enforcement. `LibraryPage`, `AssetGrid`, `DropZone`, `BootstrapPage`, `LoginPage` are migrated onto the new components; `LibraryHeader.tsx` was folded into the sidebar and deleted.

2026-04-24 — M0/M1/M2 re-verified against the tree. Since the 2026-04-23 audit the defaults flipped to `AUTH_MODE=local` + `STORAGE_MODE=local`, and local-mode bootstrap gained an HTTP path (`POST /api/auth/bootstrap` + `needs_bootstrap` on `/api/auth/mode` + the `/bootstrap` SPA page). Env-seeded bootstrap (`SeedLocalAdmin`) stays as an optional fallback. No milestone rows change — this is zero-config-dev polish on the M0 skeleton.

## Legend

- ✅ **Done** — code, tests (where the milestone implies them), and wiring land.
- ◐ **Partial** — present but with a deliberate gap noted inline.
- ⏭ **Deferred** — not implemented in this repo yet; a later milestone or an
  explicit follow-up owns it.

## M0 — Skeleton

| Item | Status | Where |
|---|---|---|
| Gin engine + GORM init + structured logging | ✅ | `cmd/server/main.go` |
| Config loader with startup-panic semantics | ✅ | `internal/config/config.go` |
| Pluggable auth with OIDC + local modes, `BootstrapFirstAdmin` path | ✅ | `internal/auth/{oidc,local,middleware}.go` |
| Health check endpoint | ✅ | `internal/handlers/api/health.go` |
| Vite SPA shell with `AuthGuard` covering both auth modes | ✅ | `web/src/components/AuthGuard.tsx`, `web/src/App.tsx` |
| Dockerfile (Node → Go → Alpine, non-root, `CGO_ENABLED=0`) | ✅ | `Dockerfile` |
| `task dev`, `task dev:spa`, `task build`, `task pre-release-checks` | ✅ | `Taskfile.yml` |
| SQL-only migration runner with advisory lock + `schema_migrations` | ✅ | `internal/db/migrate.go` |
| `//go:embed` of migrations + SPA from project root | ✅ | `damson.go` |

M0 is complete.

## M1 — Storage primitives

| Item | Status | Where |
|---|---|---|
| Content-addressed `storage.Service` interface | ✅ | `internal/services/storage/storage.go` |
| LocalFS implementation with atomic rename + SHA-256 dedup | ✅ | `internal/services/storage/localfs.go` |
| S3 implementation with endpoint override + path-style for MinIO | ✅ | `internal/services/storage/s3.go` |
| `assets` / `asset_versions` / `asset_files` / `asset_imports` SQL migrations | ✅ | `database/migrations/001_initial.sql` |
| Matching GORM models (struct tags mirror the SQL) | ✅ | `internal/models/asset.go` |
| `POST /api/imports` streams uploads, stores blob, creates asset + version + file | ✅ | `internal/handlers/api/assets.go` |
| `GET /api/assets` list with pagination, library-scoped, joined file count + size | ✅ | `internal/handlers/api/assets.go` |
| `requireLibrary` middleware with 403-never-404 semantics | ✅ | `internal/handlers/api/middleware_library.go` |
| Library switcher endpoint + library memberships API | ✅ | `internal/handlers/api/library.go`, `me.go` |
| SPA grid tolerates missing thumbnails (icon fallback) | ✅ | `web/src/components/AssetGrid.tsx` |
| Drop-zone wired to `/api/imports` | ✅ | `web/src/components/DropZone.tsx` |

M1 is complete for the scope called out in §21. Notes:

- **`DELETE /api/assets/:id` and `PATCH /api/assets/:id` are not implemented.**
  §16 lists them as part of the asset surface; M1 didn't call them out
  explicitly and nothing in-tree uses them yet. First real user of these will
  be M3 (metadata editor) — fold them in there.
- **Review workflow is not implemented.** `assets.needs_review` is written on
  import but `POST /api/assets/:id/review` doesn't exist; the import-queue
  surface waits until M7 per §21.

## M2 — Unity package handling

| Item | Status | Where |
|---|---|---|
| `.unitypackage` tar.gz extractor with per-GUID bundling, path-traversal + size caps | ✅ | `internal/services/extract/unitypackage.go` |
| Original archive retained via `asset_versions.original_package_object_key` + `package_kind='unitypackage'` | ✅ | `internal/handlers/api/assets.go` |
| File manifest rows with `unity_guid` set | ✅ | same |
| Preview harvest into `assets.thumbnail_object_key` (+ `thumbnail_content_type`) | ✅ | same |
| `POST /api/assets/:id/versions/:vid/reextract` | ✅ | same |
| Extractor unit tests (happy path, traversal, incomplete bundles, first-preview-wins, total-size cap, raw-tar input, garbage input rejected) | ✅ | `internal/services/extract/unitypackage_test.go` |
| Handler tests (Unity import, re-extract success, 409 on non-Unity version, 404 across libraries) | ✅ | `internal/handlers/api/assets_unity_test.go` |

M2 is complete.

## M2.5 — Design system

A discrete phase inserted between M2 and M3 to land the frontend's tokens,
primitives, shell compositions, and domain components before M3+ feature
work begins. Not in the tech design's §21 ladder — added here because
starting M3 without a consistent component base was setting the feature
milestones up for a visual-rework cycle.

| Item | Status | Where |
|---|---|---|
| OKLCH tokens (dark default, `data-theme="light"` override); Tailwind utilities map to them | ✅ | `web/src/design-system/tokens/tokens.css`, `web/tailwind.config.ts` |
| Self-hosted Inter + JetBrains Mono via `@fontsource/*` | ✅ | `web/src/styles/globals.css` |
| `cn()` helper with `extendTailwindMerge` for custom font-size groups | ✅ | `web/src/design-system/utils/cn.ts` |
| 24 primitives built on Radix headless (Button, Input, Badge, Pill, Chip, Tag, Kbd, StatusDot, ProgressBar, Glyph, Checkbox, Slider, Separator, Tabs, Tooltip, DropdownMenu, Dialog, Popover, Select, ScrollArea, IconButton, SegmentedControl, SearchInput, Textarea) | ✅ | `web/src/design-system/primitives/` |
| 12 shell compositions (AppShell, NavRail, ResizeHandle, FilterSidebar, Section, Toolbar, Breadcrumb, SelectionBar, DetailPanel, FullPageView, DropOverlay, EmptyState) | ✅ | `web/src/design-system/compositions/` |
| 14 domain components (LicencePill, LicenceBar, AssetThumb with 9 sprite patterns + waveform + font, AssetCard, AssetListRow, ProjectGlyph, ProjectCard, ProjectListRow, CollectionCard, QueueItem, KeyValueList, FieldRow, VersionItem, ActivityRow) | ✅ | `web/src/design-system/domain/` |
| Storybook 10 + `addon-a11y` + `addon-vitest` + `storybook-addon-pseudo-states` | ✅ | `web/.storybook/` |
| Vitest 3 with `unit` + `storybook` projects (Playwright Chromium); `a11y.test = "error"` | ✅ | `web/vitest.config.ts` |
| Migrate `LibraryPage`, `AssetGrid`, `DropZone`, `BootstrapPage`, `LoginPage` onto the new system | ✅ | `web/src/pages/`, `web/src/components/` |
| Delete obsolete `LibraryHeader.tsx` (absorbed into FilterSidebar) | ✅ | removed |
| `TooltipProvider` mounted at app root | ✅ | `web/src/main.tsx` |
| `docs/design-system.md` canonical spec; `docs/CONVENTIONS.md` updated to drop shadcn/ui; `docs/ui-design.md` §9 open questions resolved | ✅ | `docs/` |
| `task web:storybook`, `task web:test:stories`, `task web:test:all` | ✅ | `Taskfile.yml` |

Decisions locked in during M2.5 that differ from earlier drafts:

- **Accent is yellow only.** The mockup's blue/yellow theme-swap was dropped.
- **shadcn/ui is not used.** Components are hand-rolled on Radix headless
  primitives. This diverges from the previous `docs/CONVENTIONS.md` §5 note
  that called for shadcn/ui, and from `docs/ui-design.md` §2 where it was
  listed as the component library.
- **Tokens are OKLCH, not HSL.** The pre-M2.5 `globals.css` used HSL; that
  file now imports `tokens.css` and re-exports the OKLCH variables.
- **Text-3/text-4 tiers were lifted** from the mockup's L=0.60/0.46 to 0.70/0.60
  to meet WCAG AA on bg/panel. The mockup palette was designed visually; axe
  flagged it during a11y test runs and the lift came from that.

Verification: `task web:test:all` runs 68 tests (1 unit + 67 stories) in real
Chromium, all passing with a11y set to error mode.

## M3 — Metadata + licences

| Item | Status | Where |
|---|---|---|
| Audit writer (same-tx with domain mutation; `audit.Created/Updated/Deleted` helpers) | ✅ | `internal/services/audit/audit.go` |
| Migrations: tags + asset_tags + licences (FK `assets.licence_id` finalised) | ✅ | `database/migrations/002_tags_and_licences.sql` |
| Tags model + CRUD (materialised path, create-with-ancestors, descendant delete) | ✅ | `internal/models/tag.go`, `internal/handlers/api/tags.go` |
| Licence detection (licensecheck + Unity EULA fingerprint, SPDX → attribute map) | ✅ | `internal/services/licence/detect.go` |
| Licence handlers (`GET/PUT /api/assets/:id/licence`, review, `POST /api/licences/detect`, `GET /api/assets/:id/licence/detect`) | ✅ | `internal/handlers/api/licences.go` |
| Asset CRUD — `PATCH`, `DELETE`, `POST /review`, `GET /:id` detail view | ✅ | `internal/handlers/api/assets_crud.go` |
| Explicit cascade deletes for SQLite test parity (assets, projects, collections) | ✅ | same handlers |
| Files + thumbs streaming (`GET /api/files/:objectKey`, `GET /api/thumbs/:objectKey`) gated on library-scoped references | ✅ | `internal/handlers/api/files.go` |
| Licence/status derived from detected type + attributes (shared `LicenceStatusFor`) | ✅ | `internal/handlers/api/licences.go`, `quickLicenceStatus` in `projects.go`/`assets.go` |
| Handler tests: PATCH/DELETE/review/detail/tags/licences | ✅ | `*_test.go` in `internal/handlers/api/` |
| Frontend detail panel with inline name/description/URL edit, tag chip editor, licence editor + detect preview | ✅ | `web/src/components/detail/{AssetDetailPanel,LicenceEditor}.tsx` |
| Frontend tag tree + licence/file-kind/needs-review filters in sidebar | ✅ | `web/src/components/LibraryFilterSidebar.tsx` |
| Asset grid wired with licence pill + click-through to `/library/:assetId` | ✅ | `web/src/components/AssetGrid.tsx`, `web/src/pages/LibraryPage.tsx` |
| Mutations helper (`useMutations.ts`) + `fetchMaybeJson` on the API client | ✅ | `web/src/hooks/useMutations.ts`, `web/src/api.ts` |

M3 is complete for the §21 scope. Notes:

- **Audit is end-to-end mandatory.** Every library-scoped write handler
  from M3 onwards writes an `audit_log` row in the same transaction. The
  service panics on missing action/target type rather than silently
  dropping the row.
- **Unity EULA detection is a fingerprint, not a canonical-text match.**
  A regex on "Unity Asset Store" + "EULA"/"End User License Agreement" in
  the first 4KB covers the common Asset Store case. Users can override
  via the detail-panel editor.
- **Subset-version action (§9.3) is still deferred.** Noted in §21 as
  landing with the import queue; nothing in M3's detail-panel surface
  depends on it.

## M4 — Projects and pins

| Item | Status | Where |
|---|---|---|
| Migration: `projects` + `project_asset_pins` + case-insensitive unique name per library | ✅ | `database/migrations/003_projects.sql` |
| Models + CRUD (`GET/POST/PATCH/DELETE /api/projects/:id`) | ✅ | `internal/models/project.go`, `internal/handlers/api/projects.go` |
| Pin / upgrade (`POST /api/projects/:id/pins` is idempotent — second call upgrades; emits `project.pin.upgrade` audit) | ✅ | same |
| Unpin (`DELETE /api/projects/:id/pins/:pinId`) | ✅ | same |
| Project detail view with `updates_available` count derived from pinned vs. current version | ✅ | same |
| Frontend: Projects index (cards) + New project dialog + detail with pinned-asset table + upgrade/unpin actions + Pin-asset dialog | ✅ | `web/src/pages/{ProjectsPage,ProjectDetailPage}.tsx` |
| Handler tests: create, duplicate-name 409, pin, upgrade detection, cascade delete | ✅ | `internal/handlers/api/projects_test.go` |

## M5 — Collections, search, filters

| Item | Status | Where |
|---|---|---|
| Migration: `collections` + `collection_assets` | ✅ | `database/migrations/004_collections.sql` |
| Migration: `server_settings` singleton + `library_settings` per-library (+ backfill for existing libraries) | ✅ | `database/migrations/005_settings.sql` |
| Migration: `assets.search_vector` (tsvector) + GIN + trigger on name/description; `asset_tags` INSERT/DELETE trigger touches `assets` to recompute | ✅ | `database/migrations/006_search_vector.sql` |
| Collections handlers (CRUD + add/remove asset, idempotent) | ✅ | `internal/handlers/api/collections.go` |
| Search endpoint alias `/api/search` with same shape as `/api/assets` | ✅ | `internal/handlers/api/handler.go` |
| `getAssets` accepts composable filters: `q`, `tag_id`/`tag_path` (matches descendants), `licence_status`, `licence_spdx`, `project_id`, `collection_id`, `source_host`, `needs_review`, `package_kind` | ✅ | `internal/handlers/api/assets.go` |
| Postgres uses `plainto_tsquery` against `search_vector`; SQLite test tier falls back to `LIKE` on name/description — dialector-switched | ✅ | `applyAssetFilters` in `assets.go` |
| Library settings (GET/PATCH; PATCH requires library admin) with `loadOrCreateLibrarySettings` fallback | ✅ | `internal/handlers/api/settings.go` |
| Handler tests: filter-by-tag, query-string, composed filters, collection CRUD | ✅ | `internal/handlers/api/{search,collections}_test.go` |
| Frontend: Collections index + New collection dialog + Collection detail (asset grid filtered by `collection_id`) | ✅ | `web/src/pages/{CollectionsPage,CollectionDetailPage}.tsx` |
| Frontend: search box in toolbar syncs to `?q=` (URL is source of truth), filter chips from the sidebar update the same URL state | ✅ | `web/src/pages/LibraryPage.tsx` |
| Frontend: Settings page (read-only KV; admins gated at handler) | ✅ | `web/src/pages/SettingsPage.tsx` |
| Frontend: Queue page (filtered view of `needs_review=true`) | ✅ | `web/src/pages/QueuePage.tsx` |
| Top-level routes: `/library`, `/library/:assetId`, `/queue`, `/projects`, `/projects/:id`, `/collections`, `/collections/:id`, `/settings` | ✅ | `web/src/App.tsx`, `web/src/components/Shell.tsx` |

M5 is complete for the §21 scope. Notes:

- **Search is tsvector on Postgres, LIKE on SQLite.** The handler checks
  `db.Dialector.Name()`. SQLite-backed handler tests therefore exercise
  the LIKE branch; `search_vector` and its triggers are only validated
  when a real-Postgres integration tier comes online (§22 decision 5).
- **Licence status filter is post-SQL.** `licence_status` composes in
  Go — SQL returns all rows matching the other filters, then the handler
  drops rows whose derived status doesn't match. At list-page size
  (limit ≤ 200), this is cheap and avoids adding a derived column to the
  schema.
- **Settings PATCH is wired backend-only.** The SPA ships read-only for
  v1; a member-facing editor is a follow-up. The server rejects PATCH
  for non-admins with 403 so the editor can light up any time without
  schema or authz changes.

## Foundations the tech design assumes but hasn't been built yet

These are not "late" — they belong to later milestones per §21 — but listing
them here makes the gap visible while we work on subsequent slices so we
don't assume something exists and then discover it doesn't.

| Missing thing | Needed by | Expected home |
|---|---|---|
| SSE broadcaster + `/api/imports/events` + `/api/imports/:id/events` | Once imports take long enough to be worth streaming; kept synchronous for M1–M2. Blocks a richer queue UI but not the M7 audit viewers themselves | `internal/services/events/` (or alongside audit) + `internal/handlers/api/imports.go` |
| `/api/imports`, `/api/imports/:id`, `/api/imports/:id/retry` — list + detail + retry | Required for the full queue/review UI before M7 | `internal/handlers/api/imports.go` |
| Real thumbnail pipeline (raster/audio/font/Unity preview) + per-type toggles honoured | M6 | `internal/services/thumbnail/` |
| Audit viewers (per-asset, per-project, library-wide, server-wide) | M7 | frontend + `GET /api/audit` family |
| Library admin settings editor, user/membership management | M7 frontend | `web/src/pages/SettingsPage.tsx` (PATCH wired; editor TBD) |
| Asset-version subset action (§9.3) | M7 alongside the queue surface | `internal/handlers/api/assets.go` + a button in the detail panel |
| Thumbnail content-type picking for JPEG vs. PNG per category (§11) | M6 | `internal/services/thumbnail/` |

**Delivered since the previous audit (landed with M3/M4/M5):**

- Audit writer (`internal/services/audit/audit.go`) — now a prerequisite
  met, not a gap.
- Tags tree + `asset_tags` (migration 002, models, handlers, filters).
- Licences model + detection service + editor UI (migration 002,
  `internal/services/licence/`, licence handlers, detail-panel editor).
- Projects + pinning + upgrade (migration 003, handlers, UI).
- Collections (migration 004, handlers, UI).
- `server_settings` + `library_settings` tables + read/patch API
  (migration 005, `settings.go` handler).
- `assets.search_vector` + GIN + trigger for tsvector search and
  SQLite-LIKE fallback (migration 006, `applyAssetFilters`).
- `PATCH /api/assets/:id`, `DELETE /api/assets/:id`, `POST .../review`,
  `GET /api/assets/:id` detail view (`assets_crud.go`).
- `GET /api/files/:objectKey` + `GET /api/thumbs/:objectKey` gated on
  library-scoped references (`files.go`).

## Stretch goals (no firm milestone)

Real work, but not blocking any milestone. Pick up when a concrete import
blocks on them rather than speculatively.

| Stretch goal | Trigger to implement | Expected home |
|---|---|---|
| Zip-format `.unitypackage` extractor | A real package that is zip-not-tar can't be imported | `internal/services/extract/unitypackage.go` (parallel zip path producing the same `UnityResult`) |

## Decisions reflected in code that diverge from the draft docs

- **Local auth mode exists alongside OIDC.** `DAMSON_AUTH_MODE=local` is a
  real, tested path (`internal/auth/local.go`, `internal/handlers/api/auth.go`).
  The tech design only described OIDC; the product-design doc also only
  mentioned OIDC. The two are kept mutually exclusive at startup, which
  matches the "exclusive modes" framing in AGENTS.md but is not yet spelled
  out in `docs/technical-design.md`. Worth folding into §8.1 on the next
  design-doc pass.
- **`importResponse` reports `file_count` + `package_kind` instead of a
  single-file `file_id`.** The M1 shape assumed one file per import; M2 made
  that wrong for `.unitypackage`. Noted because API consumers should read the
  current handler, not the draft in §16.
