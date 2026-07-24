# Architecture

Current-state reference for Damson. The build-time design docs in
[`docs/archive/`](archive/) describe the *why* and the trade-offs;
this doc describes *what is in the tree today*.

Companions:

- [`docs/CONVENTIONS.md`](CONVENTIONS.md) — coding standards, handler
  patterns, test helpers
- [`docs/design-system.md`](design-system.md) — frontend design system
- [`docs/progress.md`](progress.md) — milestone-by-milestone status
- [`AGENTS.md`](../AGENTS.md) — quick-start guidance for agentic coding

---

## 1. What Damson is

A self-hosted digital asset manager for game-development assets.
Single Go binary that embeds a React SPA, fronted by a reverse proxy,
backed by Postgres and an S3-compatible object store (or a local
filesystem for single-node installs).

Core concerns:

- **Library-based group ownership** — a `libraries` row is the tenancy
  boundary. Users gain access via `library_memberships`; within a
  library, every member sees the same data.
- **Content-addressed storage** — file bytes live keyed by SHA-256.
  Postgres holds metadata; the same texture appearing in five packages
  occupies one object.
- **Versions track the external pack version** — an `AssetVersion`'s
  identity is its `store_version` (e.g. `1.3`), parsed from the upload
  filename. Re-extraction rewrites a version's files in place rather
  than spawning a new one; a new version only comes from uploading
  another pack. Labels are editable and may be blank (unversioned).
- **Projects are declarative** — a project pins specific versions;
  upgrade is a logged, reversible action.
- **Audit is a boundary** — every library-scoped write writes an
  `audit_log` row in the same DB transaction.

The MVP is M0–M7; see [`docs/progress.md`](progress.md) for what has
landed, what is partial, and what is deferred.

---

## 2. Stack

**Backend.** Go 1.26, Gin, GORM (Postgres in production and tests —
the suite runs against a real Postgres via `DAMSON_TEST_POSTGRES_DSN`).
Auth via `coreos/go-oidc/v3` + `oauth2` (OIDC mode)
or `golang-jwt/jwt/v5` HS256 (local mode). S3 access via
`aws-sdk-go-v2`. SPDX matching via `google/licensecheck`. Thumbnails
are pure Go: stdlib `image` + `golang.org/x/image/{webp,draw,font}`,
plus `hajimehoshi/go-mp3` and `jfreymuth/oggvorbis` for audio. WAV is
parsed in-tree. No CGo.

**Frontend.** React 19, TypeScript (strict), Vite 8. TanStack Query 5
for server state. React Router 7 (data router). Tailwind 4 with OKLCH
design tokens. Radix UI headless primitives + hand-rolled components
under `web/src/design-system/`. `class-variance-authority` for
variants; `clsx` + `tailwind-merge` via a `cn()` helper.
`@tanstack/react-virtual` for the asset grid. `@fontsource/inter` +
`@fontsource/jetbrains-mono` self-hosted fonts.

**Tooling.** Task (`Taskfile.yml`) for all developer workflows. `air`
for Go hot-reload. Vitest 4 with two projects: `unit` (jsdom) and
`storybook` (Playwright Chromium running every `*.stories.tsx` with
axe-core a11y checks). Storybook 10. Multi-stage Dockerfile (Node →
Go → Alpine).

Module path: `github.com/thomas-elliott/damson`.

---

## 3. Repository layout

```
damson/
├── Taskfile.yml
├── Dockerfile
├── damson.go                 # //go:embed for migrations + SPA (see §11)
├── cmd/server/main.go        # entry point: HTTP server, middleware, wiring
├── internal/
│   ├── auth/                 # Authenticator interface + local + oidc impls
│   ├── config/               # env loading, panic-on-missing
│   ├── db/                   # gorm.Open, migration runner
│   ├── handlers/api/         # one file per feature; receivers on *Handler
│   ├── models/               # GORM models, one file per entity
│   ├── services/
│   │   ├── audit/            # audit log writer (same-tx with mutation)
│   │   ├── extract/          # .unitypackage extractor
│   │   ├── licence/          # SPDX + Unity EULA detection
│   │   ├── storage/          # storage.Service interface + LocalFS + S3
│   │   └── thumbnail/        # raster / audio / font workers
│   └── web/                  # SPA static handler
├── database/migrations/      # NNN_*.sql, applied on startup
├── web/                      # React SPA (Vite); built into web/static/
└── docs/                     # this directory
```

`damson.go` lives at the repo root because `//go:embed` cannot
traverse upward. It exports `damson.MigrationsFS` and
`damson.WebStaticFS`, consumed by `internal/db` and `internal/web`.
`web/static/.gitkeep` exists so `//go:embed all:web/static` succeeds
before the SPA is built.

---

## 4. Data model

The authoritative schema is the SQL in `database/migrations/`; this
section sketches the shape and relationships.

### 4.1 Identity, libraries, audit

