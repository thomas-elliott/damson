# Damson

**A self-hosted catalogue for the asset packs a game team buys and reuses.**

Damson takes the pile of `.unitypackage` files, zipped kits and loose textures
that accumulates on a shared drive and turns it into something you can search,
preview, and answer questions about — including the question that matters on
ship day: *are we allowed to use this?*

It is one Go binary that serves its own web UI, backed by Postgres and either
the local filesystem or an S3-compatible object store.

> **Damson is pre-alpha.** It works, and it is useful, but it is a
> single-process application with no HA story, no backup tooling of its own,
> and no upgrade guarantees between versions. See
> [Project status](#project-status) before you put anything you care about in
> it.

**→ [Getting started](docs/getting-started.md)** — from nothing to a running
Damson with your first asset in it.

## What it does

- **Extracts what you upload.** A `.unitypackage` or `.zip` is unpacked into
  its individual files; anything else is stored whole. Thumbnails and previews
  are rendered from the contents — resampled images, waveforms for audio, type
  specimens for fonts.
- **Tracks the licence.** One licence per asset, with the terms broken out as
  six yes/no/*unknown* attributes — commercial use, modification,
  redistribution, attribution, share-alike, open-source required. *Unknown* is
  a real third state, never silently a "no". Filter a library by what it
  permits, or open a project and see whether anything in it blocks a commercial
  release.
- **Pulls the storefront listing.** Where it can, Damson finds the Unity Asset
  Store page for a pack and brings back the description, publisher, cover art
  and tags.
- **Versions things.** Uploading a newer build of a pack you already have never
  overwrites it: it lands as its own asset, and when Damson recognises the
  match it offers to fold it in as a new version instead. Projects pin the
  specific version they are shipping, so "which pack is this tree from, and
  which version is in the build" has an answer.
- **Organises three different ways.** Tags are hierarchical paths describing
  what an asset *is*; collections are flat bookmarks recording what you
  *decided* about it; projects pin versions for a build. They are deliberately
  not interchangeable.
- **Keeps libraries apart.** A library is the tenancy boundary — assets, tags,
  projects, settings and the audit log all belong to exactly one, and you only
  see a library you are a member of.
- **Has a CLI.** `dmsn` speaks the same HTTP API for scripts and agents;
  `dmsn sync` bulk-imports Unity's local Asset Store cache, which is usually
  the fastest way to fill a new library.

## Who it is for

A small game team — or one person with a decade of Asset Store purchases — who
wants the packs catalogued somewhere they control, on their own hardware,
without a per-seat subscription. It assumes you are comfortable running a
container and a Postgres, and that you would rather read a configuration
reference than click through a setup wizard.

It is **not** a general-purpose DAM for marketing collateral, and it is not a
version control system for your project's source assets. It is a catalogue of
things you *acquired*.

## What it looks like

Signing in drops you on **Library**: a paged, virtualised grid of asset tiles,
with a detail panel that opens over it and a **Filter** sidebar you toggle on
for projects, tags, file kind, licence status and release date. The left rail
gets you to **Import queue** and **Processing queue** (both live-updating),
**Projects**, **Collections**, the **Audit** log, and **Settings**.

> _[Screenshot: the Library grid with the filter sidebar open and an asset
> detail panel over it.]_

> _[Screenshot: an asset detail panel showing files, versions, tags and the
> detected licence with its six attributes.]_

## Documentation

**Running Damson**

- [Getting started](docs/getting-started.md) — install with Docker Compose,
  bootstrap the first admin, upload your first asset
- [Configuration reference](docs/configuration.md) — every environment
  variable, its default, and the OIDC and S3 setups written out
- [How-tos](docs/how-to/README.md) — [licences](docs/how-to/track-licences.md),
  [Unity packages](docs/how-to/import-unity-packages.md),
  [versions and pins](docs/how-to/pin-versions.md),
  [multiple libraries](docs/how-to/separate-libraries.md),
  [collections](docs/how-to/collections.md)
- [Observability](docs/observability.md) — log shape, request correlation, and
  queries worth having

**Building Damson**

- [CONTRIBUTING.md](CONTRIBUTING.md) — get a dev environment up, and what a
  good pull request looks like here
- [AGENTS.md](AGENTS.md) — the commands and the dev loop, for people and coding
  agents alike
- [docs/architecture.md](docs/architecture.md) — how the system fits together,
  and the traps
- [docs/CONVENTIONS.md](docs/CONVENTIONS.md) — coding standards, handler
  patterns, test helpers
- [docs/design-system.md](docs/design-system.md) — frontend tokens and
  primitives
- [docs/e2e-flows.md](docs/e2e-flows.md) — what the Playwright suite covers

## Project status

Pre-alpha, and honestly so. What that means in practice:

- **One process, no HA.** There is no clustering story and no read replica
  story. Scaling means a bigger box.
- **No backup tooling.** Two things need backing up together — the Postgres
  database and the object store (`/data/blobs` in local mode, or your bucket).
  Back up the database *after* storage. See
  [Backups](docs/getting-started.md#backups).
- **Gaps with no UI yet.** Creating a library is an API call; the web UI can
  only rename, delete and switch between them. The how-tos name these gaps
  rather than working around them quietly.
- **No upgrade guarantees.** Migrations run forward automatically at startup
  and there is no downgrade path. Nothing stops you upgrading across several
  versions at once, and nothing promises it will work.
- **Private repository.** The `ghcr.io/thomas-elliott/damson` images are
  private too, so `docker pull` needs a GitHub token with `read:packages` — or
  build the image yourself with `just image-build`.

Released versions are the `v*.*.*` tags; the published images carry matching
tags. The root `VERSION` file is not what the build reads and should not be
trusted as the current version.

## Licence

No licence file has been added to this repository yet, so default copyright
applies and there is no grant of use beyond what you have been given
explicitly. That is an omission rather than a decision.
