# Architecture

Each section describes one area of Damson and ends with its traps: rules that nothing enforces,
where getting it wrong fails silently. The code is the reference for anything finer-grained, such
as endpoint lists, column types and flags.

## What Damson is

Damson is a self-hosted digital asset manager for game-development assets. It is one Go binary
that embeds a React SPA, backed by Postgres and either an S3-compatible object store or the local
filesystem.

### The ideas the code assumes

- A library is the tenancy boundary. Access comes only from `library_memberships`. Inside a
  library, every member sees the same data.
- Storage is content-addressed. Postgres holds metadata, and a texture shipped in five packages is
  stored once.
- A version is the external pack version (`store_version`, e.g. `1.3`), parsed from the upload
  filename or left blank. Re-extraction rewrites a version's files in place. Only a new upload
  makes a new version.
- A reference asset is recorded by where it is (`location_path` and/or `source_url`) and has no
  versions or files.
- Projects are declarative. A project pins one version of each asset it uses.
- User-driven writes are audited in the same transaction as the change.
- Heavy work is queued. Imports, preview rendering and store lookups are durable River jobs in
  Postgres that survive a restart.

## Repository map

| Path | What is there |
|---|---|
| `cmd/server` | The server binary: config, middleware, wiring |
| `cmd/dmsn` | The `dmsn` CLI, a client of the HTTP API |
| `cmd/e2e-server` | A test-only server that the Playwright suite runs against |
| `damson.go` | The `//go:embed` directives for migrations and the built SPA (see Stack) |
| `database/migrations` | SQL migrations, the authoritative schema |
| `internal/handlers/api` | Every HTTP handler, as methods on one `*Handler` |
| `internal/auth` | Authenticators (local, OIDC, personal access tokens), middleware and bootstrap |
| `internal/models` | GORM structs |
| `internal/services/*` | Domain logic: `imports`, `extract`, `thumbnail`, `preview`, `imaging`, `metadata`, `licence`, `storage`, `uploads`, `audit`, `tags` |
| `internal/queue` | River setup and job kinds |
| `internal/config` | Every environment variable and its default |
| `internal/{toon,axi,tagpath,importstage,importsource,unitycache,buildinfo}` | Leaf packages that the CLI shares with the server |
| `web/src` | The SPA: `pages/`, `components/`, `hooks/`, `lib/`, `design-system/` |
| `tools/*.just` | The `just` modules behind every workflow |

## Stack

The backend is Go with Gin and GORM, on Postgres. Postgres is the only database, in production and
in tests. River runs background jobs on its own `pgx` pool. Auth uses `coreos/go-oidc` or HS256
JWTs, and S3 goes through `aws-sdk-go-v2`. Images are decoded in pure Go. Both binaries build with
`CGO_ENABLED=0`. `modernc.org/sqlite` is in `go.mod` only so that the CLI can read Unity's local
purchases cache.

The frontend is React, strict TypeScript, Vite, TanStack Query, React Router and Tailwind, with
Radix plus hand-rolled components in `web/src/design-system/` (see `design-system.md`). The
`typescript` dependency is aliased to `@typescript/typescript6`, with `@typescript/native` beside
it, so check `web/package.json` before assuming which one a script runs.

`//go:embed` can embed files below its own directory but not above it. `damson.go` is at the root
so that it can embed `database/migrations` and `web/static`, and it exports `MigrationsFS` and
`WebStaticFS`. `web/static/.gitkeep` exists so that the embed compiles before the SPA is built.

### Traps

- Never add a Go dependency that needs cgo. Under `CGO_ENABLED=0` it compiles and then fails at
  runtime. Pick a pure-Go equivalent.

## Data model

GORM structs are in `internal/models/`. The SQL migrations are the authoritative schema. A schema
change goes in both a new migration and the struct (see `CONVENTIONS.md`, Database Migrations).

- A person is one `users` row. Each OIDC sign-in is a `user_identities` row keyed by
  `(issuer, subject)`. Local users have a `password_hash` and no identity rows. Uniqueness is on
  `lower(email)` for non-blank values. An OIDC user with no verified email stays at `''`
  (`needs_profile`) until they set one.