- **`users`** — `id`, `subject` (OIDC `sub` claim or `local:<email>`),
  `email`, `display_name`, `role` (`user` | `sysadmin`),
  `current_library_id`, `password_hash` (local mode only),
  `tokens_valid_after` (local-mode revocation watermark).
- **`libraries`** — `id`, `name`, `description`.
- **`library_memberships`** — `library_id` × `user_id` × `role`
  (`admin` | `member`); `unique(library_id, user_id)`.
- **`audit_log`** — `library_id` (nullable for server-level events),
  `user_id`, `action_type`, `target_type`, `target_id`, `diff_json`.
  Indexed by `(library_id, created_at desc)`,
  `(library_id, target_type, target_id, created_at desc)`,
  `(user_id, created_at desc)`, `(action_type, created_at desc)`.

### 4.2 Assets

- **`assets`** — `library_id`, `name`, `description`, `source_url`
  (validated http/https only), `licence_id`, `current_version_id`,
  `thumbnail_object_key`, `thumbnail_content_type`, `needs_review`,
  `created_by_user_id`, `search_vector` (Postgres tsvector,
  trigger-maintained).
- **`asset_versions`** — `asset_id`, `store_version` (the version
  identity, e.g. `1.3`; blank = unversioned), `version_label` (an
  optional editable label), `notes`, `import_date`,
  `original_package_object_key` (set for `.unitypackage`),
  `package_kind` (`unitypackage` | `zip` | `loose` | null). A partial
  unique index enforces `unique(asset_id, store_version)` for non-blank
  store versions, so unversioned rows can coexist.
- **`asset_files`** — `asset_version_id`, `relative_path`,
  `size_bytes`, `content_hash`, `object_key` (== `content_hash`),
  `detected_type`, `unity_guid`. `unique(asset_version_id,
  relative_path)`.
- **`asset_imports`** — `asset_id`, `stage` (`uploaded` |
  `extracting` | `detecting` | `thumbnailing` | `done` | `failed`),
  `error`. One row per import; the queue surface filters non-`done`.

### 4.3 Tags, projects, collections, licences

- **`tags`** — `library_id`, `path` (materialised path,
  `environment/trees/deciduous`), `parent_id`. `unique(library_id,
  path)`. Path length capped at 1 KiB.
- **`asset_tags`** — `asset_id` × `tag_id`.
- **`projects`** — `library_id`, `name`, `description`,
  `target_distribution` (`commercial` | `open-source` | `personal`
  | `unspecified`). Case-insensitive unique name per library.
- **`project_asset_pins`** — `project_id` × `asset_id` ×
  `pinned_version_id`. `unique(project_id, asset_id)`. Re-pinning
  the same asset with a different version emits a
  `project.pin.upgrade` audit event.
- **`collections`** — `library_id`, `name`, `description`.
- **`collection_assets`** — `collection_id` × `asset_id`.
- **`licences`** — `asset_id`, `detected_type` (SPDX id |
  `unity-eula` | `custom` | `unknown`), `detection_confidence`,
  `canonical_diff` (nullable, populated when the text is a
  modified version of a known licence), `original_text`,
  `attributes_json` (commercial / modification / redistribution /
  attribution / share-alike / open-source — tri-state booleans;
  `null` = unknown), `reviewed_by_user_id`, `reviewed_at`.

### 4.4 Settings

- **`server_settings`** — singleton row, sysadmin-managed:
  `max_upload_bytes`, `licence_kb_auto_update`.
- **`library_settings`** — one row per library, library-admin-managed:
  `theme_default`, `grid_density_default`,
  `retain_original_packages`, `dedupe_enabled`,
  `thumbnails_{raster,audio,font,unity_preview}_enabled`,
  `audit_retention_days` (nullable; consumed by the background purge
  worker in `internal/services/audit/purge.go` — see §12).

Library-scoped entities all carry `library_id` directly.
`created_by_user_id` / `pinned_by_user_id` are **attribution, not
ownership** — they do not affect access rules.

### 4.5 Migrations

SQL-only, in `database/migrations/NNN_*.sql`. The runner
(`internal/db/migrate.go`) reads them via `damson.MigrationsFS`,
applies in lexicographic order inside a Postgres advisory lock,
tracks applied files in `schema_migrations`. Each file is wrapped
in `BEGIN; … COMMIT;` and uses `IF NOT EXISTS` throughout.

**Tests** run against a real Postgres (each test gets an isolated
schema with the SQL migrations applied), configured via
`DAMSON_TEST_POSTGRES_DSN`. A schema change must still land in **both**
an SQL migration **and** the corresponding GORM struct tags, since the
model tags remain the second source of truth. Postgres-specific
behaviour (tsvector, JSONB operators, `pg_advisory_lock`) is exercised
directly.

---

## 5. Object storage

