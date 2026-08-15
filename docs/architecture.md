# Architecture

Current-state reference for Damson. The build-time design docs in
[`docs/archive/`](archive/) describe the *why* and the trade-offs;
this doc describes *what is in the tree today*.

Companions:

- [`docs/CONVENTIONS.md`](CONVENTIONS.md) — coding standards, handler
  patterns, test helpers
- [`docs/design-system.md`](design-system.md) — frontend design system
- [`docs/progress.md`](progress.md) — what has landed, slice by slice
- [`docs/specs/`](specs/) — signed-off specs; the *why* behind each
  post-MVP slice
- [`docs/observability.md`](observability.md) — logging and log queries
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
- **Enrichment beats re-typing** — when an asset came from a
  storefront, its listing metadata is fetched rather than hand-entered
  (§9).
- **Heavy work is queued, not inline** — imports and preview rendering
  are durable Postgres-backed jobs whose state survives a restart
  (§12).

The M0–M7 MVP ladder is complete; post-MVP work is spec-driven under
[`docs/specs/`](specs/). See [`docs/progress.md`](progress.md) for
what has landed and what is deferred.

---

## 2. Stack

**Backend.** Go 1.26, Gin, GORM (Postgres in production and tests —
the suite runs against a real Postgres via `DAMSON_TEST_POSTGRES_DSN`).
Auth via `coreos/go-oidc/v3` + `oauth2` (OIDC mode)
or `golang-jwt/jwt/v5` HS256 (local mode). S3 access via
`aws-sdk-go-v2`. Background jobs via `riverqueue/river` on its own
`pgx/v5` pool (§12). SPDX matching via `google/licensecheck`.
Thumbnails and previews are pure Go: stdlib `image` +
`golang.org/x/image/{webp,tiff,bmp,draw,font}`, plus
`hajimehoshi/go-mp3` and `jfreymuth/oggvorbis` for audio. WAV is
parsed in-tree. No CGo.

**Frontend.** React 19, TypeScript 6 (strict), Vite 8. TanStack Query 5
for server state. React Router 8 (data router). Tailwind 4 with OKLCH
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
├── damson.go                 # //go:embed for migrations + SPA (see below)
├── cmd/
│   ├── server/main.go        # entry point: HTTP server, middleware, wiring
│   ├── dmsn/                 # the CLI (see §18a); stdlib flag, no new deps
│   └── e2e-server/main.go    # test-only binary for Playwright (Postgres + /api/test/*)
├── internal/
│   ├── auth/                 # Authenticator interface + local + oidc + PAT impls
│   ├── axi/                  # CLI output contract: encoding, projection, errors
│   ├── toon/                 # emit-only TOON encoder (CLI output format)
│   ├── config/               # env loading, panic-on-missing
│   ├── db/                   # gorm.Open, migration runner
│   ├── handlers/api/         # one file per feature; receivers on *Handler
│   ├── models/               # GORM models, one file per entity
│   ├── queue/                # River client + workers (import / preview queues)
│   ├── services/
│   │   ├── audit/            # audit log writer (same-tx with mutation) + retention purge
│   │   ├── extract/          # .unitypackage (tar) + .zip extractors
│   │   ├── imports/          # import pipeline, stage transitions, SSE, enrichment
│   │   ├── licence/          # SPDX + Unity EULA detection
│   │   ├── metadata/         # storefront adapters (Unity / Sketchfab / itch) + scraper
│   │   ├── preview/          # per-file preview derivatives + processing-queue state
│   │   ├── storage/          # storage.Service interface + LocalFS + S3
│   │   │   └── cleanup/      # orphan blob scan / delete
│   │   ├── tags/             # tag path helpers
│   │   └── thumbnail/        # raster / audio / font workers
│   ├── testdb/               # per-test Postgres schema harness
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

- **`users`** — `id`, `email`, `display_name`, `role`
  (`user` | `sysadmin`), `current_library_id`, `password_hash`
  (local mode only), `tokens_valid_after` (local-mode revocation
  watermark). `email` is the cross-IdP join key, so it carries a
  **partial** unique index on `lower(email) WHERE email <> ''` —
  freshly-created OIDC users with no verified email claim sit at `''`
  (`needs_profile`) until they set one.
- **`user_identities`** — `user_id`, `issuer`, `subject`;
  `unique(issuer, subject)`. One row per external IdP identity, so a
  single damson user can sign in through several IdPs. Local-mode
  users are matched by email + `password_hash` and have no row here.
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
  `thumbnail_object_key`, `thumbnail_content_type`,
  `thumbnail_from_store` (the thumbnail came from a storefront
  listing, so package extraction must not clobber it), `needs_review`,
  `created_by_user_id`, `search_vector` (Postgres tsvector,
  trigger-maintained).
- **`asset_images`** — `asset_id`, `source_url`, `object_key`,
  `content_type`, `sort_order`; `unique(asset_id, source_url)`. The
  storefront preview gallery, fetched server-side and stored as
  ordinary blobs (§9.1).
- **`asset_versions`** — `asset_id`, `store_version` (the version
  identity, e.g. `1.3`; blank = unversioned), `version_label` (an
  optional editable label), `notes`, `import_date`,
  `original_package_object_key` (set for archive imports),
  `original_package_purged` (the retained archive was reclaimed after
  extraction per `library_settings.retain_original_packages`),
  `package_kind` (`unitypackage` | `zip` | `loose` | null). A partial
  unique index enforces `unique(asset_id, store_version)` for non-blank
  store versions, so unversioned rows can coexist.
- **`asset_files`** — `asset_version_id`, `relative_path`,
  `size_bytes`, `content_hash`, `object_key` (== `content_hash`),
  `detected_type`, `unity_guid`. `unique(asset_version_id,
  relative_path)`. Preview derivative state lives here as columns
  rather than a side table: `preview_object_key`,
  `preview_content_type`, `preview_status` (`unsupported` | `queued` |
  `processing` | `done` | `failed`), `preview_updated_at` (§11).
