# Architecture

Current-state reference for Damson. The build-time design docs in
[`docs/archive/`](archive/) describe the *why* and the trade-offs;
this doc describes *what is in the tree today*.

Companions:

- [`docs/CONVENTIONS.md`](CONVENTIONS.md) — coding standards, handler
  patterns, test helpers
- [`docs/design-system.md`](design-system.md) — frontend design system
- [`docs/progress.md`](progress.md) — milestone-by-milestone status
- [`AGENTS.md`](../AGENTS.md) — quick-start guidance for agentic coding

---

## 1. What Damson is

A self-hosted digital asset manager for game-development assets.
Single Go binary that embeds a React SPA, fronted by a reverse proxy,
backed by Postgres and an S3-compatible object store (or a local
filesystem for single-node installs).

Core concerns:

- **Library-based group ownership** — a `libraries` row is the tenancy
  boundary. Users gain access via `library_memberships`; within a
  library, every member sees the same data.
- **Content-addressed storage** — file bytes live keyed by SHA-256.
  Postgres holds metadata; the same texture appearing in five packages
  occupies one object.
- **Versioning is immutable** — an `AssetVersion`'s file manifest is
  fixed once written. Re-extractions produce new versions.
- **Projects are declarative** — a project pins specific versions;
  upgrade is a logged, reversible action.
- **Audit is a boundary** — every library-scoped write writes an
  `audit_log` row in the same DB transaction.

The MVP is M0–M7; see [`docs/progress.md`](progress.md) for what has
landed, what is partial, and what is deferred.

---

## 2. Stack

**Backend.** Go 1.26, Gin, GORM (Postgres in production, in-memory
SQLite for tests). Auth via `coreos/go-oidc/v3` + `oauth2` (OIDC mode)
or `golang-jwt/jwt/v5` HS256 (local mode). S3 access via
`aws-sdk-go-v2`. SPDX matching via `google/licensecheck`. Thumbnails
are pure Go: stdlib `image` + `golang.org/x/image/{webp,draw,font}`,
plus `hajimehoshi/go-mp3` and `jfreymuth/oggvorbis` for audio. WAV is
parsed in-tree. No CGo.

**Frontend.** React 19, TypeScript (strict), Vite 8. TanStack Query 5
for server state. React Router 7 (data router). Tailwind 4 with OKLCH
design tokens. Radix UI headless primitives + hand-rolled components
under `web/src/design-system/`. `class-variance-authority` for
variants; `clsx` + `tailwind-merge` via a `cn()` helper.
`@tanstack/react-virtual` for the asset grid. `@fontsource/inter` +
`@fontsource/jetbrains-mono` self-hosted fonts.

**Tooling.** Task (`Taskfile.yml`) for all developer workflows. `air`
for Go hot-reload. Vitest 4 with two projects: `unit` (jsdom) and
`storybook` (Playwright Chromium running every `*.stories.tsx` with
axe-core a11y checks). Storybook 10. Multi-stage Dockerfile (Node →
Go → Alpine).

Module path: `github.com/thomas-elliott/damson`.

---

## 3. Repository layout

```
damson/
├── Taskfile.yml
├── Dockerfile
├── damson.go                 # //go:embed for migrations + SPA (see §11)
├── cmd/server/main.go        # entry point: HTTP server, middleware, wiring
├── internal/
│   ├── auth/                 # Authenticator interface + local + oidc impls
│   ├── config/               # env loading, panic-on-missing
│   ├── db/                   # gorm.Open, migration runner
│   ├── handlers/api/         # one file per feature; receivers on *Handler
│   ├── models/               # GORM models, one file per entity
│   ├── services/
│   │   ├── audit/            # audit log writer (same-tx with mutation)
│   │   ├── extract/          # .unitypackage extractor
│   │   ├── licence/          # SPDX + Unity EULA detection
│   │   ├── storage/          # storage.Service interface + LocalFS + S3
│   │   └── thumbnail/        # raster / audio / font workers
│   └── web/                  # SPA static handler
├── database/migrations/      # NNN_*.sql, applied on startup
├── web/                      # React SPA (Vite); built into web/static/
└── docs/                     # this directory
```

`damson.go` lives at the repo root because `//go:embed` cannot
traverse upward. It exports `damson.MigrationsFS` and
`damson.WebStaticFS`, consumed by `internal/db` and `internal/web`.
`web/static/.gitkeep` exists so `//go:embed all:web/static` succeeds
before the SPA is built.