`internal/services/storage.Service` is the abstraction; two concrete
impls picked at startup by `DAMSON_STORAGE_MODE`:

- **`local`** (default) — content-addressed under
  `<root>/blobs/<hh>/<hash>` and `<root>/thumbs/<hh>/<hash>`.
  Atomic rename, SHA-256 dedup. `pathFor` enforces hex-only on the
  hash so a bug at the HTTP boundary cannot escalate to a path
  traversal.
- **`s3`** — `aws-sdk-go-v2` with endpoint override + path-style
  for MinIO. HEAD-before-PUT dedup. Same key layout as LocalFS so
  a deployment can migrate with a plain copy.

Two key prefixes:

- `blobs/<hh>/<hhhhh…>` — every file referenced by
  `asset_files.object_key` and every original `.unitypackage`
  archive referenced by `asset_versions.original_package_object_key`.
- `thumbs/<hh>/<hhhhh…>` — generated thumbnails. Derived data;
  rebuildable.

`object_key == content_hash`. An asset file row is a *reference* to
a blob — two assets sharing a texture share `object_key`.

Garbage collection is deferred. A later sweep walks references and
deletes unreferenced blobs.

---

## 6. Authentication and authorisation

### 6.1 Modes

Exactly one authenticator is active per deployment, picked by
`DAMSON_AUTH_MODE`. Both implement the same `Authenticator`
interface (`internal/auth/auth.go`); middleware does not know which
is in play.

**`local`** (default) — bcrypt password hashes in
`users.password_hash`, HS256 JWTs, 1 h TTL (`auth.LocalTokenTTL`).
JWT `iss` and `aud` are derived from `DAMSON_BASE_URL` so a token
leaked from one Damson cannot replay against another.
`users.tokens_valid_after` is a revocation watermark — JWTs with
`iat` before that instant are rejected (used by member removal).
Synthetic subject `"local:<email>"` so the `users.subject` unique
index still works.

Login is gated by `auth.LoginThrottle` (in-memory, per-IP +
per-email; 5 burst, 1/min refill). 429 on empty bucket. Same bucket
gates `POST /api/auth/bootstrap`.

Password floor of 8 chars (`auth.MinPasswordLength`) at every write
site: env seed, HTTP bootstrap, admin-driven invite, admin password
reset.

> Recommended for development and small single-user installs only.
> No MFA, in-memory rate-limiting only (no shared store across
> replicas), minimum-length password floor only.

**`oidc`** — IdP-issued bearer tokens validated via
`coreos/go-oidc/v3`. Requires `DAMSON_OIDC_ISSUER_URL`,
`DAMSON_OIDC_CLIENT_ID`, `DAMSON_OIDC_AUDIENCE`. The SPA runs the
interactive flow with `oidc-client-ts` and `automaticSilentRenew`
keeps the bearer fresh.

### 6.2 Bootstrap

Both modes share `auth.BootstrapFirstAdmin`: the **first user on an
empty system** (no other users, no libraries) is promoted to
`role='sysadmin'`, a default `"Personal"` library is created, the
user becomes its admin member, and `current_library_id` is set — all
in one transaction. It's a no-op once any other user or library
exists, so retrying is safe.

Local mode has two entry points:

- **HTTP bootstrap** (`POST /api/auth/bootstrap`, reachable when
  no users exist; `GET /api/auth/mode` reports `needs_bootstrap`).
- **Env seed** (`SeedLocalAdmin`, active when both
  `DAMSON_LOCAL_ADMIN_EMAIL` and `_PASSWORD` are set).

OIDC mode needs no configuration here: the first IdP user to sign in
runs the same bootstrap. Identity comes from the `sub` claim alone;
if the IdP sends no email, the user is flagged `needs_profile` and
the SPA collects one on first login (`PATCH /api/me`).

### 6.3 Authorisation

Two role hierarchies, independent:

- `users.role` — `user` (default) | `sysadmin`. Server-level.
  Creates libraries, manages user records, sees server-level audit.
- `library_memberships.role` — `admin` | `member`. Per-library.
  `admin` adds: invite/remove members, edit library settings.

Enforcement:

1. `auth.Middleware(authenticator)` verifies the bearer token,
   upserts-or-loads the user, attaches it via `auth.GetUser(c)`.
2. Library-scoped routes layer `h.requireLibrary()` — resolves
   `current_library_id`, verifies the membership row exists,
   attaches `(*Library, *LibraryMembership)` via `getLibrary(c)`
   / `getMembership(c)`. **403 on any gap** (never 404, so
   membership existence is not leaked).
3. `requireSysadmin(c)` gates server-admin endpoints explicitly.

A user who is not a member of their `current_library_id` cannot
read or write it — fail-fast at the middleware before the handler
sees data.

### 6.4 HTTP security posture

