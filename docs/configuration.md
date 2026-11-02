# Configuration reference

Every environment variable Damson reads, with its default and whether it is
required. To get a server running first, see
[`docs/getting-started.md`](getting-started.md).

## How configuration is loaded

Damson reads configuration once, at startup, from environment variables
(`internal/config/config.go`, plus the logging variables in
`cmd/server/main.go`). A change needs a restart.

A `.env` file in the server's working directory is loaded if present, but real
environment variables win on a conflict. In the container image the working
directory is `/`, so the file would be `/.env`. Set the environment directly
instead.

Damson reports every configuration problem at once and then exits 1. Each
problem (a missing required value, an unparseable duration, an invalid mode) gets
its own log line naming the variable.

`DAMSON_POSTGRES_DSN` is the only variable that is always required. Auth mode
and storage mode both default to `local`. Some variables become required when
you select a non-default mode; the tables mark them.

### Secrets and `_FILE`

Five variables accept a `<NAME>_FILE` companion that reads the value from a
file path, for Docker secrets and Kubernetes secret mounts. The file's trailing
newline is stripped:

- `DAMSON_POSTGRES_DSN_FILE`
- `DAMSON_JWT_SIGNING_KEY_FILE`
- `DAMSON_LOCAL_ADMIN_PASSWORD_FILE`
- `DAMSON_S3_ACCESS_KEY_FILE`
- `DAMSON_S3_SECRET_KEY_FILE`

Adding `_FILE` to any other variable does nothing.

Set the direct variable or its `_FILE` companion, not both: both is a startup
error. A `_FILE` path that cannot be read is also a startup error, naming the
path.

```yaml
secrets:
  db_dsn:
    file: ./secrets/db_dsn
services:
  damson:
    environment:
      DAMSON_POSTGRES_DSN_FILE: /run/secrets/db_dsn
    secrets: [db_dsn]
```

## Core

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_POSTGRES_DSN` | — | **Yes** | Postgres connection string, e.g. `postgres://damson:damson@postgres:5432/damson?sslmode=disable`. The River job queue opens its own pool against the same DSN. Supports `_FILE`. |
| `DAMSON_LISTEN_ADDR` | `:8080` | No | Address the HTTP server binds, in Go `host:port` form. `127.0.0.1:8080` binds loopback only. |
| `DAMSON_BASE_URL` | `http://localhost:8080` | No | The JWT issuer, not a public URL. See below. |
| `DAMSON_TRUSTED_PROXIES` | empty | No | Comma-separated IPs or CIDRs whose `X-Forwarded-For` is believed. Blank trusts nothing. An unparseable entry fails startup. |
| `GIN_MODE` | `debug` | No | `debug`, `release` or `test`. Set `release` in production to silence Gin's startup banner and per-route output. An invalid value crashes the process; see below. |

### `DAMSON_BASE_URL` is the JWT issuer

Damson uses this value in one place: it is the `iss` and `aud` claim of the JWTs
that local auth mode issues, and the value those tokens are verified against.
OIDC redirect URIs do not use it (the browser derives them from its own origin),
nor do links, and Damson does not check it against the request host. It has no
effect in OIDC mode.

- You do not need to set it to your public URL. It only has to be stable.
- Changing it signs every local-mode user out. Existing tokens carry the old
  issuer, fail verification with a 401, and users must log in again.

### `GIN_MODE` can crash the process

`GIN_MODE` belongs to the Gin web framework. Gin reads it when its package
initialises, before Damson loads its configuration. A value other than `debug`,
`release` or `test` panics the process on startup with a raw Go panic (`gin mode
unknown: ...`) instead of a `config:` error line. If a container crash-loops
with a stack trace instead of a log line, check this first.

### `DAMSON_TRUSTED_PROXIES` and client IPs

Damson throttles login and bootstrap attempts per client IP. With this unset,
the client IP is the TCP peer's address, which behind a reverse proxy is the
proxy, so every user shares one throttle bucket.

Set it to your proxy's address so `X-Forwarded-For` is honoured:

```
DAMSON_TRUSTED_PROXIES=10.0.0.0/8,172.18.0.0/16
```

