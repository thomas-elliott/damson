# Getting started

This guide takes an operator from nothing to a running Damson with a first
asset in it. To build or change Damson, start at [`AGENTS.md`](../AGENTS.md).

## What Damson is for

Damson is a self-hosted catalogue for the asset packs a game team buys and
reuses: Unity Asset Store packages, zipped kits, loose textures and audio. It
extracts uploaded packages, renders thumbnails and previews, pulls the
storefront listing (description, publisher, cover art, tags) where it can, and
tracks each pack's licence. Projects pin a specific version of an asset, so you
can answer "which pack is this tree from, which version is the game shipping,
and are we allowed to ship it".

It is one Go binary that serves its own web UI, backed by Postgres and either
the local filesystem or an S3-compatible object store.

Damson is pre-alpha. It is a single process with no HA, no backup tooling and
no upgrade or downgrade guarantees between versions. Back up the Postgres
database and the object store, and read the release notes before upgrading.

## Before you start

- Docker with Compose v2. Everything below uses `docker compose`.
- Postgres 16. The quickstart runs one for you. `compose.yaml` and CI both run
  16, so it is the tested version.
- Access to the container image. Images are published to
  `ghcr.io/thomas-elliott/damson`, but the repository and package are private,
  so an anonymous `docker pull` is refused. Log in with a GitHub personal access
  token that has `read:packages` and access to the package:

  ```sh
  echo "$GITHUB_TOKEN" | docker login ghcr.io -u "$GITHUB_USERNAME" --password-stdin
  ```

  With a checkout instead, `just image-build` builds the same image locally as
  `damson:local`. Use that name in the Compose file below.
- Disk. Damson keeps the uploaded archive, the files extracted from it, and the
  derived thumbnails and previews, so budget several times the size of your
  packages. Nothing on the write path deletes an object. Reclaiming space is a
  deliberate two-step sysadmin action (see
  [`docs/architecture.md`](architecture.md), Object storage).

You do not need Go, Node, `just` or a checkout to run Damson.

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

`DAMSON_POSTGRES_DSN` is the only required variable. Auth mode defaults to
`local`, storage mode to `local`, and the server listens on `:8080`. Migrations
run at startup under a Postgres advisory lock, so a first boot creates its own
schema.

Check it is up:

```sh
curl -s localhost:8080/api/health
# {"status":"ok","version":"dev"}
```

The published image reports `"dev"` because it is built without the git
metadata the version stamp comes from. Track the release by the image tag.

### What the volumes hold

- `pgdata` is the database: every asset record, library, user, tag, licence
  and audit row.
- `damson-data` is mounted at `/data`. In local mode it holds `/data/jwt.key`,
  the auto-generated JWT signing key, and `/data/blobs`, every stored byte.
  Losing it logs everyone out and loses every uploaded file.

The image runs as a non-root `damson` user, and `/data` is created owned by that
user, so an empty named volume starts writable. With a host bind mount, chown it
to match, or the server fails to start with a "data directory ... is not
writable" error naming the directory.

### Ports and TLS

The image's `HEALTHCHECK` is hardcoded to `http://localhost:8080/api/health`.
If you change `DAMSON_LISTEN_ADDR` to another port, the container reports
unhealthy even though the server is fine. Remap the published port instead
(`"127.0.0.1:9000:8080"`) and leave `DAMSON_LISTEN_ADDR` alone.

Damson speaks plain HTTP and does not terminate TLS. For anything beyond a local
trial, put it behind a reverse proxy and set `DAMSON_TRUSTED_PROXIES` so the
login throttle sees real client IPs. See
[`docs/configuration.md`](configuration.md).

## Bootstrapping the first admin

Damson has no default account. On an empty system the first person to sign in
becomes the `sysadmin` and gets a library called **Personal**.

Open <http://localhost:8080>. With no users in the database, the app routes you
to `/bootstrap` and shows a **Create the first admin** form.

![The Create the first admin form with email, password and confirm password filled in](images/first-run-bootstrap.png)

Fill in **Email**, **Password** and **Confirm password**, then press **Create
account**. The password must be at least 8 characters. You land in `/library`,
signed in, with the Personal library selected.

Once that account exists the bootstrap endpoint closes. A second visit to
`/bootstrap` gets a 409, and unauthenticated visitors are sent to `/login`.

### Seeding the admin from the environment

For an unattended or config-managed install with no bootstrap form, set both of
these on the `damson` service and the first admin is created at startup:

```yaml
      DAMSON_LOCAL_ADMIN_EMAIL: admin@example.com
      DAMSON_LOCAL_ADMIN_PASSWORD: a-real-password
```

Setting only one does nothing. The password must be at least 8 characters, and
a short one fails startup. These variables only create the first user: changing
them later does not rotate an existing admin's password. Set
`DAMSON_LOCAL_ADMIN_PASSWORD_FILE` to read the password from a Docker secret
instead of the Compose file.

### Using an identity provider

Set `DAMSON_AUTH_MODE=oidc` and the local login form disappears. The first
person to sign in through your IdP becomes the sysadmin, and there is no admin
allow-list. See the OIDC section of [`docs/configuration.md`](configuration.md).

Local auth uses bcrypt passwords and one-hour JWTs. It suits a trial or a small
trusted team. Prefer OIDC where account security matters.

