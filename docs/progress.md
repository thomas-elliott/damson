# Implementation progress

Source of truth for milestone scope is the archived
`docs/archive/technical-design.md` §21 ladder; the current architecture
of what shipped lives in `docs/architecture.md`. This doc tracks what's
**actually in the tree** against that plan and calls out what's been
skipped or deferred so later slices don't assume foundations that
aren't there.

Last audited: 2026-04-27. Two entries this date: `layout-improvements`
branch work below, and the Alpha leftover-features rollup further down.

**layout-improvements branch (in progress, not yet merged)**

- Resizable `AssetDetailPanel` (PER-155) — `DetailPanel` gains `resizable`,
  `onWidthChange`, `minWidth`, `maxWidth`, and `fullscreen` props (overlay
  variant only). `AssetDetailPanel` wires in drag-resize with width persisted
  to `localStorage` (`damson.detail.w`; default 440 px, min 320 px). A
  fullscreen toggle button expands the panel to cover the grid; viewports
  ≤ 640 px trigger auto-fullscreen. `docs/design-system.md` updated.
- `tsconfig.node.json` — minor tooling fix (no behaviour change).

**Alpha leftover features (rollup PR)**

- `DELETE /api/libraries/:id` (PER-70) — library admins / sysadmins can
  tear a library down. Same-tx cascade across assets, projects, collections,
  tags, settings, memberships, in-library audit rows; deletion event written
  with `library_id=NULL` so it survives the audit-log FK and lands in the
  sysadmin server feed. Every former member's `tokens_valid_after` is
  bumped so any open SPA tab fails 401 on its next call. New "Danger zone"
  in `LibraryProfileSection.tsx` with type-the-name confirm dialog.
- Sysadmin audit: `?scope=all` (PER-74) — `/api/admin/audit` accepts a new
  scope param so sysadmins can merge every library-scoped event into the
  view alongside the server-level rows. Rows on `scope=all` carry the
  source library's name; the SPA gets a SegmentedControl that drives
  `?scope=` on `AdminAuditPage`. Default behaviour unchanged.
- Subset-version picker UI (PER-71) — `AssetDetailPanel` Files section
  grows a "Subset" action that opens a checkbox-list dialog over the
  current version's files (with a path filter and select-all-visible). Hits
  the existing `useCreateSubsetVersion` hook against the backend that
  shipped with M7.
- Last-admin / last-sysadmin UI guard (PER-75) — server already 409s on
  demoting/removing the only admin (or only sysadmin); the role select +
  remove button are now disabled with a tooltip explaining what to do
  first. `LibraryMembersSection` and `ServerAdminSection`. Server stays
  the source of truth.
- Bootstrap → /library nav fix + auth-mode cache collapse (PER-68) —
  `BootstrapPage.onSubmit` previously hung awaiting
  `queryClient.invalidateQueries(authMode)` under React 19 + RQ5 (no
  active subscriber). It now seeds the new value via
  `queryClient.setQueryData` (synchronous), fires a no-await invalidate,
  and navigates. The parallel module-level `cached` singleton in
  `lib/authMode.ts` (and `invalidateAuthMode()`) is gone — the synchronous
  accessor reads directly out of TanStack's cache. AuthGuard's
  race-workaround comment is updated; the e2e `test.fixme` for this flow
  is un-fixme'd.
- Local-mode token refresh (PER-154) — `POST
  /api/auth/refresh` (local mode only) re-issues a fresh JWT for the
  authenticated user. `getAccessToken` calls `maybeRefreshLocalSession`
  before each request; when the stored session is within 5 minutes of
  expiry it swaps in a fresh token (concurrent fetches share one
  in-flight refresh). Failures fall through to api.ts's existing 401
  handling.

**Backend security & robustness** (2026-04-25)

**Backend security & robustness**

- `cmd/server/main.go` now installs `securityHeaders` (CSP /
  X-Content-Type-Options / X-Frame-Options / Referrer-Policy), a
  `jsonBodyCap` middleware (1 MiB cap on non-multipart bodies), and full
  server timeouts (`Read{Header}Timeout`, `WriteTimeout`, `IdleTimeout`).
- `GET /api/files/:objectKey` is now `Content-Type: application/octet-stream`
  + `Content-Disposition: attachment` + `Content-Length`; `GET /api/thumbs/`
  forces an allow-listed Content-Type. Stored XSS via uploaded HTML/SVG can
  no longer reach the SPA's localStorage token.
