# Configuration reference

Every setting Damson reads from its environment, what it does, its default, and
whether it is required. For getting a server up in the first place, start at
[`docs/getting-started.md`](getting-started.md).

## How configuration is loaded

Configuration is read **once, at startup**, from environment variables
(`internal/config/config.go`, plus the logging variables in
`cmd/server/main.go`). Nothing re-reads it later, so a change needs a restart.

A `.env` file in the server's working directory is loaded if present, but real
environment variables always win on a conflict. In the container image the
working directory is `/`, so that file would be `/.env` — in practice, set the
environment directly.

Damson reports **every** configuration problem at once and then exits 1. A
missing required value, an unparseable duration, a mode that is not one of the
allowed words — each gets its own log line naming the variable, so you fix them
in one pass rather than one per restart.

The only variable that is always required is `DAMSON_POSTGRES_DSN`. Auth mode
and storage mode both default to `local`, which is the zero-configuration path.
Some variables become required once you select a non-default mode; those are
marked below.

### Secrets and `_FILE`

Five variables accept a `<NAME>_FILE` companion that reads the value from a
file path instead, for Docker secrets and Kubernetes secret mounts. The file's
trailing newline is stripped:

- `DAMSON_POSTGRES_DSN_FILE`
- `DAMSON_JWT_SIGNING_KEY_FILE`
- `DAMSON_LOCAL_ADMIN_PASSWORD_FILE`
- `DAMSON_S3_ACCESS_KEY_FILE`
- `DAMSON_S3_SECRET_KEY_FILE`

Only those five. Adding `_FILE` to any other variable does nothing — it is
read as an unrelated, ignored variable, not as a secret source.

Set the direct variable **or** its `_FILE` companion, never both: doing both is
a startup error. A `_FILE` path that cannot be read is also a startup error,
naming the path.

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
| `DAMSON_POSTGRES_DSN` | — | **Yes** | Postgres connection string, e.g. `postgres://damson:damson@postgres:5432/damson?sslmode=disable`. Also used by the River job queue, which opens its own pool against the same DSN. Supports `_FILE`. |
| `DAMSON_LISTEN_ADDR` | `:8080` | No | Address the HTTP server binds, in Go `host:port` form. `127.0.0.1:8080` binds loopback only. |
| `DAMSON_BASE_URL` | `http://localhost:8080` | No | See the warning below — it is **not** a general "public URL" setting. |
| `DAMSON_TRUSTED_PROXIES` | empty | No | Comma-separated IPs or CIDRs whose `X-Forwarded-For` is believed. Blank trusts nothing. An unparseable entry fails startup. |
| `GIN_MODE` | `debug` | No | `debug`, `release` or `test`. Set `release` in production to silence Gin's startup banner and per-route registration output. See the warning below. |

### `DAMSON_BASE_URL` is the JWT issuer, nothing else

Despite the name, this value is used in exactly one place: it is the `iss` and
`aud` claim of the JWTs that local auth mode mints, and the value those tokens
are verified against. It is **not** consulted for OIDC redirect URIs (the
browser derives those from its own origin), it is not used to build links, and
it is not checked against the host in the request.

Two consequences:

- You do not have to set it to your real public URL for things to work. It just
  has to be stable.
- **Changing it signs every local-mode user out.** Existing tokens carry the
  old issuer and fail verification, so everyone gets a 401 and has to log in
  again. Harmless, but surprising mid-day.

It has no effect at all in OIDC mode.

### `GIN_MODE` is the one variable that can crash the process

`GIN_MODE` belongs to the web framework, not to Damson, and Gin reads it
itself when its package initialises — before Damson's own configuration
loading runs. A value that is not `debug`, `release` or `test` therefore
**panics the process on startup**, with a raw Go panic (`gin mode unknown:
...`) rather than one of the tidy `config:` error lines every other variable
produces. If a container is crash-looping with a stack trace instead of a log
line, check this first.