Do not set it to `0.0.0.0/0`. That trusts a header any client can forge, so an
attacker can spread a password-guessing run across unlimited fake IPs and the
throttle stops working.

## Logging

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_LOG_LEVEL` | `info` | No | One of `debug`, `info`, `warn`, `error`. Anything else fails startup. |
| `DAMSON_LOG_FORMAT` | `json` | No | `json` or `text`. Anything else fails startup. Use `text` in a terminal and `json` for a log collector. |

`cmd/server/main.go` reads both before configuration is parsed, so a config
error is logged in the format you asked for. Every line carries `service=damson`
and a `version` field derived from the build stamp. For the log shape, request
correlation IDs and useful queries, see
[`docs/observability.md`](observability.md).

## Authentication

`DAMSON_AUTH_MODE` picks one authenticator for the whole deployment. It cannot
change at runtime and there is no mixed mode.

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_AUTH_MODE` | `local` | No | `local` or `oidc`. Any other value fails startup. |

Personal access tokens (`damson_pat_…`, created under **Settings → Access
tokens**) work in either mode, so scripts and the `dmsn` CLI are unaffected by
your choice.

### Local mode (the default)

Damson issues bcrypt-hashed passwords and HS256 JWTs itself. Tokens last one
hour and the web UI renews them silently before they expire. Set
`DAMSON_POSTGRES_DSN` and nothing else and local auth works.

This is credential handling at development and small-team grade. Where account
security matters, prefer OIDC.

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_JWT_SIGNING_KEY` | auto-generated | No | The HS256 signing key, used verbatim. Must be at least 32 bytes or startup fails. Supports `_FILE`. |
| `DAMSON_JWT_KEY_FILE` | `./data/jwt.key` | No | Where the auto-generated key is kept, when `DAMSON_JWT_SIGNING_KEY` is unset. |
| `DAMSON_LOCAL_ADMIN_EMAIL` | unset | No | With the password below, seeds the first admin at startup. |
| `DAMSON_LOCAL_ADMIN_PASSWORD` | unset | No | Used with the email above. At least 8 characters. Supports `_FILE`. |

#### The signing key

If `DAMSON_JWT_SIGNING_KEY` is unset, the server reads `DAMSON_JWT_KEY_FILE`. If
that file does not exist, it generates 32 random bytes and writes them there
with `0600` permissions, so the file's directory must be writable on first boot.
In the container image `./data/jwt.key` resolves to `/data/jwt.key`.

Set the variable directly for a config-managed or multi-replica deployment,
where every replica signs with the same key without a shared volume. Use the
file for a single container with a volume you already back up. Damson only reads
an existing key file, never rewrites it, and a key file shorter than 32 bytes is
a startup error, not a silent regeneration.

Rotating the key, by either route, invalidates every issued session.

#### Seeding the first admin

Set `DAMSON_LOCAL_ADMIN_EMAIL` and `DAMSON_LOCAL_ADMIN_PASSWORD` and Damson
creates the first admin during startup, bypassing the in-app bootstrap page. Set
only one and nothing happens: no error, no admin. The seed runs only when the
database has no users. Once an admin exists, changing these values does not
rotate their password (that is an admin action in the UI). With both unset, the
first browser visit lands on `/bootstrap`.

Either way the account becomes `sysadmin` and gets a library named "Personal".

### OIDC mode

Set `DAMSON_AUTH_MODE=oidc` and Damson verifies id_tokens from your identity
provider instead of issuing its own. The local login form and the
`/api/auth/login`, `/api/auth/bootstrap` and `/api/auth/refresh` endpoints are
not registered; session renewal is the IdP's silent-renew flow.

All three variables are required in OIDC mode, and an empty one fails startup:

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_OIDC_ISSUER_URL` | — | **In OIDC mode** | Your IdP's issuer URL, e.g. `https://auth.example.com/realms/studio`. Damson fetches the discovery document from it at startup. |
| `DAMSON_OIDC_CLIENT_ID` | — | **In OIDC mode** | The public client id you registered for Damson. |
| `DAMSON_OIDC_AUDIENCE` | — | **In OIDC mode** | The audience claim Damson requires on an incoming id_token. Usually the same string as the client id. |

#### Startup