- `assets_crud.go` validates `source_url` against `http(s)` schemes (rejects
  `javascript:` / `data:` / `file:`).
- `auth/local.go` issues per-deployment `iss` / `aud` derived from
  `DAMSON_BASE_URL`; `auth.LocalAuthenticator.Login` is gated by
  `auth.LoginThrottle` (per-IP + per-email; 5 burst, 1/min refill);
  `POST /api/auth/bootstrap` is throttled per-IP.
- All password write paths (`SeedLocalAdmin`, `CreateFirstLocalAdmin`,
  `CreateLocalUser`, `SetLocalUserPassword`) enforce
  `auth.MinPasswordLength = 8`, surfaced to handlers as
  `auth.ErrPasswordTooShort`.
- Removing a library member bumps `users.tokens_valid_after` so an open SPA
  tab is forced to log in on its next request rather than silently sliding
  into "no library access yet".
- OIDC bootstrap needs no env config: the first user to sign in on an empty
  system becomes the sysadmin. Users are keyed by `sub` only; one with no
  email claim is prompted to add a profile on first login (`PATCH /api/me`).

**Backend correctness**

- `applyAssetFilters` translates `licence_status` into an SPDX `IN (…)`
  predicate so `Total` and the page actually agree, including the "no
  licence row → unk" branch.
- `source_host` filter now escapes `%` / `_` (`LIKE … ESCAPE '\'`).
- `tags.go` caps tag-path length at 1 KiB; `localfs.go` enforces hex-only
  inside `pathFor` so a bug at the HTTP boundary can't escalate to a path
  traversal.
- `imports.go` retry path requires an `asset_files` row for non-Unity
  imports (Unity case still uses the retained `original_package_object_key`).
  Stops the retry from creating a manifest-empty asset version when the
  upload bytes were never persisted.
- `requireLibrary()` now does a single JOIN instead of two SELECTs; field
  layout matches the previous behaviour exactly.
- `helpers.go` adds `respondGORM(c, err, msg)` plus `errBadRequestf` /
  `errConflictf` sentinels. Every transaction-returning handler uses the
  shared mapper; `stringError` / `asError` are gone (apart from `asError`
  living on as a thin `errors.New` wrapper for direct 400s).
- All `PATCH /api/...` handlers issue `Updates(map)` with only the changed
  columns instead of `Save(&row)` rewriting every field.
- `_ = before` in `postProjectPin` is gone; the upgrade audit diff now
  carries the previous version.

**Frontend**

- `AssetGrid.tsx` switches to `@tanstack/react-virtual` so large libraries
  don't render every card. Column count comes from a `ResizeObserver`-driven
  hook; keyboard selection scrolls into view via `virtualizer.scrollToIndex`.
- `LibraryPage.tsx` keyboard shortcut bindings are stable across renders —
  handlers read state from a ref instead of being re-created on every
  refetch, so `useKeyboardShortcuts` no longer rebinds on each paint.

**API package**

- `Handler` is exported (was unexported `handler`). `Deps.LoginThrottle` is
  optional and defaults to a sensible bucket. `internal/handlers/api/*.go`
  receivers are `*Handler`.

**Doc updates**

- `docs/archive/technical-design.md` §16 refreshed to mirror `handler.go` exactly,
  with a "documented but not yet implemented" appendix for the deferred
  endpoints.
- `docs/archive/technical-design.md` §8.1 promotes the local-auth HTTP bootstrap
  flow, password floor, throttle, and per-deployment iss/aud; §8.3 (new)
  documents the HTTP security posture (CSP, body cap, timeouts, file
  download hardening, source_url validation, SPA token storage caveat).
- `docs/archive/product-design-doc.md` §6 calls out OIDC vs local explicitly and
  notes local mode is for development / single-user installs.
- `README.md` Layout calls out `damson.go` and the `web/static/.gitkeep`
  placeholder.

Earlier history below.

2026-04-26 — M3, M4, and M5 landed as a single pass. The
tree now has tags + licences + projects + collections + search + library
settings, with a shared audit writer, files/thumbs streaming, and a
detail-panel + projects/collections pages on the frontend built against the
design system. See the M3/M4/M5 sections below.

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
| `docs/design-system.md` canonical spec; `docs/CONVENTIONS.md` updated to drop shadcn/ui; `docs/archive/ui-design.md` §9 open questions resolved | ✅ | `docs/` |
| `task web:storybook`, `task web:test:stories`, `task web:test:all` | ✅ | `Taskfile.yml` |