- `securityHeaders` middleware sets CSP (`default-src 'self';
  script-src 'self'; style-src 'self' 'unsafe-inline'; img-src
  'self' data: blob:; font-src 'self' data:; connect-src 'self';
  object-src 'none'; base-uri 'self'; frame-ancestors 'none';`),
  `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`. TLS / HSTS is the proxy's job.
- `jsonBodyCap` wraps every non-multipart request body in
  `http.MaxBytesReader(1 MiB)`. Multipart uploads
  (`POST /api/imports`) bypass this — they have a streaming
  pipeline.
- `http.Server` carries explicit `Read{Header,}Timeout`,
  `WriteTimeout`, `IdleTimeout` so slow-loris cannot tie up an
  extraction goroutine.
- `GET /api/files/:objectKey` always responds with
  `Content-Type: application/octet-stream` +
  `Content-Disposition: attachment` + `Content-Length`.
  `GET /api/thumbs/:objectKey` is restricted to a small
  `image/{png,jpeg,webp,gif}` allow-list; anything else falls back
  to `application/octet-stream`. Stored XSS via uploaded HTML/SVG
  cannot reach the SPA's `localStorage` token.
- `source_url` PATCH validates http/https + a present host;
  `javascript:` / `data:` / `file:` are rejected.
- All HTTP entry points enforce `^[0-9a-f]{64}$` on `objectKey`
  parameters; LocalFS re-checks the constraint in `pathFor`.
- The SPA stores the bearer in `localStorage` (both modes). This
  is a pragmatic choice for a self-hosted tool — bearer-in-header
  sidesteps CSRF — and depends on the served-file hardening above
  to keep XSS from being an exfil channel.

---

## 7. Request lifecycle

`internal/handlers/api/handler.go` wires routes:

1. Public group: `/api/auth/*`, `/api/health`.
2. `authed` group: `auth.Middleware(authenticator)` runs first.
3. `scoped` sub-group: layers `requireLibrary()`.
4. Sysadmin endpoints: `requireSysadmin(c)` inside the handler.

Handlers are receiver methods on a single `*Handler` struct
(`internal/handlers/api/handler.go`), constructed via
`api.New(api.Deps{...})` in `cmd/server/main.go`. Tests construct
via `newTestHandler(t)` which wires a per-test Postgres schema (all SQL
migrations applied) + a `LocalFS` storage rooted in `t.TempDir()`. One
file per feature
(`assets.go`, `library.go`, `imports.go`, …).

Errors use `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})`.
Inside `Transaction(func)` the sentinel constructors
`errBadRequestf` / `errConflictf` plus the shared
`respondGORM(c, err, "<entity> not found")` mapper are the single
source of truth for status mapping. 500 always returns the generic
`"internal error"` string — no leaked internals.

Logging is `log/slog` (JSON in production, text in dev). A custom
`requestLogger()` middleware (not `gin.Logger()`) emits a
status/path line per response. Handlers carrying diagnostic detail
(extractor errors, validation beyond the binding layer) `slog.Warn`
the underlying error before `AbortWithStatusJSON` — the JSON body
alone does not appear in logs.

---

## 8. Import pipeline

Background-first by design, currently synchronous in code (M2+).
Stage state is tracked in `asset_imports`; the queue surface
filters non-`done`.

```
[1] Upload streamed to temp + SHA-256
[2] PutBlob (skip if already present)
[3] Create asset (needs_review=true) + first version
[4] If .unitypackage → extract (gzipped tar or raw tar)
      ├─ Stream each inner blob back to storage
      ├─ Write asset_files rows (path, size, hash, unity_guid)
      ├─ Harvest preview.png if present → thumbnail candidate
      └─ Set package_kind='unitypackage', retain original
    Otherwise → single file already persisted in [2]
[5] Licence detection on candidate licence files
[6] Thumbnail generation (raster / audio / font / icon fallback)
[7] Update search_vector (Postgres trigger)
[8] Emit audit entry: asset.import
```

Failure path: extract failures persist a stub asset +
`stage='failed'` import row so the queue surface can retry via
`POST /api/imports/:id/retry`. The retry path requires an
`asset_files` row for non-Unity imports (Unity case still uses
`original_package_object_key`) — stops the retry from creating a
manifest-empty asset version when the upload bytes were never
persisted.

**Subset version** — `POST /api/assets/:id/versions/:vid/subset`
creates a new version whose `asset_files` rows are a subset of the
source version. Object keys are reused — no re-upload. Audited as
`asset.version.subset`. Backend landed in M7. The file-picking UI is
the **scoped file browser** (`web/src/components/filebrowser/`): a
full-page tree browser entered via "Browse files" from a browsable
asset (current version with more than one file), where the user
selects files and creates a subset. It superseded the original flat
checkbox-list dialog (`SubsetVersionDialog`, retired). Source version
is the asset's current version (the only one whose files are loaded
into the panel today).

**Async pipeline** — currently deferred. SSE
`/api/imports/events` and `/api/imports/:id/events` will land when
the backend goes async; the SPA already tolerates a "thumbnail
pending" state.

