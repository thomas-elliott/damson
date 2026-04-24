# Damson — Technical Design Document

**Status:** Draft v1 **Author:** Thomas **Last updated:** 2026-04-23
**Companions:** `product-design-doc.md`, `ui-design.md`, `CONVENTIONS.md`

---

## 1. Purpose and scope

This document describes the technical design of **Damson**, the digital asset manager described in `product-design-doc.md`. It translates the product and UI designs into a concrete architecture: components, repository layout, data model, storage model, pipelines, API surface, and deployment.

It does not repeat the product or UI design. It refers to them for the *why* and focuses on the *how*. It also does not restate `CONVENTIONS.md` — that document fixes the stack and code style; this one extends it with project-specific architecture.

Scope: **MVP as defined in product design §7.** Deferred features are accounted for only where the data model or interface shape would change if we failed to plan for them.

## 2. Architectural principles

1. **Data captured, features deferred.** The MVP must store every piece of data that deferred features will need. Adding licence compliance reports, upstream update detection, or auto-tagging later must be a query/UI problem — never a migration or re-import problem.
2. **Content-addressed storage is the source of truth for bytes.** All file bytes live in S3 keyed by SHA-256. Postgres holds metadata and references. Deduplication is free at the storage layer.
3. **Import is background-first.** Nothing in the user-visible import flow blocks on extraction or thumbnailing. The UI shows progress; the queue reflects reality.
4. **Versioning is immutable.** An `AssetVersion`'s file manifest is fixed once created. Re-extractions produce new versions.
5. **Projects are declarative.** A project pins specific versions; nothing auto-upgrades. Upgrade is a logged, reversible action.
6. **Audit is a boundary.** Every write that crosses the HTTP layer writes an audit entry. Audit is not a "nice to have" retrofit.
7. **One binary, one image.** The Go server embeds the built SPA and serves it. Dev runs them split via Vite; prod runs them fused. No separate frontend deploy.
8. **Fit CONVENTIONS.md until a specific reason to diverge.** The conventions come from another working Go/React repo; they pay their way on day one.

## 3. Stack summary

Per `CONVENTIONS.md`, with the following project-specific selections:

**Backend**
- Go 1.26, Gin, GORM (Postgres prod, in-memory SQLite for tests)
- OIDC bearer auth (`coreos/go-oidc` + `oauth2`)
- S3-compatible media storage (MinIO on TrueNAS initially; AWS S3 also supported)
- `github.com/google/licensecheck` for SPDX licence matching (ships with an embedded corpus)
- Stdlib `image`, `image/png`, `image/jpeg`, `image/draw` + `golang.org/x/image/webp` (decode) + `golang.org/x/image/draw` (high-quality resampling) for raster thumbnails. No CGo. Thumbnail encoding format is a small open sub-decision (see §11).
- `golang.org/x/image/font` + Go Fonts for font samples; pure-Go audio decoding (`github.com/go-audio/wav`, minimp3) + custom SVG/PNG rendering for waveforms
- Standard library `archive/tar` + `compress/gzip` for `.unitypackage`; `archive/zip` for zip archives

**Frontend**
- React 18, TypeScript (strict), Vite
- TanStack Query v5 for server state
- React Router (data router)
- Tailwind CSS + shadcn/ui, `class` based dark mode (dark as default per UI design)
- `react-dropzone` for drag-and-drop; `@tanstack/react-virtual` for the asset grid

**Tooling**
- Task (`Taskfile.yml`) for all workflows
- `air` for Go hot-reload
- Vitest + Testing Library for the SPA
- Multi-stage Dockerfile (Node → Go → Alpine). No CGo dependencies, so the runtime image is slim.

**Module path:** `github.com/thomas-elliott/damson`.

## 4. System architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        Browser (SPA)                         │
│  React · Vite · TanStack Query · shadcn/ui · Tailwind        │
└────────────────────┬─────────────────────────────────────────┘
                     │ HTTPS (JSON REST, bearer OIDC token)
┌────────────────────▼─────────────────────────────────────────┐
│                       damson server (Go)                     │
│  Gin HTTP  ·  OIDC middleware  ·  Embedded SPA assets        │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ handlers/api                                         │    │
│  │  assets · versions · licences · projects ·           │    │
│  │  collections · import · audit · search               │    │
│  └────────────┬─────────────────────────────────────────┘    │
│  ┌────────────▼─────────────┐  ┌──────────────────────┐      │
│  │ services                 │  │ models (GORM)        │      │
│  │  storage   licence       │  │ Asset · Version ·    │      │
│  │  extract   thumbnail     │  │ File · Licence · ... │      │
│  │  search    audit         │  │                      │      │
│  └────────────┬─────────────┘  └──────────┬───────────┘      │
└───────────────┼───────────────────────────┼──────────────────┘
                │                           │
     ┌──────────▼──────────┐     ┌──────────▼──────────┐
     │ S3-compatible store │     │     PostgreSQL      │
     │  content-addressed  │     │  structured state   │
     │   (bytes + thumbs)  │     │  (incl. audit log)  │
     └─────────────────────┘     └─────────────────────┘
```

A single process. No separate worker tier in MVP — thumbnails generate synchronously in an HTTP goroutine or a small in-process worker pool, not via a queue. The pipeline is designed so swapping in a real queue later (NATS, RabbitMQ, or `river`) is a matter of moving service calls behind a queue interface, not re-architecting.

## 5. Repository layout

```
damson/
├── Taskfile.yml
├── Dockerfile
├── .dockerignore
├── .gitignore
├── .air.toml
├── go.mod
├── go.sum
├── README.md
├── cmd/
│   └── server/
│       └── main.go               # entry point
├── internal/
│   ├── config/                   # env-var loading, validation
│   ├── db/                       # gorm.Open, migration runner
│   ├── auth/                     # OIDC middleware, GetUser helper
│   ├── models/                   # GORM models (one file per entity)
│   ├── handlers/
│   │   └── api/                  # HTTP handlers, routes.go, one file per feature
│   ├── services/
│   │   ├── storage/              # S3 content-addressed put/get
│   │   ├── licence/              # detection, SPDX corpus, Unity EULA templates
│   │   ├── extract/              # .unitypackage, .zip extraction
│   │   ├── thumbnail/            # image/audio/font/unitypackage preview
│   │   ├── search/               # tsvector query building
│   │   └── audit/                # audit log writer
│   └── web/                      # go:embed of built SPA + static handler
├── database/
│   └── migrations/               # NNN_descriptive_name.sql, SQL-only
├── web/                          # React SPA
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── tailwind.config.ts
│   ├── index.html
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── api.ts
│   │   ├── lib/
│   │   │   ├── queryClient.ts
│   │   │   ├── queryKeys.ts
│   │   │   └── cachePolicy.ts
│   │   ├── hooks/
│   │   │   └── useDataQueries.ts
│   │   ├── components/
│   │   │   └── ui/               # shadcn primitives
│   │   ├── pages/
│   │   └── test/
│   │       └── setupTests.ts
│   └── static/                   # Vite build output (git-ignored)
└── docs/                         # existing product/UI/conventions docs + this
```

This mirrors the package structure in `CONVENTIONS.md` (`handlers/api`, `models`, `services`) rather than the speculative `internal/asset`, `internal/project` split in the product design doc §6. The conventions come from a working repo and we gain nothing by diverging.

## 6. Data model

### 6.1 Postgres schema

A consolidated sketch of the tables and their relationships. Field lists are suggestive; final types land in the first migration and Go model files.

#### Identity, libraries, audit

```
users
  id                     bigserial PK
  subject                text unique not null            -- OIDC sub claim
  email                  text not null
  display_name           text not null
  role                   text not null default 'user'    -- 'user' | 'sysadmin'
  current_library_id     bigint references libraries(id) nullable
  created_at, updated_at timestamptz default now()