Damson fetches the discovery document during startup. If the IdP is
unreachable, Damson fails to start with an `oidc init` error instead of starting
degraded. Order your service dependencies accordingly.

#### Access

There is no admin allow-list. As in local mode, the first person to sign in on
an empty system becomes the sysadmin and gets a "Personal" library. Everyone
after that can sign in but has no access until a library admin adds them under
**Settings → Members**. This is intended: if your whole company can reach the
IdP, the second person through should not inherit the first one's library.

#### Client registration

Register Damson as a public client. There is no client secret to configure,
because the token exchange happens in the browser. Redirect URIs come from the
browser's own origin, not from a Damson setting, so register

```
https://damson.example.com/auth/callback
```

as the redirect URI, substituting the origin your users type. You do not need a
post-logout redirect URI: signing out clears the browser's stored session
locally and never calls the IdP's `end_session` endpoint, so you stay signed in
at the IdP.

The client requests the scopes `openid profile email`. Damson verifies the
id_token (not the access token) and reads the `email`, `email_verified` and
`name` claims from it, so your IdP must put those in the id_token, not only on
the userinfo endpoint.

#### Silent renewal does not work

No IdP setting fixes this. The SPA is built to renew tokens in the background:
it registers the same `/auth/callback` URI as its silent-renew target and loads
it in a hidden iframe. Damson blocks that hop itself. Every response carries
`X-Frame-Options: DENY` and a CSP containing `frame-ancestors 'none'` (the
`securityHeaders` middleware in `cmd/server/main.go`), and the SPA fallback
serves `/auth/callback` behind that middleware, so the browser refuses to render
it in the iframe. Both headers block framing unconditionally, including
same-origin, so nothing in the IdP or in Damson's configuration changes this.

Sessions are therefore not extended in the background. When the id_token
expires, the next API call fails to renew and the user is sent back through the
IdP for an interactive sign-in. If their IdP session is still live this is a
quick redirect, but it is a page navigation and any unsaved state in the tab is
lost. Set your token lifetime with that in mind.

Registering the redirect URI above is still required: it is the interactive
sign-in target. You do not need to permit `prompt=none` for Damson today.

#### Identity matching

Damson matches identities on `(issuer, subject)` first. If that pair is unknown,
it falls back to a verified email and links the new identity to that existing
account; otherwise it creates a new user. It never links on an unverified email,
so a user whose IdP sends no email claim lands on a "finish your profile" page
and sets one in Damson. After the first sign-in, Damson owns the user's email and
display name, and later changes at the IdP do not propagate.

Changing the issuer later is disruptive. Existing `user_identities` rows are
keyed on the old issuer string, so moving IdPs (or changing a realm path)
orphans every account. Plan it as a migration, not a config edit.

Damson adds the issuer's `scheme://host` to the Content-Security-Policy
(`connect-src` and `frame-src`) so the browser can reach your IdP for discovery
and token exchange. You do not configure the CSP, and no setting relaxes
`frame-ancestors`.

## Storage

`DAMSON_STORAGE_MODE` picks the blob backend. Both backends use the same key
layout (`blobs/`, `originals/` and `thumbs/`, each addressed by the SHA-256 of
its contents and sharded as `<prefix>/<first-two-hex>/<hash>`), so moving
between them is a recursive copy plus a config change, with no re-import or
database migration.

Local mode stores each object's metadata in a `<hash>.meta.json` sidecar next to
the blob, where S3 uses native object metadata, and a copy translates neither.
That metadata only labels rows in the storage-cleanup report, so nothing breaks.
But when you copy local → S3, exclude the sidecars: S3 mode does not filter them
out as local mode does, so any that come along appear as orphan candidates in the
next cleanup scan:

```sh
# source is DAMSON_STORAGE_LOCAL_PATH (/data/blobs in the container image);
# the prefix directories inside it map to keys at the bucket root
aws s3 sync /data/blobs s3://your-bucket/ --exclude "*.meta.json"
```

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_STORAGE_MODE` | `local` | No | `local` or `s3`. Any other value fails startup. |

### Local mode (the default)

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_STORAGE_LOCAL_PATH` | `./data/blobs` | No | Directory that holds every stored object. |

The path is relative to the server's working directory, which is `/` in the
container image, so the default resolves to `/data/blobs` there, next to the JWT
key.