Decisions locked in during M2.5 that differ from earlier drafts:

- **Accent is yellow only.** The mockup's blue/yellow theme-swap was dropped.
- **shadcn/ui is not used.** Components are hand-rolled on Radix headless
  primitives. This diverges from the previous `docs/CONVENTIONS.md` §5 note
  that called for shadcn/ui, and from `docs/archive/ui-design.md` §2 where it was
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

## M6 — Thumbnails

| Item | Status | Where |
|---|---|---|
| `thumbnail.Generator` with category dispatch (raster / audio / font) | ✅ | `internal/services/thumbnail/thumbnail.go` |
| Raster worker: stdlib `image` + `golang.org/x/image/webp`, Catmull-Rom resample to 512px, JPEG-vs-PNG picker per §11.1 | ✅ | `internal/services/thumbnail/raster.go` |
| Audio worker: in-tree WAV decoder (PCM int 8/16/24/32 + IEEE float), `github.com/hajimehoshi/go-mp3`, `github.com/jfreymuth/oggvorbis`; peak-binned waveform PNG | ✅ | `internal/services/thumbnail/audio.go` |
| Font worker: `golang.org/x/image/font/opentype` rendering "The quick brown fox…" sample to PNG | ✅ | `internal/services/thumbnail/font.go` |
| Wired into `postImport` after extraction; honours `library_settings.thumbnails_*` toggles; failures log+swallow (icon fallback) | ✅ | `internal/handlers/api/assets.go` |
| Generator tests (PNG, JPEG-picker, toggles-off, WAV waveform, classify, unknown category, decode error) | ✅ | `internal/services/thumbnail/thumbnail_test.go` |
| Handler tests (raster import generates a thumbnail, raster toggle off skips it) | ✅ | `internal/handlers/api/assets_thumbnail_test.go` |
| Settings page editable for admins (theme/grid SegmentedControls, thumbnail Checkboxes, audit retention Input) | ✅ | `web/src/pages/SettingsPage.tsx` |
| `usePatchLibrarySettings` mutation hook with TanStack invalidation | ✅ | `web/src/hooks/useMutations.ts` |
| Dark/light toggle as an icon in the nav rail (bottom-left, above Settings); persists to `localStorage`, seeded from `library_settings.theme_default` | ✅ | `web/src/components/Shell.tsx`, `web/src/hooks/useTheme.ts`, `web/src/lib/theme.ts` |
| Pre-React boot in `main.tsx` applies stored theme so light-mode users don't see a dark flash | ✅ | `web/src/main.tsx` |

M6 is complete for §21 scope. Notes:

- **Audio decoders are pure-Go.** WAV is parsed in-tree (no external lib);
  MP3 uses `hajimehoshi/go-mp3`; OGG Vorbis uses `jfreymuth/oggvorbis`.
  These ship as transitive deps; no CGo is added. Decoders that need an
  `io.ReadSeeker` (mp3 / ogg) buffer the upload — bounded by the upload
  size cap.
- **JPEG-vs-PNG picker** is conservative: only inputs decoded as JPEG or
  WebP that have no transparent pixels become JPEG output. Everything else
  is PNG, mirroring §11.1's "JPEG for photographic, PNG for everything else".
- **Failures are non-fatal.** A broken decoder logs at warn and the asset
  imports without a thumbnail; the SPA falls back to the existing icon.
  Per-type toggles are read once per import via `loadOrCreateLibrarySettings`.
- **Theme toggle decision:** UI §9 left "global tweaks menu" open as an
  alternative location. Closed in favour of nav-rail-only per Thomas's
  call during the M6 plan.

## M7 — Audit, queue, polish

