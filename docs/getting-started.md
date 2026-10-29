# Getting started

This guide takes you from nothing to a running Damson with your first asset in
it. It is written for whoever is going to operate the thing; if you want to
build or change Damson, start at [`AGENTS.md`](../AGENTS.md) instead.

## What Damson is for

Damson is a self-hosted catalogue for the asset packs a game team buys and
reuses: Unity Asset Store packages, zipped kits, loose textures and audio. It
extracts an uploaded package, renders thumbnails and previews, pulls the
storefront listing (description, publisher, cover art, tags) where it can, and
tracks the licence each pack came with. Projects pin a specific version of an
asset, so you can answer "which pack is this tree from, which version is the
game shipping, and are we allowed to ship it".

It is one Go binary that serves its own web UI, backed by Postgres and either
the local filesystem or an S3-compatible object store.

Damson is **pre-alpha**. It is a single-process application with no HA story,
no backup tooling of its own, and no upgrade/downgrade guarantees between
versions. Treat the Postgres database and the object store as the things you
must back up, and expect to read release notes before upgrading.

## Before you start

- **Docker with Compose v2.** Everything below uses `docker compose`.
- **Postgres 16.** The quickstart runs one for you. If you bring your own,
  16 is what `compose.yaml` and CI both run against — that is the version
  Damson is actually tested on.
- **Access to the container image.** Damson's images are published to
  `ghcr.io/thomas-elliott/damson`, but the repository and its package are
  private, so an anonymous `docker pull` is refused. Log in first with a GitHub
  personal access token that has `read:packages` and access to the package:

  ```sh
  echo "$GITHUB_TOKEN" | docker login ghcr.io -u "$GITHUB_USERNAME" --password-stdin
  ```

  If you have a checkout instead of registry access, `just image-build` builds
  the same image locally as `damson:local`; substitute that for the image name
  in the Compose file below.
- **Disk.** Budget for your packages several times over: Damson keeps the
  uploaded archive *and* the files it extracts from it *and* the thumbnails and
  previews it derives. Nothing on the write path ever deletes an object —
  reclaiming space is a deliberate two-step sysadmin action (see
  [`docs/architecture.md`](architecture.md), Object storage).

You do not need Go, Node, `just`, or a checkout to run Damson.

## Quickstart with Docker Compose

Create an empty directory and put this in `compose.yaml`:

```yaml
name: damson

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: damson
      POSTGRES_PASSWORD: damson
      POSTGRES_DB: damson
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U damson -d damson"]
      interval: 5s
      timeout: 3s
      retries: 20

  damson:
    image: ghcr.io/thomas-elliott/damson:latest
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DAMSON_POSTGRES_DSN: postgres://damson:damson@postgres:5432/damson?sslmode=disable
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - damson-data:/data
    restart: unless-stopped

volumes:
  pgdata:
  damson-data:
```

Then:

```sh
docker compose up -d
```

`DAMSON_POSTGRES_DSN` is the only variable Damson requires. Everything else
has a default: auth mode is `local`, storage mode is `local`, and the server
listens on `:8080`. Migrations run at startup under a Postgres advisory lock,
so a first boot sets up its own schema.

Check it is up:

```sh
curl -s localhost:8080/api/health
# {"status":"ok","version":"dev"}
```

(The published image reports `"dev"` — it is built without the git metadata the
version stamp comes from. Track which release you are on by the image tag.)

### What the volumes hold

- `pgdata` is the database: every asset record, library, user, tag, licence
  and audit row.
- `damson-data` is mounted at `/data` and holds two things in local mode —
  `/data/jwt.key`, the auto-generated JWT signing key, and `/data/blobs`, every
  stored byte. **Losing it logs everyone out and loses every uploaded file.**

The image runs as a non-root `damson` user and `/data` is created owned by that
user, so an empty *named* volume mounted there starts writable. If you swap in
a host bind mount, chown it to match or the server will refuse to start with a
"data directory ... is not writable" error naming the directory.

### A note on the port

