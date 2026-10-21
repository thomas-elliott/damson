# Architecture

Current-state reference for Damson. The build-time design docs in
[`docs/archive/`](archive/) describe the *why* and the trade-offs;
this doc describes *what is in the tree today*.

---

## 1. What Damson is

A self-hosted digital asset manager for game-development assets: one Go binary that embeds a React
SPA, behind a reverse proxy, backed by Postgres and an S3-compatible object store or a local
filesystem.

The ideas everything else assumes:

- **A library is the tenancy boundary.** Access comes from `library_memberships`; inside a library
  every member sees the same data.
- **Storage is content-addressed** (§5). Postgres holds metadata; a texture shipped in five packages
  is one object.
- **A version is the external pack version.** An `AssetVersion` is identified by its
  `store_version`, parsed from the upload filename or left blank (§18b). Re-extraction rewrites a
  version's files in place; only another upload makes a new version.
- **Projects are declarative.** A project pins versions; an upgrade re-points the pin and is
  audited.
- **User-driven writes are audited** in the same transaction as the change (§14).
- **Storefront metadata is fetched, not retyped** (§9).
- **Heavy work is queued.** Imports, preview rendering and store lookups are durable
  Postgres-backed jobs that survive a restart (§12).

---

## 2. Stack

**Backend.** Go with Gin and GORM on Postgres — the only database, in production and in tests
(`DAMSON_TEST_POSTGRES_DSN`). River runs background jobs on its own `pgx` pool (§12). Auth is
`coreos/go-oidc` (OIDC mode) or `golang-jwt` HS256 (local mode); S3 through `aws-sdk-go-v2`.
Thumbnails and previews are decoded in pure Go (§11). Both binaries build with `CGO_ENABLED=0`.

`modernc.org/sqlite` in `go.mod` is not a database backend: only the `dmsn` CLI uses it, read-only,
to open Unity's local purchases cache (`cmd/dmsn/purchases_cache.go`).

**Frontend.** React, TypeScript (strict), Vite, TanStack Query, React Router, Tailwind.
Radix plus hand-rolled components under `web/src/design-system/` (docs/design-system.md).
Two TypeScript compilers are installed side by side: the `typescript` dependency is aliased to
`@typescript/typescript6`, which is what `tsc -b` runs; `@typescript/native` sits alongside it.
Check `web/package.json` before assuming which one a given script uses.

**Tooling.** `just` (`justfile` + `tools/*.just`) for every workflow; `air` for Go hot-reload.
Vitest has two projects: `unit` (jsdom) and `storybook` (every `*.stories.tsx` in Playwright
Chromium with axe-core checks).

Module path: `github.com/thomas-elliott/damson`.

**The `damson.go` quirk.** `//go:embed` patterns can only reach **into** a directory, not **up**
from the embedding file. Since `database/migrations/*.sql` and `web/static/*` are at project root
but the Go packages that need them live under `internal/`, there is a single file at project root —
`damson.go` — that holds both `//go:embed` directives and exports `damson.MigrationsFS` and
`damson.WebStaticFS`. Consumers import `github.com/thomas-elliott/damson` and use those vars.

`web/static/.gitkeep` exists because `//go:embed all:web/static` requires the directory to match at
least one file at compile time, even before the SPA has been built.

### Traps

- **Never add a Go dependency that needs cgo.** With `CGO_ENABLED=0` a cgo-only package such as
  `mattn/go-sqlite3` still compiles, then fails at runtime. Pick a pure-Go equivalent, as the CLI
  did with `modernc.org/sqlite`.

---

## 4. Data model

Postgres, mapped through GORM structs in `internal/models/`. The SQL in `database/migrations/` is
the authoritative schema; this section covers only what the columns do not say.

### 4.1 Identity, libraries, audit

A person is one `users` row; each external sign-in is a `user_identities` row keyed by
`(issuer, subject)`, so one user can sign in through several IdPs. Local-mode users have no
identity rows and authenticate by email and `password_hash`.

`users.email` is the cross-IdP join key: unique on `lower(email)` among non-blank values. An OIDC
user whose IdP sent no verified email sits at `''` (`needs_profile`) until they set one, so any
number of users can share the blank value.

Access to a library comes only from `library_memberships` (`admin` | `member`). `audit_log` rows
carry a null `library_id` for server-level events, and are deleted with their library.

### 4.2 Assets

An asset owns versions, a version owns files, and each import of a version has an `asset_imports`
row. That row, not the River job, is the durable import state (§8, §12).

- A version's identity is `store_version` (`1.3`). It is unique per asset among non-blank values;
  blank means unversioned, and any number of unversioned versions coexist. `version_label` is an
  optional, editable display label with no uniqueness.
- `original_package_object_key` is an `originals/` key only when `package_kind` is `unitypackage` or
  `zip`. For a loose upload it is the `blobs/` key of the version's single file.
- `original_package_purged` means the archive's bytes were reclaimed after extraction (per
  `library_settings.retain_original_packages`). The key stays on the row.
- Preview state lives on `asset_files` itself (`preview_status`, §11), not in a side table.
- A **reference asset** (`is_reference`) is recorded by where it lives (`location_path` and/or
  `source_url`) and has no versions or files. The database enforces it: only a reference asset may
  carry a `location_path`, a reference asset keeps at least one of the two, and a path is unique per
  library.

### 4.3 Tags, projects, collections, licences

- Tags are a materialised path per library (`environment/trees/deciduous`); descendants are
  `path LIKE '<p>/%'`. `internal/tagpath` is the one normalisation rule, shared with the `dmsn` CLI.
- Project and collection names are unique per library, case-insensitively.
- A project pins one version of each asset. A pin of a reference asset has a null
  `pinned_version_id`.
- Licence `attributes_json` booleans are tri-state: `null` means unknown, distinct from `false`.

`created_by_user_id` / `pinned_by_user_id` are **attribution, not ownership** — they do not affect
access rules.

### 4.4 Settings

`server_settings` is a singleton row (sysadmin). `library_settings` is one row per library (library
admin), created with defaults on first read if missing. `audit_retention_days` drives the purge
worker (§14).

### 4.5 Upload sessions