### `DAMSON_TRUSTED_PROXIES` and client IPs

Damson throttles login and bootstrap attempts per client IP. With this unset,
the client IP is the TCP peer's address — which, behind a reverse proxy, is the
proxy. Every user then shares one throttle bucket.

Set it to your proxy's address so `X-Forwarded-For` is honoured:

```
DAMSON_TRUSTED_PROXIES=10.0.0.0/8,172.18.0.0/16
```

Do not set it to `0.0.0.0/0`. That trusts a header any client can forge, which
lets a single attacker spread a password-guessing run across unlimited fake
IPs and makes the throttle useless.

## Logging

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_LOG_LEVEL` | `info` | No | One of `debug`, `info`, `warn`, `error`. Anything else fails startup. |
| `DAMSON_LOG_FORMAT` | `json` | No | `json` or `text`. Anything else fails startup. `text` is for reading in a terminal; keep `json` for a log collector. |

Both are read in `cmd/server/main.go` rather than the config package, so they
are applied before configuration is parsed — a config error is logged in the
format you asked for. Every line carries `service=damson` and a `version` field
derived from the build stamp. The log shape, request correlation IDs and some
queries worth having are in [`docs/observability.md`](observability.md).

## Authentication

`DAMSON_AUTH_MODE` picks one authenticator for the whole deployment. It cannot
be changed at runtime and there is no mixed mode.

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_AUTH_MODE` | `local` | No | `local` or `oidc`. Any other value fails startup. |

Personal access tokens (`damson_pat_…`, created under **Settings → Access
tokens**) work in either mode, so scripts and the `dmsn` CLI do not care which
you chose.

### Local mode (the default)

Bcrypt passwords and HS256 JWTs, issued by Damson itself. Tokens last one hour
and the web UI renews them silently before they expire. This is the
zero-configuration path: set `DAMSON_POSTGRES_DSN` and nothing else and you get
working local auth.

It is credential handling at development-and-small-team grade. For a
deployment where account security genuinely matters, prefer OIDC.

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_JWT_SIGNING_KEY` | auto-generated | No | The HS256 signing key, used verbatim. **Must be at least 32 bytes** or startup fails. Supports `_FILE`. |
| `DAMSON_JWT_KEY_FILE` | `./data/jwt.key` | No | Where the auto-generated key is kept, when `DAMSON_JWT_SIGNING_KEY` is unset. |
| `DAMSON_LOCAL_ADMIN_EMAIL` | unset | No | With the password below, seeds the first admin at startup. |
| `DAMSON_LOCAL_ADMIN_PASSWORD` | unset | No | Partner of the above. At least 8 characters. Supports `_FILE`. |

**The signing key.** If `DAMSON_JWT_SIGNING_KEY` is not set, the server reads
`DAMSON_JWT_KEY_FILE`; if that file does not exist either, it generates 32
random bytes and writes them there with `0600` permissions. That is why the
zero-config path works, and why the key file's directory must be writable on
first boot. In the container image `./data/jwt.key` resolves to
`/data/jwt.key`.

Either shape is safe. Setting the variable directly is the better fit for a
config-managed or multi-replica deployment, since every replica then signs with
the same key without sharing a volume; the file is the better fit for a single
container with a volume you already back up. An existing key file is only read,
never rewritten — and a key file shorter than 32 bytes is a startup error, not
a silent regeneration.

Rotating the key, by either route, invalidates every issued session.

**Seeding the first admin.** Set `DAMSON_LOCAL_ADMIN_EMAIL` *and*
`DAMSON_LOCAL_ADMIN_PASSWORD` and the first admin is created during startup,
bypassing the in-app bootstrap page. Set only one and nothing happens — no
error, no admin. The seed runs only when the database has no users at all;
once an admin exists, changing these values does **not** rotate their password
(that is an admin action in the UI). Leave both unset and the first browser
visit lands on `/bootstrap` instead.

Either way the resulting account becomes `sysadmin` and gets a library named
"Personal".

### OIDC mode

Set `DAMSON_AUTH_MODE=oidc` and Damson verifies id_tokens from your identity
provider instead of issuing its own. The local login form and the
`/api/auth/login`, `/api/auth/bootstrap` and `/api/auth/refresh` endpoints are
not registered at all in this mode; session renewal is the IdP's silent-renew
flow.

All three of these are required once OIDC mode is selected, and an empty one
fails startup:

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_OIDC_ISSUER_URL` | — | **In OIDC mode** | Your IdP's issuer URL, e.g. `https://auth.example.com/realms/studio`. Damson fetches the discovery document from it at startup. |
| `DAMSON_OIDC_CLIENT_ID` | — | **In OIDC mode** | The public client id you registered for Damson. |
| `DAMSON_OIDC_AUDIENCE` | — | **In OIDC mode** | The audience claim Damson requires on an incoming id_token. Usually the same string as the client id. |

