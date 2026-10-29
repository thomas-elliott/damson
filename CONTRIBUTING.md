# Contributing to Damson

Thanks for looking. Damson is pre-alpha, which means the code moves, the
conventions are already written down, and a change that follows them is far
easier to take than one that is merely correct.

This file is the short version for someone sending a change. The long version
lives in [`AGENTS.md`](AGENTS.md) and [`docs/`](docs/) — read the section for
whatever you are touching before you touch it. If you want to *run* Damson
rather than change it, you want [`docs/getting-started.md`](docs/getting-started.md)
instead.

## Getting a dev environment up

### Prerequisites

| Tool | Version | Why |
|---|---|---|
| [Go](https://go.dev) | 1.26 | the backend |
| [Node](https://nodejs.org) | 24+ | the SPA |
| [pnpm](https://pnpm.io) | 12.6.0 | frontend package manager — `corepack enable` picks up the version pinned in `web/package.json` |
| [just](https://just.systems) | 1.52.0+ | every workflow in this repo goes through it |
| Docker with Compose v2 | — | the dev Postgres |

The `just` floor is real, not cautious: `tools/release.just` uses `set
default-list`, which arrived in just 1.52.0. An older `just` fails to *parse*
the justfile, so every command — including `just install` below — dies with
`Unknown setting 'default-list'` before it runs anything.

`mise.toml` pins all four for [mise](https://mise.jdx.dev) users, so
`mise install` gets you the right versions in one step; `just` is pinned to
`latest` there, which clears the floor.

### Setup

```sh
just install   # Go modules, pnpm packages, Playwright browsers
just dev       # Postgres, the Go API, and the Vite SPA together
```

Then open <http://localhost:5173> and sign in as `dev@example.com` /
`password1`.

`just dev` needs no `.env` and no configuration. It starts this checkout's own
Postgres from `compose.yaml`, applies migrations, seeds that throwaway local
admin, installs [air](https://github.com/air-verse/air) for Go hot-reload if it
is missing, serves the API on `127.0.0.1:8080`, and runs Vite on `:5173`
proxying `/api` to it. Anything you set in `.env` or the environment wins over
those defaults — see [`.env.example`](.env.example).

With `just dev` up, `just seed` loads three sample packs into the dev admin's
library and writes about 300 generated reference assets straight into Postgres.
Running it a second time adds nothing.

Useful neighbours:

```sh
just backend        # the Go API alone (air hot-reload)
just frontend       # the Vite SPA alone
just storybook      # the design system on :6006
just db reset       # empty the dev and test databases
just db dsn         # print this checkout's Postgres address
```

### Each checkout gets its own Postgres

`just db` names the Compose project after a hash of the checkout's path and
lets Docker pick a free loopback port, so clones and worktrees never fight over
a database. Two consequences worth knowing:

- After deleting a clone or worktree, run `just db prune` to remove the
  Postgres and data of every checkout whose directory is gone.
- Docker installed as a snap cannot see `/tmp`, so a worktree there cannot
  start its Postgres. **Keep worktrees under your home directory.**

### Everything goes through `just`

Do not invoke `go`, `pnpm` or `docker` directly for build, test, lint or format
operations — the recipes set up databases, DSNs and browser paths that a bare
command will not. Run `just` with no argument to list everything.

Recipe names follow a fixed vocabulary — `build`, `test`, `lint`, `format`,
`check`, `clean`, `dev`, `install` — with the backend/frontend split as the
second word (`lint backend`, `format frontend`). Reach for the umbrella when in
doubt.

## Tests

```sh
just test backend    # Go tests, Postgres-backed
just test frontend   # Vitest unit project (jsdom)
just test e2e        # Playwright
just test            # the three above
just test stories    # every *.stories.tsx in real Chromium, with axe-core a11y checks
```

**`just test` does not run `just test stories`.** It is `unit` (backend plus
frontend) then `e2e`, and nothing else — which is why `just check` has to list
`just test stories` as a step of its own. Stories is a separate CI job and the
one carrying the axe-core accessibility assertions, so run it yourself after
touching a component or a story, or let `just check` do it for you.

Narrowing down:

```sh
just test backend ./internal/handlers/api -run '^TestName$'
just test backend ./internal/handlers/api -run '^TestName$/SubtestName$'
just test frontend src/path/to/file.test.tsx
just test e2e --ui
```

`just test backend` resolves `DAMSON_TEST_POSTGRES_DSN` itself and starts the
checkout's Postgres if it needs to — there is no SQLite backend, and every Go
test runs against real Postgres in its own schema. `just test e2e` uses its own
servers on 5174/8081 and the `damson_test` database, so it can run while
`just dev` is up.

## Before you push

```sh
just check
```

That is the full preflight and it is what CI gates on: lint, format check,
`go mod tidy` drift, the SPA build (which is the only type check), then every
test suite including Playwright. `just check fast` is the read-only subset —
lint, formatting and module tidiness, no database and no browser. It is useful
mid-edit, but **running only `just check fast` before a push is what lets a red
build through.**

CI runs four jobs on every pull request — `go`, `web`, `stories` and `e2e` (see
[`.github/workflows/ci.yml`](.github/workflows/ci.yml)). A failed `e2e` job
uploads its Playwright report as an artifact.

Two things that catch people out:

- **Run `just check tidy` after adding a Go import.** CI runs `go mod tidy`
  and fails on any `go.mod` / `go.sum` drift.
- **Always `just format` before committing.** No hand-formatting; the format
  check is a gate, not a suggestion.

## Conventions

The full rules are in [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) and the
system's shape is in [`docs/architecture.md`](docs/architecture.md). Read the
section for the area you are changing. The ones that bite hardest:

**Backend**

- **A 500 always returns the generic `"internal error"`.** Never leak
  internals. Respond through the helpers in `internal/handlers/api/helpers.go`
  (`respond400`, `respond404`, `respondForbidden`, `respond500`, `respondGORM`)
  — only `respond400` and `respond500` attach the cause via `c.Error`, which is
  what puts `err` on the request log line.
- **Every library-scoped query filters on `library_id`.** Child rows of an
  asset are scoped by joining to `assets`. Library-scoped routes go in the
  `scoped` group behind `h.requireLibrary()`, which 403s and never 404s.
- **A schema change lands in both a new SQL migration and the GORM struct.**
  Migrations are SQL-only, `NNN_descriptive_name.sql` in
  `database/migrations/`, wrapped in `BEGIN; … COMMIT;`, idempotent. A
  `default:` tag makes GORM omit a zero value on Create, so writing `false` to
  a default-true column needs an explicit `Update`.
- **Never log tokens or raw session values** — not in `slog` output, not in an
  error wrap.

**Frontend**

- **Never call `fetch` directly from a component.** Go through `fetchJson` /
  `fetchMaybeJson` in `src/api.ts` so bearer-token attachment, 401 renewal and
  `ApiError` shape stay uniform.
- **Query keys live in `lib/queryKeys.ts` only**, and data-fetching hooks wrap
  TanStack Query in `hooks/useDataQueries.ts`, returning the normalized
  `{ data, loading, error, reload }` shape.
- **Delete the palette `shadcn add` appends to `globals.css`.** It lands as an
  unlayered `:root` / `.dark` that beats `tokens.css` and silently repaints the
  app. `just lint frontend` will tell you if you forget.
- **Read [`docs/design-system.md`](docs/design-system.md) before touching a
  token, a primitive, or the imports in `globals.css`.** New components go in
  `src/design-system/{primitives,compositions,domain}/` alongside a
  `*.stories.tsx` covering variants and states — those stories run as tests.

**Both**

- **Mirror existing patterns. Don't refactor unrelated code.** Focused changes
  only.
- **When a change makes a doc untrue, update the doc in the same change.**

## What a good pull request looks like here

- **One change, described in the title.** Commit subjects in this repo are
  imperative, sentence case, no `feat:` / `fix:` prefix, and describe the
  behaviour rather than the files — "Draw the grid at the library's density on
  a cold load", "Give every checkout its own compose Postgres". Do the same for
  the PR title.
- **Scoped to its stated purpose.** A drive-by rename buried in a bug fix makes
  the fix harder to review and harder to revert.
- **`just check` passes locally** before you push, not after CI tells you.
- **Tests that would have caught it.** Go tests live beside the handler
  (`assets_test.go` next to `assets.go`). The core helpers are in `api_test.go`
  (`newTestHandler`, `setupRouter`, `performGET`, `requireStatus`, …) and the
  seed helpers sit beside the feature they seed (`seedLibrary` in
  `library_test.go`). A
  user-visible frontend change wants a Vitest test or a story; a flow worth
  protecting wants an e2e — see [`docs/e2e-flows.md`](docs/e2e-flows.md) for
  what the suite already covers and what it leaves out on purpose.
- **Docs updated in the same commit** when the change makes one untrue.
- **A description that says why.** What you changed is in the diff; what you
  were trying to achieve, what you ruled out, and anything you are unsure about
  are not.

Changes to `web/static/` do not belong in a PR — it is build output, gitignored
except for the `.gitkeep` placeholder that lets `//go:embed` succeed before the
SPA has been built. Don't delete that file.

## Releases

You will not need this to send a change, but it explains the image tags.

A push to `main` that passes every CI job builds and pushes an image tagged
`main-<sha7>` and `latest`. A release is a *promotion*, not a rebuild:
`just release patch` (or `minor` / `major`, or `just release tag vX.Y.Z`) pushes
a `vX.Y.Z` tag from an up-to-date `main`, and
[`release.yml`](.github/workflows/release.yml) retags that already-tested
`main-<sha7>` image as `X.Y.Z`, `X.Y` and `X`. It runs no tests and builds
nothing, so a released image is byte-for-byte the one that passed CI — and a
commit CI never built on `main` cannot be released.

`just release` gates nothing locally. Run `just check` yourself first.

The root `VERSION` file is **not** read by the build; the version the server
reports comes from the VCS stamp. Treat the git tags as the source of truth.

## Reporting a bug or proposing a feature

Open an issue — the templates under
[`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/) ask for what is usually
needed. For a bug, the version (the image tag, or the commit you built), the
auth and storage modes, and the relevant log lines save a round trip. Damson
logs JSON; [`docs/observability.md`](docs/observability.md) explains the shape
and how to correlate a request.

If you found something security-sensitive, please raise it privately with the
maintainer rather than in a public issue.