An `upload_sessions` row is one resumable upload in flight (§8.0). It carries the resume state
(`hash_state`, `parts_json`, `next_chunk`) and the import hints given at open. No asset, version or
import row exists until the session completes, so an abandoned upload leaves nothing in the
library.

### 4.6 Migrations

SQL-only, embedded through `damson.MigrationsFS` and applied by `internal/db/migrate.go` in
filename order under a Postgres advisory lock, tracked in `schema_migrations`. File shape and
idempotency rules: `docs/CONVENTIONS.md`, Database Migrations. River's `river_*` tables are not in
this sequence; River migrates its own schema from `queue.New` under the same lock (§12).

Tests run against a real Postgres, each in its own schema with the migrations applied. The DSN comes
from `DAMSON_TEST_POSTGRES_DSN`, which is **required** — with it unset the suite fails rather than
skipping. A schema change must still land in **both** an SQL migration **and** the GORM struct
(CONVENTIONS, Database Migrations).

### Traps

- **Scope a child row by joining to `assets` and filtering `assets.library_id`.** Versions, files,
  imports, images and licences have no `library_id`, so fetching one by id alone behind
  `requireLibrary()` serves another library's row.
- **Look up a user by email only when the email is non-blank, and compare `lower()`.** A blank email
  matches every `needs_profile` user, so the lookup adopts a stranger's account.
- **Match an IdP identity on `(issuer, subject)`, never `subject` alone.** Subjects are unique per
  issuer only.
- **Treat a version as having an original only when the key is set, `original_package_purged` is
  false and `package_kind` is an archive.** A purged version keeps its key, and a loose version's key
  points into `blobs/`, so a key check alone offers a download that is not there.
- **Normalise a tag path with `tagpath.Normalise` before comparing it.** Stored paths are normalised,
  so a raw user-typed path silently matches nothing.

---

## 5. Object storage

`internal/services/storage.Service`, chosen at startup by `DAMSON_STORAGE_MODE`: `local` (default)
or `s3` (`aws-sdk-go-v2`, path-style for MinIO / Garage). Both use the same key layout, so a
deployment migrates between them with a plain copy.

Storage is content-addressed: `object_key == content_hash` (SHA-256), all `<prefix>/<hh>/<hash>`.
A file row is a *reference*: two assets sharing a texture share one object, and a write whose key
already exists reuses it.

- `blobs/` — extracted content (`asset_files.object_key`).
- `originals/` — uploaded archives (`asset_versions.original_package_object_key`), kept apart so
  retention and purge act on them alone.
- `thumbs/` — derived data: thumbnails, per-file previews, gallery images, store covers. Rebuildable.

Nothing on the write path deletes an object. Reclaiming them is a sysadmin two-step in
`internal/services/storage/cleanup`: `POST /api/admin/storage/cleanup/scan` returns unreferenced
objects older than `DAMSON_ORPHAN_MIN_AGE`; `.../cleanup/delete` re-checks the given keys against a
fresh reference set and audits each deletion. A purge of originals that failed part-way becomes a
cleanup target on the next sweep.

### Traps

- **A new column holding an object key must be added to `knownKeys` in `cleanup.go`**, or the
  sweep deletes its objects once they pass the age floor.
- **The age floor does not protect a reused object.** Dedup reuses an existing key without touching
  its modification time, so an old orphan stays deletable after a new row points at it.
- **Keep LocalFS's hex-only check in `pathFor`.** It is what stops a bug at the HTTP boundary
  becoming a path traversal.


---

## 6. Authentication and authorisation

### 6.1 Modes

`DAMSON_AUTH_MODE` picks exactly one authenticator per deployment, behind the `Authenticator`
interface in `internal/auth/auth.go`. The middleware does not know which mode is in play. Mixing
modes is out of scope. Personal access tokens sit beside both modes, not inside them (§18a).

**`local`** (default) — bcrypt hashes in `users.password_hash`, HS256 JWTs lasting
`auth.LocalTokenTTL`, extended through `POST /api/auth/refresh`. `iss` and `aud` are
`DAMSON_BASE_URL`, so a token signed for one Damson does not verify against another. Local users
are found by `lower(email)` with a non-null hash and have no `user_identities` row. Login and
bootstrap are rate-limited by `auth.LoginThrottle`, which is in memory and per process. Every
password write enforces `auth.MinPasswordLength`. Local mode suits development and small
installs: there is no MFA and no shared rate-limit store.

`users.tokens_valid_after` is the local-mode revocation watermark: a JWT issued before it is
rejected. Member removal, library deletion and admin password reset bump it. OIDC mode does not
read it.

**`oidc`** — `coreos/go-oidc/v3` verifies bearers from the IdP. The SPA runs the interactive flow
with `oidc-client-ts`, configured at runtime from `GET /api/auth/mode`. The SPA sends the id_token
as the bearer; set `DAMSON_OIDC_AUDIENCE` to the client_id.

`OIDCAuthenticator.resolveUser` maps a verified token to a user in one transaction:
1. the `user_identities` row for `(issuer, subject)`;
2. otherwise an existing user whose email matches a **verified** email claim, linking a new
   identity, which lets one person sign in through several IdPs;
3. otherwise a new user plus its first identity, then `BootstrapFirstAdmin` (§6.2).

Claims only seed a new profile or act as the join key. After that the app owns email and display
name (`PATCH /api/me`), and later logins never overwrite them. If a new user has no verified
email, their email stays blank and they are flagged `needs_profile`.

### 6.2 Bootstrap

`auth.BootstrapFirstAdmin` is shared by both modes. On an empty system, the first user becomes
`sysadmin`, gets a `"Personal"` library as its admin, and has `current_library_id` set to it. It
does nothing once another user or any library exists. No env-var allowlist exists: the first
person to sign in is the admin.

Local mode reaches it in two ways:
- `POST /api/auth/bootstrap`, which returns 409 once any user exists. `GET /api/auth/mode`
  reports `needs_bootstrap`.
- `SeedLocalAdmin` at startup, when `DAMSON_LOCAL_ADMIN_EMAIL` and `_PASSWORD` are both set.

Re-seeding never rotates the password. OIDC mode needs no setup: the first IdP sign-in runs the
same bootstrap.