Points worth knowing before you wire this up:

**The discovery document is fetched during startup.** If the IdP is
unreachable, Damson fails to start with an `oidc init` error rather than
starting degraded. Order your service dependencies accordingly.

**There is no admin allow-list.** As in local mode, the first person to sign in
on an empty system becomes the sysadmin and gets a "Personal" library. Everyone
after that signs in successfully but has access to nothing until a library
admin adds them under **Settings → Members**. That is the intended shape — if
your whole company can reach the IdP, you do not want the second person through
the door inheriting the first one's library.

**Register Damson as a public client.** There is no client secret to configure,
because the token exchange happens in the browser. Redirect URIs are derived
from the browser's own origin rather than from any Damson setting, so register

```
https://damson.example.com/auth/callback
```

— substituting the origin your users actually type — as the redirect URI. The
same URI is also the silent-renew target, loaded in a hidden iframe, so your
IdP has to permit `prompt=none` renewal and allow that page to be framed. You
do **not** need a post-logout redirect URI: signing out of Damson clears the
browser's stored session locally and never calls the IdP's `end_session`
endpoint, so you stay signed in at the IdP.

The client requests the scopes `openid profile email`. Damson verifies the
**id_token** (not the access token) and reads the `email`, `email_verified`
and `name` claims from it, so make sure your IdP puts those in the id_token
rather than only on the userinfo endpoint.

**Identities are matched on `(issuer, subject)` first.** If that pair is
unknown, Damson falls back to matching a *verified* email and links the new
identity to that existing account; otherwise it creates a new user. An
unverified email is never used for linking, so a user whose IdP sends no email
claim lands on a "finish your profile" page and sets one in Damson. After the
first sign-in, Damson owns that user's email and display name — later changes
at the IdP do not propagate.

**Changing issuer later is disruptive.** Existing `user_identities` rows are
keyed on the old issuer string, so moving IdPs (or changing a realm path)
orphans every account. Plan it as a migration, not a config edit.

The issuer's `scheme://host` is automatically added to the Content-Security
-Policy so the browser can reach your IdP for discovery, token exchange and
the hidden silent-renew iframe. You do not configure the CSP yourself.

## Storage

`DAMSON_STORAGE_MODE` picks the blob backend. Both backends use the same key
layout (`blobs/`, `originals/` and `thumbs/`, each addressed by the SHA-256 of
its contents), so moving between them is a plain recursive copy plus a config
change — no re-import, no database migration.

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_STORAGE_MODE` | `local` | No | `local` or `s3`. Any other value fails startup. |

### Local mode (the default)

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_STORAGE_LOCAL_PATH` | `./data/blobs` | No | Directory that holds every stored object. |

Relative to the server's working directory, which is `/` in the container
image — so the default resolves to `/data/blobs` there, alongside the JWT key.

Damson creates the directory if it is missing and **proves it can write a file
there before it finishes starting**, so a volume that was mounted read-only or
with the wrong owner fails loudly at boot, naming the directory, instead of
failing on the first upload.