### 8.1 Unity package extraction

`internal/services/extract/unitypackage.go`. Sniffs the first two
bytes: `1f 8b` → gzipped tar; otherwise raw tar. Each tar entry
sits inside a directory named by a Unity GUID containing `asset`,
`asset.meta`, `pathname`, optional `preview.png`. Bundles per
GUID; for each complete bundle reads `pathname`, streams `asset`
to storage, records `(guid, path, size, hash, object_key)`.

Extraction safety: per-entry size cap, total uncompressed size
cap, reject paths containing `..` or absolute prefixes
(zip-slip). Re-extraction reads the retained original from
storage and produces a new version.

Zip-format `.unitypackage` (Unity Hub exports) is not supported
yet — current uploads fail at import with a descriptive 400. See
`progress.md` "Stretch goals".

---

## 9. Licence detection

`internal/services/licence/detect.go`:

1. Locate candidate licence text — root-level files (`LICENSE`,
   `LICENSE.md`, `LICENSE.txt`, `COPYING`, `COPYING.LESSER`,
   `UNLICENSE`), or `.unitypackage` Unity-paths matching the same.
2. Run `google/licensecheck` on each candidate. Returns matched
   SPDX identifiers with confidence and modification info.
3. Fall back to a Unity Asset Store EULA fingerprint — a regex
   on "Unity Asset Store" + "EULA"/"End User License Agreement"
   in the first 4 KB. Not a canonical-text match.
4. Otherwise → `detected_type='unknown'`; user enters attributes
   manually in the detail panel.

Result writes to `licences` with `original_text` verbatim,
`canonical_diff` populated when modified, `attributes_json`
filled from a built-in SPDX → attributes map. The shared
`LicenceStatusFor` derives a status (`ok` / `warn` / `err` /
`unk`) from detected type + attributes; reused by both the assets
list and the projects view.

---

## 10. Thumbnails

`internal/services/thumbnail/`. One worker per category,
dispatched by detected type:

- **Raster** — stdlib `image` + `golang.org/x/image/webp`,
  resample to 512 px on the longer edge with
  `golang.org/x/image/draw` and `draw.CatmullRom`. JPEG
  (quality ~85) for inputs decoded as JPEG/WebP with no
  transparent pixels; PNG otherwise.
- **Audio** — in-tree WAV decoder (PCM int 8/16/24/32 + IEEE
  float), `github.com/hajimehoshi/go-mp3`,
  `github.com/jfreymuth/oggvorbis`. Peak-binned waveform PNG.
  Decoders that need a `ReadSeeker` (mp3 / ogg) buffer the
  upload — bounded by the upload-size cap.
- **Font** — `golang.org/x/image/font/opentype` rendering "The
  quick brown fox…" sample text → PNG.
- **`.unitypackage`** — first harvested `preview.png` from the
  extractor.
- **Otherwise** — none; SPA renders a file-type icon.

`assets.thumbnail_content_type` stored alongside
`thumbnail_object_key` so the serving handler sets the right
`Content-Type` without sniffing bytes. Honours
`library_settings.thumbnails_*` toggles. Failures log+swallow —
the SPA falls back to icons.

---

## 11. Search

A `search_vector` column on `assets`, maintained by triggers on
`assets` (`name`, `description`) and `asset_tags` (path
aggregation). Postgres uses
`search_vector @@ plainto_tsquery('simple', $1)` with a GIN index.

The SQLite test tier falls back to `LIKE` on `name` /
`description` — the handler dispatches on `db.Dialector.Name()`.
SQLite-backed handler tests therefore exercise the `LIKE` branch;
`search_vector` and triggers are validated only when a real-
Postgres integration tier comes online.

`applyAssetFilters` composes filters: `q` (search), `tag_id` /
`tag_path` (matches descendants), `licence_status`,
`licence_spdx`, `project_id`, `collection_id`, `source_host`
(LIKE with `%` / `_` escaped), `needs_review`, `package_kind`.
`/api/search` is an alias of `/api/assets`.

`licence_status` is post-SQL: SQL returns rows matching the other
filters, then the handler drops rows whose derived status doesn't
match. At list-page size (limit ≤ 200) this is cheap and avoids
adding a derived column.

---

## 12. Audit

- Written in the same DB transaction as the domain mutation it
  describes. The service panics on missing action / target type
  rather than silently dropping the row.
- `diff_json` is `{"before": …, "after": …}` for updates,
  `{"created": …}` for creates, `{"deleted": …}` for deletes.
- `library_id` populated for library-scoped events; `NULL` for
  server-level events (library created/deleted, user role
  changed, server settings updated).