| Item | Status | Where |
|---|---|---|
| Audit endpoints: `GET /api/audit`, `/api/assets/:id/audit`, `/api/projects/:id/audit`, `/api/admin/audit` (sysadmin-only) | ✅ | `internal/handlers/api/audit.go` |
| Shared `applyAuditFilters` + `loadAuditPage` helpers; user join inline so the SPA renders without N+1 | ✅ | same |
| `requireSysadmin(c)` helper (counterpart to `requireAdmin`) | ✅ | same |
| `GET /api/imports`, `GET /api/imports/:id`, `POST /api/imports/:id/retry` (in-flight + failed by default; `?stage=all` for the historical view) | ✅ | `internal/handlers/api/imports.go` |
| Failure-path stage write: extract failures persist a stub asset + `stage='failed'` import row so the queue surface can retry | ✅ | `internal/handlers/api/assets.go` (`recordFailedUploadImport`) |
| `POST /api/assets/:id/versions/:vid/subset` — reuses object keys, validates `file_ids` belong to the source version, audits as `asset.version.subset` | ✅ | `internal/handlers/api/subset.go` |
| Library member endpoints: list, invite (creates local-mode user when missing + admin-supplied password), role change, remove. Last-admin guard on demote/remove | ✅ | `internal/handlers/api/members.go` |
| `auth.CreateLocalUser` + `auth.SetLocalUserPassword` — non-bootstrap counterparts to `CreateFirstLocalAdmin` for admin-driven invites + password resets | ✅ | `internal/auth/bootstrap.go` |
| Admin endpoints: `GET/PATCH /api/admin/users/:id` (role + password reset), `GET/PATCH /api/admin/settings`, last-sysadmin guard | ✅ | `internal/handlers/api/admin.go` |
| `PATCH /api/libraries/:id` for the library profile editor | ✅ | `internal/handlers/api/library.go` |
| Handler tests: audit (filters, scope, sysadmin-only), members (invite/duplicate/remove/last-admin), imports (filter, retry-precondition), subset (happy path + cross-version reject), failed-upload-stub | ✅ | `*_test.go` |
| Frontend audit page (`/audit`) with action chip filter + user search + activity rows | ✅ | `web/src/pages/AuditPage.tsx` |
| Frontend admin audit (`/admin/audit`) sysadmin-gated with redirect for non-sysadmins | ✅ | `web/src/pages/AdminAuditPage.tsx` |
| Audit sections inside `AssetDetailPanel` and `ProjectDetailPage` (last 25 entries each) | ✅ | `web/src/components/detail/AssetDetailPanel.tsx`, `web/src/pages/ProjectDetailPage.tsx` |
| Reusable `AuditList` + `auditFormat` formatter (verb map, actor, target, timestamp) | ✅ | `web/src/components/AuditList.tsx`, `web/src/lib/auditFormat.ts` |
| Queue v2 backed by `/api/imports` with retry, status badge, error inline | ✅ | `web/src/pages/QueuePage.tsx` |
| Nav-rail queue badge wired to in-flight count (count > 0) | ✅ | `web/src/components/Shell.tsx` |
| Members editor (invite dialog with auth-mode-aware password field, role select, remove) | ✅ | `web/src/components/settings/LibraryMembersSection.tsx` |
| Library profile editor (name + description) | ✅ | `web/src/components/settings/LibraryProfileSection.tsx` |
| Sysadmin server-admin section (server settings + users list with role + password reset) | ✅ | `web/src/components/settings/ServerAdminSection.tsx` |
| Keyboard shortcuts (`useKeyboardShortcuts` hook) on `LibraryPage`: `/` and `cmd+k` focus search, arrow keys navigate grid, `enter` opens, `escape` closes | ✅ | `web/src/hooks/useKeyboardShortcuts.ts`, `web/src/pages/LibraryPage.tsx` |
| `AssetGrid` scroll-into-view for keyboard-selected card | ✅ | `web/src/components/AssetGrid.tsx` |
| Routes added: `/audit`, `/admin/audit`. Disabled audit nav-rail entry replaced with the live link | ✅ | `web/src/App.tsx`, `web/src/components/Shell.tsx` |
| New TanStack hooks + queryKeys: `useLibraryAudit`, `useAdminAudit`, `useAssetAudit`, `useProjectAudit`, `useImports`, `useLibraryMembers`, `useAdminUsers`, `useServerSettings` | ✅ | `web/src/hooks/useDataQueries.ts`, `web/src/lib/queryKeys.ts` |
| Mutation hooks: `usePatchLibrarySettings`, `useRetryImport`, `useInviteMember`, `usePatchMember`, `useRemoveMember`, `useCreateSubsetVersion`, `usePatchUserRole`, `useResetUserPassword`, `usePatchServerSettings`, `usePatchLibrary` | ✅ | `web/src/hooks/useMutations.ts` |