One real limitation: **do not run more than one replica against local
storage.** Resumable chunked uploads keep their parts on the local disk of
whichever process received them, so a second replica will not find them. Local
mode is for single-instance deployments.

### S3 mode

For MinIO, Garage, AWS S3, or anything else speaking the S3 API. All five of
these are required once S3 mode is selected:

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_S3_ENDPOINT` | — | **In S3 mode** | Full endpoint URL, e.g. `http://minio:9000`. Required even on real AWS. |
| `DAMSON_S3_REGION` | — | **In S3 mode** | Region name. MinIO and Garage do not care, but the SDK requires one — `us-east-1` is the conventional filler. |
| `DAMSON_S3_BUCKET` | — | **In S3 mode** | Bucket name. See the warning below. |
| `DAMSON_S3_ACCESS_KEY` | — | **In S3 mode** | Access key id. Supports `_FILE`. |
| `DAMSON_S3_SECRET_KEY` | — | **In S3 mode** | Secret access key. Supports `_FILE`. |
| `DAMSON_S3_FORCE_PATH_STYLE` | `true` | No | `true` sends `endpoint/bucket/key` (what MinIO and Garage want). Set `false` for virtual-host-style addressing, which is what real AWS S3 expects. |

**Create the bucket yourself first.** Damson does not create it, and does not
check that it exists at startup — the credentials are only exercised on the
first object read or write. A wrong bucket name, a wrong key, or a missing
bucket therefore shows up as a failed import rather than a failed boot. Do a
test upload after any change here.

Credentials are supplied explicitly and are not optional: there is no fallback
to the ambient AWS credential chain, so instance roles, `~/.aws/credentials`
and `AWS_*` environment variables are all ignored. You must set
`DAMSON_S3_ACCESS_KEY` and `DAMSON_S3_SECRET_KEY`.

