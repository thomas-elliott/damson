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
- [`docs/progress.md`](docs/progress.md) — milestone-by-milestone
  implementation status
- [`docs/archive/`](docs/archive/) — original design documents kept for
  historical reference (no longer maintained against the code)
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
  ghcr.io/thomas-elliott/damson:main
```

Visit <http://localhost:8080>; the SPA lands on `/bootstrap` so you can
create the first admin.

For OIDC and S3 see [Configuration](#configuration) below.

## Running locally (development)

### Prerequisites

- Go 1.26
- Node 24+
- Postgres 14+ (or higher)
- [Task](https://taskfile.dev) — workflow runner
- [air](https://github.com/air-verse/air) — Go hot-reload
- For the Storybook test project: a Chromium install via
  `npx playwright install chromium`

### Setup

```sh
# 1. clone, then:
go mod tidy
task web:deps

# 2. point at a Postgres
cp .env.example .env
$EDITOR .env       # set DAMSON_POSTGRES_DSN

# 3. dev loop — backend + Vite SPA in parallel
task dev:spa
```

Migrations apply on startup. The SPA's first request will hit
`GET /api/auth/mode` and route you to `/bootstrap` — submit an email +
password and you're in.

`task dev` runs the backend only (no Vite). Useful when you don't need
SPA changes or you're running Vite separately via `task web:dev`.

### Tests

```sh
task test                # Go: go test ./...
task web:test            # Vitest unit project (jsdom)
task web:test:stories    # Vitest storybook project (Playwright Chromium)
task web:test:all        # Both Vitest projects
task pre-release-checks  # fmt + vet + go.mod tidiness + Go tests + web tests
```

Single Go test:

```sh
go test ./internal/handlers/api -run '^TestName$'
go test ./internal/handlers/api -run '^TestName$/SubtestName$'
```

Single Vitest file:

```sh
cd web && npm test -- src/path/to/file.test.tsx --run
```

### Build

```sh
task build           # SPA + Go binary (CGO_ENABLED=0) → ./damson
task docker-build    # multi-stage container image → damson:local
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
signing key) see [`docs/architecture.md`](docs/architecture.md) §13 and
the comments in `.env.example`.

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

Pushing a tag matching `v*.*.*` to `main` triggers
`.github/workflows/release.yml`, which builds and pushes the image to
`ghcr.io/<owner>/damson` with `{version}`, `{major}.{minor}`, and
`{major}` tags. `main-build.yml` builds `main-<sha>` on every push to
`main`. Locally, `task pre-release-checks` runs the same checks CI
runs.