- **`asset_imports`** — `asset_id`, `stage` (`uploaded` |
  `extracting` | `detecting` | `thumbnailing` | `enriching` | `done` |
  `failed`), `error`. One row per import and the durable source of
  truth for import state; the queue surface filters non-`done`.

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
  `max_upload_bytes` (default 64 GiB), `licence_kb_auto_update`.
- **`library_settings`** — one row per library, library-admin-managed:
  `theme_default`, `grid_density_default`,
  `retain_original_packages`, `dedupe_enabled`,
  `thumbnails_{raster,audio,font,unity_preview}_enabled`,
  `previews_enabled` (per-file preview derivatives, §11),
  `auto_enrich_enabled` (storefront lookup during import, §9),
  `preferred_store` (default storefront adapter, e.g. `unity`),
  `audit_retention_days` (nullable; consumed by the background purge
  worker in `internal/services/audit/purge.go` — see §14).

### 4.5 Upload sessions

- **`upload_sessions`** — one resumable upload in flight (§8.0):
  declared `filename` / `size_bytes` / `sha256`, the advertised
  `chunk_size`, the backend's `backend_upload_id`, per-part ETags in
  `parts_json`, the marshalled digest in `hash_state`, and the
  `received_bytes` / `next_chunk` resume point. Import hints
  (`target_asset_id`, `store_version`, `source_kind`,
  `source_relative_path`) are carried from the open call and applied at
  complete. `status` is `open | completed | aborted | expired`.

Library-scoped entities all carry `library_id` directly.
`created_by_user_id` / `pinned_by_user_id` are **attribution, not
ownership** — they do not affect access rules.

### 4.6 Migrations

SQL-only, in `database/migrations/NNN_*.sql` (001–023 at time of
writing). The runner (`internal/db/migrate.go`) reads them via
`damson.MigrationsFS`, applies in lexicographic order inside a
Postgres advisory lock, tracks applied files in `schema_migrations`.
Each file is wrapped in `BEGIN; … COMMIT;` and uses `IF NOT EXISTS`
throughout.

River's `river_*` tables are **not** in this sequence — River owns its
own migration set, run from `queue.New` under the same advisory-lock
discipline (§12).

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
  for MinIO / Garage. HEAD-before-PUT dedup. Same key layout as
  LocalFS so a deployment can migrate with a plain copy.
  Blob puts stream through `manager.Uploader` (multipart), so a
  multi-GB upload never lands in memory; only the small, bounded
  `PutThumb` path buffers.

Three key prefixes, all `<prefix>/<hh>/<hhhhh…>`:

- `blobs/` — every file referenced by `asset_files.object_key`. The
  extracted-content pool.
- `originals/` — uploaded archives (`.unitypackage` / `.zip`)
  referenced by `asset_versions.original_package_object_key`, kept
  separate from the extracted-content pool so retention and purge
  can act on them independently.
- `thumbs/` — generated thumbnails **and** per-file preview
  derivatives (`asset_files.preview_object_key`). Derived data;
  rebuildable.

`object_key == content_hash`. An asset file row is a *reference* to
a blob — two assets sharing a texture share `object_key`.

Objects are stamped with `ObjectMeta` (filename, owning ids) on first
write, readable back via `StatMeta` — enough to identify an object
from the store side alone. `List` enumerates a prefix, which is what
the orphan sweep walks.

### 5.1 Orphan cleanup

Nothing on the write path deletes blobs: deleting an asset removes
its rows and leaves the objects. Reclaiming them is a deliberate,
sysadmin-driven two-step in `internal/services/storage/cleanup`,
exposed as `POST /api/admin/storage/cleanup/scan` and
`.../cleanup/delete`.

- **Scan** (`FindOrphans`) lists every object under the swept
  prefixes and returns those referenced by no live DB row *and* last
  modified before `now - DAMSON_ORPHAN_MIN_AGE` (default `168h`).
  The age floor is the guard against deleting an in-flight upload
  whose row has not committed yet.
- **Delete** (`DeleteOrphans`) takes an explicit key list and
  re-verifies each one against a freshly-read reference set and a
  fresh listing immediately before deleting. A key that became
  referenced, dropped below the minimum age, or already vanished is
  reported as skipped with a reason rather than deleted. Every
  successful deletion is audited.

Purged originals are deliberately excluded from the known set, so a
`retain_original_packages=false` purge that failed part-way becomes a
legitimate cleanup target on the next sweep.

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
Local users are looked up by `lower(email)` with a non-null
`password_hash`; they hold no `user_identities` row.

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

`OIDCAuthenticator.resolveUser` maps a validated token to a damson
user in three steps, all inside one transaction:

1. An existing `user_identities` row for `(issuer, subject)` → its
   user.
2. Else, when the IdP asserts a **verified** email, an existing user
   with that email (case-insensitive) → link a new identity row to
   them. This is what lets someone who signed up through one IdP log
   in through another and land on the same account.
3. Else, create a user seeded from the claims plus its first identity
   row, then run `BootstrapFirstAdmin` inside the same transaction.

Unverified email claims are dropped entirely — a new user created
from one gets a blank email and the `needs_profile` flag. Email and
display name are only ever used to seed a new profile or as the join
key; once a user exists the app owns those fields (`PATCH /api/me`)
and later logins never overwrite them.

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
- `img-src` forbids store CDN origins, so external storefront images
  (candidate covers, preview galleries) are fetched server-side
  through the SSRF-guarded metadata client and streamed from
  `GET /api/metadata/image?url=` — gated to `image/*` responses — so
  the SPA renders them as same-origin `blob:` URLs without widening
  the policy.
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

Asynchronous. Bytes arrive by one of two transports (§8.0), then the
handler creates the rows and returns `202` with an `asset_imports` row
in the `uploaded` stage; a River job on the `import` queue does the
rest (§12). `asset_imports.stage` is the durable source of truth for
progress — SSE is a notification channel over the top of it, so a
dropped event is recoverable by refetching.