Note that `DAMSON_STORAGE_LOCAL_PATH` is not read in S3 mode and
`DAMSON_JWT_KEY_FILE` still is — in local auth mode with S3 storage, the
container still needs one small writable path for the signing key. Set
`DAMSON_JWT_SIGNING_KEY` directly and it needs no writable disk at all.

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
Postgres, so they survive a restart; anything left unfinished is re-enqueued at
the next boot. The defaults are sensible and most deployments never touch these.

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_IMPORT_QUEUE_WORKERS` | number of CPUs | No | Concurrent import jobs (extract, thumbnail, enrich). Must be a whole number above zero. |
| `DAMSON_PREVIEW_QUEUE_WORKERS` | number of CPUs | No | Concurrent preview-rendering jobs. Must be a whole number above zero. |
| `DAMSON_IMPORT_JOB_TIMEOUT` | `30m` | No | How long one import job may run before its context is cancelled. Go duration string. |
| `DAMSON_PREVIEW_JOB_TIMEOUT` | `30m` | No | Same, for one preview job. |
| `DAMSON_STORE_LOOKUP_INTERVAL` | `2s` | No | Minimum gap between two requests to the same storefront, across all callers. |

Lower the worker counts on a small box: both queues default to one worker per
CPU, and extraction is both CPU- and IO-hungry. Raise
`DAMSON_IMPORT_JOB_TIMEOUT` if you import multi-gigabyte packages over slow
storage. Treat `DAMSON_STORE_LOOKUP_INTERVAL` as a politeness floor toward
storefronts you do not own; lowering it risks getting rate-limited or blocked.

Durations are Go duration strings — `30m`, `24h`, `90s`, `1h30m`. A value that
does not parse, or that is zero or negative, fails startup.

## Maintenance intervals

| Variable | Default | Required | What it does |
|---|---|---|---|
| `DAMSON_AUDIT_PURGE_INTERVAL` | `24h` | No | How often the retention sweeper deletes audit rows older than each library's `audit_retention_days`. |
| `DAMSON_UPLOAD_JANITOR_INTERVAL` | `1h` | No | How often abandoned resumable upload sessions are expired and their backing multipart uploads aborted. |
| `DAMSON_ORPHAN_MIN_AGE` | `168h` (7 days) | No | How old an object must be before the manual storage-cleanup scan will call it orphaned. |

Audit retention itself is a **per-library** setting, edited under **Settings →
Audit**, not an environment variable; the interval above only controls how
often the sweep runs. A library with no retention set keeps its audit rows
forever.

`DAMSON_ORPHAN_MIN_AGE` is a safety floor, not a schedule. Nothing deletes
objects automatically: a sysadmin runs a scan and then a delete under
**Settings → Storage cleanup**, and the age floor keeps the scan from flagging
an upload whose database rows have not committed yet.
Shortening it on a busy server risks deleting in-flight uploads. Note that the
floor does not protect a *reused* object — content-addressed dedupe reuses a
key without touching its modification time.

## Settings that are not environment variables

Plenty of what you might expect to configure here lives in the database
instead, so it can be changed without a restart. Two scopes:

**Server-wide** (sysadmin only, **Settings → Server settings**):

| Setting | Default | What it does |
|---|---|---|
| Max upload bytes | `68719476736` (64 GiB) | Hard ceiling on any single upload. |
| Auto-update licence knowledge base | off | Refresh the SPDX licence overlay from upstream. The UI labels this "when supported" — treat it as not yet doing anything. |

The same sysadmin screen also carries **Users** (role changes and local
password resets) and **Storage cleanup**.

**Per-library** (library admin, **Settings**):

| Setting | Default | What it does |
|---|---|---|
| Default grid density | `medium` | Starting density of the asset grid. |
| Retain original packages | on | Keep the uploaded archive after extraction. Turning it off reclaims a lot of space but makes re-extraction impossible. |
| Dedupe enabled | on | Store identical content once. |
| Raster / audio / font thumbnails | all on | Per-kind thumbnail generation for loose files. |
| Unity preview thumbnails | on | Use the preview image inside a `.unitypackage` as the asset's cover. |
| Previews enabled | on | Render the larger browser-safe preview derivatives. |
| Auto-enrich | on | Fetch storefront metadata during import. |
| Preferred store | `unity` | Which storefront to try first when enriching. |
| Audit retention days | unset (keep forever) | Age at which audit rows are purged. |

## Variables Damson does not read

A few things that look like they belong here, and do not:

- **No TLS settings.** Damson serves plain HTTP. TLS and HSTS are the reverse
  proxy's job.
- **No `PORT`.** Use `DAMSON_LISTEN_ADDR`.
- **No `DATABASE_URL`.** Use `DAMSON_POSTGRES_DSN`.
- **No `AWS_*` credentials.** S3 credentials come only from the
  `DAMSON_S3_*` variables.
- **`DAMSON_TEST_POSTGRES_DSN`** is for the Go and Playwright test suites, not
  the server.
- **`DAMSON_SERVER`, `DAMSON_TOKEN`, `DAMSON_CONFIG`** configure the `dmsn`
  CLI, not the server.

## Checking what a running server is using

Damson does not expose its resolved configuration over HTTP — deliberately,
since most of it is secret. What you can check:

```sh
curl -s localhost:8080/api/health
# {"status":"ok","version":"dev"}
```

`version` is derived from the Go build stamp. The published container image
builds without the `.git` directory, so there is no stamp and it reports
`"dev"` rather than the release it was built from — do not use this endpoint to
tell two image versions apart. Use the image digest or tag instead.

The startup log lines name the modes that were actually selected, which is
usually what you are trying to confirm:

```
{"msg":"auth mode","mode":"local"}
{"msg":"storage mode","mode":"local"}
{"msg":"job queue started","import_workers":8,"preview_workers":8}
{"msg":"starting server","addr":":8080"}
```

Everything else is in [`docs/observability.md`](observability.md).
