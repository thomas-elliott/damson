# AGENTS.md

`CLAUDE.md` is a symlink to this file — edit `AGENTS.md`.

/implement commits per stage in this repo

## Project documents

Before making non-trivial changes, read the docs in `docs/`. They describe the code as it stands today.

- `docs/architecture.md` — current architecture and data model
- `docs/CONVENTIONS.md` — formatting rules, handler patterns, test helpers
- `docs/design-system.md` — tokens, primitives and the traps. Read before touching a token, a primitive or `globals.css`'s imports
- `docs/roadmap.md` — what *isn't* built: the queue (`Next`), scoped-but-unqueued work (`Shaped`), unshaped maybes (`Ideas`). Read this before starting a new slice, and before proposing anything — it carries decisions already banked, so `/spec` confirms rather than re-decides
- `docs/progress.md` — the rejection log (Known gaps) and where the code diverges from the archived drafts. Read before proposing something, so a rejected idea is not re-analysed
- `docs/specs/` — signed-off specs, `YYYY-MM-DD-slug.md`. Post-MVP work is spec-driven: each file is the agreed scope for one slice, written before implementation and left afterwards as the record of *why* it was built that way. **The top level is what is in flight; `docs/specs/history/` is what shipped**, with its own index README — so built-vs-not-built is answerable by location without reading anything. Never rewrite a spec after the fact: a changed decision is a new dated spec — though appending a `## What implementation found` section to record where the tree disagreed with the plan is encouraged. Shipping a spec — flipping its Status, moving it to `history/` → `docs/specs/history/README.md`. **Do not write into `docs/` during a run** — record what the change made untrue in the spec and leave the docs to `/distil`
- `docs/specs/history/README.md` — the shipped-spec index: date, one line on what each did, and its PR. Read the one a doc names, never the folder
- `docs/observability.md` — logging shape, request correlation, LogQL queries
- `docs/e2e-flows.md` — Playwright coverage: what's tested, what's deliberately not
- `docs/archive/` — historical inputs, kept verbatim: the original product / technical / UI design docs (Draft v1/v2) that drove M0–M7, plus point-in-time research artefacts (preview feasibility, dependency audit). Section numbers and API shapes have drifted — prefer `architecture.md` for the live shape and consult the archive only for the *why*

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

- **500 always returns the generic `"internal error"`** — never leak internals. Status mapping → `docs/CONVENTIONS.md`, Error responses; the transaction sentinels → `docs/architecture.md`, Request lifecycle.
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
