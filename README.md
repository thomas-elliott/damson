# Damson

Self-hosted digital asset manager for game-development assets — licence
tracking, Unity package extraction, versioning, per-library workspaces.

Status: pre-alpha. Single Go binary serves an embedded React SPA backed
by Postgres and an S3-compatible object store (or a local filesystem).

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — architecture, data
  model, API surface, shipped decisions
- [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) — coding standards,
  handler patterns, test helpers
- [`docs/design-system.md`](docs/design-system.md) — frontend design
  system spec
- [`docs/progress.md`](docs/progress.md) — what has landed, slice by slice
- [`docs/specs/`](docs/specs/) — signed-off specs; post-MVP work is
  spec-driven and each file records the *why* behind one slice
- [`docs/observability.md`](docs/observability.md) — logging shape and
  LogQL queries
- [`docs/e2e-flows.md`](docs/e2e-flows.md) — Playwright coverage
- [`docs/archive/`](docs/archive/) — original design documents and
  point-in-time research, kept for historical reference (no longer
  maintained against the code)
- [`AGENTS.md`](AGENTS.md) — quick-start guidance for agentic coding

## Quick start (Docker)

The smallest possible local setup: Postgres + Damson, local-mode auth,
local-filesystem storage, in-app first-admin bootstrap.

```sh
docker run -d --name damson-pg \
  -e POSTGRES_USER=damson -e POSTGRES_PASSWORD=damson \
  -e POSTGRES_DB=damson -p 5432:5432 postgres:16

docker run --rm -p 8080:8080 \
  -e DAMSON_POSTGRES_DSN='postgres://damson:damson@host.docker.internal:5432/damson?sslmode=disable' \
  -v damson-data:/data \
  ghcr.io/thomas-elliott/damson:latest
```

Visit <http://localhost:8080>; the SPA lands on `/bootstrap` so you can
create the first admin.