- `library_memberships.role` is `admin` | `member`. `users.role` is `user` | `sysadmin`. The two
  hierarchies are independent.
- An asset owns versions, and a version owns `asset_files`. Each import of a version has an
  `asset_imports` row, which is the durable import state. Preview state is on `asset_files`
  (`preview_status`).
- `asset_versions.original_package_object_key` holds an `originals/` key when `package_kind` is
  `unitypackage` or `zip`. For a loose upload it holds the `blobs/` key of the single file.
  `original_package_purged` means the archive's bytes were reclaimed, while the key stays on the
  row.
- Tags are materialised paths per library (`environment/trees/deciduous`). `internal/tagpath`
  holds the one normalisation rule.
- The licence `attributes_json` booleans are tri-state, and `null` means unknown.
- `server_settings` is a singleton row. `library_settings` has one row per library, created with
  defaults on first read.
- An `upload_sessions` row is one resumable upload in flight. No asset, version or import row
  exists until it completes.

`created_by_user_id` and `pinned_by_user_id` record attribution, not ownership.

### Traps

- Scope a child row by joining to `assets` and filtering `assets.library_id`. Versions, files,
  imports, images and licences have no `library_id` of their own. Fetching one by id alone serves
  another library's row.
- Look up a user by email only when the email is non-blank, and compare `lower()`. A blank email
  matches every `needs_profile` user.
- Match an IdP identity on `(issuer, subject)`, never on `subject` alone.
- A version has a downloadable original only when the key is set, `original_package_purged` is
  false and `package_kind` is an archive. Checking the key alone offers a download that does not
  exist.
- Normalise a tag path with `tagpath.Normalise` before comparing it.
- Writing `false` to a default-true column needs an explicit `Update`. GORM omits zero values on
  Create when the field has a `default:` tag.

## Object storage

`internal/services/storage` provides a `local` backend (the default) and an `s3` backend
(path-style, for MinIO or Garage). Both use the same key layout, so a deployment can move between
them with a plain copy. Keys are `<prefix>/<hh>/<sha256>`:

- `blobs/` holds extracted content.
- `originals/` holds uploaded archives, kept apart so that retention can purge them.
- `thumbs/` holds derived images (thumbnails, previews, gallery, store covers), all rebuildable.

Nothing on the write path deletes an object. Reclaiming space is a two-step sysadmin action in
`storage/cleanup`: a scan finds objects that nothing references and that are older than
`DAMSON_ORPHAN_MIN_AGE`, and a delete re-checks those keys and audits each deletion.

### Traps

- Add any new column that holds an object key to `knownKeys` in `cleanup.go`. Otherwise the sweep
  deletes its objects.
- The age floor does not protect a reused object. Dedup reuses a key without touching its
  modification time.
- Keep LocalFS's hex-only check in `pathFor`. It stops an HTTP bug from becoming a path traversal.

## Authentication and authorisation

`DAMSON_AUTH_MODE` picks one authenticator per deployment:

- `local` (the default) uses bcrypt passwords and HS256 JWTs, refreshed through
  `/api/auth/refresh`. `users.tokens_valid_after` is the revocation watermark. Login is throttled
  in memory, per process.
- `oidc` verifies the IdP's id_token. `OIDCAuthenticator.resolveUser` matches on
  `(issuer, subject)`, then on a verified email (linking a new identity), and otherwise creates a
  user. After that first sign-in the app owns the user's email and display name.

Personal access tokens (`damson_pat_…`, stored as a SHA-256 digest) work in either mode. Each one
is pinned to a single library, and a `read` token may only use GET and HEAD.

On an empty system, the first user to sign in becomes `sysadmin` and gets a "Personal" library
(`auth.BootstrapFirstAdmin`). In local mode this happens through `/api/auth/bootstrap` or through
`DAMSON_LOCAL_ADMIN_EMAIL`/`_PASSWORD` at startup.

`AddRoutes` in `internal/handlers/api/handler.go` puts each route in one of these layers:

- The public layer serves health and auth mode, plus login and bootstrap in local mode.
- `authed` runs `auth.Middleware`. Every authentication failure is a generic 401.
- `admin` is `authed` plus `auth.RefuseTokens()`, for the sysadmin surface.
- `scoped` is `authed` plus `requireLibrary()`. This scopes the request to the caller's current
  library and answers 403 on any gap, never 404.

Routes that address a library by `:id` stay on `authed` and gate inside the handler
(`userCanAdminLibrary`), so that a sysadmin can administer any library.

The SPA keeps its bearer in `localStorage`, so the server's HTTP posture treats every stored byte
as hostile. The server sets a same-origin CSP. Stored files are served only as
`application/octet-stream` attachments, and thumbnails only with an image content type from an
allow-list. Third-party images are proxied server-side. TLS and HSTS are the reverse proxy's job.

### Traps

- Never join an IdP login to an existing user on an unverified email. Doing so is an account
  takeover.
- Put a new privileged route in the `admin` group, not on `authed` with only `requireSysadmin`. A
  sysadmin's token passes `requireSysadmin`.
- Do not give a GET handler a side effect. Read-scoped tokens may call every GET.
- Never serve stored bytes inline or with their recorded content type. An uploaded HTML or SVG file
  would run on Damson's origin and could read the bearer. Copy `getFile`'s headers or `getThumb`'s
  allow-list.
- Proxy a third-party image through `/api/metadata/image`. Do not add its origin to the CSP.

## Request lifecycle

The global middleware in `cmd/server/main.go` runs request ID, `requestLogger`, recovery, security
headers and the body cap, in that order. Handlers are methods on one `*Handler`, built by
`api.New(api.Deps{...})`. Tests build one with `newTestHandler(t)`, which gives each test its own
Postgres schema and a temporary `LocalFS`. It does not run the server middleware.

Errors go through the `respondNNN` helpers in `helpers.go`. Inside a transaction, return
`errBadRequestf` / `errConflictf` and map the result with `respondGORM`. A 500 always returns
`"internal error"`. Logging is covered in `observability.md`.

### Traps

- Pass the cause to `respond500` / `respond400`; never hand-roll `AbortWithStatusJSON(500, …)`. The
  helpers record the cause on `c.Errors`, which is the only way a cause reaches the log line.

## Import pipeline

Bytes arrive by one of two transports, which share `import_create.go`:

- A one-shot upload is a streamed multipart `POST /api/imports`. Text fields must come before the
  `file` part.
- A chunked, resumable upload goes to `/api/imports/uploads`. The client declares the SHA-256 up
  front, the chunks arrive in sequence, and the running hash state is persisted so that chunks can
  land on different pods.

Clients pick the transport by size. The handler creates the asset, version and import rows, stores
the bytes, then enqueues an `import` job. `internal/services/imports.Run` moves the import through
the stages (`internal/importstage`): `uploaded` → `extracting` → `detecting` → `thumbnailing` →
`enriching` → `done` | `failed`. `asset_imports.stage` is the record. SSE on
`/api/imports/events` only announces transitions.

Extraction (`internal/services/extract`) handles `.unitypackage`, a tar of GUID bundles, and
`.zip`. Both enforce size and entry-count caps and skip unsafe paths. Zip needs random access, so
the archive is copied to a local temp file first. Thumbnailing and enrichment are best-effort and
never fail an import. Once the files are committed, the version is handed to the preview queue.

In the SPA, the upload queue is in the notification centre (`components/notifications/`). It is
mounted above `<Routes>` so that uploads survive navigation.

### Traps

- Purge an archive original only after the import reaches `done`, and only when no other live
  version references the object. Otherwise a restart re-extracts from bytes that no longer exist.
- Fire the `enriching` transition even when enrichment is disabled. The SPA depends on a stable
  stage sequence.
- Add a new one-shot form field to `uploadFormFields` in `upload_form.go`. Unlisted parts are
  skipped silently.
- Do not run chunked uploads on LocalFS with more than one replica. The parts sit on one pod's
  disk.