Damson creates the directory if it is missing and proves it can write a file
there before it finishes starting. A volume mounted read-only or with the wrong
owner therefore fails at boot, naming the directory, instead of on the first
upload.

Do not run more than one replica against local storage. Resumable chunked
uploads keep their parts on the local disk of the process that received them, so
a second replica cannot find them. Local mode is for single-instance
deployments.

### S3 mode

For MinIO, Garage, AWS S3, or anything else speaking the S3 API. The first five
variables are required in S3 mode:

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_S3_ENDPOINT` | — | **In S3 mode** | Full endpoint URL, e.g. `http://minio:9000`. Required even on real AWS. |
| `DAMSON_S3_REGION` | — | **In S3 mode** | Region name. MinIO and Garage ignore it, but the SDK requires one; `us-east-1` is the conventional placeholder. |
| `DAMSON_S3_BUCKET` | — | **In S3 mode** | Bucket name. Create it first; see below. |
| `DAMSON_S3_ACCESS_KEY` | — | **In S3 mode** | Access key id. Supports `_FILE`. |
| `DAMSON_S3_SECRET_KEY` | — | **In S3 mode** | Secret access key. Supports `_FILE`. |
| `DAMSON_S3_FORCE_PATH_STYLE` | `true` | No | `true` sends `endpoint/bucket/key` (what MinIO and Garage want). Set `false` for virtual-host-style addressing, which real AWS S3 expects. |

Create the bucket yourself first. Damson does not create it or check that it
exists at startup, and only exercises the credentials on the first object read
or write. A wrong bucket name, a wrong key or a missing bucket therefore shows
up as a failed import, not a failed boot. Do a test upload after any change
here.

Credentials are required and come only from the two variables above. Damson does
not use the ambient AWS credential chain, so instance roles, `~/.aws/credentials`
and `AWS_*` environment variables are ignored.

`DAMSON_STORAGE_LOCAL_PATH` is not read in S3 mode, but `DAMSON_JWT_KEY_FILE`
still is. In local auth mode with S3 storage, the container still needs one
small writable path for the signing key, unless you set `DAMSON_JWT_SIGNING_KEY`
directly.

A MinIO example:

```yaml
      DAMSON_STORAGE_MODE: s3
      DAMSON_S3_ENDPOINT: http://minio:9000
      DAMSON_S3_REGION: us-east-1
      DAMSON_S3_BUCKET: damson
      DAMSON_S3_ACCESS_KEY: minioadmin
      DAMSON_S3_SECRET_KEY: minioadmin
      DAMSON_S3_FORCE_PATH_STYLE: "true"
```

## Background jobs

Imports, preview rendering and storefront lookups run as durable River jobs in
Postgres, so they survive a restart; Damson re-enqueues anything unfinished at
the next boot. Most deployments never change these defaults.

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_IMPORT_QUEUE_WORKERS` | number of CPUs | No | Concurrent import jobs (extract, thumbnail, enrich). Must be a whole number above zero. |
| `DAMSON_PREVIEW_QUEUE_WORKERS` | number of CPUs | No | Concurrent preview-rendering jobs. Must be a whole number above zero. |
| `DAMSON_IMPORT_JOB_TIMEOUT` | `30m` | No | How long one import job may run before its context is cancelled. Go duration string. |
| `DAMSON_PREVIEW_JOB_TIMEOUT` | `30m` | No | Same, for one preview job. |
| `DAMSON_STORE_LOOKUP_INTERVAL` | `2s` | No | Minimum gap between two requests to the same storefront, across all callers. |

Lower the worker counts on a small machine: extraction is CPU- and IO-heavy.
Raise `DAMSON_IMPORT_JOB_TIMEOUT` if you import multi-gigabyte packages over slow
storage. Keep `DAMSON_STORE_LOOKUP_INTERVAL` as a courtesy floor toward
storefronts you do not own; lowering it risks rate limiting or a block.

Durations are Go duration strings (`30m`, `24h`, `90s`, `1h30m`). A value that
does not parse, or is zero or negative, fails startup.

## Maintenance intervals

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_AUDIT_PURGE_INTERVAL` | `24h` | No | How often the retention sweeper deletes audit rows older than each library's `audit_retention_days`. |
| `DAMSON_UPLOAD_JANITOR_INTERVAL` | `1h` | No | How often abandoned resumable upload sessions are expired and their backing multipart uploads aborted. |
| `DAMSON_ORPHAN_MIN_AGE` | `168h` (7 days) | No | How old an object must be before the manual storage-cleanup scan calls it orphaned. |