### 6.3 Authorisation

Two role hierarchies, independent of each other:
- `users.role`, `user` | `sysadmin`: the server-wide surface (`/api/admin/*`), gated by
  `requireSysadmin(c)` in the handler. Any user may create a library and becomes its admin.
- `library_memberships.role`, `admin` | `member`: library administration, gated by
  `requireAdmin(c)` on scoped routes and `userCanAdminLibrary` on `:id` routes.

`h.requireLibrary()` scopes a request to the caller's `current_library_id`. It attaches the library
and membership (`getLibrary(c)` / `getMembership(c)`) and answers **403 on any gap, never 404**,
so whether a membership exists does not leak. A non-member therefore never reaches a handler with
the library's data.

Routes addressing a library by `:id` stay on `authed` and gate in the handler
(`userCanAdminLibrary`), so a sysadmin can administer any library; moving them into `scoped` locks
sysadmins out.

#### Traps

- **Never join an IdP login to an existing user on an unverified email.** Anyone who controls an
  IdP account claiming that address would take over the account. `resolveUser` drops unverified
  email claims entirely.
- **Declare a new privileged route inside the `admin` group, not on `authed` with only
  `requireSysadmin`.** A sysadmin's personal access token passes `requireSysadmin`. Only the
  group's `auth.RefuseTokens()` keeps tokens off the server-wide surface (§18a).

### 6.4 HTTP security posture

The posture assumes that **every stored byte is hostile**, because the SPA keeps its bearer in
`localStorage` in both modes. A bearer in a header avoids CSRF. The cost is that any script running
on Damson's origin can read the token. Everything below keeps uploaded content from becoming that
script. TLS and HSTS are the reverse proxy's job.

- `securityHeaders` (`cmd/server/main.go`) sets a same-origin CSP plus `nosniff`, `DENY` framing
  and `no-referrer`. In OIDC mode it adds the issuer's origin to `connect-src` and `frame-src`,
  for discovery and the silent-renew iframe.
- `GET /api/files/:objectKey` always serves `application/octet-stream` as an attachment.
  `/api/thumbs/` and `/api/previews/` forward only an image content type from
  `allowedThumbContentTypes`, and fall back to octet-stream otherwise.
- External storefront images are fetched server-side by the SSRF-guarded metadata client and
  streamed from `GET /api/metadata/image`, which accepts only `image/*` responses. The SPA shows
  them as `blob:` URLs.
- `objectKey` parameters must match `objectKeyRE` (64 lower-case hex). `source_url` must be an
  absolute http(s) URL (`validateSourceURL`).
- `jsonBodyCap` caps non-multipart request bodies. `http.Server` sets explicit read, write and
  idle timeouts.

#### Traps

- **Never serve stored bytes with their recorded content type, or inline.** An uploaded HTML or
  SVG file would run on Damson's origin and could read the bearer. New byte-serving endpoints copy
  `getFile`'s headers or `getThumb`'s allow-list.
- **Proxy a third-party image through `/api/metadata/image`; do not add its origin to `img-src`.**
  The CSP is the second line of defence for the token.

---

## 7. Request lifecycle

`internal/handlers/api/handler.go` (`AddRoutes`) mounts everything under `/api` in layers:
- **Public:** `/health` and `/auth/mode`. Local mode adds `/auth/login` and `/auth/bootstrap`.
- **`authed`:** `auth.Middleware(authenticator, tokens)` runs first. A `damson_pat_` bearer goes
  to the token authenticator, and anything else goes to the mode's authenticator. Every
  authentication failure is a generic 401. A read-scoped token gets 403 on anything but GET/HEAD.
- **`admin`:** an `authed` group carrying `auth.RefuseTokens()`, for the sysadmin surface.
- **`scoped`:** an `authed` group carrying `requireLibrary()` (§6.3).

Global middleware in `cmd/server/main.go` runs first: request ID, `requestLogger`, panic recovery,
security headers, then the body cap.

Handlers are methods on one `*Handler`, built by `api.New(api.Deps{...})`. Tests use
`newTestHandler(t)`: a per-test Postgres schema with all migrations, plus `LocalFS` in
`t.TempDir()`. It does not run the `cmd/server` middleware.

Errors go through the `respondNNN` helpers in `helpers.go`. Inside a GORM transaction, return
`errBadRequestf` / `errConflictf` and map the result with `respondGORM(c, err, "<entity> not
found")`. A 500 always returns the generic `"internal error"` string.

Logging is `log/slog`: JSON by default, text when `DAMSON_LOG_FORMAT=text`. `requestLogger` (not
`gin.Logger()`) writes one line per response. On 4xx and 5xx that line includes the causes
attached to `c.Errors`. It never logs bodies, headers or query strings.

### Traps

- **Pass the cause to `respond500` / `respond400`; don't hand-roll
  `AbortWithStatusJSON(500, …)`.** A hand-rolled abort either leaks the error to the client or
  drops it from the log line. The helpers record it on `c.Errors`, which is the only route into
  `requestLogger`'s output.

---

## 8. Import pipeline

Bytes reach storage by one of two transports (§8.0). The handler then records the stored key and
enqueues a River job on the `import` queue (§12), which does the rest in
`internal/services/imports.Run`. `asset_imports.stage` is the durable record of progress. SSE on
`/api/imports/events` only announces transitions, so a client that misses an event recovers by
refetching.

Stages (`internal/importstage`, shared with `dmsn`), in order: `uploaded`, `extracting`,
`detecting` (reserved; no work runs in it), `thumbnailing`, `enriching`, then `done` or `failed`.

- The handler creates the asset (`needs_review=true`, or reuses the target asset), the version and the
  import row *before* storing bytes, so the object metadata can carry their ids. It enqueues only
  after the version's object key is recorded. A crash in between leaves a keyless version, which
  resume on boot marks failed (§12). If the bytes fail to store, the rows are removed.
- `extracting` runs the archive extractor for `.unitypackage` (§8.1) or `.zip` (§8.2). A loose file
  is already stored, and the handler has already written its single `asset_files` row.
- `thumbnailing` (§11.1) and `enriching` (§9) are best-effort and never fail an import.
- Once the files are committed, the version is handed to the `preview` queue (§11.2). After `done`,
  if `library_settings.retain_original_packages` is off, the archive original is purged.

