# AGENTS.md

`CLAUDE.md` is a symlink to this file — edit `AGENTS.md`.

/implement commits per stage in this repo

## Docs

`docs/` explains the repo to contributors and their agents. It describes the code as it stands.

- `docs/architecture.md`: the shape of the system, the ideas the code assumes, and the traps.
  Read the section for the area you are changing.
- `docs/CONVENTIONS.md`: formatting, handler patterns, migrations and test helpers.
- `docs/design-system.md`: tokens, primitives and their traps. Read it before touching a token, a
  primitive or the imports in `globals.css`.
- `docs/observability.md`: log shape, request correlation and example queries.
- `docs/e2e-flows.md`: what the Playwright suite covers, and what it leaves out on purpose.

When a change makes a doc untrue, update the doc in the same change.

## Development commands

All workflows go through [just](https://just.systems) (`justfile`, plus modules in `tools/`) — do not invoke `go` or `pnpm` directly for build/test/lint operations. Run `just` to list everything. Names follow a canonical vocabulary (`build`, `test`, `lint`, `format`, `check`, `clean`, `dev`, `install`), with the backend/frontend split as the second word (`lint backend` / `lint frontend`) — use the umbrella when in doubt.

**`just check` is the full preflight, not a quick one.** Run `just check` before pushing — it is what CI gates on.

Run one Go test — `just test backend` creates a database for the run on the `PG*` server, points `PGDATABASE` at it and drops it afterwards:

```
just test backend ./internal/handlers/api -run '^TestName$'
just test backend ./internal/handlers/api -run '^TestName$/SubtestName$'
```

Build the CLI (`dmsn`):

```
just build cli
```

Run `dmsn setup` from the folder you work on the library from, not the damson root — it writes `./.claude/skills/damson*` into the current folder.

Vitest single-file:

```
just test frontend src/path/to/file.test.tsx
```

## Environment

There is no Docker and no 1Password in the dev or test path. `mise install` then `just install` is the whole setup. Everything else comes from the environment every dev-box shell already loads, plus a gitignored `.env` at the checkout root that the recipes load (`set dotenv-load`); a variable already in the environment wins over `.env`. The environment holds test-only credentials for the shared dev services; never print or commit them. `.env` holds no secrets on the dev box, only the sign-in settings below, and you may read and repair it. Never commit it: this repository is public, and it names the maintainer's homelab. `.env.example` lists what it can hold.

- **Postgres:** `PGHOST`, `PGPORT`, `PGUSER`, `PGSSLMODE` and `PGDATABASE`, with the password in `~/.pgpass` (`PGPASSWORD` in CI). Recipes and the test harness build URLs from these and leave the password out; pgx and psql read it themselves. A missing variable fails with a message naming it. There is no `localhost` default and no skip. The recipes use `psql`, `createdb` and `dropdb` from the system.
- **Databases:** `just dev` uses `damson_dev_<user>`, or `DEV_DATABASE`, created if missing and migrated on boot. Each `just test backend` and `just test e2e` run creates `damson_test_<user>_<random>` and drops it on exit; the Go suite gives each test its own schema inside it. `just db drop-leaked` drops the ones a killed run left, but only those your `PGUSER` owns. `just db reset` empties your dev database, and `just db psql` opens it.
- **Storage:** when `AWS_ENDPOINT_URL_S3`, `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `E2E_S3_BUCKET` are set, `just dev` stores files in that bucket under `damson/<dev database>/`, with path-style addressing. With none of them set it uses `./data`; with only some it fails and names the missing ones. The e2e server always uses a temporary directory.
- **Ports:** `PORT` (API, default 8080) and `WEB_PORT` (Vite, default 5173). Vite's proxy reads `PORT`, and Vite fails rather than moving to another port.
- **Sign-in:** without OIDC settings, `just dev` runs in local mode and seeds a throwaway admin, `dev@example.com` / `password1`. When `DAMSON_OIDC_ISSUER_URL` and `DAMSON_OIDC_CLIENT_ID` are set, it runs in OIDC mode instead. On the dev box they are set in `.env` and name the homelab-dev Zitadel client, a public PKCE client; the factory copies that `.env` into each worktree. The client accepts `/auth/callback` on `http://localhost:*`, `http://127.0.0.1:*` and the dev box's `ts.net` address on any port, and the SPA builds its callback from the address the browser used. So sign-in also works when the operator opens `just dev` through `tailscale serve --https=<port> http://127.0.0.1:$WEB_PORT`; Vite already allows `.ts.net` hosts. Pick a tailscale port other than `WEB_PORT`, because tailscale binds that number on the tailnet address and Vite's `strictPort` then refuses to start. Prove sign-in on `localhost`. In OIDC mode, agents sign in as `agent-e2e` with `E2E_OIDC_USERNAME` and `E2E_OIDC_PASSWORD`, and the first person to sign in to a dev database becomes its sysadmin. The e2e server keeps local sign-in with its own seeded users.

## Running the app

`just dev` serves the API on `127.0.0.1:$PORT` and the SPA on `http://localhost:$WEB_PORT`, which proxies `/api` to it. While it is up (in OIDC mode, after you have signed in once), `just seed` loads three sample packs into the sysadmin's current library. It then writes about 300 generated reference assets (paths, store links or both, with made-up store ids, tagged with store-style categories two to four levels deep) straight into the dev Postgres. It signs its uploads with a personal access token it creates in the database and deletes afterwards. A second run adds nothing.

Driving it as an agent:

1. Start `just dev` as a background process, with `PORT` and `WEB_PORT` set if the defaults are taken. Poll until `curl -sf localhost:$WEB_PORT/api/health` succeeds. The first run also installs air and the web packages.
2. Open `http://localhost:$WEB_PORT` with the Playwright in `web/node_modules` (`just install` fetches its Chromium) and sign in: through Zitadel as `agent-e2e` in OIDC mode, or as the dev admin in local mode. For the API, create a personal access token in Settings, which `dmsn --token` also accepts; in local mode, `POST /api/auth/login` with the dev admin's `{"email","password"}` also returns a bearer token.
3. Start it under `setsid` and stop it with `pkill -s <session id>`. air and pnpm each start their own process group, so killing `just` or its group leaves the API and Vite holding their ports.

`just test e2e` uses its own servers on 5174/8081 (`E2E_PORT`, `E2E_BACKEND_PORT`) and its own database, so it can run while `just dev` is up.

## Conventions

The full rules live in `docs/CONVENTIONS.md`. Load-bearing ones worth keeping in mind while editing:

- **500 always returns the generic `"internal error"`** — never leak internals. Status mapping is in `docs/CONVENTIONS.md`, Error responses, and the transaction sentinels are in `docs/architecture.md`, Request lifecycle.
- **Delete the palette `shadcn add` appends to `globals.css`.** It lands as an unlayered `:root` / `.dark` that beats `tokens.css` and silently repaints the app; `just lint frontend` will tell you if you forget.
- **Frontend API discipline.** Never call `fetch` directly from components — go through `fetchJson` / `fetchMaybeJson` in `src/api.ts` so bearer-token attachment, 401 renewal, and `ApiError` shape stay uniform. Query keys live in `lib/queryKeys.ts` only. Data-fetching hooks wrap TanStack Query in `hooks/useDataQueries.ts` and return the normalized `{ data, loading, error, reload }` shape (`useAuthMode` excepted).
- **Never log tokens or raw session values.** Applies to `slog` debug output, error wraps, anything that might end up in a log collector.
- **Mirror existing patterns; don't refactor unrelated code.** Focused changes only.
- **Run `just check tidy` after adding a Go import** — CI fails on any `go.mod`/`go.sum` drift.

## Architecture

- **Every library-scoped query filters on `library_id`** — top-level entities carry it directly; child rows of an asset (versions, files, imports, images, licences) are scoped by joining to `assets` and filtering `assets.library_id`. Library-scoped routes go in the `scoped` group behind `h.requireLibrary()`, which 403s and never 404s.
- **A schema change lands in both a new SQL migration and the GORM struct.** A `default:` tag makes GORM omit a zero value on Create, so writing `false` to a default-true column needs an explicit `Update`.
- Writing a migration → `docs/CONVENTIONS.md`, Database Migrations.
- Touching auth, storage, the import pipeline, River jobs or the SPA auth flow → read the matching section of `docs/architecture.md` first.
- Environment variables → `internal/config/config.go`: every var, its default, and the `_FILE` secret variants.