M7 is complete for §21 scope. Notes:

- **SSE import progress stays deferred.** Imports are still synchronous;
  the queue surfaces only `failed` rows (and any other non-`done` stages
  if/when the pipeline goes async). The `imports.failed` path needed for
  retry is real, but `extracting`/`detecting`/`thumbnailing` are not
  emitted until the backend goes async — see `docs/archive/technical-design.md` §9.1.
- **Local-mode invites assume admin-set-password.** OIDC invites require
  the user to have signed in at least once before the membership can be
  attached — there's no "create identity in IdP" path. Both modes share the
  same membership write once the user row exists.
- **Library audit endpoint returns library-scoped events only.** The
  sysadmin `/api/admin/audit` returns `library_id IS NULL` events. We do
  not yet merge cross-library library-scoped events into the sysadmin
  feed; tech-design §13 mentioned that as a possible v2 view but it isn't
  needed for v1.
- **Last-admin and last-sysadmin guards are server-side only.** The
  members editor and admin users list rely on the server returning 409;
  the SPA doesn't disable the offending UI.
- **Subset action is wired but not surfaced in the SPA yet.** The handler
  + mutation hook exist; the detail-panel "create subset version" button
  is a follow-up so we don't bloat M7 with a file-picker UI.

## Foundations the tech design assumes but hasn't been built yet

These are not "late" — they belong to later milestones per §21 — but listing
them here makes the gap visible while we work on subsequent slices so we
don't assume something exists and then discover it doesn't.

| Missing thing | Needed by | Expected home |
|---|---|---|
| SSE broadcaster + `/api/imports/events` + `/api/imports/:id/events` | Once imports go async (background pipeline) | `internal/services/events/` (or alongside audit) + `internal/handlers/api/imports.go` |
| Real-Postgres integration tier exercising tsvector + advisory locks (§22 decision 5) | Verifying the search-vector trigger + GIN behaviour the SQLite tests can't see | A new test target / Docker-Compose Postgres fixture |
| `POST /api/libraries` (library creation flow) | Multi-library deployments — sysadmins / users can't yet create a second library through the API | `internal/handlers/api/library.go` + UX surface (sidebar / settings) |

**Delivered since the 2026-04-25 audit (Alpha leftover-features rollup):**

- `DELETE /api/libraries/:id` + danger-zone UI (PER-70).
- Sysadmin audit `?scope=all` for cross-library activity (PER-74).
- Subset-version file-picker dialog wiring the M7 backend handler (PER-71).
- Last-admin / last-sysadmin disabled-state guard in the SPA (PER-75).
- Bootstrap nav-after-success fix + auth-mode cache collapse (PER-68).
- Local-mode `POST /api/auth/refresh` + proactive in-window refresh in
  `getAccessToken`. The "Local mode has no refresh endpoint yet" caveat
  in `architecture.md` §SPA auth is no longer accurate.

**Delivered since the previous audit (landed with M6/M7):**

- Real thumbnail pipeline (`internal/services/thumbnail/`) — raster, audio
  waveform, font sample. Wired into `postImport`, honours toggles.
- `/api/imports`, `/api/imports/:id`, `/api/imports/:id/retry` — list +
  detail + retry against a retained source blob.
- Audit viewers (`/api/audit`, `/api/assets/:id/audit`,
  `/api/projects/:id/audit`, `/api/admin/audit`) + the matching SPA pages.
- Library admin settings editor + members management + library profile +
  sysadmin server-settings + sysadmin user role/password editor.
- Asset-version subset action (§9.3) — backend handler + mutation hook
  (UI picker still TODO; see new gap row).
- Keyboard shortcuts per UI §5.3 (`/`, `cmd+k`, arrows, `enter`, `escape`).

**Delivered with M3/M4/M5 (kept for context):**

- Audit writer (`internal/services/audit/audit.go`).
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
  out in `docs/archive/technical-design.md`. Worth folding into §8.1 on the next
  design-doc pass.
- **`importResponse` reports `file_count` + `package_kind` instead of a
  single-file `file_id`.** The M1 shape assumed one file per import; M2 made
  that wrong for `.unitypackage`. Noted because API consumers should read the
  current handler, not the draft in §16.