```
HTTP (POST /api/imports, or a completed upload session)
[1] Body streamed straight to storage while hashing
[2] PutOriginal / PutBlob (skip if the content hash is already present)
[3] Create asset (needs_review=true) + first version + import row
[4] Enqueue an import job → 202

Worker (internal/services/imports.Run)
[5] stage=extracting
      .unitypackage → tar extractor  (§8.1)
      .zip          → zip extractor  (§8.2)
      loose file    → already persisted in [2]
      ├─ Stream each inner blob back to storage
      ├─ Write asset_files rows (path, size, hash, unity_guid)
      ├─ Harvest a cover / preview.png if present → thumbnail candidate
      └─ Set package_kind, retain or purge the original per settings
[6] stage=detecting      licence detection on candidate licence files
[7] stage=thumbnailing   raster / audio / font / icon fallback
[8] stage=enriching      storefront metadata lookup (§9), gated by
                         library_settings.auto_enrich_enabled
[9] stage=done           audit entry: asset.import
    Preview work is enqueued onto the preview queue (§11, §12)
```

### 8.0 Upload transports

Two ways in. Both end at the same rows and the same queue: the shared
half lives in `internal/handlers/api/import_create.go`
(`checkImportConflict` → `createImportRows` → `finishImport`), so the
transports differ only in how bytes reach storage.

**One-shot — `POST /api/imports`, `multipart/form-data`.** The handler
reads the body with `r.MultipartReader()` and hands the file part
straight to the storage backend; nothing is spooled to local scratch.
That imposes a wire constraint: every text field (`asset_id`,
`store_version`, `source_kind`, `source_relative_path`) must be sent
**before** the `file` part, because the server reads fields as it
passes them and the file part is handed over unread. A recognised
field arriving after the file is a `400` naming it.

Because the key is the content hash, a body too large to hash in memory
(over 16 MiB) lands at a random `staging/<random>` key first and is
copied server-side to `originals/<hh>/<hash>` once the hash is known.
`staging/` sits outside `cleanup.sweptPrefixes` — a staging key is not a
content hash, so the orphan sweep would list it and then fail to delete
it.

**Chunked and resumable — the upload-session endpoints.** For archives
where a single multi-hour request is a liability. The client declares
the SHA-256 up front, so the final key is known before the first byte:
there is no staging object, no copy-on-complete, and content the
library already holds is imported without transferring anything.

| Endpoint | Behaviour |
| --- | --- |
| `POST /api/imports/uploads` | Opens a session. `sha256` (64 hex) and a positive `size_bytes` are required. HEAD the content key first: present ⇒ `202 {"deduped": true}` with the import already enqueued and no session row. Otherwise `BeginMultipart` on the final key ⇒ `201` with `upload_id`, `chunk_size`, `expires_at`. Duplicate-store-version and target-asset checks run here so a conflict costs one round trip, not 20 GB. |
| `PUT /api/imports/uploads/:id/chunks/:n` | Streams the raw octet-stream body into `UploadPart`. `n` is 0-based and strictly sequential — `n < next_chunk` is a no-op `200`, `n > next_chunk` a `409` naming the index to send. `Content-Length` must equal the expected chunk size. |
| `GET /api/imports/uploads/:id` | The resume point: `received_bytes`, `next_chunk`, `status`, `expires_at`. |
| `POST /api/imports/uploads/:id/complete` | Verifies the running hash against the declared one (mismatch ⇒ abort the multipart, `400`, nothing published), creates the rows, completes the multipart, enqueues ⇒ `202`. Idempotent once completed. |
| `DELETE /api/imports/uploads/:id` | Aborts the multipart and marks the session aborted ⇒ `204`. |

Sequential chunks are the price of verifying the hash without a second
full read. `crypto/sha256`'s digest implements
`encoding.BinaryMarshaler`, so the running state is marshalled into
`upload_sessions.hash_state` after each chunk and restored before the
next — chunks can therefore land on different pods. Advancing the state
is a conditional `UPDATE … WHERE next_chunk = ?`, so two requests racing
the same index cannot both apply.

Clients pick the transport by size: the SPA and `dmsn` both chunk at or
above 256 MiB (`dmsn --resume` forces it at any size). The one-shot
endpoint stays for everything smaller — the full-file hashing pass would
cost more than it saves.

`upload_sessions` holds no asset, version or import rows while a session
is open, so an abandoned upload costs an aborted multipart and nothing
else. `internal/services/uploads.Janitor` sweeps sessions past
`expires_at` (24h) every `DAMSON_UPLOAD_JANITOR_INTERVAL` (default 1h),
aborting the backing multipart — which no bucket listing would otherwise
reveal — and marking the row `expired`.

**LocalFS caveat.** The LocalFS `MultipartUpload` keeps its parts under
`<root>/uploads/<id>/`, on the pod's own filesystem. Chunked uploads
therefore only work single-replica on that backend. Fine for the
dev/small-deploy target; S3 keeps all state in the bucket and the DB.

The `enriching` transition fires even when enrichment is disabled, so
the SSE stage sequence stays stable for the SPA; `enrichAsset` itself
is the gated no-op.

Failure path: a failure at any stage persists `stage='failed'` plus
the reason on the import row, so the queue surface can retry via
`POST /api/imports/:id/retry`. Retry re-runs the pipeline against the
retained source blob and creates a brand-new version plus a fresh
import row in `uploaded` — the same code path as a first upload.
Only `failed` rows are retryable; `done` returns 409. The retry path
requires an `asset_files` row for non-archive imports (archives still
use `original_package_object_key`) — this stops a retry from creating
a manifest-empty version when the upload bytes were never persisted.

On boot, `imports.ResumeInFlight` re-enqueues rows left mid-stage by a
restart. River dedupes by args, so it is safe to run every start.

**Upload intent.** The SPA caps upload concurrency and asks what a
multi-file drop means before sending: separate assets, or one asset
bundled from a folder. Bundling is client-side — the browser zips the
folder and uploads a single archive. Uploads are owned by the
notification centre (§16.7) rather than by the library page, so
navigating away no longer aborts them.

