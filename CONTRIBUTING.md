# Contributing to Damson

Damson is pre-alpha: the code moves and the conventions are written down, so a change that follows them is far easier to take than one that is merely correct.

This file is the short version. The long version is in [`AGENTS.md`](AGENTS.md) and [`docs/`](docs/); read the section for whatever you are touching first. To run Damson rather than change it, see [`docs/getting-started.md`](docs/getting-started.md).

## Getting a dev environment up

### Prerequisites

| Tool | Version | Why |
|---|---|---|
| [Go](https://go.dev) | 1.26 | the backend |
| [Node](https://nodejs.org) | 24+ | the SPA |
| [pnpm](https://pnpm.io) | 12.6.0 | frontend package manager; `corepack enable` picks up the version pinned in `web/package.json` |
| [just](https://just.systems) | 1.52.0+ | every workflow in this repo goes through it |
| Postgres client tools | any | `psql`, `createdb` and `dropdb`, which the database recipes call |
| A Postgres server | 16+ | any server where your role can create databases; see Setup |

The `just` minimum is a hard one. `tools/release.just` uses `set default-list`, which arrived in just 1.52.0. An older `just` fails to parse the justfile, so every command, including `just install`, dies with `Unknown setting 'default-list'`.

`mise.toml` pins all four for [mise](https://mise.jdx.dev) users, so `mise install` gets the right versions. It pins `just` to `latest`, which clears the minimum.

### Setup

The recipes read the standard libpq variables `PGHOST`, `PGPORT`, `PGUSER`, `PGSSLMODE` and `PGDATABASE`, with the password in `~/.pgpass` or `PGPASSWORD`. `PGDATABASE` is only where the recipes connect to create and drop databases. Set them in your shell, or copy [`.env.example`](.env.example) to `.env` (git-ignored), which the recipes load; a variable already in your environment wins. A recipe that needs one and finds it unset fails and names it.

Any Postgres 16 or later will do. For a throwaway one in Docker:

```sh
docker run -d --name damson-pg -p 127.0.0.1:5432:5432 -e POSTGRES_PASSWORD=damson postgres:16-alpine
```

and in `.env`:

```sh
PGHOST=127.0.0.1
PGPORT=5432
PGUSER=postgres
PGPASSWORD=damson
PGSSLMODE=disable
PGDATABASE=postgres
```

Then:

```sh
just install   # Go modules, pnpm packages, Playwright's Chromium
just dev       # the Go API and the Vite SPA together
```

Open <http://localhost:5173> and sign in as `dev@example.com` / `password1`.

`just dev` creates and migrates your own dev database, `damson_dev_<user>` (or `DEV_DATABASE`), and seeds that throwaway local admin. It installs [air](https://github.com/air-verse/air) for Go hot-reload if it is missing, serves the API on `127.0.0.1:$PORT` (default 8080), and runs Vite on `$WEB_PORT` (default 5173), proxying `/api` to `PORT`.

To sign in through your own identity provider instead, set `DAMSON_OIDC_ISSUER_URL` and `DAMSON_OIDC_CLIENT_ID` in `.env`. The client must be a public PKCE client, with no secret, that accepts `/auth/callback` on `http://localhost:*`. The first person to sign in becomes the sysadmin.

If `AWS_ENDPOINT_URL_S3`, `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `E2E_S3_BUCKET` are all set, files go to that S3-compatible bucket under `damson/<dev database>/`; otherwise they stay in `./data`.

With `just dev` up, `just seed` loads three sample packs into the admin's library and writes about 300 generated reference assets straight into Postgres. A second run adds nothing. In OIDC mode, sign in once first.

Other recipes:

```sh
just backend         # the Go API alone (air hot-reload)
just frontend        # the Vite SPA alone
just storybook       # the design system on :6006
just db reset        # empty your dev database
just db psql         # psql on your dev database
just db drop-leaked  # drop test databases a killed run left behind
```

### Everything goes through `just`

Do not run `go` or `pnpm` directly for build, test, lint or format. The recipes set up databases, DSNs and browser paths that a bare command lacks. Run `just` with no argument to list everything.

Recipe names use a fixed vocabulary (`build`, `test`, `lint`, `format`, `check`, `clean`, `dev`, `install`), with the backend/frontend split as the second word (`lint backend`, `format frontend`). When in doubt, use the umbrella recipe.

## Tests

```sh
just test backend    # Go tests, Postgres-backed
just test frontend   # Vitest unit project (jsdom)
just test e2e        # Playwright
just test            # the three above
just test stories    # every *.stories.tsx in real Chromium, with axe-core a11y checks
```

`just test` does not run `just test stories`. It runs `unit` (backend plus frontend) then `e2e`, which is why `just check` lists `just test stories` as a separate step. Stories is a separate CI job and carries the axe-core accessibility assertions, so run it after touching a component or a story, or let `just check` do it.

To narrow a run:

```sh
just test backend ./internal/handlers/api -run '^TestName$'
just test backend ./internal/handlers/api -run '^TestName$/SubtestName$'
just test frontend src/path/to/file.test.tsx
just test e2e --ui
```

`just test backend` and `just test e2e` each create a database for the run on the `PG*` server and drop it afterwards, so two checkouts never share one. There is no SQLite backend and no skip: every Go test runs against real Postgres in its own schema inside the run's database. `just test e2e` uses its own servers on 5174/8081, so it can run while `just dev` is up.

## Before you push

```sh
just check
```

This is the full preflight and what CI gates on: lint, format check, `go mod tidy` drift, the SPA build (the only type check), then every test suite including Playwright. `just check fast` is the read-only subset (lint, formatting and module tidiness; no database, no browser). It is useful mid-edit, but running only `just check fast` before a push is what lets a red build through.

CI runs four jobs on every pull request: `go`, `web`, `stories` and `e2e` (see [`.github/workflows/ci.yml`](.github/workflows/ci.yml)). A failed `e2e` job uploads its Playwright report as an artifact.

- Run `just check tidy` after adding a Go import. CI runs `go mod tidy` and fails on any `go.mod` / `go.sum` drift.
- Run `just format` before committing. The format check is a gate.

## Conventions

The full rules are in [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) and the system's shape is in [`docs/architecture.md`](docs/architecture.md). Read the section for the area you are changing. The ones that cause the most trouble:

Backend:

- A 500 always returns the generic `"internal error"`. Never leak internals. Respond through the helpers in `internal/handlers/api/helpers.go` (`respond400`, `respond404`, `respondForbidden`, `respond500`, `respondGORM`). Only `respond400` and `respond500` attach the cause via `c.Error`, which puts `err` on the request log line.
- Every library-scoped query filters on `library_id`. Child rows of an asset are scoped by joining to `assets`. Library-scoped routes go in the `scoped` group behind `h.requireLibrary()`, which 403s and never 404s.
- A schema change goes in both a new SQL migration and the GORM struct. Migrations are SQL-only, `NNN_descriptive_name.sql` in `database/migrations/`, wrapped in `BEGIN; … COMMIT;`, idempotent. A `default:` tag makes GORM omit a zero value on Create, so writing `false` to a default-true column needs an explicit `Update`.
- Never log tokens or raw session values, in `slog` output or in an error wrap.

Frontend:

- Never call `fetch` directly from a component. Use `fetchJson` / `fetchMaybeJson` in `src/api.ts` so bearer-token attachment, 401 renewal and `ApiError` shape stay uniform.
- Define query keys in `lib/queryKeys.ts` only. Data-fetching hooks wrap TanStack Query in `hooks/useDataQueries.ts` and return the normalized `{ data, loading, error, reload }` shape.
- Delete the palette `shadcn add` appends to `globals.css`. It is added as an unlayered `:root` / `.dark` that beats `tokens.css` and silently repaints the app. `just lint frontend` reports it if you forget.
- Read [`docs/design-system.md`](docs/design-system.md) before touching a token, a primitive, or the imports in `globals.css`. New components go in `src/design-system/{primitives,compositions,domain}/` with a `*.stories.tsx` covering variants and states; those stories run as tests.

Both:

- Mirror existing patterns. Don't refactor unrelated code.
- When a change makes a doc untrue, update the doc in the same change.

## What a good pull request looks like here

- One change, described in the title. Commit subjects are imperative, sentence case, with no `feat:` / `fix:` prefix, and describe the behaviour rather than the files: "Draw the grid at the library's density on a cold load", "Give every test run its own database". Do the same for the PR title.
- Scoped to its stated purpose. A drive-by rename buried in a bug fix makes the fix harder to review and to revert.
- `just check` passes locally before you push.
- Tests that would have caught it. Go tests live beside the handler (`assets_test.go` next to `assets.go`). The core helpers are in `api_test.go` (`newTestHandler`, `setupRouter`, `performGET`, `requireStatus`, …) and the seed helpers sit beside the feature they seed (`seedLibrary` in `library_test.go`). A user-visible frontend change wants a Vitest test or a story; a flow worth protecting wants an e2e. [`docs/e2e-flows.md`](docs/e2e-flows.md) lists what the suite covers and what it leaves out on purpose.
- Docs updated in the same commit when the change makes one untrue.
- A description that says why. The diff shows what you changed; the description should say what you were trying to achieve, what you ruled out, and anything you are unsure about.

Do not include changes to `web/static/`. It is build output, gitignored except for the `.gitkeep` placeholder that lets `//go:embed` succeed before the SPA is built. Don't delete that file.

## Releases

This section explains the image tags. You do not need it to send a change.

A push to `main` that passes every CI job builds and pushes an image tagged `main-<sha7>` and `latest`. A release is a promotion, not a rebuild: `just release patch` (or `minor` / `major`, or `just release tag vX.Y.Z`) pushes a `vX.Y.Z` tag from an up-to-date `main`, and [`release.yml`](.github/workflows/release.yml) retags that already-tested `main-<sha7>` image as `X.Y.Z`, `X.Y` and `X`. It runs no tests and builds nothing, so a released image is byte-for-byte the one that passed CI, and a commit CI never built on `main` cannot be released.

`just release` gates nothing locally. Run `just check` first.

The version the server reports comes from the VCS stamp. The git tags are the source of truth.

## Reporting a bug or proposing a feature

Open an issue. The templates under [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/) ask for what is usually needed. For a bug, include the version (the image tag, or the commit you built), the auth and storage modes, and the relevant log lines. Damson logs JSON; [`docs/observability.md`](docs/observability.md) explains the shape and how to correlate a request.

If you found something security-sensitive, please raise it privately with the maintainer rather than in a public issue.