A failure at any stage stores `stage='failed'` and the reason. `POST /api/imports/:id/retry` accepts
only `failed` rows. It re-runs the pipeline into the same version, re-extracting in place, under a
fresh import row.

Version matching: `POST /api/imports` may return `suggested_match` (`{asset_id, name}`) when the name
matches an existing asset, and the SPA offers to re-target the upload as a new version. The store
version comes from the filename (`imports.ParseImportFilename`), or from the `store_version` field,
which overrides it. A `unity_cache` source suppresses the filename version. A store version that
already exists on the target asset is a 409, and so is uploading into a reference asset. Versions
order component-wise by `imports.CompareStoreVersions`. `POST /api/assets/:id/merge` folds one asset
into another: versions, tags, collection memberships and project pins. A version whose store
version already exists on the target stays behind, and its pins are remapped.
`POST /api/assets/:id/versions/:vid/subset` creates a version whose `asset_files` reuse the source's
object keys, audited as `asset.version.subset`.

The SPA's upload queue, concurrency cap and folder bundling (the browser zips a folder into one
archive) live in the notification centre (§16.7).

### 8.0 Upload transports

Two ways in. Both end in the same rows and the same queue: the shared half lives in
`internal/handlers/api/import_create.go` (`checkImportConflict` → `createImportRows` →
`finishImport`), so the transports differ only in how bytes reach storage. Both refuse a body over
`server_settings.max_upload_bytes`.

**One-shot: `POST /api/imports`, `multipart/form-data`.** The handler reads the body with
`r.MultipartReader()` and passes the `file` part to the storage backend unread. Every text field
(`asset_id`, `store_version`, `source_kind`, `source_relative_path`) must therefore come **before**
the `file` part. A recognised field after it is a 400 naming that field. On S3 the key is the
content hash, so a large body lands at a random `staging/` key and is copied server-side once the
hash is known. `staging/` sits outside the orphan sweep's prefixes (§5).

**Chunked and resumable: `/api/imports/uploads`.** This transport is for archives where a single
multi-hour request is a liability. The client declares `sha256` and `size_bytes` when it opens a
session, so the final key is known before the first byte arrives. There is no staging copy. If the
library already holds the content, the open call imports it at once (`202`, `deduped: true`) and
creates no session. Conflict checks (target asset, duplicate store version) also run at open, so a
conflict costs one round trip instead of the whole transfer.
- `PUT …/:id/chunks/:n` accepts chunks strictly in sequence. An index already received is a no-op
  200; an index ahead of the next expected one is a 409 naming the index to send.
- The running SHA-256 state is marshalled into `upload_sessions.hash_state` after each chunk, so
  chunks may land on different pods. Advancing it is conditional on `next_chunk`, so two requests
  racing the same index cannot both apply.
- `POST …/:id/complete` checks the running hash against the declared one. A mismatch aborts the
  multipart with a 400 and publishes nothing. On a match it creates the rows, completes the
  multipart and enqueues. It is idempotent once completed. `GET …/:id` is the resume point, and
  `DELETE …/:id` aborts.
- An open session holds no asset, version or import rows, so abandoning one costs only an aborted
  multipart. A session past `expires_at` returns 410. `internal/services/uploads.Janitor` (interval
  `DAMSON_UPLOAD_JANITOR_INTERVAL`) aborts the backing multipart, which no bucket listing would
  reveal, and marks the session `expired`.

Clients choose the transport by size (`CHUNKED_UPLOAD_THRESHOLD` in `web/src/lib/chunkedUpload.ts`,
`chunkedUploadThreshold` in `cmd/dmsn/chunked.go`). `dmsn --resume` forces chunking at any size.

### 8.1 Unity package extraction

`internal/services/extract/unitypackage.go`. A `.unitypackage` is a tar (gzipped or raw, detected
from the first two bytes) with one directory per Unity GUID, holding `asset`, `asset.meta`,
`pathname` and an optional `preview.png`. Each `asset` streams to storage. Files are assembled from
GUID bundles that have both content and a safe `pathname`. Only the first line of `pathname` is the
path. The cover is the `preview.png` of the first image-type asset, else the first preview found.

Extraction enforces a per-entry cap, a total cap and an entry-count cap. A bundle whose path is
absolute, carries a drive letter or contains `..` is skipped. Re-extraction
(`POST /api/assets/:id/versions/:vid/reextract`, retained `.unitypackage` only) rewrites that
version's files in place. It does not create a new version.

### 8.2 Zip extraction

`internal/services/extract/zip.go`. A `.zip` is its own archive kind (`package_kind='zip'`). It
covers Unity Hub's zip exports and ordinary asset archives, and produces the same `UnityResult` as
the tar path, under the same caps and the same unsafe-path skipping. An image in the archive is
harvested as the cover.

Zip extraction needs random access, so the worker copies the archive
to a local temp file first; size the pod's ephemeral disk to the
largest zip.

### Traps

- **Purge an archive original only after the import is `done`.** If you purge earlier, a restart
  re-runs extraction against an archive that no longer exists. Originals are content-addressed,
  so delete the object only when no other live version references it. The purged flag is
  per-version.
- **Fire the `enriching` transition even when enrichment is disabled.** The SPA relies on a stable
  SSE stage sequence. `enrichAsset` is the gated no-op.
- **Add a new one-shot form field to `uploadFormFields` in `upload_form.go`.** The reader skips
  unlisted parts without error, so the handler's `form.get` returns `""` and the import goes ahead
  without the field.
- **Do not run chunked uploads on LocalFS with more than one replica.** LocalFS keeps multipart
  parts under `<root>/uploads/<id>/` on the pod's own disk, so a chunk that lands on another pod
  cannot find its upload. S3 keeps all state in the bucket and the database.

---

## 9. Storefront metadata enrichment

`internal/services/metadata/`. An uploaded package carries only what its bytes reveal: a
filename-shaped name, an extracted cover and maybe a licence file. A storefront listing holds the
description, publisher, licence terms, a proper cover, a gallery and tags. This subsystem fetches
them.