**Version matching and merge.** The `POST /api/imports` response
carries an optional `suggested_match` (`{asset_id, name}`) when the
incoming pack looks like a new version of an existing asset; the SPA
prompts, and on accept re-targets the upload as a version of that
asset instead of a new one. Store versions parsed from the filename
order component-wise via `imports.CompareStoreVersions`.
`POST /api/assets/:id/merge` folds one asset into another after the
fact, moving tags, collection references and project pins (remapping
dropped pins) onto the target.

**Subset version** — `POST /api/assets/:id/versions/:vid/subset`
creates a new version whose `asset_files` rows are a subset of the
source version. Object keys are reused — no re-upload. Audited as
`asset.version.subset`. The file-picking UI is the **scoped file
browser** (`web/src/components/filebrowser/`): a full-page tree
browser at `/library/:assetId/browse`, entered via "Browse files" from
a browsable asset (current version with more than one file). It
superseded the original flat checkbox-list dialog
(`SubsetVersionDialog`, retired). Source version is the asset's
current version — the only one whose files are loaded into the panel
today.

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
(zip-slip). Re-extraction reads the retained original from storage
and rewrites the version's files **in place** — it does not spawn a
new version (§1).

### 8.2 Zip extraction

`internal/services/extract/zip.go`. `.zip` uploads are a first-class
archive kind (`package_kind='zip'`), covering both Unity Hub's
zip-format exports and ordinary asset archives, and produce the same
`UnityResult` the tar path does. Same safety envelope: per-entry and
total-uncompressed caps, zip-slip rejection. A cover image found in
the archive is harvested as the thumbnail candidate.

---

## 9. Storefront metadata enrichment

`internal/services/metadata/`. An uploaded package carries only what
can be derived from its bytes — a filename-shaped name, an extracted
thumbnail, a locally detected licence. When the asset came from an
online storefront, the listing holds description, publisher, store
URL, licence terms, a proper cover and a preview gallery. This
subsystem fetches that.

**Registry and adapters.** `Registry` maps a URL host to a `Fetcher`,
falling back to a generic HTML scraper for unknown hosts. Matching is
exact or dot-suffixed on a registered host (e.g.
`assetstore.unity.com`). Dedicated adapters: Unity Asset Store
(`unity.go`), Sketchfab (`sketchfab.go`), itch.io (`itch.go`).
Adapters that can also search by name implement `Searcher`; the
generic scraper does not. `library_settings.preferred_store` picks
which store wins when several could match.

**Entry points.**

- `POST /api/assets/:id/metadata/fetch` — fetch a listing and return
  the fields for the user to confirm.
- `POST /api/assets/:id/metadata/apply` — write the confirmed fields
  onto the asset, including storefront tags.
- `GET /api/metadata/search` — search by name across the adapters
  that support it, for picking a listing when no URL is known.
- `GET /api/metadata/preview` — non-mutating fetch-by-URL, used to
  expand a candidate before committing to it.

Auto-enrichment runs as the `enriching` stage of the import pipeline
(§8), gated by `library_settings.auto_enrich_enabled`.

**SSRF guard.** The shared `Client` (`client.go`) is the only thing
that fetches remote URLs — adapters never dial out directly — so it
is the single choke point for outbound protections. Its custom dialer
resolves each destination host and refuses non-public addresses
(loopback, RFC1918 / `fc00::/7` private, link-local, …) with
`ErrBlockedAddress`. Because the check runs at dial time rather than
on the URL, a redirect into internal space is caught too. It also
caps body size, bounds the redirect chain, rejects redirects to
unsupported schemes, and rate-limits requests.

### 9.1 Storefront images

The CSP's `img-src` deliberately does not include store CDN origins
(§6.4), so listing imagery cannot be loaded by the browser directly.
Instead:

- The chosen **cover** is fetched server-side and stored as an
  ordinary content-addressed blob, becoming the asset thumbnail with
  `assets.thumbnail_from_store=true` so a later extraction or rescan
  does not overwrite it.
- The **preview gallery** lands in `asset_images`, one row per image,
  ordered by `sort_order` and unique per `(asset_id, source_url)`.
  Video URLs are filtered out. `POST
  /api/assets/:id/metadata/gallery/backfill` populates the gallery for
  assets enriched before it existed.
- Anything still remote is proxied through
  `GET /api/metadata/image?url=`, which is SSRF-guarded and gated to
  `image/*` responses, so the SPA renders it as a same-origin `blob:`
  URL without widening the policy.

---

## 10. Licence detection

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

## 11. Thumbnails and previews

Two related but distinct derivative pipelines:

- **Thumbnails** — one small image *per asset*, for the grid.
  Generated inline during the import job.
- **Previews** — one larger image *per file*, for the file browser's
  detail panel. Generated as their own tracked, retryable work on the
  preview queue.

### 11.1 Thumbnails

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
the SPA falls back to icons. A thumbnail adopted from a storefront
listing sets `thumbnail_from_store` and is not overwritten by a
later extraction or rescan (§9.1).

### 11.2 File previews

`internal/services/preview/`. Where a thumbnail is a small per-asset
image, a preview is a **web-safe, browser-renderable derivative of one
file** — larger, so the file detail panel shows real detail — for
formats a browser cannot open directly. Currently pure-Go raster
decoding (TIFF, BMP, and friends) with the rest of the taxonomy
staged behind it. Previews are stored content-addressed under the
same `thumbs/` prefix.

Guards: a long-edge cap on the output, and a decompression-bomb guard
that reads `DecodeConfig` first and skips anything whose declared
dimensions exceed ~100 MP.

**Per-file state.** `asset_files.preview_status` is first-class state,
not an implicit property, which is what makes the work observable and
retryable:

| Status | Meaning |
|---|---|
| `unsupported` | no backend renderer for this type — the common case |
| `queued` | supported, awaiting a worker |
| `processing` | a worker is rendering it |
| `done` | `preview_object_key` is set |
| `failed` | the render errored; retryable via requeue |