## Your first library

A library is Damson's tenancy boundary. Assets, tags, projects, collections,
settings and the audit log each belong to exactly one library, and you see only
libraries you are a member of. Most small teams never need a second one.

Bootstrap created **Personal**, so you can skip to uploading.

To add another, for example one per game, use the API, because the web UI can
rename, delete and switch libraries but cannot create them. Authenticate and
post to `/api/libraries`:

```sh
TOKEN=$(curl -sS -X POST localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"your-password"}' \
  | jq -r .token)

curl -sS -X POST localhost:8080/api/libraries \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Sample Library","description":"Assets for the sample build"}'
# {"id":2,"name":"Sample Library","description":"Assets for the sample build"}
```

Whoever creates a library becomes its admin. The new library does not become
your current one. Switch to it from the library picker in the header (the
**Switch library** button, which lists every library you belong to), or post the
id to `/api/me/current-library`.

![The Switch library menu open, listing Personal and Sample Library with their role badges and a tick on the current library](images/library-switcher.png)

Local-mode login tokens last one hour, so log in again when you return to a
shell the next day.

### Adding other people

Under **Settings → Members**, a library admin presses **Invite** and enters an
email, an optional display name, and a role (`admin` or `member`). In local auth
mode, set an initial password for a new user. For someone who already has an
account, leave it blank and they are added to this library.

Membership is the only source of access, and there is no "everyone can see
everything" mode. A user who belongs to no library can sign in but has nowhere
to go.

## Your first asset

Go to **Library** in the left rail. Drag files onto the page, or press
**Upload** in the toolbar and pick them.

![The Library page with the dashed Drop files to import overlay showing during a drag](images/library-drop-overlay.png)

What you can upload:

- `.unitypackage`: extracted into its individual files. The asset's cover image
  comes from the preview image Unity ships inside the package. A per-library
  setting turns that off and falls back to the storefront cover.
- `.zip`: extracted the same way, with a cover harvested from its contents.
- Anything else: stored as a single "loose" file and thumbnailed from its own
  bytes. Images are resampled, audio gets a rendered waveform, and fonts get a
  type specimen. Each has its own per-library toggle under **Settings →
  Thumbnails**.

The file extension alone decides the format, so a zip named `.7z` is stored as
one loose blob and not extracted.

One file, or a set of files that are all archives, uploads straight away as
separate assets. A mix of loose files, or a dropped folder, first asks whether
you want separate assets or one bundle zipped together. Files of 256 MiB or more
switch automatically to a resumable chunked upload that survives a dropped
connection. A sysadmin can change the server-wide upload ceiling, 64 GiB by
default, under **Settings → Server settings → Max upload bytes**.

Damson reads the file name. It peels off a trailing version token and turns
separators into spaces, so `Forest_Pack_v2.1.unitypackage` becomes an asset
named "Forest Pack" at version `2.1`. A bare trailing integer stays in the name,
so `Rock Set 2.zip` is "Rock Set 2" with no version.

Uploading a new file for an existing asset never overwrites it. In the SPA it
is added as its own asset first, and if Damson recognises a match, a toast offers
to fold it in as a new version. See
[Pin a project to a version you have tested](how-to/pin-versions.md).

The upload returns as soon as the asset record is written, so a placeholder
tile appears in the grid while the rest happens in the background:

`uploaded` → `extracting` → `detecting` → `thumbnailing` → `enriching` →
`done` (or `failed`)

Watch it on the **Import queue** page. Preview rendering has its own
**Processing queue** page. Both update live. A failed import stays listed with
its stage and can be retried. Thumbnailing and enrichment are best-effort, and
neither failing fails the import.

![The Import queue with one pack in the extracting stage and finished imports below it](images/import-queue.png)

Click the finished asset to open its panel: files, versions, tags, licence, and
the storefront listing if enrichment found one. Tags are paths
(`environment/trees/deciduous`), and search covers names, descriptions,
publishers, tags and locations.

## Where to go next

- [`docs/how-to/`](how-to/README.md): task guides for
  [tracking licences](how-to/track-licences.md),
  [Unity packages](how-to/import-unity-packages.md),
  [versions and pins](how-to/pin-versions.md),
  [more than one library](how-to/separate-libraries.md) and
  [collections](how-to/collections.md).
- [`docs/configuration.md`](configuration.md): every environment variable, its
  default, and the OIDC and S3 setups.
- [`docs/observability.md`](observability.md): the log shape, request
  correlation and useful queries.
- [`docs/architecture.md`](architecture.md): how the pieces fit.

### Scripting it

`dmsn` is a CLI client for the same HTTP API, built for agents and scripts.
`dmsn sync` imports Unity's local Asset Store cache in bulk, which is usually
the fastest way to fill a new library. It authenticates with a personal access
token. Create one under **Settings → Access tokens**, choosing `read` or
`write` scope and an optional expiry. A token is pinned to the library that was
current when you created it.

### Backups

There is no built-in backup command. Back up two things, consistently with each
other:

- the Postgres database (`pg_dump` the `damson` database), and
- the object store: `/data/blobs` in local mode, or your bucket in S3 mode.

Back up the database after storage, not before. A blob with no row is wasted
space, but a row pointing at a missing blob is a broken asset.