`Registry` maps a URL host (exact or dot-suffixed) to a store adapter and falls back to a generic
HTML scraper. If an adapter fails, the generic scraper runs and the result is flagged
`StoreLookupFailed`. Adapters that can search by name implement `Searcher`.
`library_settings.preferred_store` orders search candidates so that store's hits come first.

Entry points:
- `POST /api/assets/:id/metadata/fetch` fetches the listing, **writes blank fields, licence (if
  none), cover (if none), gallery and tags at once**, stores the source record, and returns every
  differing field as a conflict.
- `POST /api/assets/:id/metadata/apply` overwrites the named conflict fields from the stored record.
- `GET /api/metadata/search` searches by name across searchers. `GET /api/metadata/preview` fetches
  by URL without touching any asset.

Auto-enrichment runs as the `enriching` stage of the import pipeline
(§8), gated by `library_settings.auto_enrich_enabled`. Auto-enrich
fills blanks only, except the name, which it replaces with the matched
store title, and it re-flags the asset `needs_review`.

The shared `Client` (`client.go`) is the single outbound choke point. Its dialer resolves every
destination and refuses non-public addresses (`ErrBlockedAddress`). The check runs at dial time, so
a redirect into internal space is caught too. The client also caps body size and redirects, and
spaces requests per host. A store that answers 429 or 503 is paused, and requests to it fail with
`RateLimitedError` until the pause ends. `Registry.FetchWaiting` waits the pause out.

### 9.1 Storefront images

The CSP's `img-src` excludes store CDN origins (§6.4), so listing imagery never loads in the browser
directly.
- The cover is fetched server-side, re-rendered by the raster thumbnail worker, stored under
  `thumbs/`, and set as the thumbnail with `assets.thumbnail_from_store=true`.
- The gallery lands in `asset_images`, also re-rendered, unique per `(asset_id, source_url)`.
  `POST /api/assets/:id/metadata/gallery/backfill` rebuilds it from the stored source record and
  skips URLs already held.
- Anything still remote is proxied through `GET /api/metadata/image?url=`, which uses the guarded
  client and passes only `image/*` responses. The SPA renders it as a same-origin `blob:` URL.

### Traps

- **Use `metadata/preview`, not `metadata/fetch`, to show a listing.** Fetch writes to the asset.
- **Fetch remote URLs only through `metadata.Client`.** An adapter that calls `http.Get` or builds
  its own client skips the SSRF guard and the per-store rate limit without any error.
- **Check `thumbnail_from_store` before writing an asset thumbnail.** A store cover is meant to
  outlive later extractions. Rescan and re-extract leave it in place.

---

## 10. Licence detection

`internal/services/licence/detect.go`. `Detect(text)` first checks a Unity Asset Store EULA
fingerprint, because licensecheck's SPDX corpus does not cover that EULA. It then runs
`google/licensecheck` and takes the longest match. Low coverage yields `unknown`. Coverage short of
canonical sets `canonical_diff` to a coverage note (`coverage NN%`); it is not a textual diff.
Attributes come from a built-in SPDX → attributes map (`AttributesForType`).

Detection runs on demand, not during import: rescan (`POST /api/assets/:id/rescan`), the licence
endpoints (`/api/licences/detect`, `/api/assets/:id/licence/detect`), and licence text taken from a
storefront listing (§9). For an asset's files, `pickLicenceCandidate` (`handlers/api/licences.go`)
chooses the shallowest file named `LICENSE`, `COPYING` or a variant, and reads its head.

### Traps

- **Never let an automatic path replace a licence with `reviewed_at` set.** Rescan skips reviewed
  licences, and enrichment writes only when the asset has none. A new path that writes
  unconditionally wipes out a human's review without any error.
- **Leave an unknown attribute `nil`, never `false`.** The UI shows `nil` as "unknown" and `false` as
  "no". A defaulted bool turns "not known" into a stated restriction.
- **Change licence status rules in all three places:** `LicenceStatusFor` (`licences.go`, licence
  endpoints), `quickLicenceStatus` (`projects.go`, grid and projects) and `licenceStatusSPDXBuckets`
  (`assets.go`, the SQL status filter). If you change one, the pill, the grid and the filter
  disagree with no failing test.

---

## 11. Thumbnails and previews

Two derivative pipelines, both stored content-addressed under `thumbs/`:
- **Thumbnails** are one small image *per asset*, for the grid, generated inside the import job.
- **Previews** are one larger, browser-safe image *per file*, for formats a browser cannot open.
  They are generated as their own tracked, retryable work on the `preview` queue.

### 11.1 Thumbnails

`internal/services/thumbnail/`. `Generator.Generate` classifies the input by content type, then by
extension, and renders one of three kinds. Raster input is resampled, then written as JPEG when the
source is opaque JPEG/WebP and as PNG otherwise. Audio becomes a waveform PNG (WAV in-tree, MP3,
Ogg Vorbis). A font becomes a rendered sample-text PNG. Each kind honours its
`library_settings.thumbnails_{raster,audio,font}_enabled` toggle. Anything else gets no thumbnail,
and the SPA shows a file-type icon.

Sources, by path:
- A loose upload is thumbnailed from its own bytes.
- An archive uses the extractor's cover (§8.1, §8.2).
- Enrichment can supply the store cover (§9.1).
- Rescan tries a renderable file first, then reuses an already rendered preview derivative.

`assets.thumbnail_content_type` is stored beside the key, so the serving handler sets
`Content-Type` from it within the §6.4 allow-list instead of sniffing. A thumbnail failure is logged
and swallowed.

### 11.2 File previews

`internal/services/preview/`. A preview is a web-safe derivative of one file, larger than a
thumbnail. `preview.Supports` (by extension) decides which files get one. Output has a long-edge cap.
A decompression-bomb guard reads `DecodeConfig` first and refuses images above its pixel ceiling.
Such a file ends `failed`, and a requeue fails it again.

`asset_files.preview_status` is first-class state: `unsupported`, `queued`, `processing`, `done`
(`preview_object_key` set) or `failed`. `preview.MarkVersionQueued` classifies a version's files,
marking the supported ones `queued` and the rest `unsupported`. A version's rollup is `processing`
if any file is rendering, else `queued` while any remain, else terminal. Failures show as a count,
not as the status. Re-rendering keeps the old derivative until the new one is `done`.

