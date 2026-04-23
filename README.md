# Damson

A self-hosted digital asset manager for game-development assets — licence tracking, Unity package extraction, versioning, per-library workspaces.

Early development. See `docs/` for the full design:

- `docs/product-design-doc.md` — product scope and MVP
- `docs/ui-design.md` — frontend design brief
- `docs/technical-design.md` — architecture, schema, API surface
- `docs/CONVENTIONS.md` — repo conventions (stack, formatting, test helpers)

## Running locally

Prerequisites: Go 1.26, Node 22+, Postgres, an S3-compatible bucket, an OIDC provider, [Task](https://taskfile.dev), and [air](https://github.com/air-verse/air).

```
# First time
go mod tidy
task web:deps

# Dev loop (backend + Vite SPA in parallel)
task dev:spa
```

Required env vars are listed in `docs/technical-design.md` §14.1.

## Layout

- `cmd/server/` — binary entry point
- `internal/` — backend packages (handlers, models, services)
- `database/migrations/` — SQL migrations applied at startup
- `web/` — React SPA (Vite build output at `web/static/`)
- `docs/` — design documents