- Surfaces:
  - Per-asset / per-project (inside the current library).
  - Library audit (`/api/audit`).
  - Server audit (sysadmin only; `/api/admin/audit`). Two scopes
    via `?scope=` (PER-74): `server` (default — `library_id IS NULL`
    only, the original behaviour) or `all` (server + library-scoped
    events across every library, with each row's `library_name`
    joined in for the SPA to render).
- Retention: per-library via `library_settings.audit_retention_days`
  (nullable — unset means "keep forever"). A background sweep in
  `internal/services/audit/purge.go` deletes `audit_log` rows older
  than that window for each library that has one set. Sweep cadence
  is `DAMSON_AUDIT_PURGE_INTERVAL` (default `24h`). Server-level
  events (`library_id IS NULL`) are not yet purged — `server_settings`
  has no retention column.

---

## 13. Configuration

Loaded once at startup by `internal/config/config.go`. A `.env` in
the repo root is auto-loaded; real env vars win on conflict.
Missing required values panic at startup.

Secrets (`DAMSON_POSTGRES_DSN`, `DAMSON_JWT_SIGNING_KEY`,
`DAMSON_LOCAL_ADMIN_PASSWORD`, `DAMSON_S3_ACCESS_KEY`,
`DAMSON_S3_SECRET_KEY`) also accept a `<NAME>_FILE` variant that
reads the value from a path — for Docker Secrets. Setting both the
direct var and the `_FILE` variant panics at startup.

The only always-required value is `DAMSON_POSTGRES_DSN` (or its
`_FILE` form).

### 13.1 Always

| Var | Default |
|---|---|
| `DAMSON_POSTGRES_DSN` | **required** |
| `DAMSON_LISTEN_ADDR` | `:8080` |
| `DAMSON_BASE_URL` | `http://localhost:8080` |
| `DAMSON_LOG_LEVEL` | `info` |
| `DAMSON_AUTH_MODE` | `local` (or `oidc`) |
| `DAMSON_STORAGE_MODE` | `local` (or `s3`) |
| `DAMSON_AUDIT_PURGE_INTERVAL` | `24h` (Go duration; non-positive falls back to default) |

### 13.2 `DAMSON_AUTH_MODE=local` (default)

| Var | Default / required |
|---|---|
| `DAMSON_JWT_SIGNING_KEY` | optional; if empty, 32 random bytes are generated and persisted |
| `DAMSON_JWT_KEY_FILE` | `./data/jwt.key` |
| `DAMSON_LOCAL_ADMIN_EMAIL` | optional — when set with `_PASSWORD`, seeds first admin |
| `DAMSON_LOCAL_ADMIN_PASSWORD` | optional |

### 13.3 `DAMSON_AUTH_MODE=oidc`

| Var | Required? |
|---|---|
| `DAMSON_OIDC_ISSUER_URL` | required |
| `DAMSON_OIDC_CLIENT_ID` | required |
| `DAMSON_OIDC_AUDIENCE` | required |

Frontend env (Vite, OIDC only): `VITE_OIDC_AUTHORITY`,
`VITE_OIDC_CLIENT_ID`, `VITE_OIDC_SCOPE`, `VITE_OIDC_REDIRECT_URI`,
`VITE_OIDC_SILENT_REDIRECT_URI`, `VITE_OIDC_POST_LOGOUT_REDIRECT_URI`.

### 13.4 `DAMSON_STORAGE_MODE=local` (default)

| Var | Default |
|---|---|
| `DAMSON_STORAGE_LOCAL_PATH` | `./data/blobs` |

### 13.5 `DAMSON_STORAGE_MODE=s3`

| Var | Default / required |
|---|---|
| `DAMSON_S3_ENDPOINT` | required |
| `DAMSON_S3_REGION` | required |
| `DAMSON_S3_BUCKET` | required |
| `DAMSON_S3_ACCESS_KEY` | required |
| `DAMSON_S3_SECRET_KEY` | required |
| `DAMSON_S3_FORCE_PATH_STYLE` | `true` |

See `.env.example` for the canonical list.

---

## 14. Frontend

### 14.1 Layout shell

`AppShell` composes `NavRail` + `FilterSidebar` + main area.
Routes (`web/src/App.tsx`):

```
/                       → redirect to /library
/library                → asset grid + filter sidebar
/library/:assetId       → grid + detail panel open
/queue                  → filtered library view (needs_review=true)
/projects               → projects index
/projects/:id           → project detail
/collections            → collections index
/collections/:id        → collection detail
/audit                  → library audit log
/admin/audit            → server audit log (sysadmin)
/settings               → settings (theme, density, thumbnails,
                          retention, members, server admin)
/login                  → local-mode login
/bootstrap              → local-mode first-admin form
```

The asset detail panel is URL-addressable so a pasted link opens
the right asset. It is user-resizable (overlay mode only): a left-edge
drag handle controls width, persisted to `localStorage` under
`damson.detail.w` (default 440 px, min 320 px); a header button toggles
fullscreen (covers the full grid area). Viewports ≤ 640 px auto-fullscreen.