Surfaces: `GET /api/previews` (queue) and `GET /api/previews/events` (SSE), rendered at
`/processing`. Requeue works per asset (`POST /api/assets/:id/previews/requeue`) or per file
(`…/files/:fileId/previews/requeue`), and is how a `failed` render is retried.
`GET /api/previews/:objectKey` serves the bytes. Import handoff, rescan and requeue are all gated
by `library_settings.previews_enabled`; requeue returns 409 when it is off.

### Traps

- **After writing a version's `asset_files`, call `preview.MarkVersionQueued` and enqueue the
  version.** New rows default to `unsupported` with no derivative, so a path that rewrites or copies
  files without it leaves the version with no previews and nothing queued.

---

## 12. Background jobs (River)

`internal/queue` runs imports, previews and store lookups as River jobs, a queue kept in Postgres.
The pipelines themselves live in their services (`internal/services/imports`,
`internal/services/preview`, and the API handler for store lookups); the queue only schedules them.
River has its own `pgxpool`, apart from GORM's, so its LISTEN/NOTIFY pickup and its connections
stay out of web traffic's way.

The database is the source of truth: the queue tracks *work*, while `asset_imports.stage`,
`asset_files.preview_status` and `store_lookups.status` track *state*.

Contract:

- **Queues.** `import` and `preview` are sized and timed out from `internal/config`
  (default worker count `GOMAXPROCS`, default timeout `queue.DefaultJobTimeout`). `store_lookup`
  always runs one worker, so a storefront sees one caller however many lookups are queued.
- **Ids only.** A job payload carries identifiers; the worker reloads everything from the DB.
  `PreviewArgs.FileID == 0` means every queued file in the version.
- **Deduped by args** across River's non-terminal states, so enqueuing the same work twice is
  safe.
- **Enqueue is fire-and-forget.** An insert failure is logged, not returned. At boot,
  `imports.ResumeInFlight`, `preview.ResumeInFlight` and `ResumeStoreLookups` re-enqueue every
  row left non-terminal, which also recovers work a restart interrupted.
- **River owns its `river_*` tables** and migrates them from `queue.New`, under the same advisory
  lock key as the app's migrator. They are not in `database/migrations/`.

### Traps

- **Failure.** Terminal import errors must mark the row failed before
  returning, so the worker cancels with `JobCancel`. Anything else
  retries (5 attempts).
- **Give a new job kind a boot-time reconcile.** Because enqueue swallows its error, a job type
  without a `ResumeInFlight`-style sweep leaves its rows queued forever after one failed insert.
- **Do not raise `store_lookup`'s worker count to speed lookups up.** The single worker is what
  keeps Damson to one caller per store.

---

## 13. Search

Free-text search is a `search_vector` column on `assets`, matched with a GIN index. A trigger on
`assets` builds it from name, description, publisher, tag paths and `location_path`, at falling
weights; a trigger on `asset_tags` re-fires it when tags change. The query side is
`buildPrefixTsQuery`: every word is required, and the last one is a prefix, so typing "pack"
matches "package".

`applyAssetFilters` (`internal/handlers/api/assets.go`) composes `q` with every other list filter
and is shared by `GET /api/assets`, its alias `/api/search`, and bulk store lookups. Its `LIKE`
branch for non-Postgres dialects is dead code: Postgres is the only dialect, in tests too.

### Traps

- The trigger's `UPDATE OF` list must grow
  with the vector's inputs, or the index goes silently stale; the
  `asset_tags` trigger re-fires it with `SET name = name` and must not
  touch `updated_at` (migration 007).

---

## 14. Audit

`internal/services/audit` writes `audit_log` rows for user-driven changes, made by request
handlers. Library-scoped events carry `library_id`; server-level events (library created or
deleted, user role changed, server settings updated) carry `NULL`.

Contract:

- **Scope.** Handler writes are audited, most of them inside the mutation's transaction; metadata
  apply and asset rescan write theirs after the change, and a failure there is only logged. The
  import path writes none for creating the asset and version, for auto-enrichment, or for the
  cache publisher hint (§18b). Tags it attaches are audited with no user.
- **`audit.Write` returns an error** on a missing action or target type; inside a transaction that
  fails the action.
- **Diff shape** comes from `audit.Created`, `audit.Updated` and `audit.Deleted`:
  `{"created": …}`, `{"before": …, "after": …}`, `{"deleted": …}`.
- **Read surfaces.** Per asset and per project, the library feed (`/api/audit`), and the sysadmin
  feed (`/api/admin/audit`): `?scope=server` (default, `library_id IS NULL` only) or `?scope=all`
  (every library too, each row with its `library_name`).
- **Retention** is per library, `library_settings.audit_retention_days`; unset keeps forever.
  `audit.Purger` sweeps every `DAMSON_AUDIT_PURGE_INTERVAL`. Server-level events are never purged.

### Traps

- **Pass the mutation's `tx` to `audit.Write`, not `h.db`.** On `h.db` the row commits even when
  the change rolls back, and a failed audit write no longer fails the action.

---

## 16. Frontend

### 16.1 Layout shell

`web/src/App.tsx` holds the routes; `components/Shell.tsx` frames every authenticated page as
`AppShell` (rail | optional sidebar | main) with the `NavRail`. Pages pass their own sidebar, and
omit it to collapse the column.

Everything except `/login`, `/bootstrap` and `/auth/callback` sits behind `<AuthGuard>`, which
also holds a first-login user on the profile-setup step until they have an email. The asset detail
panel is URL-addressable (`/library/:assetId`) so a pasted link opens the right asset;
`/library/:assetId/browse` is the scoped file browser over a multi-file asset's manifest.

### 16.2 State

- **Server state** goes through TanStack Query. Query keys live in `lib/queryKeys.ts` only; data
  hooks in `hooks/useDataQueries.ts` return the normalised `{ data, loading, error, reload }`
  shape; mutations live in `hooks/useMutations.ts`.
- **UI state** stays local to components. Library filter and search state lives in URL search
  params (`lib/libraryFilters.ts` parses it): the URL is the source of truth, so a reload or a
  pasted link reproduces the view.