Audit retention is a per-library setting under **Settings → Audit**, not an
environment variable. The interval above only sets how often the sweep runs. A
library with no retention set keeps its audit rows forever.

`DAMSON_ORPHAN_MIN_AGE` is a safety floor, not a schedule. Nothing deletes
objects automatically: a sysadmin runs a scan and then a delete under
**Settings → Storage cleanup**, and the age floor stops the scan flagging an
upload whose database rows have not committed yet. Shortening it on a busy
server risks deleting in-flight uploads. The floor does not protect a reused
object, because content-addressed dedupe reuses a key without touching its
modification time.

## Settings that are not environment variables

These live in the database, so they change without a restart.

Server-wide settings (sysadmin only, **Settings → Server settings**):

| Setting | Default | What it does |
|---|---|---|
| Max upload bytes | `68719476736` (64 GiB) | Hard ceiling on any single upload. |

The same screen carries **Users** (role changes and local password resets) and
**Storage cleanup**.

Per-library settings (library admin, **Settings**):

| Setting | Default | What it does |
|---|---|---|
| Grid density default | `medium` | Starting density of the asset grid. |
| Retain original packages | on | Keep the uploaded archive after extraction. Turning it off reclaims a lot of space but makes re-extraction impossible. |
| Raster / audio / font thumbnails | all on | Per-kind thumbnail generation for loose files. |
| Unity preview harvest | on | Use the preview image inside a `.unitypackage` as the asset's cover. |
| Auto-enrich | on | Fetch storefront metadata during import. |
| Preferred store | `unity` | Which storefront to try first when enriching. |
| Audit retention days | unset (keep forever) | Age at which audit rows are purged. |

### A per-library setting with no UI

`library_settings.previews_enabled` (default `true`) works: when it is off,
Damson refuses to queue preview jobs for that library's assets and skips them in
the backfill sweep. Unlike the inert licence-KB toggle above, it does what its
name says.

It is not reachable from the UI or API. It appears in neither the settings API
response nor its PATCH payload, and the **Settings** screen has no control for
it. The only way to change it today is to write to the `library_settings` row
directly:

```sql
UPDATE library_settings SET previews_enabled = false WHERE library_id = 1;
```

Do that only if you have a reason to. It is listed here so that if you find the
column, or wonder why previews stopped, you know what it is and that the UI will
not show its state.

## Variables Damson does not read

- No TLS settings. Damson serves plain HTTP; TLS and HSTS are the reverse
  proxy's job.
- No `PORT`. Use `DAMSON_LISTEN_ADDR`.
- No `DATABASE_URL`. Use `DAMSON_POSTGRES_DSN`.
- No `AWS_*` credentials. S3 credentials come only from the `DAMSON_S3_*`
  variables.
- `DAMSON_TEST_POSTGRES_DSN` is for the Go and Playwright test suites, not the
  server.
- `DAMSON_SERVER`, `DAMSON_TOKEN` and `DAMSON_CONFIG` configure the `dmsn` CLI,
  not the server.

## Checking what a running server is using

Damson does not expose its resolved configuration over HTTP, because most of it
is secret. You can check:

```sh
curl -s localhost:8080/api/health
# {"status":"ok","version":"dev"}
```

`version` comes from the Go build stamp. The published container image builds
without the `.git` directory, so it has no stamp and reports `"dev"` instead of
its release. Do not use this endpoint to tell image versions apart; use the
image digest or tag.

The startup log lines name the modes Damson selected, which is usually what you
want to confirm:

```
{"msg":"auth mode","mode":"local"}
{"msg":"storage mode","mode":"local"}
{"msg":"job queue started","import_workers":8,"preview_workers":8}
{"msg":"starting server","addr":":8080"}
```

Everything else is in [`docs/observability.md`](observability.md).