### 14.2 State

- **Server state** through TanStack Query 5. Query keys live in
  `lib/queryKeys.ts` only; data hooks in `hooks/useDataQueries.ts`
  return a normalised `{ data, loading, error, reload }` shape.
  Mutations live in `hooks/useMutations.ts`.
- **UI state** local to components; filter / search state lives
  in URL search params (the URL is the source of truth).
- **Theme** in `localStorage`, seeded from
  `library_settings.theme_default`. A pre-React boot in `main.tsx`
  applies the stored theme so light-mode users do not see a dark
  flash.

### 14.3 API client

`src/api.ts` — `fetchJson<T>` / `fetchMaybeJson<T>` attach the
bearer, handle 401 (silent renew on OIDC; `AuthRedirectError` on
local), retry once, throw `ApiError`. **Never `fetch` directly
from components** — the only exceptions are pre-auth endpoints in
`lib/authMode.ts` and `lib/localAuth.ts`.

`uploadFile(url, file, onProgress)` uses `XMLHttpRequest` for
upload progress (fetch lacks it).

### 14.4 SPA auth flow

1. App boots → `useAuthMode()` hits `/api/auth/mode` (public) and
   the result is cached in TanStack Query (single source of truth —
   the previous parallel `cached` singleton in `lib/authMode.ts` was
   removed in the Alpha leftover-features rollup, PER-68).
2. **OIDC mode** — `AuthGuard` uses `oidc-client-ts`'s
   `UserManager`; missing/expired user → `signinRedirect` (browser
   leaves the SPA).
3. **Local mode** — `AuthGuard` reads the stored JWT
   (`damson.local.token`); missing/expired → `<Navigate to="/login">`.
4. `lib/token.ts:getAccessToken()` provides the bearer to
   `fetchJson` regardless of mode. In local mode it calls
   `maybeRefreshLocalSession` first — when the stored session is
   within five minutes of expiry, it swaps in a fresh JWT via
   `POST /api/auth/refresh` (concurrent fetches share one in-flight
   refresh). Failures fall through to step 5.
5. On 401, `api.ts` tries silent renew (OIDC) or throws
   `AuthRedirectError` (local). The dedicated local refresh keeps
   typical sessions from ever reaching the 401 path.

### 14.5 Grid

`@tanstack/react-virtual`. Column count comes from a
`ResizeObserver`-driven hook; keyboard selection scrolls into view
via `virtualizer.scrollToIndex`. `LibraryPage` keyboard shortcut
bindings are stable across renders — handlers read state from a
ref instead of being re-created on each refetch, so
`useKeyboardShortcuts` does not rebind on every paint.

Keyboard shortcuts (`useKeyboardShortcuts`): `/` and `cmd+k` focus
search, arrow keys navigate, `enter` opens, `escape` closes.

### 14.6 Design system

Spec: [`docs/design-system.md`](design-system.md). Three tiers
under `web/src/design-system/`: **primitives** (Button, Input,
Dialog, …), **compositions** (AppShell, NavRail, DetailPanel, …),
**domain** (LicencePill, AssetCard, ProjectCard, …). Every
component ships a `*.stories.tsx`; Vitest runs them as real
Chromium tests with axe-core a11y checks (`a11y.test = "error"`).

---

## 15. API surface

REST-ish JSON under `/api`. The list mirrors
`internal/handlers/api/handler.go`. Library-scoped endpoints
operate against the user's `current_library_id` and are wrapped by
`requireLibrary()`. The `/api/libraries/:id/...` family is gated
inside each handler.

```
# Public
GET    /api/auth/mode
POST   /api/auth/login                # local mode (rate-limited)
POST   /api/auth/bootstrap            # local mode (rate-limited)
GET    /api/health

# Session / user
GET    /api/me
GET    /api/libraries
POST   /api/me/current-library
POST   /api/auth/refresh              # local mode (re-issues JWT)

# Libraries (cross-library; handler-gated)
PATCH  /api/libraries/:id
DELETE /api/libraries/:id
GET    /api/libraries/:id/members
POST   /api/libraries/:id/members
PATCH  /api/libraries/:id/members/:userId
DELETE /api/libraries/:id/members/:userId

# Server admin (sysadmin only)
GET    /api/admin/users
PATCH  /api/admin/users/:id
GET    /api/admin/settings
PATCH  /api/admin/settings
GET    /api/admin/audit               # ?scope=server (default) | all

# Assets (library-scoped)
GET    /api/assets
GET    /api/assets/:id
PATCH  /api/assets/:id
DELETE /api/assets/:id
POST   /api/assets/:id/review
GET    /api/assets/:id/audit
GET    /api/assets/:id/licence
PUT    /api/assets/:id/licence
POST   /api/assets/:id/licence/review
GET    /api/assets/:id/licence/detect
POST   /api/assets/:id/tags
DELETE /api/assets/:id/tags/:tagId
POST   /api/assets/:id/versions/:vid/reextract
POST   /api/assets/:id/versions/:vid/subset

# Imports (library-scoped)
POST   /api/imports
GET    /api/imports
GET    /api/imports/:id
POST   /api/imports/:id/retry

# Tags / licences (library-scoped)
GET    /api/tags
POST   /api/tags
DELETE /api/tags/:id
POST   /api/licences/detect

# Projects (library-scoped)
GET    /api/projects
POST   /api/projects
GET    /api/projects/:id
PATCH  /api/projects/:id
DELETE /api/projects/:id
GET    /api/projects/:id/audit
POST   /api/projects/:id/pins
DELETE /api/projects/:id/pins/:pinId

# Collections (library-scoped)
GET    /api/collections
POST   /api/collections
GET    /api/collections/:id
PATCH  /api/collections/:id
DELETE /api/collections/:id
POST   /api/collections/:id/assets
DELETE /api/collections/:id/assets/:assetId

# Search alias
GET    /api/search                    # same shape as /api/assets

# Settings (library-scoped; PATCH gated to library admin)
GET    /api/settings
PATCH  /api/settings

# Files (library-scoped)
GET    /api/files/:objectKey
GET    /api/thumbs/:objectKey

# Audit (library-scoped)
GET    /api/audit
```