- **Theme** is `damson.theme` in `localStorage`, else `prefers-color-scheme`. Light is the base;
  dark is the `.dark` class on `<html>`.

#### Traps

- **Keep the blocking theme script in `web/index.html`.** `main.tsx` applies the stored theme too,
  which makes the inline script look redundant, but `main.tsx` runs after first paint: moving the
  logic there gives every dark user a white flash.

### 16.3 API client

`src/api.ts` is the only door to the API. `fetchJson<T>` / `fetchMaybeJson<T>` (204-tolerant) and
`fetchBlob` attach the bearer from `lib/token.ts:getAccessToken()`. On a 401 they try one renewal,
retry once, and then throw `AuthRedirectError`. Any other failure throws `ApiError` carrying the
server's `error` message. `uploadFile` uses `XMLHttpRequest` because fetch has no upload progress.
Components never call `fetch`. The raw callers that remain are the pre-auth calls in
`lib/authMode.ts` and `lib/localAuth.ts`, the SSE hooks (`fetchEventSource`) and the chunk PUT in
`lib/chunkedUpload.ts`. All of them except the pre-auth calls still take their bearer from
`getAccessToken()`.

### 16.4 SPA auth flow

The auth mode is a server deployment choice that the SPA learns from `GET /api/auth/mode`, through
`useAuthMode()` into TanStack Query. That cache is the only copy. `getCachedAuthMode()` reads it
synchronously for `api.ts`, `lib/token.ts` and `lib/logout.ts`. In OIDC mode, `fetchAuthMode`
also configures the `UserManager` singleton from the response, so it must resolve before anything
calls `getUserManager()`. `/auth/callback` is a cold load, which is why it calls `fetchAuthMode`
itself.

`AuthGuard` gates the app:
- **OIDC:** no stored user, or an expired one, leads to `signinRedirect`. The browser leaves the
  SPA. `automaticSilentRenew` and the 401 path's `signinSilent` keep the session alive, both
  through the same `/auth/callback` route.
- **Local:** no stored session (`damson.local.token` in `localStorage`), or an expired one, leads
  to `/login`, or to `/bootstrap` while `needs_bootstrap` holds. `getAccessToken()` refreshes the
  JWT through `POST /api/auth/refresh` when it is close to expiry, sharing one in-flight refresh
  between callers. A local 401 cannot be renewed: it clears the session and redirects.

Once a session is valid, `ProfileGate` holds a `needs_profile` user on the profile step (§6.1).
Logout (`lib/logout.ts`) is local only in both modes. In OIDC it removes the stored user and does
not end the IdP session, which other apps may share.

#### Traps

- **Take the bearer from `getAccessToken()`; never read `damson.local.token` or the
  `UserManager` user yourself.** Reading local storage skips the near-expiry refresh. Picking the
  OIDC user's `access_token` sends a token the backend rejects, because it verifies the id_token
  (§6.1).
- **Don't issue an authenticated request before `useAuthMode()` has resolved.** With the mode
  cache empty, `getAccessToken()` returns no bearer. The 401 path then treats the mode as local and
  clears the stored session, so the user is logged out with no visible error.
- **After bootstrap, seed the mode with `queryClient.setQueryData` before any invalidation.**
  Awaiting `invalidateQueries` with no active subscriber hangs, which leaves the user on a frozen
  submit button. `BootstrapPage` seeds the value, then fires a no-await invalidate.

### 16.5 Grid

`components/AssetGrid.tsx` virtualises rows with `@tanstack/react-virtual`; its column count comes
from a `ResizeObserver`-driven hook, and keyboard selection scrolls into view through
`virtualizer.scrollToIndex`. `LibraryPage` binds the library's shortcuts through
`hooks/useKeyboardShortcuts.ts`, which skips editable targets unless a binding sets
`allowInEditable`.

#### Traps

- **Pass `useKeyboardShortcuts` a memoised bindings array whose handlers read state through a
  ref**, as `LibraryPage` does. An inline array is re-created on every refetch and rebinds the
  document listener on every paint.
- **Make a new library shortcut stand down when `browsing` is set.** In browse mode the file
  browser owns Escape, the arrows and `/`; an unguarded grid binding fights it for them.

### 16.6 Design system

`web/src/design-system/`, in three tiers — primitives, compositions, domain — behind the
`@/design-system` barrel. The token contract and the traps: `docs/design-system.md`.

### 16.7 Notification centre

`web/src/components/notifications/`. Client-side only — there is no notifications table or
endpoint. `NotificationProvider` is mounted above `<Routes>` in `App.tsx`, which is the point:
**it owns the upload queue**, so uploads survive navigation.

It also drives the shell's unread / in-flight badge, and is where upload concurrency capping
(`MAX_CONCURRENT_UPLOADS`), per-upload cancel and the "add as a version of an existing asset?"
prompt live (§8.0).

#### Traps

- **Keep upload state above `<Routes>`.** In a page component, a navigation unmounts it and aborts
  the in-flight XHR.

---

## 18. Deployment

The `Dockerfile` builds in three stages: the SPA into `web/static/`, then a `CGO_ENABLED=0` Go
binary that embeds it, then a minimal Alpine image running as the non-root `damson` user with a
`HEALTHCHECK` on `/api/health`. The binary serves the SPA at `/`: any path outside `/api/` that is
not a static asset returns `index.html`.

CI/CD is Woodpecker (`.woodpecker.yml`):

- **Pull request** — tests only.
- **Push to `main`** — gofmt, `go vet`, module tidiness, the Go suite, the frontend build and unit
  tests, Playwright e2e; then the image is pushed to `ghcr.io/thomas-elliott/damson` as
  `main-<sha>` and `latest`, and the test kustomization in the gitops repo is bumped for ArgoCD. An
  advisory SonarQube scan runs last and cannot fail the pipeline.
- **Tag `v*`** — no rebuild and no test run: `crane` promotes the existing `main-<sha>` image to
  the version tags, then the prod kustomization is bumped.

The target is the k8s cluster (namespaces in docs/observability.md).

### Traps

- **Bump the Playwright step's image tag with `@playwright/test`.** `web/package.json` pins it
  exactly, and the image's bundled browsers only resolve when the two versions match.
