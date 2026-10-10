# AGENTS.md

`CLAUDE.md` is a symlink to this file — edit `AGENTS.md`.

/implement commits per stage in this repo

## Project documents

Before making non-trivial changes, read the docs in `docs/`. They describe the code as it stands today.

- `docs/architecture.md` — current architecture and data model
- `docs/CONVENTIONS.md` — formatting rules, handler patterns, test helpers
- `docs/design-system.md` — tokens, primitives and the `@telliott/shared-ui` boundary. Read before touching a token, a primitive, `globals.css`'s imports, or bumping the package
- `docs/roadmap.md` — what *isn't* built: the queue (`Next`), scoped-but-unqueued work (`Shaped`), unshaped maybes (`Ideas`). Read this before starting a new slice, and before proposing anything — it carries decisions already banked, so `/spec` confirms rather than re-decides
- `docs/progress.md` — the rejection log (Known gaps) and where the code diverges from the archived drafts. Read before proposing something, so a rejected idea is not re-analysed
- `docs/specs/` — signed-off specs, `YYYY-MM-DD-slug.md`. Post-MVP work is spec-driven: each file is the agreed scope for one slice, written before implementation and left afterwards as the record of *why* it was built that way. **The top level is what is in flight; `docs/specs/history/` is what shipped**, with its own index README — so built-vs-not-built is answerable by location without reading anything. Never rewrite a spec after the fact: a changed decision is a new dated spec — though appending a `## What implementation found` section to record where the tree disagreed with the plan is encouraged. Shipping a spec — flipping its Status, moving it to `history/` → `docs/specs/history/README.md`. **Do not write into `docs/` during a run** — record what the change made untrue in the spec and leave the docs to `/distil`
- `docs/specs/history/README.md` — the shipped-spec index: date, one line on what each did, and its PR. Read the one a doc names, never the folder
- `docs/observability.md` — logging shape, request correlation, LogsQL queries
- `docs/e2e-flows.md` — Playwright coverage: what's tested, what's deliberately not
- `docs/archive/` — historical inputs, kept verbatim: the original product / technical / UI design docs (Draft v1/v2) that drove M0–M7, plus point-in-time research artefacts (preview feasibility, dependency audit). Section numbers and API shapes have drifted — prefer `architecture.md` for the live shape and consult the archive only for the *why*

## Development commands

All workflows go through [just](https://just.systems) (`justfile`, plus modules in `tools/`) — do not invoke `go`, `pnpm` or `docker` directly for build/test/lint operations. Run `just` to list everything. Names follow a canonical vocabulary (`build`, `test`, `lint`, `format`, `check`, `clean`, `dev`, `install`), with the backend/frontend split as the second word (`lint backend` / `lint frontend`) — use the umbrella when in doubt.

**`just check` is the full preflight, not a quick one.** Run `just check` before pushing — it is what CI gates on.

Run one Go test — `just test backend` sets `DAMSON_TEST_POSTGRES_DSN` for you, defaulting to the shared local Postgres on 5432 via the `damson_test` database:

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

Local dev sign-in (`just dev`, local auth mode) uses a throwaway account — email `dev@example.com`, password `password1`, seeded from `DAMSON_LOCAL_ADMIN_EMAIL` / `DAMSON_LOCAL_ADMIN_PASSWORD` in the gitignored `.env`; a fresh clone must set them there. Not a real credential; it exists so the SPA can be driven end-to-end against the dev server.

## Conventions

The full rules live in `docs/CONVENTIONS.md`. Load-bearing ones worth keeping in mind while editing:

- **500 always returns the generic `"internal error"`** — never leak internals. Status mapping → `docs/CONVENTIONS.md`, Error responses; the transaction sentinels → `docs/architecture.md`, Request lifecycle.
- **Never fork a component out of `@telliott/shared-ui` into the tree.** A Damson-specific difference goes in a token override or a wrapper — `design-system/primitives/Button.tsx` is the worked example. `shadcn add` still works as a one-off escape hatch, but it takes that component off the upgrade path, and every install re-appends the contract's reference palette to `globals.css`; delete it, and `just lint frontend` will tell you if you forget.
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