Classification runs over a version's files and marks the supported set
`queued`; everything else `unsupported`. A version's rollup status is
derived from its file counts — `processing` if any file is mid-render,
else `queued` while any remain, else terminal (failures surface as a
Failed count rather than a status).

**Surfaces.** `GET /api/previews` is the processing-queue view and
`GET /api/previews/events` streams transitions; the SPA renders them
at `/processing`. Requeue is available per asset
(`POST /api/assets/:id/previews/requeue`) or per file
(`.../files/:fileId/previews/requeue`), which is also how a `failed`
render is retried. `GET /api/previews/:objectKey` serves the bytes.
Gated by `library_settings.previews_enabled`.

---

## 12. Background jobs (River)

`internal/queue`. Imports and previews run as
[River](https://riverqueue.com) jobs — a Postgres-backed queue —
replacing the hand-rolled bounded goroutine pools draining in-memory
channels that preceded it. The pipelines themselves are unchanged;
River wraps them, and enqueue becomes a `river_job` row instead of a
channel send. See `docs/specs/history/2026-07-19-river-queue-migration.md`
for the reasoning.

- **Two queues.** `import` and `preview`, sized independently so each
  can be tuned — and, later, moved to a dedicated worker container.
- **Its own pool.** River runs on a dedicated `pgxpool`, separate
  from GORM's, so its LISTEN/NOTIFY-driven low-latency pickup and its
  connection usage stay isolated from web traffic.
- **Ids only.** `ImportArgs{ImportID, LibraryID}` and
  `PreviewArgs{LibraryID, VersionID, FileID}` — the job payload
  carries identifiers and the worker reloads everything from the DB.
  `FileID == 0` means every queued file in the version.
- **Migrations.** River owns its `river_*` schema and migrates it
  from `queue.New` under the same advisory-lock discipline as the
  app's own runner. These tables are not part of
  `database/migrations/`.
- **Timeouts and sizing.** Per-queue job timeout, defaulting to
  `DefaultJobTimeout` (30 min — generous, because imports can be
  multi-GB). Worker counts default to `GOMAXPROCS`. Both are
  overridable per queue (§15.1).
- **Resume on boot.** `imports.ResumeInFlight` /
  `preview.ResumeInFlight` re-enqueue rows left mid-flight by a
  restart. River dedupes by args, so this is idempotent and also
  covered the one-time cutover of rows in flight before River existed.

The database stays the source of truth throughout: the queue tracks
*work*, while `asset_imports.stage` and `asset_files.preview_status`
track *state*.

---

## 13. Search

A `search_vector` column on `assets`, maintained by triggers on
`assets` (`name`, `description`) and `asset_tags` (path
aggregation). Postgres matches with a GIN index against a
prefix-aware `to_tsquery('simple', …)` built by `buildPrefixTsQuery`,
so "pack" matches "package".

`applyAssetFilters` still carries a non-Postgres `LIKE` fallback
branch on `name` / `description`, dispatched on
`db.Dialector.Name()`. It is dead code — Postgres is the only
dialect in production *and* in tests — and the tsvector path,
triggers and GIN index are now exercised directly by the suite.

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

## 14. Audit

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

## 15. Configuration

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

### 15.1 Always

| Var | Default |
|---|---|
| `DAMSON_POSTGRES_DSN` | **required** |
| `DAMSON_LISTEN_ADDR` | `:8080` |
| `DAMSON_BASE_URL` | `http://localhost:8080` |
| `DAMSON_LOG_LEVEL` | `info` |
| `DAMSON_LOG_FORMAT` | `json` (`text` for local dev — see `docs/observability.md`) |
| `DAMSON_AUTH_MODE` | `local` (or `oidc`) |
| `DAMSON_STORAGE_MODE` | `local` (or `s3`) |
| `DAMSON_AUDIT_PURGE_INTERVAL` | `24h` (Go duration; non-positive falls back to default) |
| `DAMSON_ORPHAN_MIN_AGE` | `168h` (7 days; minimum object age before orphan cleanup will delete it — §5.1) |
| `DAMSON_UPLOAD_JANITOR_INTERVAL` | `1h` (how often abandoned upload sessions expire and their multipart uploads abort — §8.0) |

Background queues (§12). Unset / unparseable / non-positive values
fall back to the default in every case.

| Var | Default |
|---|---|
| `DAMSON_IMPORT_QUEUE_WORKERS` | `GOMAXPROCS` |
| `DAMSON_PREVIEW_QUEUE_WORKERS` | `GOMAXPROCS` |
| `DAMSON_IMPORT_JOB_TIMEOUT` | `30m` (Go duration) |
| `DAMSON_PREVIEW_JOB_TIMEOUT` | `30m` (Go duration) |

### 15.2 `DAMSON_AUTH_MODE=local` (default)

| Var | Default / required |
|---|---|
| `DAMSON_JWT_SIGNING_KEY` | optional; if empty, 32 random bytes are generated and persisted |
| `DAMSON_JWT_KEY_FILE` | `./data/jwt.key` |
| `DAMSON_LOCAL_ADMIN_EMAIL` | optional — when set with `_PASSWORD`, seeds first admin |
| `DAMSON_LOCAL_ADMIN_PASSWORD` | optional |

### 15.3 `DAMSON_AUTH_MODE=oidc`

| Var | Required? |
|---|---|
| `DAMSON_OIDC_ISSUER_URL` | required |
| `DAMSON_OIDC_CLIENT_ID` | required |
| `DAMSON_OIDC_AUDIENCE` | required |

Frontend env (Vite, OIDC only): `VITE_OIDC_AUTHORITY`,
`VITE_OIDC_CLIENT_ID`, `VITE_OIDC_SCOPE`, `VITE_OIDC_REDIRECT_URI`,
`VITE_OIDC_SILENT_REDIRECT_URI`, `VITE_OIDC_POST_LOGOUT_REDIRECT_URI`.

### 15.4 `DAMSON_STORAGE_MODE=local` (default)

| Var | Default |
|---|---|
| `DAMSON_STORAGE_LOCAL_PATH` | `./data/blobs` |

### 15.5 `DAMSON_STORAGE_MODE=s3`

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

## 16. Frontend

### 16.1 Layout shell

`AppShell` composes `NavRail` + `FilterSidebar` + main area.
Routes (`web/src/App.tsx`):

```
/                        → redirect to /library
/library                 → asset grid + filter sidebar
/library/:assetId        → grid + detail panel open
/library/:assetId/browse → scoped file browser (full-page tree, §8)
/queue                   → filtered library view (needs_review=true)
/processing              → preview processing queue (§11.2)
/projects                → projects index
/projects/:id            → project detail
/collections             → collections index
/collections/:id         → collection detail
/audit                   → library audit log
/admin/audit             → server audit log (sysadmin)
/settings                → settings (theme, density, thumbnails,
                           previews, enrichment, retention, members,
                           server admin, storage cleanup)
/login                   → local-mode login
/bootstrap               → local-mode first-admin form
/auth/callback           → OIDC redirect landing
```

Everything except `/login`, `/bootstrap` and `/auth/callback` sits
behind `<AuthGuard>`.

The asset detail panel is URL-addressable so a pasted link opens
the right asset. It is user-resizable (overlay mode only): a left-edge
drag handle controls width, persisted to `localStorage` under
`damson.detail.w` (default 440 px, min 320 px); a header button toggles
fullscreen (covers the full grid area). Viewports ≤ 640 px auto-fullscreen.

### 16.2 State

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

### 16.3 API client

`src/api.ts` — `fetchJson<T>` / `fetchMaybeJson<T>` attach the
bearer, handle 401 (silent renew on OIDC; `AuthRedirectError` on
local), retry once, throw `ApiError`. **Never `fetch` directly
from components** — the only exceptions are pre-auth endpoints in
`lib/authMode.ts` and `lib/localAuth.ts`.

`uploadFile(url, file, onProgress)` uses `XMLHttpRequest` for
upload progress (fetch lacks it).

### 16.4 SPA auth flow

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

### 16.5 Grid

`@tanstack/react-virtual`. Column count comes from a
`ResizeObserver`-driven hook; keyboard selection scrolls into view
via `virtualizer.scrollToIndex`. `LibraryPage` keyboard shortcut
bindings are stable across renders — handlers read state from a
ref instead of being re-created on each refetch, so
`useKeyboardShortcuts` does not rebind on every paint.

Keyboard shortcuts (`useKeyboardShortcuts`): `/` and `cmd+k` focus
search, arrow keys navigate, `enter` opens, `escape` closes.

### 16.6 Design system

Spec: [`docs/design-system.md`](design-system.md). Three tiers
under `web/src/design-system/`: **primitives** (Button, Input,
Dialog, …), **compositions** (AppShell, NavRail, DetailPanel, …),
**domain** (LicencePill, AssetCard, ProjectCard, …). Every
component ships a `*.stories.tsx`; Vitest runs them as real
Chromium tests with axe-core a11y checks (`a11y.test = "error"`).

### 16.7 Notification centre

`web/src/components/notifications/`. Client-side only — there is no
notifications table or endpoint. `NotificationProvider` is mounted
above the router in `App.tsx`, which is the point: **it owns the
upload queue**, so uploads survive navigation. Previously upload
state lived in `DropZone` inside `LibraryPage`, and React Router
unmounting the page aborted the in-flight XHR.

It also drives the shell's unread / in-flight badge, and is where
upload concurrency capping, per-upload cancel, clear-all, and the
"add as a version of an existing asset?" prompt live (§8).

---

## 17. API surface

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
PATCH  /api/me                        # app-owned email / display name
GET    /api/libraries
POST   /api/me/current-library
POST   /api/auth/refresh              # local mode (re-issues JWT)

# Libraries (cross-library; handler-gated)
POST   /api/libraries
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
POST   /api/admin/storage/cleanup/scan    # list orphaned objects
POST   /api/admin/storage/cleanup/delete  # delete an explicit key list

# Assets (library-scoped)
GET    /api/assets
POST   /api/assets/bulk               # action over a selection: delete, review,
                                      #   unreview, add_tag, remove_tag,
                                      #   add_to_collection, set_licence
GET    /api/assets/:id
PATCH  /api/assets/:id
DELETE /api/assets/:id
POST   /api/assets/:id/review
POST   /api/assets/:id/rescan         # re-derive licence + thumbnail from files
POST   /api/assets/:id/merge          # fold another asset into this one
GET    /api/assets/:id/audit
GET    /api/assets/:id/licence
PUT    /api/assets/:id/licence
POST   /api/assets/:id/licence/review
GET    /api/assets/:id/licence/detect
POST   /api/assets/:id/tags
DELETE /api/assets/:id/tags/:tagId

# Versions (library-scoped)
PATCH  /api/assets/:id/versions/:vid
DELETE /api/assets/:id/versions/:vid
POST   /api/assets/:id/versions/:vid/reextract
POST   /api/assets/:id/versions/:vid/subset

# Storefront metadata (library-scoped)
POST   /api/assets/:id/metadata/fetch          # fetch + fill/confirm from a listing
POST   /api/assets/:id/metadata/apply
POST   /api/assets/:id/metadata/gallery/backfill
GET    /api/metadata/preview          # non-mutating fetch-by-url (candidate expand)
GET    /api/metadata/image            # SSRF-guarded image proxy → same-origin bytes
GET    /api/metadata/search           # name search across storefront adapters

# Imports (library-scoped)
POST   /api/imports                   # multipart; 202 + optional suggested_match
GET    /api/imports                   # ?stage=<name> | all
GET    /api/imports/events            # SSE: library-wide stage transitions
GET    /api/imports/:id
GET    /api/imports/:id/events        # SSE: one import
POST   /api/imports/:id/retry

# Previews (library-scoped)
GET    /api/previews                  # processing-queue view
GET    /api/previews/events           # SSE: preview transitions
GET    /api/previews/:objectKey       # serve a preview derivative
POST   /api/assets/:id/previews/requeue
POST   /api/assets/:id/files/:fileId/previews/requeue

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

Every endpoint the archived tech design assumed is now implemented —
the last gaps (`POST /api/libraries`, SSE imports) closed with the
async-pipeline and River work. `docs/progress.md` tracks what has
landed since.

---

## 18. Deployment

Multi-stage Dockerfile (no CGo; runtime image is slim):

1. `node:24-alpine` — `pnpm install --frozen-lockfile` + `vite build` → `web/static/`.
2. `golang:1.26-alpine` — `CGO_ENABLED=0 go build` with
   `-ldflags="-s -w"`, embedding the SPA via `//go:embed`.
3. `alpine:3.20` — `ca-certificates` + `tzdata`, non-root
   `damson` user, `HEALTHCHECK` hitting `/api/health`.

The binary serves the SPA at `/` with a fallback (any non-`/api`
route that is not a static asset returns `index.html`).

CI/CD is **Woodpecker** (`.woodpecker.yml`) — not GitHub Actions.
Two pipelines off one file:

- **Push to `main`** — lint, Go tests, web tests, Playwright e2e,
  then a buildkit-rootless image build pushed to
  `ghcr.io/thomas-elliott/damson:main-<sha>` (plus `latest`), then a
  kustomize image-tag bump in the gitops repo for the **test**
  environment, which ArgoCD reconciles.
- **Tag `v*.*.*`** — no rebuild. The already-built, already-tested
  `main-<sha>` image is *promoted* with `crane` to `{version}`,
  `{major}.{minor}` and `{major}` tags, then the **prod** kustomize
  tag is bumped.

Two consequences worth knowing before touching the file: the
Playwright step's image tag must stay in lockstep with
`@playwright/test` in `web/package.json` (pinned exactly, so the
bundled browsers resolve), and a tag can only be released if that
commit was previously built on `main`.

Deployment target is the kerbol k8s cluster, namespaces
`damson-test` and `damson-prod` (see `docs/observability.md`).

### 18.1 First-run flow

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
  email list). Users are keyed by `(issuer, subject)` in
  `user_identities`; one with no verified email claim fills it in on
  first login (§6.1).