- **Do not tag a commit that was never built on `main`.** The tag pipeline promotes an existing
  image; with none to promote, no release happens.
- **Run `just check` before pushing; a green CI does not mean lint passed.** ESLint and Prettier
  have no CI step.

### 18.1 First-run flow

`DAMSON_POSTGRES_DSN` is the only required variable. Local auth and local storage are the defaults;
the JWT signing key is generated on first boot, and the key and blobs are written under `./data/`
relative to the working directory unless configured elsewhere. Migrations run at startup under a
Postgres advisory lock.

The first admin comes from one of three paths, all running `BootstrapFirstAdmin`, which also
creates a default "Personal" library:

- **In-app (local).** `GET /api/auth/mode` reports `needs_bootstrap`, the SPA shows `/bootstrap`,
  and `POST /api/auth/bootstrap` creates the admin and returns a signed-in session.
- **Env seed (local).** Set both `DAMSON_LOCAL_ADMIN_EMAIL` and `DAMSON_LOCAL_ADMIN_PASSWORD`; the
  admin is created at startup and the bootstrap page never appears.
- **OIDC.** The first user to sign in on an empty system is bootstrapped. Users are keyed by
  `(issuer, subject)` in `user_identities` (§6.1).

Everyone after that signs in to "no library access yet" until a library admin invites them.

---

## 18a. The `dmsn` CLI

`cmd/dmsn` is a companion binary for scripting a library and for handing to an agent. It talks to
the same HTTP API as the SPA and builds with `just build cli`; there is deliberately no
distribution (docs/progress.md, Known gaps).

**Credential.** Personal access tokens in `api_tokens`. The secret is `damson_pat_` plus random
bytes, stored as a plain SHA-256 hex digest. It is not bcrypt: the secret is already
high-entropy, so a slow KDF buys nothing, and a fast digest allows the indexed lookup every request
makes.

`auth.Middleware` recognises the `damson_pat_` prefix before the mode authenticator, so tokens work
the same in local and OIDC deployments. A token is pinned to one library: the middleware sets
`CurrentLibraryID` on the request's copy of the user only, so a running sync is unaffected by its
owner switching library in the SPA.

Two gates sit outside the handlers:

- **Scope.** A `read` token may only use GET and HEAD; anything else is 403 in `auth.Middleware`.
  `POST /api/imports/preflight` is a pure read that a read-scoped token is nonetheless refused.
- **`auth.RefuseTokens()`** closes a route to tokens entirely: token management and the whole
  `admin` group in `AddRoutes`. It runs after authentication, so an invalid token still gets 401
  and cannot probe which routes are gated. `POST /me/tokens` also requires a current library,
  because the new token is pinned to it.

**Output contract.** `internal/toon` is an emit-only TOON encoder; nothing in Damson reads TOON.
`internal/axi` layers the AXI contract on top: the TOON/JSON switch, `--fields` projection,
truncation, `help[]`, and the error envelope with its exit codes. Both take an `io.Writer` and do
no other I/O, so the contract is table-testable without a command. An empty result always states
why; a bare `key[]:` is never emitted.

**Leaf packages.** The CLI imports only small packages under `internal/` that pull in no gin,
GORM or River: `toon`, `axi`, `tagpath`, `importstage`, `importsource`, `unitycache` and
`buildinfo`. The server uses the same packages, so a rule shared by both lives in one place.

**Folder upload** (`upload --as asset`) zips the directory client-side and sends an ordinary
`.zip` (§8.2). The retained "original package" is that CLI-made archive.

### Traps

- **Render a multi-part result in one call** — `axi.RenderComposite`, or `axi.Record.Lists` for an
  entity's arrays. Two render calls emit two concatenated JSON documents, which do not parse, and
  `--json` quietly ends up lossier than TOON.
- **Never import `internal/services/...` from `cmd/dmsn`, even for one constant.** It pulls GORM
  and the AWS S3 SDK into the client binary. Move the rule into a leaf package instead.
- **Do not give a GET handler a side effect.** Read-scoped tokens are allowed every GET, so the
  write would be open to them.

## 18b. Importing the Unity Asset Store cache

`dmsn sync` imports Unity's local download cache, laid out as
`<Publisher>/<CategoryPath>/<Name>.unitypackage` under the per-OS `Asset Store-5.x` directory
(`unitycache.DefaultRoots`), or under `ASSETSTORE_CACHE_PATH`.

**The cache path is metadata.** It is the only cheap source of the publisher and the store's
category tree. Unity joins category segments with no separator (`3D ModelsCharactersHumanoids`);
`unitycache.SplitCategory` splits on each lowercase-to-uppercase boundary, so `3D Models` stays
whole.

Contract:

- **Provenance** is `asset_versions.source_kind`, `source_relative_path` and `source_size_bytes`,
  all nullable so a hand-dragged upload stays distinguishable. The client sends the first two with
  the upload (`POST /api/imports` or an upload session); the size is written from what storage
  reports, never from the client.
- **Idempotency is a path-and-size lookup, not a hash.** `POST /api/imports/preflight` answers
  `already_imported` (same path, same size), `updated` (same path, new size, naming the asset to
  add a version to), `name_match` or `new`. Where a path has several versions, the newest by
  `import_date` wins.
- **`ApplySourceHints`** runs before the enrich stage, so category tags land even with
  `auto_enrich_enabled` off. It sets `assets.publisher` only when blank, so a resumed import cannot
  overwrite a correction. That publisher then breaks ties between title matches in enrichment and
  lets a whole-word-prefix title qualify as a near match, recorded with lower confidence than an
  exact one.

### Traps

- **Derive publisher and category on the server, from the raw path.** `ParseCachePath` is a
  heuristic, so a fix must ship with the server to reach every installed CLI. The CLI uses the
  package for display only.
- **Do not let a `unity_cache` upload take its `store_version` from the filename.** The cache holds
  no version at all, yet `ParseImportFilename` reads `v2` out of `DOTween HOTween v2`; since
  `store_version` is unique per asset, that phantom would block the real `2.1.0` forever.
  `source_kind = unity_cache` suppresses the parse, and enrichment supplies the listing version.
