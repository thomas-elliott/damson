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

All workflows go through [just](https://just.systems) (`justfile`, plus modules in `tools/`) — do not invoke `go`, `pnpm` or `docker` directly for build/test/lint operations. Run `just` to list everything. Names follow a canonical vocabulary (`build`, `test`, `lint`, `format`, `check`, `clean`, `dev`, `install`), with the backend/frontend split as the second word (`lint backend` / `lint frontend`) — use the umbrella when in doubt.

**`just check` is the full preflight, not a quick one.** Run `just check` before pushing — it is what CI gates on.

Run one Go test — `just test backend` sets `DAMSON_TEST_POSTGRES_DSN` for you, defaulting to the `damson_test` database on the compose Postgres (`:55432`), which it starts if needed:

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

## Running the app

`just dev` needs no `.env`. It starts the compose Postgres, seeds a throwaway local admin — `dev@example.com` / `password1` — and serves the API on `127.0.0.1:8080` and the SPA on <http://localhost:5173>, which proxies `/api` to it. While it is up, `just seed` loads three sample packs into the admin's library. Anything set in `.env` or the environment wins over these defaults. `just db reset` empties both databases.

Driving it as an agent:

1. Start `just dev` as a background process, then poll until `curl -sf localhost:5173/api/health` succeeds. The first run also installs air and the web packages.
2. Sign in through the SPA with the dev admin. For the API, `POST /api/auth/login` with `{"email","password"}` returns a bearer token, which `dmsn --token` also accepts.
3. Look at the UI with the Playwright in `web/node_modules`; `just install` fetches its Chromium.
4. Stop it by killing the whole process group. Killing only `just` leaves Vite holding :5173.

`just test e2e` uses its own servers on 5174/8081 and the `damson_test` database, so it can run while `just dev` is up.

Docker installed as a snap cannot see `/tmp`, so a worktree there cannot start the compose Postgres; keep worktrees under the home directory.

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
