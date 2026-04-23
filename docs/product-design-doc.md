
# Digital Asset Manager — Product Design Document

**Status:** Draft v1 **Author:** Thomas **Last updated:** 2026-04-23

---

## 1. Problem statement

Game development projects pull assets from many sources — Unity Asset Store purchases, free downloads from itch.io, Kenney, Poly Haven, GitHub repos, bundles — and each asset carries its own licence with its own terms. Managing this sprawl has three persistent pain points:

1. **Licence tracking is fragile.** Licences vary wildly (SPDX-standard, Unity EULA, custom proprietary, modified opens-source), and the cost of getting it wrong only surfaces at release. Knowing whether a project's asset mix is compatible with its intended distribution is hard to see at a glance.
2. **Unity Asset Store packages are all-or-nothing.** A single `.unitypackage` may contain hundreds of files when only a handful are needed. There is no tooling to extract a subset and treat it as the "real" asset while keeping the original for future reference.
3. **Upstream updates are invisible.** Once an asset is copied into a project, there is no link back to the source. When the original is updated upstream, the local copy silently drifts, and there is no record that an update exists.

The tool addresses these problems as a personal, self-hosted digital asset manager oriented around game development workflows, with licences and versioned extractions as first-class concepts.

## 2. Goals and non-goals

### Goals

- Centralised, self-hosted storage for arbitrary game-dev assets with rich metadata
- Multi-project model where one asset library feeds many game projects
- **Library-based group ownership** — a library is a workspace holding assets, projects, collections, and tags; multiple users can be granted access to a library, and members share its contents without per-user partitioning. One user may belong to multiple libraries.
- First-class licence tracking: store the original text, identify where possible, allow custom/proprietary licence definitions
- `.unitypackage` extraction into loose files with both the original package and the extraction retained
- Versioned assets with explicit pinning per project
- Strong search: text, tags (free-form + hierarchical), filters, thumbnails for easy previews
- Audit log of who did what, to support later collaboration without retrofit
- Deployable as a Go binary in a Docker container, backed by Postgres and S3-compatible object storage

### Non-goals (v1)

- Per-user isolation within a library (members of a library share all its contents)
- SaaS-style multi-tenancy with hard customer isolation, quotas, or billing
- Real-time collaboration features (shared editing, comments, assignments)
- Asset editing or transformation beyond extraction
- Direct Unity or Unreal project integration (plugin, import/export into game projects)
- Marketplace or asset-sharing features
- Automatic backup or sync beyond what the underlying infrastructure provides

### Deferred (post-MVP, in scope for the product long-term)

- Automatic update detection against upstream sources
- Licence compliance reports and release snapshots
- Content-aware search (file-name indexing inside packages, embedded metadata)
- 3D, video, and rendered-model thumbnails
- Auto-tagging based on file types and content analysis
- CLI for scripted import
- Unity Hub integration for direct `.unitypackage` detection
- Asset components (grouping related files within an asset, e.g. tree model + textures)

## 3. Core concepts and domain model

The domain centres on seven concepts. Relationships are deliberately loose so the system can evolve without schema churn.

### Library

The top-level workspace. A library holds assets, projects, collections, and the tag tree; everything a user sees in the main UI is scoped to the library they are currently working in. Users gain access to a library through membership — either as an `admin` (can manage membership and library settings) or a `member` (full read/write access to library contents). One user can belong to many libraries and switches between them in the UI. Within a library there is no per-user partitioning: every member sees the same assets, projects, tags, and collections.

This models small shared workspaces — a solo user's "Personal" library, a small team's shared library, several siblings co-existing on one server — without the complexity of SaaS-style multi-tenancy.

### Asset

The central unit. An asset represents a logical thing — a tree pack, a music track, a font, a character model — that a user would recognise and reuse. An asset owns its licence, has metadata (name, description, tags, source, thumbnail), and has one or more versions.

An asset is not a single file. It is a container of files grouped by the user. A Unity package containing 200 files becomes one asset; the 200 files are the asset's contents. Individual files are not promoted to assets unless the user chooses to create separate assets during import.

### Asset version

Each asset has one or more versions. A version is an immutable snapshot of the asset's file contents at a point in time, with its own file manifest and its own object-storage references. Versions carry a version identifier (user-supplied or derived from upstream), an import date, and optional notes.

When an asset is extracted from a Unity package and later re-extracted from a newer upstream package, the two extractions become two versions of the same asset, not two separate assets.

### Project

A named container representing one of the user's game-development projects. A project has a declared target distribution profile (e.g. commercial, open source, personal-only) which informs licence compliance later. A project references specific pinned versions of assets. Pinning is explicit: projects do not auto-upgrade.

### Licence

A record of the licence terms that apply to an asset. A licence record stores:

- Detected type (SPDX identifier, Unity EULA, custom, unknown)
- Detection confidence and diff from canonical (if a known licence is modified)
- Original licence text verbatim
- Structured attributes that the user confirms or fills in: commercial use allowed, modification allowed, redistribution allowed, attribution required, share-alike required, open-source requirement