For OIDC and S3 see [Configuration](#configuration) below.

## Running locally (development)

### Prerequisites

- Go 1.26
- Node 24+
- [pnpm](https://pnpm.io) — frontend package manager (run `corepack enable` to
  use the version pinned in `web/package.json`)
- Docker with Compose, for the dev Postgres — or your own Postgres 14+
- [just](https://just.systems) — workflow runner
- For the Storybook test project: a Chromium install via
  `npx playwright install chromium`

### Setup

```sh
# 1. clone, then install all deps (Go modules, web deps via pnpm, Playwright browsers):
just install

# 2. dev loop — Postgres, backend and Vite SPA
just dev

# 3. in another terminal, optionally load sample packs
just seed
```

Open <http://localhost:5173> and sign in as `dev@example.com` / `password1`.

`just dev` needs no configuration. It starts Postgres from `compose.yaml` on
port 55432, applies migrations, creates that dev admin and installs
[air](https://github.com/air-verse/air) for Go hot-reload if it is missing. To
use your own Postgres or a different admin, set `DAMSON_POSTGRES_DSN` or
`DAMSON_LOCAL_ADMIN_EMAIL` / `DAMSON_LOCAL_ADMIN_PASSWORD` in a `.env` (see
`.env.example`); those win over the defaults.

`just backend` runs the backend only (no Vite). Useful when you don't need
SPA changes or you're running Vite separately via `just frontend`.

`just db up`, `down`, `reset` and `psql` manage the dev Postgres directly.

### Tests

```sh
just test backend        # Go unit tests (go test ./...)
just test frontend       # Vitest unit project (jsdom)
just test stories        # Vitest storybook project (Playwright Chromium)
just test                # Full suite: Go unit + Postgres integration + Playwright e2e
just check               # Preflight: lint + format check + go.mod tidy + SPA build + tests + e2e
```

Single Go test (the recipe supplies the Postgres DSN):

```sh
just test backend ./internal/handlers/api -run '^TestName$'
just test backend ./internal/handlers/api -run '^TestName$/SubtestName$'
```

Single Vitest file:

```sh
just test frontend src/path/to/file.test.tsx
```

### Build

```sh
just build           # SPA + Go binary (CGO_ENABLED=0) → ./damson
just image-build     # multi-stage container image → damson:local
```

## Configuration

Config is loaded from environment variables once at startup
(`internal/config/config.go`). A `.env` in the repo root is
auto-loaded; real env vars win on conflict. Missing required values
panic at startup. See [`.env.example`](.env.example) for the full set.

The only **always**-required value is `DAMSON_POSTGRES_DSN`. The auth
and storage modes default to `local`, which is the zero-config path.

Common knobs:

| Var | Default | Notes |
|---|---|---|
| `DAMSON_POSTGRES_DSN` | — | Required. Or `..._FILE=/path` for Docker Secrets. |
| `DAMSON_LISTEN_ADDR` | `:8080` | |
| `DAMSON_BASE_URL` | `http://localhost:8080` | Used for OIDC redirects and local-mode JWT `iss`/`aud`. |
| `DAMSON_AUTH_MODE` | `local` | `local` (bcrypt + JWT) or `oidc`. |
| `DAMSON_STORAGE_MODE` | `local` | `local` (filesystem) or `s3`. |
| `DAMSON_LOG_LEVEL` | `info` | |

Secrets accept a `<NAME>_FILE` variant that reads from a path — for
Docker Secrets / Kubernetes secrets. Setting both the direct var and
the `_FILE` form panics at startup.

For mode-specific variables (OIDC, S3, local-mode bootstrap, JWT
signing key) see the comments in `.env.example`.

### First-run flow

1. Start Damson with at least `DAMSON_POSTGRES_DSN` set. Migrations run
   inside a Postgres advisory lock.
2. First browser hit → `/api/auth/mode` returns
   `{ mode: "local", needs_bootstrap: true }`. The SPA renders
   `/bootstrap`.
3. Submit email + password. The server runs `BootstrapFirstAdmin`:
   creates the user as `sysadmin`, creates a `"Personal"` library,
   grants admin membership, sets `current_library_id`, returns a JWT.
4. Subsequent users sign in but land on "no library access yet" until
   a library admin invites them.

Alternatives:

- **Env seed.** Set both `DAMSON_LOCAL_ADMIN_EMAIL` and
  `DAMSON_LOCAL_ADMIN_PASSWORD` — first-admin creation runs at
  startup, the bootstrap page is skipped.
- **OIDC.** `DAMSON_AUTH_MODE=oidc` — the first user to sign in on an
  empty system runs the same bootstrap transaction (no admin email list
  needed). If the IdP sends no email claim, that user is prompted to add
  one on first login.

## Layout

- `cmd/server/` — binary entry point
- `internal/` — backend packages (`auth`, `handlers/api`, `models`,
  `services/{audit,extract,licence,storage,thumbnail}`, …)
- `database/migrations/` — `NNN_*.sql` migrations applied at startup
- `web/` — React SPA (Vite build output at `web/static/`)
- `docs/` — architecture, conventions, design system, progress
- `damson.go` — module-root file holding the `//go:embed` directives
  for migrations and SPA assets. Lives at the root because
  `//go:embed` cannot traverse upward from `internal/`.
- `web/static/.gitkeep` — placeholder that lets
  `//go:embed all:web/static` succeed before the SPA has been built.
  Don't delete it; don't commit anything else into `web/static/`.

## Releases

CI runs on [Woodpecker](https://woodpecker-ci.org) (`.woodpecker.yml`):

- **push to `main`** — tests → build & push
  `ghcr.io/thomas-elliott/damson:main-<sha>` + `:latest` → bump the `damson-test`
  gitops deployment.
- **tag `v*.*.*`** — promotes the already-built, already-tested `main-<sha>`
  image to the `{version}`, `{major}.{minor}`, and `{major}` tags (a retag, no
  rebuild) → bump the `damson` (prod) gitops deployment.

Locally, `just check` runs the same preflight (lint + format check + go.mod tidy +
SPA build + unit tests + e2e) before you push.