- Do not let a `unity_cache` upload take its `store_version` from the filename. The Unity cache
  has no versions, and a phantom `v2` parsed from a title would block the real one forever.
- Keep upload state above `<Routes>`. A navigation unmounts a page and aborts its in-flight
  request.

## Storefront metadata

`internal/services/metadata` fetches a listing's description, publisher, licence, cover, gallery
and tags. A `Registry` maps a URL host to a store adapter and falls back to a generic scraper.
Auto-enrichment is the import's `enriching` stage. It fills blank fields only, apart from the name.
Store covers and gallery images are fetched server-side and stored under `thumbs/`.

### Traps

- Fetch remote URLs only through `metadata.Client`. It is the SSRF guard (checked at dial time, so
  redirects are covered too) and the per-store rate limit.
- Use `metadata/preview` to show a listing, not `metadata/fetch`, which writes to the asset.
- Check `thumbnail_from_store` before writing an asset thumbnail. A store cover is kept across
  later extractions.

## Licence detection

`internal/services/licence` first checks a Unity Asset Store EULA fingerprint, then runs
`google/licensecheck`. Detection runs on demand (rescan, the licence endpoints, storefront text),
never during import.

### Traps

- Never let an automatic path replace a licence that has `reviewed_at` set.
- Leave an unknown attribute `nil`, never `false`. `false` is shown to the user as a stated
  restriction.
- Change a licence status rule in both `LicenceStatusFor` and `licenceStatusSQL` (`licences.go`).
  The grid, project pins and the status filter use the SQL form.
  `TestLicenceStatus_AgreesAcrossGridProjectFilterAndDetail` checks that the two agree.

## Thumbnails and previews

- Thumbnails (`internal/services/thumbnail`) are one small image per asset, made inside the import
  job. Raster images are resampled, audio becomes a waveform and fonts become a sample. Each kind
  has its own library setting.
- Previews (`internal/services/preview`) are one larger, browser-safe image per file, made on the
  `preview` queue. `asset_files.preview_status` moves through `unsupported` | `queued` |
  `processing` | `done` | `failed`. A re-render keeps the old derivative until the new one is done.

`internal/services/imaging` is the shared decoder. It rejects any image above its pixel ceiling.

### Traps

- After writing a version's `asset_files`, call `preview.MarkVersionQueued` and enqueue the
  version. New rows default to `unsupported`, and nothing else will queue them.

## Background jobs

`internal/queue` runs the `import`, `preview` and `store_lookup` queues on River, with its own
`pgxpool`. River migrates its own `river_*` tables. The database rows (`asset_imports.stage`,
`asset_files.preview_status`, `store_lookups.status`) are the state, and the queue only schedules
work. Job payloads carry ids only. Jobs are deduplicated by their arguments. An enqueue failure is
logged, not returned, and at boot each pipeline's `ResumeInFlight` re-enqueues every row left
non-terminal.

### Traps

- Mark the row failed before returning a terminal error, and cancel the job with `JobCancel`. Any
  other error is retried.
- Give a new job kind a boot-time resume sweep. Enqueue logs its error instead of returning it, so
  without a sweep the rows stay queued forever.
- Keep `store_lookup` at one worker. The single worker keeps Damson to one caller per store.

## Search

`assets.search_vector` is a GIN-indexed tsvector, built by a trigger from the name, description,
publisher, tag paths and `location_path`. A second trigger on `asset_tags` re-fires it.
`buildPrefixTsQuery` requires every word and treats the last as a prefix. `applyAssetFilters` in
`assets.go` composes search with every list filter.

### Traps

- When you add an input to the vector, add its column to the trigger's `UPDATE OF` list, or the
  index goes stale silently. The `asset_tags` trigger must not touch `updated_at`.

## Audit

`internal/services/audit` writes `audit_log` rows for user-driven changes. A library-scoped event
carries `library_id`, and a server-level event carries `NULL`. Diffs use `audit.Created`,
`audit.Updated` and `audit.Deleted`. Retention is a per-library setting, swept by `audit.Purger`.