The licence is attached to the asset. When an asset is used across multiple projects, one licence applies; the tool evaluates compatibility against each project's distribution profile.

### Collection

A user-curated grouping of assets. Collections are organisational, not structural — think "Medieval Village Set" pulling assets from several sources. An asset can belong to many collections. Collections are distinct from projects: collections are for browsing and organisation, projects are for licence scope and release.

### User

An authenticated identity. Users gain access to content through library memberships; a user with no memberships can sign in but cannot read or write any library-scoped data until invited. A small set of server-level administrators (`sysadmin`) can create libraries and manage user records; this is independent from per-library roles. Every action that creates or modifies data is attributed to a user in the audit log.

## 4. Key workflows

### 4.1 Import an arbitrary file or package

The user drags a file into the web UI. The file may be a `.unitypackage`, a `.zip`, an image, a model, a font, or anything else.

On import:

- The file is hashed (SHA-256). Content-addressed storage in S3 means duplicate uploads deduplicate automatically.
- A new asset is created with the filename as a default name. The user is prompted to confirm or edit metadata: name, description, tags, source URL, licence.
- For `.unitypackage` files, the tool offers to extract. Extraction is described in 4.2.
- For archives, the tool offers to unpack into the same asset (all files become the asset's content) or prompts the user to treat the archive opaquely.
- Otherwise the file becomes the single content of the asset's first version.
- File manifest (paths, sizes, hashes, detected type) is stored for every file.

Every import is recorded in the audit log.

### 4.2 Extract a Unity package

The original `.unitypackage` is retained in object storage. The user selects which files or folders within the package to extract. The extraction becomes version 1 of the asset. The folder structure from the package is preserved.

The user can re-extract a different subset later. The original package is retained indefinitely; the user can re-extract without re-uploading.

If a newer version of the package is uploaded later, it appears as version 2 of the asset. The user chooses which files to extract from v2 — the extraction subset does not have to match v1.

### 4.3 Add an asset to a project

The user opens a project, browses or searches assets, and adds one. The project pins a specific version of the asset. If the asset later gains a new version, the project continues to reference the pinned version until the user explicitly upgrades. Upgrade is a deliberate action that records which version was pinned before, the new version, and who performed the upgrade.

### 4.4 Create or update a licence

When an asset is created or edited, the licence UI shows:

- The detected licence type with confidence score
- A diff view if the text matches a known licence but has been modified
- Editable attributes (commercial use, attribution, etc.) pre-filled from the detected type, which the user can confirm or override
- The original licence text in full

If no match is found, the user is prompted to define a custom licence with attributes filled in manually.

The licence knowledge base ships with the application and covers common SPDX identifiers and the Unity Asset Store EULA. It is not user-editable in v1 (though the user can always create custom licences on a per-asset basis).

### 4.5 Search and browse

The main browse view shows a grid of assets with thumbnails, name, and a licence pill. Users can:

- Full-text search across name, description, and tags
- Filter by tag (hierarchical tree view), licence type, project usage, source, file type
- Sort by import date, name, size
- Switch between grid and list view

Collections appear in a sidebar. Selecting a collection filters the grid.

### 4.6 Audit log

Every action that writes data is recorded: uploads, edits to asset metadata or licences, creations and deletions of projects or collections, pinning and upgrading of asset versions. Each log entry records who, what, when, and a structured diff where applicable.

The audit log is viewable per asset, per project, and globally.

## 5. Data model (Postgres)

A sketch of the principal tables. Field lists are indicative, not final; see `technical-design.md` §6 for the authoritative schema.

- **users**: id, subject (OIDC), email, display_name, role (`user` | `sysadmin`), current_library_id, created_at, updated_at
- **libraries**: id, name, description, created_at, updated_at
- **library_memberships**: id, library_id, user_id, role (`admin` | `member`), created_at
- **assets**: id, library_id, name, description, source_url, licence_id, current_version_id, thumbnail_object_key, needs_review, created_by_user_id, created_at, updated_at
- **asset_versions**: id, asset_id, version_label, import_date, notes, original_package_object_key (nullable), created_by_user_id
- **asset_files**: id, asset_version_id, relative_path, size_bytes, content_hash, object_key, detected_type, created_at
- **licences**: id, asset_id, detected_type, detection_confidence, canonical_diff (nullable), original_text, attributes_json, created_at, updated_at
- **tags**: id, library_id, path (materialised path, e.g. `environment/trees/deciduous`), parent_id; unique on `(library_id, path)`
- **asset_tags**: asset_id, tag_id
- **projects**: id, library_id, name, description, target_distribution, created_by_user_id, created_at
- **project_asset_pins**: project_id, asset_id, pinned_version_id, pinned_at, pinned_by_user_id
- **collections**: id, library_id, name, description, created_by_user_id, created_at
- **collection_assets**: collection_id, asset_id
- **audit_log**: id, library_id (nullable for server-level events), user_id, action_type, target_type, target_id, diff_json, created_at

Library-scoped entities carry `library_id` directly rather than inferring through parent foreign keys — queries always filter at the top level, and cascading deletes work cleanly. `created_by_user_id` / `pinned_by_user_id` are attribution only; they do not affect access rules.

File storage in S3 is content-addressed: the object key is the SHA-256 of the file content. `asset_files.object_key` points at the hashed object. Blob storage is global — the same texture appearing in five packages (even across libraries) occupies one object. User-facing asset identity, however, is library-scoped: the same bytes showing up in two libraries produce two distinct `assets` rows sharing blob references.

## 6. Architecture

### Backend (Go)

A single Go binary exposing a REST-ish JSON API. Suggested module layout:

- `cmd/server` — entry point
- `internal/api` — HTTP handlers, routing, auth middleware
- `internal/asset` — asset, version, and file management
- `internal/licence` — detection, knowledge base, attribute modelling
- `internal/project` — project and pinning logic
- `internal/search` — search and filter queries (starts in Postgres with `tsvector`; can move to a dedicated engine later if needed)
- `internal/storage` — S3 client abstraction, content-addressed put/get
- `internal/thumb` — thumbnail generation (images, audio waveforms, package preview harvesting)
- `internal/audit` — audit log writer
- `internal/auth` — sessions, user management
- `internal/db` — migrations, Postgres access

### Frontend (React)

Single-page app served by the Go binary in production (static file handler). Talks to the backend over the JSON API. Key views:

- Library browse (grid/list with filters and search)
- Asset detail (versions, files, licence, projects using, audit)
- Project detail (pinned assets, distribution profile, audit)
- Collection browse and edit
- Import flow (drag-and-drop, metadata confirmation, Unity extraction picker)
- Licence editor (detection result, diff, attributes, original text)
- Settings and audit log

### Storage

- **Postgres** for all structured data
- **S3-compatible object storage** (MinIO on TrueNAS for Thomas's setup; swappable) for all file bytes, keyed by content hash
- Thumbnails live in the same bucket under a predictable prefix

### Deployment

A Dockerfile produces a small image containing the Go binary and the built React assets. Runs behind a reverse proxy. Requires Postgres and S3 endpoints as configuration. First-run migrations create the schema.

## 7. MVP scope

The MVP is deliberately narrow. It establishes the schema, the import flow, and the storage model correctly so that deferred features can be built on stable foundations.

**In scope for MVP:**

- User auth (login, basic account management)
- Audit log for all write actions
- Drag-and-drop web upload of arbitrary files
- `.unitypackage` import with subset extraction into loose files; original package retained
- Assets with metadata, tags (free-form and hierarchical), versions with explicit pinning
- Content-addressed object storage with deduplication
- File manifest stored per asset version (supports future component detection)
- Licence records: detection against SPDX + Unity EULA, canonical diff flagging, original text stored, custom licence attribute entry
- Projects with declared distribution profile and asset pins; upgrade is explicit
- Collections (user-curated)
- Thumbnails for images and audio (waveform); harvested preview images from Unity packages where available; file-type icon fallback for everything else
- Search: full-text over name/description/tags, filters by tag, licence type, project, file type, source
- Docker deployment with Postgres and S3-compatible backend

**Explicitly deferred out of MVP:**

- Licence compliance reports (data is captured; reporting UI waits)
- Release snapshots
- Automatic upstream update detection
- 3D/video thumbnails
- Content-aware search inside packages
- Auto-tagging
- CLI
- Unity Hub integration
- Asset components (intra-asset grouping)

The principle: the MVP must capture every piece of data needed for deferred features, so that unlocking them later is a UI and logic problem, not a data problem.

## 8. Open questions

These are worth revisiting before or during implementation:

- **Licence knowledge base format and update cadence.** Ships with the app, but how are updates delivered — embedded in binary and shipped with releases, or pulled from a remote manifest?
- **Search engine trajectory.** Postgres full-text is almost certainly enough for v1. At what scale (or feature ambition — e.g. embeddings for similarity search) does it make sense to introduce a dedicated search layer?
- **Thumbnail generation pipeline.** Synchronous on import (simple, blocks UX) or async via a background job queue (more infra, better UX)? Given RabbitMQ experience, a job queue is natural but adds a moving part.
- **Audit log retention.** Unbounded initially; revisit if volume becomes a problem.
- **How to represent "original source" of an asset.** Is it just a URL, or a richer object (vendor, product ID, purchase date, receipt reference) that would help with update detection later?
- **Collaboration boundary.** If multi-user ever means "users editing each other's assets," concurrency and conflict handling enter scope. The current design assumes a shared library but low contention.

## 9. References (to be filled)

- Standard Go project stack (TBD by Thomas)
- Standard React project stack (TBD by Thomas)
- ADR template and location (TBD)