Endpoints documented in the archived tech design but **not
implemented** are listed in `progress.md` "Foundations the tech
design assumes but hasn't been built yet" — notably `POST
/api/libraries` and SSE imports (`/api/imports/events`). `DELETE
/api/libraries/:id` and `POST /api/auth/refresh` (local mode) landed
in the Alpha leftover-features rollup.

---

## 16. Deployment

Multi-stage Dockerfile (no CGo; runtime image is slim):

1. `node:24-alpine` — `pnpm install --frozen-lockfile` + `vite build` → `web/static/`.
2. `golang:1.26-alpine` — `CGO_ENABLED=0 go build` with
   `-ldflags="-s -w"`, embedding the SPA via `//go:embed`.
3. `alpine:3.20` — `ca-certificates` + `tzdata`, non-root
   `damson` user, `HEALTHCHECK` hitting `/api/health`.

The binary serves the SPA at `/` with a fallback (any non-`/api`
route that is not a static asset returns `index.html`).

Releases are automated via GitHub Actions
(`.github/workflows/release.yml`): pushing a `v*.*.*` tag builds
and pushes to `ghcr.io/<owner>/damson` with `{version}`,
`{major}.{minor}`, `{major}` tags. `main-build.yml` builds
`main-<sha>` on every push to `main`.

### 16.1 First-run flow

1. Start the container. `DAMSON_POSTGRES_DSN` is the only
   always-required env var — local auth + local storage are the
   defaults; the JWT signing key is auto-generated on first boot.
2. Migrations run on startup inside a Postgres advisory lock.
3. First browser hit → `GET /api/auth/mode` returns
   `{ mode: "local", needs_bootstrap: true }`. The SPA renders
   `/bootstrap`.
4. The user submits email + password →
   `POST /api/auth/bootstrap` runs `BootstrapFirstAdmin` and
   returns a logged-in JWT.
5. Subsequent users sign in but land on "no library access yet"
   until a library admin invites them.

Alternatives:

- **Env seed (local).** Set both `DAMSON_LOCAL_ADMIN_EMAIL` and
  `DAMSON_LOCAL_ADMIN_PASSWORD` — first-admin creation runs at
  startup, the bootstrap page is skipped.
- **OIDC.** `DAMSON_AUTH_MODE=oidc` — the first user to sign in on
  an empty system runs the same bootstrap transaction (no admin
  email list). Users are keyed by their `sub`; one with no email
  claim fills it in on first login.

---

## 17. Confirmed decisions

Reproduced from the archived tech design §22 and updated where
the code diverges:

1. Module path `github.com/thomas-elliott/damson`.
2. Frontend OIDC library `oidc-client-ts`.
3. **Local auth mode is real and tested**, alongside OIDC. The
   archived tech design described OIDC only; local mode is
   `internal/auth/local.go` + `internal/handlers/api/auth.go`.
4. Upload path: in-server streaming (Go server hashes in-stream
   and puts to storage). Revisit only if the server becomes a
   bottleneck.
5. Import staging: real `asset_imports` table.
6. Integration test tier: SQLite-only today. A real-Postgres tier
   for tsvector / advisory locks is a follow-up.
7. Thumbnail pipeline: stdlib `image` + `golang.org/x/image/*`.
   No CGo, no third-party encoder. JPEG for photographic content,
   PNG for everything else.
8. Ownership model: library-based group ownership.
9. **Accent colour is yellow only** — no theme-swap. **Dark default,
   light override on `data-theme`.** Hand-rolled components on
   Radix headless primitives — no shadcn/ui.