libraries
  id                     bigserial PK
  name                   text not null
  description            text
  created_at, updated_at timestamptz default now()
  index(name)

library_memberships
  id              bigserial PK
  library_id      bigint references libraries(id) on delete cascade
  user_id         bigint references users(id) on delete cascade
  role            text not null                           -- 'admin' | 'member'
  created_at      timestamptz default now()
  unique(library_id, user_id)
  index(user_id)

audit_log
  id              bigserial PK
  library_id      bigint references libraries(id) on delete cascade   -- nullable only for server-level events
  user_id         bigint references users(id)            -- the actor
  action_type     text not null                          -- 'asset.create', 'project.pin.upgrade', 'library.member.invite', ...
  target_type     text not null                          -- 'asset', 'project', 'library', ...
  target_id       bigint
  diff_json       jsonb
  created_at      timestamptz default now()
  index(library_id, created_at desc)
  index(library_id, target_type, target_id, created_at desc)
  index(user_id, created_at desc)
  index(action_type, created_at desc)
```

`users.subject` is either an OIDC `sub` claim or a synthetic `"local:<email>"` in local-auth mode. Passwords, when present (local mode only), live in `users.password_hash` as bcrypt hashes. First-run bootstrap (either mode — see §8.1): the first qualifying user is created with `role='sysadmin'`, a default library ("Personal") is created, the user becomes its admin member, and `current_library_id` is set. Thereafter new users require an invitation (`library_memberships` row created by a library admin) before any non-identity API call succeeds.

**Current library context.** Every authenticated request operates against the user's `current_library_id`. The user switches library via `POST /api/me/current-library`, which updates the column and the next request picks it up. Stateless enough (the context is a single DB column read per request, cached with the user), bookmarkable (library-scoped URLs remain valid within the user's session), and avoids threading library IDs through every path. Multi-library users whose two browser tabs want different libraries are out of v1 scope — revisit with a `X-Damson-Library` header override later if it matters.

#### Assets, versions, files

```
assets
  id                    bigserial PK
  library_id            bigint references libraries(id) on delete cascade
  name                  text not null
  description           text
  source_url            text
  source_metadata_json  jsonb                         -- future: vendor, product id, purchase ref
  licence_id            bigint references licences(id) nullable
  current_version_id    bigint references asset_versions(id) nullable
  thumbnail_object_key  text                          -- SHA256 of thumb bytes
  needs_review          bool not null default true    -- import queue marker
  created_by_user_id    bigint references users(id)   -- attribution, not ownership
  search_vector         tsvector                      -- maintained by trigger
  created_at, updated_at timestamptz default now()
  index(library_id)
  index(library_id, needs_review) where needs_review
  gin index on search_vector
  gin index on (tags via asset_tags)

asset_versions
  id                          bigserial PK
  asset_id                    bigint references assets(id) on delete cascade
  version_label               text not null
  notes                       text
  import_date                 timestamptz default now()
  original_package_object_key text                  -- nullable, set for .unitypackage
  package_kind                text                  -- 'unitypackage' | 'zip' | 'loose' | null
  created_by_user_id          bigint references users(id)
  index(asset_id, import_date desc)
  unique(asset_id, version_label)

asset_files
  id                   bigserial PK
  asset_version_id     bigint references asset_versions(id) on delete cascade
  relative_path        text not null                -- e.g. Textures/grass.png
  size_bytes           bigint not null
  content_hash         text not null                -- SHA256 hex
  object_key           text not null                -- == content_hash
  detected_type        text                         -- MIME or custom category
  unity_guid           text                         -- nullable, from .unitypackage
  created_at           timestamptz default now()
  index(asset_version_id)
  index(content_hash)
  unique(asset_version_id, relative_path)
```

Content-addressing: `object_key == content_hash`. An asset file row is a *reference* to a blob; two assets sharing a texture share the row's `object_key` but not the row itself.

#### Licences

```
licences
  id                     bigserial PK
  asset_id               bigint references assets(id) on delete cascade
  detected_type          text                       -- SPDX id, 'unity-eula', 'custom', 'unknown'
  detection_confidence   real                       -- 0.0–1.0
  canonical_diff         text                       -- nullable, diff from canonical if modified
  original_text          text not null
  attributes_json        jsonb not null             -- { commercial_use, modification, ... }
  reviewed_by_user_id    bigint references users(id) nullable
  reviewed_at            timestamptz nullable
  created_at, updated_at timestamptz default now()
  index(asset_id)
  index(detected_type)
```

`attributes_json` carries structured booleans the UI presents as toggles. Canonical shape:

```json
{
  "commercial_use": true,
  "modification": true,
  "redistribution": true,
  "attribution_required": false,
  "share_alike": false,
  "open_source_required": false,
  "custom_terms": null
}
```

Unknown values are `null` (tri-state) — the UI distinguishes "user confirmed no" from "unknown."

#### Tags, collections, projects

```
tags
  id          bigserial PK
  library_id  bigint references libraries(id) on delete cascade
  path        text not null               -- materialised path: 'environment/trees/deciduous'
  parent_id   bigint references tags(id) nullable
  unique(library_id, path)
  index(library_id, parent_id)

asset_tags
  asset_id    bigint references assets(id) on delete cascade
  tag_id      bigint references tags(id) on delete cascade
  PK (asset_id, tag_id)
  index(tag_id)

collections
  id                    bigserial PK
  library_id            bigint references libraries(id) on delete cascade
  name                  text not null
  description           text
  created_by_user_id    bigint references users(id)
  created_at, updated_at timestamptz default now()
  index(library_id)

collection_assets
  collection_id   bigint references collections(id) on delete cascade
  asset_id        bigint references assets(id) on delete cascade
  added_at        timestamptz default now()
  added_by_user_id bigint references users(id)
  PK (collection_id, asset_id)