---

## 18a. The `dmsn` CLI

`cmd/dmsn` is a companion binary for scripting a library and for handing
to an agent. Scoped by
[`specs/2026-08-01-dmsn-cli.md`](specs/2026-08-01-dmsn-cli.md).

**Credential.** Personal access tokens, `api_tokens` (migration 020). The
secret is `damson_pat_` + 32 random bytes, base64url, stored as a plain
SHA-256 hex digest — deliberately *not* bcrypt: the secret is already
high-entropy, so a slow KDF buys nothing, while a fast digest allows the
direct indexed lookup every authenticated request performs.

`auth.Middleware(authenticator, tokens)` takes a nil-able
`*TokenAuthenticator` and branches on the `damson_pat_` prefix *ahead of*
the mode authenticator, so PATs work identically in local and OIDC
deployments. On success it pins `CurrentLibraryID` to the token's library
**on the request-scoped user copy only**, so `requireLibrary()` needs no
change and a running sync is unaffected by the owner switching library in
the SPA. `last_used_at` is written at most once a minute and never fails a
request.

Two rules are enforced outside the handlers:

- `scope = 'read'` rejects every non-GET with 403, in `auth.Middleware`.
  (Verified safe: no GET handler on the authed surface performs a write.
  The converse does not hold — `POST /api/imports/preflight` is a pure read
  that a read-scoped token is nonetheless refused.)
