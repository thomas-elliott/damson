# Every dev command for this repo. `just` with no argument lists them.
#
# Vocabulary: code-skills-marketplace/coding-practices/general/recipe-grammar.toml.
#
# Two `just` traps this file and the modules in tools/ are written around:
#   * A recipe's doc comment goes ABOVE its attributes. A comment between an
#     attribute and its recipe detaches the attribute ("extraneous attributes").
#   * `[no-cd]` and `[working-directory]` cannot both be on one recipe, and
#     `[working-directory]` in a module is relative to the module file — so the
#     frontend is `../web` there and plain `web` here.

set shell := ["bash", "-euo", "pipefail", "-c"]

mod build 'tools/build.just'
mod check 'tools/check.just'
mod format 'tools/format.just'
mod lint 'tools/lint.just'
mod release 'tools/release.just'
mod test 'tools/test.just'

# Every Go test and the e2e server run against Postgres — there is no SQLite
# backend. They default to the shared local Postgres on 5432, isolating
# themselves in a dedicated `damson_test` database rather than a per-project
# container. CI sets DAMSON_TEST_POSTGRES_DSN to Woodpecker's own
# `services: postgres` (.woodpecker.yml), which wins over these.
TEST_PG_DSN := "postgres://postgres:postgres@localhost:5432/damson_test?sslmode=disable"
TEST_PG_ADMIN_DSN := "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
TEST_PG_DB := "damson_test"

_default:
    @just --list --list-submodules

# The runnable apps are flat and short: `just backend`, `just frontend`, and
# `just dev` for both.
#
# `[parallel]` rather than a shebang recipe backgrounding both under
# `trap "kill 0" INT TERM`: it reuses `backend` and `frontend` instead of
# restating what they run, and a terminal Ctrl-C takes both servers down either
# way. The trap only ever existed because Task's mvdan/sh could not run it.

# Run the Go API and the Vite SPA together
[parallel]
dev: backend frontend

# Go API with air hot-reload
backend:
    air

# Vite dev server for the SPA
[working-directory('web')]
frontend:
    just _web-deps
    pnpm run dev

# Install everything a fresh clone needs — Go modules, pnpm packages, Playwright browsers
install:
    go mod download
    cd web && pnpm install
    cd web && pnpm run e2e:install

# Storybook is a noun, so no bare `just storybook` sweep is owed; these two are
# the whole family and stay flat.

# Storybook dev server on :6006 (design system)
[working-directory('web')]
storybook:
    just _web-deps
    pnpm run storybook

# Static Storybook build (smoke test)
[working-directory('web')]
storybook-build:
    just _web-deps
    pnpm run build-storybook

# Container image → damson:local
image-build:
    docker build -t damson:local .

# Remove build artefacts
clean:
    rm -f damson dmsn coverage.out coverage.html
    rm -rf web/dist web/e2e/.bin web/storybook-static web/static

# Resolve the DSN the Go suite and the e2e server run against. An explicit
# DAMSON_TEST_POSTGRES_DSN wins and is left alone; otherwise this creates
# `damson_test` on the shared local Postgres and prints its DSN. It prints rather
# than exports because a recipe cannot export into its caller — the callers read
# it as `DAMSON_TEST_POSTGRES_DSN="$(just _test-dsn)"`.

# Print the test Postgres DSN, creating the database if it is the local default
[private]
_test-dsn:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "${DAMSON_TEST_POSTGRES_DSN:-}" ]; then
      echo "$DAMSON_TEST_POSTGRES_DSN"
      exit 0
    fi
    if command -v psql >/dev/null; then
      psql "{{ TEST_PG_ADMIN_DSN }}" -c "CREATE DATABASE {{ TEST_PG_DB }}" >/dev/null 2>&1 || true
    else
      # Without this the suite fails as `database "{{ TEST_PG_DB }}" does not
      # exist`, which reads as a broken Postgres rather than a missing client.
      echo "warning: psql not found — create {{ TEST_PG_DB }} yourself, or set DAMSON_TEST_POSTGRES_DSN." >&2
    fi
    echo "{{ TEST_PG_DSN }}"

# Task ran this as a `status:` guard on every frontend recipe. `just` always runs
# a recipe, and a module recipe cannot depend on a root one, so the frontend
# recipes call this by name as their first line instead.

# Install frontend packages if they are missing
[private]
_web-deps:
    [ -d web/node_modules ] || (cd web && pnpm install)