projects
  id                    bigserial PK
  library_id            bigint references libraries(id) on delete cascade
  name                  text not null
  description           text
  target_distribution   text not null            -- 'commercial', 'open-source', 'personal', 'unspecified'
  created_by_user_id    bigint references users(id)
  created_at, updated_at timestamptz default now()
  index(library_id)

project_asset_pins
  id                 bigserial PK
  project_id         bigint references projects(id) on delete cascade
  asset_id           bigint references assets(id) on delete cascade
  pinned_version_id  bigint references asset_versions(id)
  pinned_at          timestamptz default now()
  pinned_by_user_id  bigint references users(id)
  unique(project_id, asset_id)
  index(asset_id)
```

Every library-scoped entity carries `library_id` directly rather than inferring via parent FKs. This keeps queries simple (always filter by `library_id` at the top level), protects against orphan rows, and makes per-library cascading deletes straightforward. `created_by_user_id` / `pinned_by_user_id` are attribution, not ownership — they let the UI show who did what without changing access rules.

Tags are library-scoped: `(library_id, path)` is unique; the same tag path (`environment/trees`) can exist in two different libraries as two different rows. This is the stricter default — if a shared taxonomy proves useful later, a cross-library tag dictionary is additive.

### 6.2 Derived views / view models

Per the conventions' note on "enriched views": the following are likely to become Go view-model types with dedicated handlers or query builders rather than being leaked raw:

- `AssetGridRow` — asset + current-version file count + licence pill fields + thumbnail key
- `AssetDetailView` — asset + all versions (with file counts) + licence + tag chips + project pins + collections + recent audit
- `ProjectAssetRow` — pin + current-pinned-version + newest-available-version (flag if upgrade available)
- `ImportQueueRow` — asset + imported_at + detected_type + detected_licence + file count

### 6.3 Migrations

SQL-only, per `CONVENTIONS.md`. Initial sequence:

```
database/migrations/
  001_users.sql
  002_libraries.sql
  003_library_memberships.sql
  004_audit_log.sql
  005_assets_versions_files.sql
  006_licences.sql
  007_tags.sql
  008_collections.sql
  009_projects_and_pins.sql
  010_library_settings.sql
  011_search_vector_trigger.sql
  012_indexes.sql
```

Each file: `BEGIN; … COMMIT;`, `IF NOT EXISTS` throughout. A migration runner at startup reads the `database/migrations/` directory (embedded via `go:embed`), tracks applied files in a `schema_migrations` table, and applies new files in order. No Go migration framework — matches the repo style.

## 7. Object storage

### 7.1 Content-addressed layout

Single bucket. Two prefixes for clarity and lifecycle policy:

```
s3://<bucket>/
  blobs/<hh>/<hhhhh…>      # sha256-keyed blobs (first 2 hex chars sharded)
  thumbs/<hh>/<hhhhh…>     # thumbnail blobs, also content-addressed
```

- `blobs/` carries every file referenced by an `asset_files.object_key` and every original `.unitypackage` archive referenced by `asset_versions.original_package_object_key`.
- `thumbs/` carries generated thumbnails. Thumbnails are derived data — they can be rebuilt, so the bucket lifecycle can permit aggressive expiry if desired. They are content-addressed so identical thumbnails dedupe.

Upload path: stream to a temp location, SHA-256 on the way, `PutObject` with the hash as key (if not already present; cheap HEAD before PUT avoids bandwidth).

### 7.2 Service interface

```go
package storage