The image's `HEALTHCHECK` is hardcoded to `http://localhost:8080/api/health`.
If you change `DAMSON_LISTEN_ADDR` to a different port, the container's health
status goes unhealthy even though the server is fine — remap the published port
instead (`"127.0.0.1:9000:8080"`) and leave `DAMSON_LISTEN_ADDR` alone.

Damson speaks plain HTTP and does not terminate TLS. Put it behind a reverse
proxy for anything beyond a local trial, and set `DAMSON_TRUSTED_PROXIES` so
the login throttle sees real client IPs — see
[`docs/configuration.md`](configuration.md).

## Bootstrapping the first admin

Damson has no default account. On an empty system the first person to sign in
becomes the `sysadmin` and gets a library called **Personal** created for them.

Open <http://localhost:8080>. With no users in the database, the app routes you
to `/bootstrap` and shows a **Create the first admin** form.

> _[Screenshot: the `/bootstrap` "Create the first admin" form on a fresh
> install.]_

Fill in **Email**, **Password** and **Confirm password**, then press **Create
account**. The password must be at least 8 characters. You land in `/library`,
signed in, with the Personal library selected.

Once that account exists the bootstrap endpoint closes itself — a second visit
to `/bootstrap` gets a 409 and unauthenticated visitors are sent to `/login`
instead.

### Seeding the admin from the environment instead

If you would rather not have a bootstrap form exposed at all — an unattended
install, or a config-managed deployment — set **both** of these on the `damson`
service and the first admin is created at startup:

```yaml
      DAMSON_LOCAL_ADMIN_EMAIL: admin@example.com
      DAMSON_LOCAL_ADMIN_PASSWORD: a-real-password
```

Both must be set; setting one alone does nothing. The password still has to be
at least 8 characters, and a short one fails startup rather than being
silently ignored. This only ever *creates* the first user: changing the values
later does not rotate an existing admin's password. Use
`DAMSON_LOCAL_ADMIN_PASSWORD_FILE` to read it from a Docker secret instead of
baking it into the Compose file.

### Or use your identity provider

Set `DAMSON_AUTH_MODE=oidc` and the local login form disappears entirely; the
first person to sign in through your IdP becomes the sysadmin by the same
rule. There is no admin allow-list to configure. See the OIDC section of
[`docs/configuration.md`](configuration.md).

Local auth is bcrypt passwords and one-hour JWTs. It is fine for a trial or a
small trusted team; OIDC is the path to prefer for anything where account
security matters.

## Your first library

A **library** is Damson's tenancy boundary. Everything — assets, tags,
projects, collections, settings, the audit log — belongs to exactly one, and
you only see a library you are a member of. Most small teams never need a
second one.

Bootstrap already created **Personal** for you, so you have a working library
and can skip straight to uploading.

If you do want another — say one library per game — note that **the web UI
cannot create libraries today.** It can rename, delete, and switch between
them, but creating one is an API call. Authenticate and post to
`/api/libraries`:

```sh
TOKEN=$(curl -sS -X POST localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"your-password"}' \
  | jq -r .token)

curl -sS -X POST localhost:8080/api/libraries \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Project Hollow","description":"Assets for the Hollow build"}'
# {"id":2,"name":"Project Hollow","description":"Assets for the Hollow build"}
```

Whoever creates a library becomes its admin. The new library does *not* become
your current one automatically — switch to it from the library picker in the
header (the button labelled **Switch library**, which lists every library you
belong to), or post the id to `/api/me/current-library`.

> _[Screenshot: the library picker dropdown open, showing two libraries with
> their role badges.]_

Local-mode login tokens last one hour, so expect to re-run the login call if
you come back to a shell the next day.

### Adding other people

Under **Settings → Members**, a library admin presses **Invite** and fills in
an email, an optional display name, and a role (`admin` or `member`). In local
auth mode you also set an initial password for a brand-new user; for someone
who already has an account, leave it blank and they are simply added to this
library.

This matters because a user who exists but belongs to no library can sign in
and then has nowhere to go. Membership is the only source of access — there is
no "everyone can see everything" mode.

