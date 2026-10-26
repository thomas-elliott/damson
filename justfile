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

# Load `.env` into every recipe, so a DSN set there wins over the defaults below.
set dotenv-load

mod build 'tools/build.just'
mod check 'tools/check.just'
mod db 'tools/db.just'
mod format 'tools/format.just'
mod lint 'tools/lint.just'
mod release 'tools/release.just'
mod test 'tools/test.just'

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

# A fresh clone runs with no `.env`: the backend defaults to the compose
# database and a throwaway local admin, listening on loopback only, which Vite
# proxies to. Anything set in `.env` or the environment wins over these.
DEV_LISTEN_ADDR := "127.0.0.1:8080"
DEV_ADMIN_EMAIL := "dev@example.com"
DEV_ADMIN_PASSWORD := "password1"
AIR_VERSION := "v1.67.4"

# Go API with air hot-reload
backend:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${DAMSON_POSTGRES_DSN:-}${DAMSON_POSTGRES_DSN_FILE:-}" ]; then
      DAMSON_POSTGRES_DSN="$(just db dsn damson)"
      export DAMSON_POSTGRES_DSN
    fi
    if [ -z "${DAMSON_LOCAL_ADMIN_EMAIL:-}${DAMSON_LOCAL_ADMIN_PASSWORD:-}${DAMSON_LOCAL_ADMIN_PASSWORD_FILE:-}" ]; then
      export DAMSON_LOCAL_ADMIN_EMAIL="{{ DEV_ADMIN_EMAIL }}"
      export DAMSON_LOCAL_ADMIN_PASSWORD="{{ DEV_ADMIN_PASSWORD }}"
    fi
    export DAMSON_LISTEN_ADDR="${DAMSON_LISTEN_ADDR:-{{ DEV_LISTEN_ADDR }}}"
    command -v air >/dev/null || go install github.com/air-verse/air@{{ AIR_VERSION }}
    exec air

# Load sample packs and ~300 reference assets into the dev library; run while `just dev` is up
seed:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${DAMSON_POSTGRES_DSN:-}${DAMSON_POSTGRES_DSN_FILE:-}" ]; then
      DAMSON_POSTGRES_DSN="$(just db dsn damson)"
      export DAMSON_POSTGRES_DSN
    fi
    DAMSON_LOCAL_ADMIN_EMAIL="${DAMSON_LOCAL_ADMIN_EMAIL:-{{ DEV_ADMIN_EMAIL }}}" \
    DAMSON_LOCAL_ADMIN_PASSWORD="${DAMSON_LOCAL_ADMIN_PASSWORD:-{{ DEV_ADMIN_PASSWORD }}}" \
      go run ./tools/devseed

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

# Resolve the DSN the Go suite and the e2e server run against. Every Go test and
# the e2e server run against Postgres — there is no SQLite backend. An exported
# DAMSON_TEST_POSTGRES_DSN, from CI or `.env`, wins; otherwise this starts the
# checkout's compose Postgres, which creates `damson_test` on first start. It prints rather than exports because a
# recipe cannot export into its caller — the callers read it as
# `DAMSON_TEST_POSTGRES_DSN="$(just _test-dsn)"`.

# Print the test Postgres DSN, starting the compose Postgres if it is the default
[private]
_test-dsn:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "${DAMSON_TEST_POSTGRES_DSN:-}" ]; then
      echo "$DAMSON_TEST_POSTGRES_DSN"
      exit 0
    fi
    just db dsn damson_test

# Task ran this as a `status:` guard on every frontend recipe. `just` always runs
# a recipe, and a module recipe cannot depend on a root one, so the frontend
# recipes call this by name as their first line instead.

# Install frontend packages if they are missing
[private]
_web-deps:
    [ -d web/node_modules ] || (cd web && pnpm install)