### Traps

- Pass the mutation's `tx` to `audit.Write`, not `h.db`. Otherwise the audit row commits even when
  the change rolls back.
- A new action type goes on `audit.ActionTypes`, or `audit.Write` refuses it. Copy the list to
  `web/src/lib/auditActionTypes.json`, which a Go test keeps in step. Give the type a verb in
  `lib/auditFormat.ts` and, unless it is `admin.*`, a filter chip: the audit page's chips
  (`ACTION_FILTERS` in `lib/auditFilters.ts`) list action types, or `family.*` prefixes, for the
  `action_type` filter, and a type no chip covers shows only under All. A Vitest fails on a listed
  type with no verb or chip.

## Frontend

- Routes are in `App.tsx`. `components/Shell.tsx` frames every authenticated page. The asset panel
  is URL-addressable (`/library/:assetId`).
- Server state goes through TanStack Query. Keys are in `lib/queryKeys.ts`, read hooks in
  `hooks/useDataQueries.ts` and mutations in `hooks/useMutations.ts`. Filter, search and paging
  state is kept in the URL (`lib/libraryFilters.ts`: `page` and `page_size` beside the filters).
  Anything that changes the filters, search or sort writes through `withPage(search, 1)` so the
  view starts again from page 1; a page past the end is moved to the last page once the total is
  known.
- API calls go through `fetchJson` / `fetchMaybeJson` / `fetchBlob` in `src/api.ts`. These attach
  the bearer, renew once on a 401, and throw `ApiError`.
- The SPA reads the auth mode from `GET /api/auth/mode` (`useAuthMode()`), and `AuthGuard` gates
  the app. OIDC uses `oidc-client-ts`, and local mode keeps its JWT in `localStorage`. Logout is
  local only.
- The theme is light by default, with dark as the `.dark` class on `<html>`.

### Traps

- Take the bearer from `getAccessToken()`. Never read `damson.local.token` or the OIDC user
  yourself: you skip the refresh, or you send the access_token when the backend verifies the
  id_token.
- Don't issue an authenticated request before `useAuthMode()` resolves. The 401 path then clears
  the session and logs the user out without showing an error.
- After bootstrap, seed the mode with `setQueryData` before any invalidation. Awaiting
  `invalidateQueries` with no subscriber hangs.
- Keep the blocking theme script in `web/index.html`. Moving it to `main.tsx` gives dark-mode users
  a white flash.
- Pass `useKeyboardShortcuts` a memoised bindings array whose handlers read state through a ref,
  and make a new library shortcut stand down while `browsing` is set.

## The `dmsn` CLI

`cmd/dmsn` scripts a library over the same HTTP API as the SPA, authenticating with a personal
access token. `dmsn sync` imports Unity's local Asset Store cache. The cache path
(`<Publisher>/<CategoryPath>/<Name>.unitypackage`) is the source of the publisher and category
tags, which the server parses with `unitycache.ParseCachePath` when it applies source hints.
Re-runs stay idempotent through `POST /api/imports/preflight`, which matches on path and size, not
on hash.

Output follows the AXI contract (`internal/axi` over the TOON encoder in `internal/toon`), which
supports TOON or JSON, `--fields`, truncation and an error envelope.

### Traps

- Never import `internal/services/...` from `cmd/dmsn`. It pulls GORM and the S3 SDK into the
  client. Move the shared rule into a leaf package.
- Render a multi-part result in one call (`axi.RenderComposite`). Two calls emit two JSON
  documents, which do not parse as one.
- Derive publisher and category on the server, from the raw path, so that a fix reaches every
  installed CLI.

## Build and deploy

The `Dockerfile` builds the SPA, then a static Go binary that embeds it, then a minimal Alpine
image that runs as a non-root user. The binary serves the SPA for any path outside `/api/`.
`DAMSON_POSTGRES_DSN` is the only required variable, and migrations run at startup under an
advisory lock.

### Traps

- Run `just check` before pushing. It is the full preflight: lint, format, `go.mod` tidiness, the
  SPA build, the unit tests and e2e.