- `auth.RefuseTokens()` closes a route to token auth, so a token can neither
  mint another token nor reach sysadmin surface. It is applied at route
  registration — on the three `/me/tokens` routes and on the `admin` group in
  `AddRoutes` — rather than matched from the URL, so a new privileged route
  inherits the gate from the group it is declared in instead of from the shape
  of its path. It runs *after* authentication, so an invalid token still gets
  401 and cannot probe which routes are gated.

`POST /me/tokens` additionally carries `requireLibrary()`: minting pins the
caller's current library, so it is library-scoped even though listing and
revoking are not.

**Output contract.** `internal/toon` is an emit-only TOON encoder — tabular
arrays, primitive arrays, objects, the empty forms, and the quoting rules.
There is no decoder; nothing in damson reads TOON. `internal/axi` layers the
[AXI](https://axi.md/) contract on top: the TOON/JSON switch, `--fields`
projection and validation, truncation with a total-length note, `help[]`
emission, and the error envelope with its 0/1/2 exit-code mapping. Both are
pure Go with no I/O beyond a supplied `io.Writer`, so the whole contract is
table-testable without a command.

Two rules live in the render layer rather than in each command: an empty
result always states *why* (a bare `key[]:` is not allowed through), and a
multi-part result must render through one call — `RenderComposite` for
blocks/lists/tables, or `axi.Record.Lists` for an entity whose arrays ride
alongside its object block. Rendering the parts separately would emit
concatenated JSON documents, which do not parse, and it is how `--json`
quietly ends up lossier than TOON.

**Leaf packages shared with the CLI.** `internal/toon` and `internal/axi`
above, plus `internal/tagpath` (path normalisation), `internal/importstage`
(the `asset_imports.stage` enum and `IsTerminal`, aliased back into
`internal/services/imports`), `internal/importsource` (the
`asset_versions.source_kind` values and the four preflight statuses, likewise
aliased back), `internal/unitycache` (the Asset Store cache layout — see §18b)
and `internal/buildinfo` (the `debug.ReadBuildInfo` rule behind both
`GET /api/health` and `dmsn version`). Each exists so the CLI can share a rule
without importing gin, gorm or River. The discipline is load-bearing rather
than tidy: reaching into `internal/services/imports` for a single constant
pulls GORM and the AWS S3 SDK into the client binary.

**Endpoints added for it.** `GET /api/overview` (the home view in one round
trip), `POST /api/imports/preflight` (batch name matching, backing
`--skip-existing`), `max_upload_bytes` read-only on `GET /api/settings`, a
`token` block on `GET /api/me` describing the authenticating token, and a
build identifier on `GET /api/health`.

**Folder upload needs no server change.** `.zip` is already a first-class
package kind, so `upload --as asset` zips a directory client-side and the
existing `extract.ZipExtractor` unpacks it with relative paths preserved. The
cost accepted: temp disk equal to the compressed folder, and a retained
"original package" that is a CLI-synthesised archive.

**Build.** `task build:cli`. There is deliberately no cross-compile matrix,
no Woodpecker step and no release artefacts — see §19.13.

## 18b. Importing the Unity Asset Store cache

`dmsn sync` ingests Unity's local download cache, laid out as
`<Publisher>/<CategoryPath>/<Name>.unitypackage` under
`%APPDATA%\Unity\Asset Store-5.x` (macOS `~/Library/Unity/…`, Linux
`~/.local/share/unity3d/…`), overridable by `ASSETSTORE_CACHE_PATH`. Scoped by
[`specs/2026-08-01-unity-local-cache-import.md`](specs/2026-08-01-unity-local-cache-import.md).

**The cache path is metadata.** Publisher and the store category tree are free
from the directory layout and available nowhere else cheaply. Unity concatenates
category segments with no separator (`3D ModelsCharactersHumanoids`), which
splits cleanly on each lowercase→uppercase boundary — a bare `3D Models` stays
whole because that boundary is uppercase-space-uppercase.

**Provenance columns.** `asset_versions.source_kind`, `.source_relative_path`,
`.source_size_bytes` (migration 021), all nullable so a hand-dragged upload
stays distinguishable from a sourced one. `POST /api/imports` takes the first
two as optional form fields; `source_size_bytes` is written from what storage
returns, never from the client, so the client can pose the idempotency question
but not answer it.

**Idempotency is a JSON round trip, not a hash.** `POST /api/imports/preflight`
answers `already_imported` (same path, same size), `updated` (same path,
different size, naming the asset to add a version to), `name_match` or `new`.
The lookup joins `asset_versions → assets` on `library_id` — versions carry no
library of their own — and the newest version by `import_date` wins where an
`updated` re-upload has left two on one path. Nobody hashes gigabytes.

**Two placements worth knowing.**

- **The server's parse is authoritative.** `internal/unitycache.ParseCachePath`
  is a heuristic, so what gets persisted is derived server-side from the raw
  path a client sends; a correction ships with the server and reaches every
  already-installed binary. The CLI shares the package for display only.
- **`store_version` stays blank.** The cache holds no version information at
  all — not in the filename, not in the archive. `ParseImportFilename` would
  nonetheless read `v2` out of `DOTween HOTween v2`, and because `store_version`
  is a per-asset uniqueness key that phantom would block the real `2.1.0`
  forever. So `source_kind = unity_cache` suppresses the filename version
  parse; enrichment supplies the listing version afterwards.

**`ApplySourceHints`** runs before the enrich stage, so category tags land
whether or not `auto_enrich_enabled` is on. It writes `assets.publisher`
blanks-only — a resumed import cannot clobber a correction — and that publisher
then feeds `exactMatchCandidate`, breaking ties among title matches and
unlocking a whole-word-prefix near match when the publisher agrees exactly. A
near match records confidence `0.8` rather than claiming an exact `1.0`.

---

## 19. Confirmed decisions

Reproduced from the archived tech design §22 and updated where
the code diverges:

1. Module path `github.com/thomas-elliott/damson`.
2. Frontend OIDC library `oidc-client-ts`.
3. **Local auth mode is real and tested**, alongside OIDC. The
   archived tech design described OIDC only; local mode is
   `internal/auth/local.go` + `internal/handlers/api/auth.go`.
4. Upload path: in-server streaming (Go server hashes in-stream
   and puts to storage, multipart to S3). Revisit only if the server
   becomes a bottleneck.
5. Import staging: real `asset_imports` table. It stays the durable
   source of truth for stage state even now that River owns the
   scheduling (§12).
6. Test tier: **real Postgres everywhere**, per-test schema via
   `internal/testdb`, `DAMSON_TEST_POSTGRES_DSN` required. SQLite is
   gone — the driver is no longer a dependency, and tsvector, JSONB
   operators and advisory locks are exercised directly.
7. Thumbnail pipeline: stdlib `image` + `golang.org/x/image/*`.
   No CGo, no third-party encoder. JPEG for photographic content,
   PNG for everything else. File previews (§11.2) follow the same
   pure-Go constraint.
8. Ownership model: library-based group ownership.
9. **Brand colour is yellow only** — no theme-swap, and it is `--primary`
   (shadcn's `--accent` is the hover surface). **Light at `:root`, dark on
   `.dark`**, with `prefers-color-scheme` deciding the initial value until the
   user picks one. Primitives come from shadcn/ui via the shared `@shared`
   registry as copy-in source; Damson-specific components are still hand-rolled
   on Radix. See
   [`specs/history/2026-08-15-shared-design-system.md`](specs/history/2026-08-15-shared-design-system.md).
10. Background jobs: River on Postgres, replacing hand-rolled
    goroutine pools. Chosen for durability across restarts and
    observable job state without building a scheduler.
11. External identities live in `user_identities`, keyed
    `(issuer, subject)`, with verified email as the cross-IdP join
    key. `users.subject` was dropped (migration 008).
12. Storefront metadata is fetched server-side through one
    SSRF-guarded client, and listing imagery is re-hosted as local
    blobs rather than widening the CSP to store CDNs.
13. The CLI is named `dmsn`, not `damsonctl`: an agent types it on
    every call, and the server binary keeps `damson` so the Dockerfile
    and manifests are untouched. It ships with **no distribution
    plumbing** — `task build:cli` and `dmsn setup`, nothing else. The
    accepted consequence is that the binary exists only where someone
    builds it.