## Your first asset

Go to **Library** in the left rail. Either drag files onto the page or press
the **Upload** button in the toolbar and pick them.

> _[Screenshot: the Library page with the drop overlay active during a drag.]_

What you can upload:

- **`.unitypackage`** — extracted into its individual files. The asset's cover
  image comes from the preview image Unity ships inside the package (a
  per-library setting can turn that off and fall back to the storefront cover).
- **`.zip`** — extracted the same way, with a cover harvested from its
  contents.
- **Anything else** — stored as a single "loose" file, thumbnailed from its own
  bytes: a resampled image, a rendered waveform for audio, a type specimen for
  a font. Each of those three has its own per-library toggle under **Settings →
  Thumbnails**.

The format is decided purely by the file extension, so a zip named `.7z` is
stored as one loose blob rather than extracted.

One file, or a set of files that are all archives, uploads straight away as
separate assets. Anything more ambiguous — a mix of loose files, or a dropped
folder — asks first whether you meant separate assets or one bundle zipped
together. Files at or above 256 MiB switch automatically to a resumable chunked
upload that survives a dropped connection. There is also a server-wide upload
ceiling, 64 GiB by default, which a sysadmin can change under **Settings →
Server settings → Max upload bytes**.

**Name the file well — Damson reads it.** A trailing version token is peeled
off and separators become spaces, so `Forest_Pack_v2.1.unitypackage` becomes an
asset named "Forest Pack" at version `2.1`. A bare trailing integer is kept as
part of the name, so `Rock Set 2.zip` is just "Rock Set 2" with no version.
Uploading a new file for an asset you already have never overwrites it. In the
SPA it lands as its own asset first, and if Damson recognises a match a toast
offers to fold it in as a new version instead — see
[Pin a project to a version you have tested](how-to/pin-versions.md).

The upload returns as soon as the asset record is written, so a placeholder
tile appears in the grid immediately while the rest happens in the background:

`uploaded` → `extracting` → `detecting` → `thumbnailing` → `enriching` →
`done` (or `failed`)

Watch it on the **Import queue** page; preview rendering has its own
**Processing queue** page. Both update live. A failed import stays listed with
its stage and can be retried. Thumbnailing and enrichment are best-effort —
neither failing will fail the import.

> _[Screenshot: the Import queue page with one import mid-pipeline and one
> finished.]_

Click the finished asset to open its panel: files, versions, tags, licence, and
the storefront listing if enrichment found one. Tags are paths
(`environment/trees/deciduous`) and search covers names, descriptions,
publishers, tags and locations.

## Where to go next

- [`docs/how-to/`](how-to/README.md) — task guides for the rest of it:
  [tracking licences](how-to/track-licences.md), [Unity
  packages](how-to/import-unity-packages.md), [versions and
  pins](how-to/pin-versions.md), [more than one
  library](how-to/separate-libraries.md), and
  [collections](how-to/collections.md).
- [`docs/configuration.md`](configuration.md) — every environment variable,
  its default, and the OIDC and S3 setups written out properly.
- [`docs/observability.md`](observability.md) — the log shape, request
  correlation, and queries worth having.
- [`docs/architecture.md`](architecture.md) — how the pieces fit, if you want
  to know what you are operating.

### Scripting it

`dmsn` is a CLI client for the same HTTP API, built for agents and scripts.
`dmsn sync` imports Unity's local Asset Store cache in bulk, which is usually
the fastest way to fill a new library. It authenticates with a personal access
token — create one under **Settings → Access tokens**, choosing `read` or
`write` scope and an optional expiry. A token is pinned to the library that was
current when you minted it.

### Backups

There is no built-in backup command. Two things need backing up, and they need
to be consistent with each other:

- the Postgres database (`pg_dump` the `damson` database), and
- the object store — `/data/blobs` in local mode, or your bucket in S3 mode.

Back up the database *after* storage, not before: a blob with no row is wasted
space, but a row pointing at a blob you do not have is a broken asset.