type Service interface {
    // PutBlob streams r to the bucket; if the hash is already present, returns
    // the existing key and size without re-uploading.
    PutBlob(ctx context.Context, r io.Reader) (objectKey string, size int64, err error)

    GetBlob(ctx context.Context, objectKey string) (io.ReadCloser, error)
    HeadBlob(ctx context.Context, objectKey string) (size int64, exists bool, err error)

    PutThumb(ctx context.Context, objectKey string, r io.Reader) error
    GetThumb(ctx context.Context, objectKey string) (io.ReadCloser, error)

    // Presigned URL for browser-direct download (streamed originals).
    PresignGet(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
}
```

Implementation uses the AWS SDK v2 with an endpoint override (MinIO). A local-filesystem implementation exists for tests (`testing` subpackage), keyed on the same SHA-256 hashes.

### 7.3 Garbage collection

Deferring now. The product design permits indefinite retention. A later job (nightly) walks `asset_files.object_key`, `asset_versions.original_package_object_key`, `assets.thumbnail_object_key` and deletes blobs not referenced.

## 8. Authentication and authorisation

### 8.1 Auth modes

Exactly one authenticator is active per deployment, picked at startup by `DAMSON_AUTH_MODE`. Both present the same `Authenticator` interface to the rest of the server (see `internal/auth/auth.go`); the middleware doesn't know which is in play.

**`local` (default).** Server-issued HS256 JWTs against users stored in Postgres with bcrypt password hashes.

- Signing key: `DAMSON_JWT_SIGNING_KEY` is honoured verbatim when set. When unset, `internal/auth.LoadOrCreateJWTKey` generates 32 random bytes on first boot and persists them atomically to `DAMSON_JWT_KEY_FILE` (default `./data/jwt.key`, mode 0600). Subsequent boots read the same file, so sessions survive restarts. The generated-file path is ideal for single-node dev; production deployments should still set `DAMSON_JWT_SIGNING_KEY` via the normal secret-injection path so the key rotates with the rest of the config.
- First-user bootstrap has two paths:
  1. **HTTP bootstrap (zero-config).** If no users exist, `GET /api/auth/mode` reports `needs_bootstrap=true` and the SPA renders a "create first admin" page. `POST /api/auth/bootstrap` creates the user, promotes them to `sysadmin` via `BootstrapFirstAdmin`, creates the default "Personal" library with them as its admin member, and returns a logged-in JWT. The endpoint 409s once any user exists, so it's safe to leave wired in perpetuity.
  2. **Env seed (ops setups).** If both `DAMSON_LOCAL_ADMIN_EMAIL` and `DAMSON_LOCAL_ADMIN_PASSWORD` are set, the same transaction runs at startup — useful for scripted provisioning. Seed is a no-op if any user already exists.
- Token TTL: 1 hour (`auth.LocalTokenTTL`). No refresh endpoint in v1 — an expired token forces a fresh login. `users.tokens_valid_after` is a revocation watermark: JWTs with `iat` before it are rejected.

**`oidc`.** Identity-provider-issued bearer tokens validated per request via `coreos/go-oidc`.

- Requires `DAMSON_OIDC_ISSUER_URL`, `DAMSON_OIDC_CLIENT_ID`, `DAMSON_OIDC_AUDIENCE`.
- First user whose email matches `DAMSON_BOOTSTRAP_ADMIN_EMAILS` (CSV) is promoted via the same `BootstrapFirstAdmin` path on their first sign-in.
- The SPA runs the interactive login flow with `oidc-client-ts`; `automaticSilentRenew` keeps the bearer fresh.

Frontend env (Vite-time, OIDC only):

```
VITE_OIDC_AUTHORITY
VITE_OIDC_CLIENT_ID
VITE_OIDC_SCOPE                   # 'openid profile email'
VITE_OIDC_REDIRECT_URI
VITE_OIDC_SILENT_REDIRECT_URI
VITE_OIDC_POST_LOGOUT_REDIRECT_URI
```

On first sign-in (either mode), the server upserts the user and the shared `BootstrapFirstAdmin` transaction runs if the user qualifies.

### 8.2 Authorisation model (v1)

**Library-based group ownership** — confirmed 2026-04-23. Assets, projects, collections, and tags belong to a **library**; users gain access by being granted membership in that library. Within a library, every member sees the same data — there is no per-user partitioning below the library level. This is a form of multi-tenancy but distinct from per-user isolation: it models small shared workspaces (a solo user, or a small team) without the complexity of true SaaS tenants.

Two role hierarchies, independent:

```
users.role                    text not null default 'user'    -- 'user' | 'sysadmin'
library_memberships.role      text not null                    -- 'admin' | 'member'
```

- `sysadmin` (server-level) — creates libraries, manages user records, sees server-level audit, edits server-level settings. Very few of these; typically just the bootstrap user.
- `library admin` — manages library membership and library-scoped settings; may delete the library.
- `library member` — full read/write over library content (assets, projects, collections, tags). No ability to invite or remove members.

A user who is not a member of their `current_library_id` cannot read or write it — all library-scoped handlers fail fast with 403 at the middleware before they see any data.

**Enforcement pattern:**

1. OIDC middleware attaches the authenticated user to the Gin context.
2. A `library` middleware resolves the user's `current_library_id`, verifies the corresponding `library_memberships` row exists, attaches `(library, membership)` to the context, and caches the lookup for the request.
3. A `scoped(db, library)` helper applies `WHERE library_id = ?` to every library-scoped query. Handlers call it once at the top of each query rather than remembering to thread the predicate through.
4. `sysadmin` access is explicit per endpoint via a `RequireSysadmin` middleware on dedicated routes (e.g. `POST /api/libraries`), not a blanket override on library endpoints.

This gives us three guarantees: library-scoped data never escapes the library by accident; cross-library reads require an explicit sysadmin handler (none in MVP); and every handler's authorisation story is readable at the route-definition site.

### 8.3 Audit attribution

Every write handler derives the user via `auth.GetUser(c)` and passes it into the service call, which writes the audit row as part of the same DB transaction as the domain mutation. Audit writes that would fail should fail the enclosing action — audit is not best-effort.

## 9. Import pipeline

Background-first per UI design §5.1. The pipeline is a sequence of in-process stages; each stage is synchronous but the whole pipeline runs on a pool of goroutines so the HTTP request returns as soon as the upload is persisted.

```
[1] Upload streamed to temp + SHA-256   ──────── HTTP request holds open until done ↓
[2] Put blob (skip if already present)
[3] Create Asset (needs_review=true) + first AssetVersion
                 ─────── HTTP request returns; remaining stages run in background ↓
[4] If .unitypackage → extract in a worker
      ├─ Stream each inner blob back to S3
      ├─ Write asset_files rows
      ├─ Harvest preview.png if present → thumbnail candidate
      └─ Set package_kind='unitypackage', retain original
    If .zip → offer to unpack (user action later) OR import opaquely
    Otherwise → single file already persisted in [2]
[5] Licence detection on known licence files within the asset
      (LICENSE, LICENSE.md, *.txt at root, or user-supplied)
[6] Thumbnail generation (image / audio waveform / font sample / icon fallback)
[7] Compute/update search_vector
[8] Emit audit entry: asset.import
```

### 9.1 Staging and progress

Stage state is tracked in a dedicated `asset_imports` table so imports survive a restart:

```
asset_imports
  id           bigserial PK
  asset_id     bigint references assets(id) on delete cascade
  stage        text           -- 'uploaded' | 'extracting' | 'detecting' | 'thumbnailing' | 'done' | 'failed'
  error        text
  updated_at   timestamptz default now()
  index(asset_id)
  index(stage)
```

One row per import. `done` rows are retained for debugging but UI filters them out. The queue view in the UI is `assets WHERE needs_review = true`.

> **Implementation status (2026-04):** M2 extracts synchronously inside
> the HTTP request, so only `done` is written to `asset_imports.stage`
> today. The intermediate stages (`extracting`, `detecting`,
> `thumbnailing`, `failed`) and the SSE event stream land with M3+; see
> `docs/progress.md` "Foundations the tech design assumes but hasn't
> been built yet".

Progress reaches the SPA over **Server-Sent Events**: a handler at `GET /api/imports/:id/events` streams stage transitions for one import, and `GET /api/imports/events` streams all in-flight imports for the current user. A broadcaster sits inside `services/audit` (or a sibling `services/events`) with one channel per user; the import pipeline publishes stage updates; SSE handlers subscribe and fan them out. The SPA connects via `EventSource`; if the connection drops, it reconnects and re-reads the current row state from `GET /api/imports/:id` to backfill whatever was missed.

### 9.2 Unity package handling

`.unitypackage` is a gzipped tar archive. Each entry is a directory named by a GUID (32 hex chars) containing:

- `asset` — the actual file bytes
- `asset.meta` — YAML metadata (keep verbatim if desired; not required for v1)
- `pathname` — text file giving the asset's Unity project-relative path
- `preview.png` — optional thumbnail

Extraction algorithm:

1. Stream the tar.gz.
2. Buffer entries per-GUID (bounded — see extraction-safety below).
3. For each complete GUID bundle, read `pathname` to determine the target path; stream `asset` to S3 via `storage.PutBlob`; record `(guid, path, size, content_hash, object_key)`.
4. If `preview.png` exists, stash it as a candidate thumbnail.
5. After the tar is consumed, bulk-insert `asset_files` rows.

Extraction safety: cap per-entry size, cap total uncompressed size, and reject paths containing `..` or absolute prefixes. These are standard archive bomb / zip-slip mitigations — put them in `services/extract` from day one.

Re-extraction: the original `.unitypackage` stays in `blobs/`. Re-extraction reads it from S3, produces a new `asset_version`, and writes a new set of `asset_files`. The previous version's rows are untouched.

### 9.3 Subset selection (post-import)

Per the UI design, subset selection is not a blocking import step. Instead, on an extracted Unity-package version, the user can trigger "create subset version": pick paths, and a new `asset_version` is created whose `asset_files` rows are a subset of the source. Object keys are the same — no re-upload — so subsetting costs a handful of DB rows.

## 10. Licence detection

### 10.1 Detection strategy

1. Locate candidate licence text in the asset:
   - Root-level files: `LICENSE`, `LICENSE.md`, `LICENSE.txt`, `COPYING`, `COPYING.LESSER`, `UNLICENSE`
   - For `.unitypackage`: files at Unity paths matching the above (case-insensitive)
   - Fallback: user supplies text via UI
2. Run `github.com/google/licensecheck` on each candidate. It returns matched SPDX identifiers with confidence and "modifications" where present.
3. If no match, try the **Unity Asset Store EULA** template (shipped with the app as a fingerprint + canonical text). Canonical fingerprint is a normalised text hash.
4. Otherwise → `detected_type='unknown'`; user enters attributes manually.

Result is persisted to `licences` with `original_text` verbatim, `canonical_diff` populated if modified, `attributes_json` filled from the SPDX → attributes map in the knowledge base.

### 10.2 Licence knowledge base

Ships with the binary. Two parts:

- **SPDX corpus** — bundled via `licensecheck`'s own data.
- **Project overlay** — a small YAML under `internal/services/licence/knowledge/` defining:
  - Attribute maps per SPDX id (commercial, attribution, share-alike, etc.)
  - Unity EULA entry
  - Custom templates (empty for v1)

Updates to the overlay ship with new app releases (compiled in via `go:embed`). Post-MVP: pull from a remote manifest. Open question in product design §8 — noted, not resolved.

### 10.3 Compliance evaluation

Not in MVP. The `projects.target_distribution` and `licences.attributes_json` fields carry enough data for a later compliance view; no code writes or reads that relationship in v1 beyond surfacing the raw attributes.

## 11. Thumbnail generation

### 11.1 Pipeline

Synchronous after import, per UI design §5.5. One worker function per input category:

- **Raster image** (`image/*`): decode via stdlib `image` (+ `golang.org/x/image/webp` for WebP inputs), resample to 512px on the longer edge using `golang.org/x/image/draw` with `draw.CatmullRom`, encode as JPEG (quality ~85) for photographic content, PNG for palette/alpha content. Put to `thumbs/`.
- **Audio** (`audio/*`, `.wav`, `.mp3`, `.ogg`): decode to PCM (pure-Go decoders per format), downsample to waveform bins, render into an in-memory `image.RGBA` → PNG.
- **Font** (`.ttf`, `.otf`): load with `golang.org/x/image/font/opentype`, render sample text ("The quick brown fox jumps over the lazy dog") at a reasonable size into an `image.RGBA` → PNG.
- **`.unitypackage`**: use harvested `preview.png` from extraction if present (already PNG, no re-encode); otherwise first harvested preview across entries; otherwise icon fallback.
- **Otherwise**: no thumbnail; frontend renders a file-type icon.

Thumbnail format per category: **JPEG for photographic content, PNG for everything else** (palette, alpha channel, rendered text, audio waveforms, Unity previews). `assets.thumbnail_content_type` stored alongside `thumbnail_object_key` so the serving handler can set `Content-Type` correctly without inspecting bytes.

### 11.2 Deferred-compatibility

Even though thumbnailing is synchronous in MVP, the `assets` model carries `thumbnail_object_key` as nullable and the SPA tolerates `null` by rendering an icon fallback. Moving to a background queue means writing `null` at import time and filling it later — pure additive change.

## 12. Search

### 12.1 MVP: Postgres tsvector

A `search_vector` column on `assets`, maintained by a trigger on `assets` + `asset_tags`:

```sql
search_vector =
    setweight(to_tsvector('simple', coalesce(name, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(description, '')), 'B') ||
    setweight(to_tsvector('simple', string_agg(tag.path, ' ')), 'C')
```

Query path: `search_vector @@ plainto_tsquery('simple', $1)` with ranking. A GIN index on `search_vector` handles volume.

### 12.2 Filters

Filters compose via SQL joins; the handler builds a query with GORM's `Where`/`Joins` chain. Filter types supported:

- `tag=<path>` (matches all descendants via `path LIKE '<path>%'`)
- `licence=<spdx-id>` (joins `licences`)
- `project=<id>` (joins `project_asset_pins`)
- `collection=<id>` (joins `collection_assets`)
- `file_type=<mime-class>` (joins `asset_files` with a semi-join)
- `source=<domain>` (exact-match on `assets.source_url` parsed host)

All filters AND together. Pagination is `limit`/`offset`; result ordering is search rank → import date.

### 12.3 Future

Product design §8 raises similarity/embeddings. Deferred — noted here so that `search_vector` is treated as one of several possible indexes, not "the" index.

## 13. Audit log

- Written in the same transaction as the domain mutation it describes.
- `diff_json` is a JSON object `{ "before": {...}, "after": {...} }` for updates; `{ "created": {...} }` for creates; `{ "deleted": {...} }` for deletes.
- `library_id` populated for library-scoped events; `NULL` for server-level events (library created/deleted, user role changed, server settings updated).
- Querying: indexed by `(library_id, created_at desc)`, `(library_id, target_type, target_id, created_at desc)`, `(user_id, created_at desc)`, `(action_type, created_at desc)`.
- Retention: unbounded in v1; policy value carried in library settings but not yet enforced.
- UI surfaces:
  - Per-asset / per-project view (inside the current library).
  - Library audit (all events with matching `library_id`).
  - Server audit (sysadmin only; events with `library_id IS NULL` plus any cross-library events).

## 14. Configuration and secrets

Per UI design §6: **env vars for bootstrap/secrets only; everything else in DB, editable in Settings.** Settings now split across two scopes: **server** (one row, sysadmin-only) and **library** (one row per library, library-admin-editable).

### 14.1 Env vars (bootstrap)

The only always-required variable is the Postgres DSN. Everything else either defaults or is only needed when its mode is selected.

**Always:**

| Var | Required? | Default |
|---|---|---|
| `DAMSON_POSTGRES_DSN` | **required** | — |
| `DAMSON_LISTEN_ADDR` | optional | `:8080` |
| `DAMSON_BASE_URL` | optional | `http://localhost:8080` |
| `DAMSON_LOG_LEVEL` | optional | `info` |
| `DAMSON_BOOTSTRAP_ADMIN_EMAILS` | optional | empty — comma-separated emails promoted to sysadmin on first sign-in |
| `DAMSON_AUTH_MODE` | optional | `local` — also accepts `oidc` |
| `DAMSON_STORAGE_MODE` | optional | `local` — also accepts `s3` |

**`DAMSON_AUTH_MODE=local`** (the default; see §8.1 local mode) additionally reads:

| Var | Required? | Default |
|---|---|---|
| `DAMSON_JWT_SIGNING_KEY` | optional | unset — if empty, a 32-byte key is generated on first boot and persisted to `DAMSON_JWT_KEY_FILE` |
| `DAMSON_JWT_KEY_FILE` | optional | `./data/jwt.key` — consulted only when `_SIGNING_KEY` is unset |
| `DAMSON_LOCAL_ADMIN_EMAIL` | optional | unset — when both this and `_PASSWORD` are set, seeds the initial sysadmin at startup; otherwise the first browser hit lands on the in-app bootstrap page (§8.1) |
| `DAMSON_LOCAL_ADMIN_PASSWORD` | optional | unset |

**`DAMSON_AUTH_MODE=oidc`** additionally requires:

| Var | Required? |
|---|---|
| `DAMSON_OIDC_ISSUER_URL` | required |
| `DAMSON_OIDC_CLIENT_ID` | required |
| `DAMSON_OIDC_AUDIENCE` | required |

**`DAMSON_STORAGE_MODE=local`** (the default) additionally reads:

| Var | Required? | Default |
|---|---|---|
| `DAMSON_STORAGE_LOCAL_PATH` | optional | `./data/blobs` |

**`DAMSON_STORAGE_MODE=s3`** additionally requires:

| Var | Required? | Default |
|---|---|---|
| `DAMSON_S3_ENDPOINT` | required | — |
| `DAMSON_S3_REGION` | required | — |
| `DAMSON_S3_BUCKET` | required | — |
| `DAMSON_S3_ACCESS_KEY` | required | — |
| `DAMSON_S3_SECRET_KEY` | required | — |
| `DAMSON_S3_FORCE_PATH_STYLE` | optional | `true` |

Loaded once at startup by `internal/config`. Missing required values cause `MustGetEnv`-style panics (matches the conventions' startup-panic rule).

### 14.2 DB-backed settings

Split into **server settings** (singleton, sysadmin-managed — things that genuinely span libraries) and **library settings** (one row per library, library-admin-managed — everything a library's members might reasonably want to control for their workspace).

```
server_settings
  id                          int primary key default 1
  max_upload_bytes            bigint default 2147483648  -- 2 GiB; hard server cap
  licence_kb_auto_update      bool default false
  updated_at                  timestamptz default now()
  updated_by_user_id          bigint references users(id)

library_settings
  library_id                  bigint primary key references libraries(id) on delete cascade
  theme_default               text default 'dark'
  grid_density_default        text default 'medium'
  retain_original_packages    bool default true
  dedupe_enabled              bool default true        -- scope: dedupe within this library
  thumbnails_raster_enabled   bool default true
  thumbnails_audio_enabled    bool default true
  thumbnails_font_enabled     bool default true
  thumbnails_unity_preview    bool default true
  audit_retention_days        int                      -- null = unbounded
  updated_at                  timestamptz default now()
  updated_by_user_id          bigint references users(id)
```

A `library_settings` row is created alongside each new library with defaults. Reads are cached in-process with a 30s TTL, keyed by `library_id` (server settings cached globally).

Note on dedupe scope: **blob storage dedupes globally** (same SHA-256 → one object) regardless of the per-library `dedupe_enabled` flag; the flag controls whether the library's import flow creates one asset for a re-upload or two. Bytes are always shared; user-facing asset identity is library-scoped.

## 15. Frontend architecture

### 15.1 Layout shell

The shell matches UI design §3.2 — nav rail + filter sidebar + main area, persistent across library/project/collection views. Implemented as a React Router data-router with three nested routes:

```
/                       → redirect to /library
/library                → default library view (grid + filter sidebar)
/library/:assetId       → library with asset detail panel open
/projects               → projects index
/projects/:id           → project detail (same three-column shape)
/collections
/collections/:id
/queue                  → import queue (filtered library view)
/audit                  → audit log
/settings
/settings/:section
```

The asset detail panel is URL-addressable (deep-linkable) so a pasted link opens the right asset. "Open as full page" (§4.3) swaps `/library/:assetId` → `/assets/:assetId` which uses a full-width layout.

### 15.2 State

- **Server state** through TanStack Query per conventions. Query keys in `lib/queryKeys.ts`; custom hooks in `hooks/useDataQueries.ts` returning normalised shapes.
- **UI state** (filter selection, selection set, panel open/closed) local to components; filter state serialised to URL search params so the library view is bookmarkable.
- **Global client state** (theme, command palette visibility) via a tiny React context — not enough to justify Zustand or similar.
- **Import progress** via `EventSource` on `/api/imports/events`, dispatched into TanStack Query caches with `setQueryData`. On reconnect, the component refetches the imports list to backfill missed events.

### 15.3 Grid virtualisation

The grid is designed for many thousands of assets. `@tanstack/react-virtual` with a fixed thumbnail cell size keeps DOM nodes bounded. Density toggle changes cell size and triggers remeasure.

### 15.4 Drag-and-drop

`react-dropzone` wraps the main area; dropping files calls `POST /api/imports` with `multipart/form-data`. The upload component shows a persistent progress tray; the file itself streams directly to the backend (no direct-to-S3 signed URL in v1 — simpler, and all uploads go through a single Go streaming pipeline that can do the hashing).

### 15.5 API client

`src/api.ts` follows the conventions' `fetchJson<T>` pattern — attach bearer, handle 401 via silent token renewal with single-flight deduping, retry once, throw `ApiError`. For uploads, a parallel `uploadFile` helper uses `XMLHttpRequest` (fetch lacks good upload progress events) and emits progress via callback.

## 16. API surface

REST-ish JSON under `/api`. Not exhaustive; indicative for review.

All library-scoped endpoints (everything below `# Assets`) operate against the authenticated user's `current_library_id` and are wrapped by the library middleware described in §8.2. Cross-library operations go through the dedicated `/api/libraries/...` paths.

```
# Auth (public)
GET    /api/auth/mode                 # { mode, needs_bootstrap } — SPA boots with this
POST   /api/auth/login                # local mode only — email+password → JWT
POST   /api/auth/bootstrap            # local mode only — creates first sysadmin; 409 once any user exists

# Session / user
GET    /api/me                        # user + current library summary
POST   /api/me/current-library        # { library_id } — switches active library

# Libraries (membership-gated; reads any library the user is a member of)
GET    /api/libraries                 # libraries the user can see (membership-filtered)
POST   /api/libraries                 # sysadmin only
GET    /api/libraries/:id
PATCH  /api/libraries/:id             # library admin
DELETE /api/libraries/:id             # library admin
GET    /api/libraries/:id/members
POST   /api/libraries/:id/members     # library admin — invite by email
PATCH  /api/libraries/:id/members/:userId   # change role
DELETE /api/libraries/:id/members/:userId   # remove
GET    /api/libraries/:id/settings
PATCH  /api/libraries/:id/settings    # library admin
GET    /api/libraries/:id/audit       # library-scoped audit

# Server admin (sysadmin only)
GET    /api/admin/users
PATCH  /api/admin/users/:id           # set role, etc.
GET    /api/admin/settings
PATCH  /api/admin/settings
GET    /api/admin/audit               # server-level events (library.create, user.role.change, ...)

# Assets
GET    /api/assets                    # list with filters
POST   /api/assets                    # rarely used — most assets come via imports
GET    /api/assets/:id
PATCH  /api/assets/:id                # name, description, source, needs_review
DELETE /api/assets/:id
POST   /api/assets/:id/review         # mark reviewed
GET    /api/assets/:id/versions
POST   /api/assets/:id/versions/:vid/subset    # create subset version
POST   /api/assets/:id/versions/:vid/pin       # pin to project (convenience)

# Imports
POST   /api/imports                   # multipart upload; returns asset_id + import id
GET    /api/imports                   # list in-flight + failed
GET    /api/imports/:id               # stage, error, progress
POST   /api/imports/:id/retry

# Licences
GET    /api/assets/:id/licence
PUT    /api/assets/:id/licence        # full replacement (keeps original_text)
POST   /api/assets/:id/licence/review # mark licence reviewed

# Tags
GET    /api/tags                      # tree
POST   /api/tags
DELETE /api/tags/:id
POST   /api/assets/:id/tags
DELETE /api/assets/:id/tags/:tagId

# Projects
GET    /api/projects
POST   /api/projects
GET    /api/projects/:id
PATCH  /api/projects/:id
DELETE /api/projects/:id
POST   /api/projects/:id/pins         # pin an asset-version
PATCH  /api/projects/:id/pins/:pinId  # upgrade
DELETE /api/projects/:id/pins/:pinId

# Collections
GET    /api/collections
POST   /api/collections
GET    /api/collections/:id
PATCH  /api/collections/:id
DELETE /api/collections/:id
POST   /api/collections/:id/assets
DELETE /api/collections/:id/assets/:assetId

# Search
GET    /api/search?q=&filters=…       # convenience; same result shape as GET /api/assets

# Files (download/preview)
GET    /api/files/:objectKey          # streamed (302 to presigned URL, or direct pipe)
GET    /api/thumbs/:objectKey         # streamed, long cache headers

# Audit
GET    /api/audit                     # global, filterable
GET    /api/assets/:id/audit
GET    /api/projects/:id/audit

# Settings (admin)
GET    /api/settings
PATCH  /api/settings
```

Shape conventions from `CONVENTIONS.md`: handlers on a shared `handler` struct in `internal/handlers/api`, receiver methods, `ctx.AbortWithStatusJSON(status, gin.H{"error": msg})` for errors, log with `log.Printf`.

## 17. Error handling and observability

- Logging via `log/slog` (stdlib) at JSON format in production, text in dev. Fields: `op`, `user_id`, `asset_id`, `stage` for pipeline logs.
- Request IDs via a Gin middleware; stamped on every log line.
- No error tracker integration in MVP; structured logs flow to whatever the deployment's log collector is (TrueNAS → Promtail → Loki, in Thomas's setup — out of scope for this doc).
- Metrics deferred. If added later, Prometheus via `promhttp` on a separate port.

## 18. Testing

Per `CONVENTIONS.md`:

- Handler tests alongside handlers (`assets_test.go` next to `assets.go`).
- `newTestHandler(t)` returns `(*handler, *gorm.DB)` backed by in-memory SQLite with migrations `AutoMigrate`d. For the specific Postgres features we use (tsvector, JSONB), tests either:
  - use a stubbed equivalent in SQLite where possible, or
  - run against a real Postgres in a CI-only test tier (tag `// +build integration`).

  Start with SQLite for breadth; add a Postgres-backed integration tier once search or JSONB behaviour needs verification against the real engine. (Memory: validates by real DB where divergence bites.)

- Service-layer packages carry their own unit tests with storage/licence/thumbnail fakes.
- Frontend tests in Vitest + Testing Library; pattern matches conventions.

CI target: `task pre-release-checks` (fmt + vet + Go tests + web tests).

## 19. Deployment

Multi-stage Dockerfile:

1. **Node stage** — `node:24-alpine`, install SPA deps, `vite build` → `web/static/`.
2. **Go build stage** — `golang:1.26-alpine`, copy source + built SPA, `CGO_ENABLED=0 go build -o /out/damson ./cmd/server`. No C toolchain required.
3. **Runtime stage** — `alpine:3.20` with `ca-certificates`. Non-root user. Binary only.

The binary embeds `web/static/` via `go:embed` and serves it at `/` with a SPA fallback (any non-`/api` route that's not a static asset serves `index.html`).

Deployment target: a reverse proxy (Caddy/Traefik) fronting the container. TLS is the proxy's problem. Postgres and MinIO run as sibling containers or external services.

### 19.1 First-run flow

1. Start the container. `DAMSON_POSTGRES_DSN` is the only always-required env var — local auth and local storage are the defaults, and the JWT signing key is auto-generated at `./data/jwt.key` on first boot if `DAMSON_JWT_SIGNING_KEY` isn't set.
2. Migrations run on startup (inside a DB-level advisory lock to prevent parallel-startup races).
3. First browser hit → `GET /api/auth/mode` returns `{ mode: "local", needs_bootstrap: true }`, and the SPA renders the "create first admin" page.
4. The new user submits email + password → `POST /api/auth/bootstrap` creates the user with `role='sysadmin'`, creates the default `"Personal"` library, grants the user library admin membership, sets `current_library_id`, and returns a logged-in JWT. The SPA stores the token and redirects to the library.
5. Subsequent sign-ins (local) or first sign-ins (OIDC with email not in `DAMSON_BOOTSTRAP_ADMIN_EMAILS`) succeed but land on "No library access yet — ask your admin for an invite" until a library admin grants membership.

Alternative bootstrap paths:
- **Env seed (local mode).** Setting both `DAMSON_LOCAL_ADMIN_EMAIL` and `DAMSON_LOCAL_ADMIN_PASSWORD` creates the first user at startup; step 3's bootstrap page is skipped because `needs_bootstrap` is already `false`.
- **OIDC mode.** `DAMSON_AUTH_MODE=oidc` + `DAMSON_BOOTSTRAP_ADMIN_EMAILS` — first sign-in whose email matches the list runs the same `BootstrapFirstAdmin` transaction.

## 20. Non-MVP design hooks

Data/structure that exists in v1 specifically because deferred features need it:

| Deferred feature | What v1 must capture |
|---|---|
| Licence compliance reports | `projects.target_distribution`, `licences.attributes_json` — already in schema |
| Upstream update detection | `assets.source_url`, `source_metadata_json` (jsonb), `asset_versions.import_date`, `asset_versions.version_label` |
| Content-aware search in packages | `asset_files` manifest with `detected_type` per file; names searchable once tokeniser expands |
| Auto-tagging | `asset_files.detected_type` and a later `tags.generated_from` column additive |
| Release snapshots | `project_asset_pins.pinned_version_id` — already historied via audit diffs |
| Asset components | A later `component_id` on `asset_files`, plus a `components` table — pure additive |
| Background thumbnail/extraction queue | Pipeline stages isolated in `services/*`; swap in a queue behind the same interface |
| Shared-vocabulary tags across libraries | `tags` is library-scoped now; a later `global_tags` table + reference from `tags.global_tag_id` is additive |
| Cross-library move / copy of an asset | Blob storage is already global (content-addressed); only DB rows need to be duplicated with new `library_id` |

## 21. Milestones (proposed)

Rough, for review:

1. **M0 — Skeleton.** Repo bootstrapped per §5: Go + Gin + GORM + OIDC middleware + health check + Vite SPA shell with AuthGuard and a blank library view. Dockerfile builds a working image. `task dev` and `task dev:spa` work. Empty Postgres migration runner.
2. **M1 — Storage primitives.** S3 client + content-addressed put/get, `POST /api/imports` accepts uploads and stores blobs, `assets` + `asset_versions` + `asset_files` tables, basic list endpoint, grid renders thumbnails-absent with icon fallback.
3. **M2 — Unity package handling.** Tar.gz extraction, original retention, file manifest writes, preview harvest, re-extraction endpoint.
4. **M3 — Metadata + licences.** Tags (with hierarchy), licence detection via `licensecheck`, licence editor in the detail panel, Unity EULA template.
5. **M4 — Projects and pins.** Project CRUD, pin/upgrade, upgrade indicator in project view.
6. **M5 — Collections, search, filters.** Collections CRUD, tsvector search, composable filters wired end-to-end.
7. **M6 — Thumbnails.** Raster, audio waveform, font sample, Unity preview. Settings table with toggles.
8. **M7 — Audit, queue, polish.** Audit viewers, import queue surfaces, review workflow, empty states, keyboard shortcuts per UI §5.3.

Each milestone ends releasable (behind auth) and demoable.

## 22. Confirmed decisions (2026-04-23)

1. **Module path:** `github.com/thomas-elliott/damson`.
2. **Frontend OIDC library:** `oidc-client-ts` (matches Thomas's other projects; the thin React wrapper is written in-repo).
3. **Upload path:** in-server streaming. The Go server receives the upload, hashes in-stream, and puts to S3. Revisit only if the server becomes a bottleneck in real use.
4. **Import progress channel:** Server-Sent Events. See §9.1.
5. **Integration test tier:** SQLite-only to start. Real-Postgres integration tier added at M5 when tsvector search lands.
6. **Thumbnail image pipeline:** stdlib `image`, `image/png`, `image/jpeg` + `golang.org/x/image/webp` (decode) + `golang.org/x/image/draw` (high-quality resampling). No CGo, no third-party encoder. Output format is **JPEG for photographic content, PNG for everything else** (palette/alpha/rendered-text/waveform/Unity preview). See §11.
7. **Ownership model:** library-based group ownership. A `libraries` top-level entity holds assets, projects, collections, tags; users gain access through `library_memberships`; within a library all members share the same data. Reframes the product-design-doc §2 non-goal from "no multi-tenancy" to "no per-user isolation within a library" — product doc updated in step.
8. **Import staging:** a real `asset_imports` table, so imports survive restart and SSE reconnect can backfill.

**UI direction:**
- **Accent colour:** yellow (resolves ui-design §9). Implemented as a Tailwind theme token.

## 23. Open questions inherited from product/UI design

Tracked so they don't fall off the edge; resolution is product-driven, not technical:

- Licence knowledge base update cadence (product §8)
- Long-term search engine direction (product §8)
- Accent colour (UI §9)
- Detail panel vs. full-page promotion distinction (UI §9)
- Mobile viewer — which edits to disable are likely to cause least surprise

---

## Appendix A — First-pass DB DDL (illustrative)

The first migration will be something close to the following. Included here only to make the design concrete; the migration file is the authoritative version.

```sql
-- 001_users.sql
BEGIN;

CREATE TABLE IF NOT EXISTS users (
    id                  bigserial PRIMARY KEY,
    subject             text NOT NULL UNIQUE,
    email               text NOT NULL,
    display_name        text NOT NULL,
    role                text NOT NULL DEFAULT 'user',          -- 'user' | 'sysadmin'
    current_library_id  bigint,                                 -- FK added in 002
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMIT;

-- 002_libraries.sql
BEGIN;

CREATE TABLE IF NOT EXISTS libraries (
    id          bigserial PRIMARY KEY,
    name        text NOT NULL,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE users
    ADD CONSTRAINT users_current_library_fk
    FOREIGN KEY (current_library_id) REFERENCES libraries(id) ON DELETE SET NULL;

COMMIT;

-- 003_library_memberships.sql
BEGIN;

CREATE TABLE IF NOT EXISTS library_memberships (
    id          bigserial PRIMARY KEY,
    library_id  bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id     bigint NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
    role        text   NOT NULL,                         -- 'admin' | 'member'
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (library_id, user_id)
);
CREATE INDEX IF NOT EXISTS library_memberships_user_idx
    ON library_memberships (user_id);

COMMIT;

-- 004_audit_log.sql
BEGIN;

CREATE TABLE IF NOT EXISTS audit_log (
    id           bigserial PRIMARY KEY,
    library_id   bigint REFERENCES libraries(id) ON DELETE CASCADE,   -- null for server-level events
    user_id      bigint REFERENCES users(id),
    action_type  text NOT NULL,
    target_type  text NOT NULL,
    target_id    bigint,
    diff_json    jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_log_library_time_idx
    ON audit_log (library_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_library_target_idx
    ON audit_log (library_id, target_type, target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_user_idx
    ON audit_log (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_action_idx
    ON audit_log (action_type, created_at DESC);

COMMIT;
```

## Appendix B — Module layout notes

Each `services/*` package exports a small interface and a constructor that takes its dependencies explicitly. No DI framework. `main.go` wires them into a single `handler` struct per the conventions. Tests construct the `handler` with fake service implementations; the fakes live in `services/*/testing.go` guarded by a build tag or by convention (e.g. `_test.go` in the same package).

## Appendix C — Glossary

- **Library** — a top-level workspace holding assets, projects, collections, and tags. Users gain access via `library_memberships`. One user can belong to many libraries; one library holds many users' work without per-user partitioning.
- **Library member** — user with `library_memberships.role='member'`; full read/write access to library content.
- **Library admin** — user with `library_memberships.role='admin'`; member capabilities plus membership and library-settings management.
- **Sysadmin** — user with `users.role='sysadmin'`; creates libraries, manages user records, edits server settings. Orthogonal to library roles.
- **Current library** — the library a user is actively working in, stored as `users.current_library_id` and used to scope every library-aware API call.
- **Asset** — a logical thing a user would recognise and reuse (tree pack, music track), scoped to a library.
- **Asset version** — immutable snapshot of an asset's file contents.
- **Project** — a game project; pins specific asset versions. Scoped to a library.
- **Collection** — curated asset grouping; organisational only. Scoped to a library.
- **Licence** — licence record attached to an asset (detected or custom).
- **Pin** — a project's reference to a specific asset version.
- **Import queue** — assets marked `needs_review=true`; not yet confirmed by the user.
- **Object key** — SHA-256 of a blob; the S3 key under `blobs/`. Blobs are global (content-addressed); asset identity is library-scoped.
