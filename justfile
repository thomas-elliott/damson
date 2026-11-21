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

# No .env, no Docker. Every recipe reads the agent environment: the libpq
# variables PGHOST, PGPORT, PGUSER, PGSSLMODE and PGDATABASE name the Postgres
# server, with the password in ~/.pgpass (or PGPASSWORD in CI), which psql and
# pgx read themselves. Image storage reads the standard AWS variables plus
# E2E_S3_BUCKET. See AGENTS.md, Environment.

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

# The API and Vite ports. Vite's proxy reads PORT too, so moving the API needs
# no second setting.
PORT := env("PORT", "8080")
WEB_PORT := env("WEB_PORT", "5173")

# One dev database per user on the shared server, so two agents never share
# rows. DEV_DATABASE names another.
DB_USER := `id -un | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9_\n' '_'`
DEV_DB := env("DEV_DATABASE", "damson_dev_" + DB_USER)

# The homelab-dev Zitadel client `just dev` signs in through. Neither value is a
# secret: it is a public PKCE client that accepts /auth/callback on
# http://localhost:*, http://127.0.0.1:* and https://dev.ghost-eagle.ts.net:*,
# any port. The id_token's audience is the client id.
DEV_OIDC_ISSUER := "https://login.smol.cloud"
DEV_OIDC_CLIENT_ID := "396076555912151326"

AIR_VERSION := "v1.67.4"

# Go API with air hot-reload, on PORT, against your dev database
backend:
    #!/usr/bin/env bash
    set -euo pipefail
    just _db-ensure '{{ DEV_DB }}'
    # Assigned before export: `export X="$(…)"` reports export's status, not the guard's.
    dsn="$(just _dsn '{{ DEV_DB }}')"
    export DAMSON_POSTGRES_DSN="$dsn"
    export DAMSON_AUTH_MODE=oidc
    export DAMSON_OIDC_ISSUER_URL='{{ DEV_OIDC_ISSUER }}'
    export DAMSON_OIDC_CLIENT_ID='{{ DEV_OIDC_CLIENT_ID }}'
    export DAMSON_OIDC_AUDIENCE='{{ DEV_OIDC_CLIENT_ID }}'
    export DAMSON_LISTEN_ADDR='127.0.0.1:{{ PORT }}'
    s3="$(just _s3-env)"
    eval "$s3"
    command -v air >/dev/null || go install github.com/air-verse/air@{{ AIR_VERSION }}
    exec air

# Load sample packs and ~300 reference assets into the dev library; run while `just dev` is up, after signing in once
seed:
    #!/usr/bin/env bash
    set -euo pipefail
    dsn="$(just _dsn '{{ DEV_DB }}')"
    DAMSON_POSTGRES_DSN="$dsn" PORT='{{ PORT }}' go run ./tools/devseed

# Vite dev server for the SPA, on WEB_PORT, proxying /api to PORT
[working-directory('web')]
frontend:
    just _web-deps
    PORT='{{ PORT }}' WEB_PORT='{{ WEB_PORT }}' pnpm run dev

# Install everything a fresh clone needs — Go modules, pnpm packages, Playwright's Chromium
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

# A database URL for DB on the server the PG* variables name. The password
# never appears in it: pgx reads ~/.pgpass or PGPASSWORD itself.
[private]
_dsn DB:
    #!/usr/bin/env bash
    set -euo pipefail
    just _pg-required
    printf 'postgres://%s@%s:%s/%s?sslmode=%s\n' "$PGUSER" "$PGHOST" "$PGPORT" '{{ DB }}' "$PGSSLMODE"

# Your dev database's name. A recipe, not a variable, because the modules in
# tools/ cannot read this file's variables.
[private]
_dev-db:
    @echo '{{ DEV_DB }}'

# Create database DB if it is missing. The server migrates it on boot.
[private]
_db-ensure DB:
    #!/usr/bin/env bash
    set -euo pipefail
    just _pg-required
    if [ -z "$(psql -XAtc "select 1 from pg_database where datname = '{{ DB }}'")" ]; then
      createdb '{{ DB }}'
      echo "created database {{ DB }}" >&2
    fi

# Create a database for one test run and print its name. Each run gets its own,
# so two agents or two checkouts never share one; the caller drops it on exit.
# A run that is killed leaks it: `just db drop-leaked` sweeps those, and the
# nightly pg-e2e-sweep catches the rest.
[private]
_test-db-create:
    #!/usr/bin/env bash
    set -euo pipefail
    just _pg-required
    db="damson_test_{{ DB_USER }}_$(head -c4 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    createdb "$db"
    echo "$db"

# Names every missing PG* variable and fails. Nothing falls back to a local default.
[private]
_pg-required:
    #!/usr/bin/env bash
    set -uo pipefail
    missing=()
    for v in PGHOST PGPORT PGUSER PGSSLMODE PGDATABASE; do
      [ -n "${!v:-}" ] || missing+=("$v")
    done
    [ "${#missing[@]}" -eq 0 ] && exit 0
    printf 'No Postgres configured. Not set: %s\n%s\n' "${missing[*]}" '{{ PG_UNSET }}' >&2
    exit 1

PG_UNSET := '''
Recipes that need Postgres read the libpq variables PGHOST, PGPORT, PGUSER,
PGSSLMODE and PGDATABASE, with the password in ~/.pgpass or PGPASSWORD. The
agent environment sets all of them; see AGENTS.md, Environment.
'''

# Prints exports of DAMSON_S3_* from the standard AWS variables, when they are
# set. With none set, the server keeps files on local disk under ./data; with
# only some, this names the missing ones rather than booting half-configured.
# Path-style addressing, which garage-test needs, is the server's default. The
# key prefix is the dev database's, so each user's objects stay apart in the
# one shared bucket.
[private]
_s3-env:
    #!/usr/bin/env bash
    set -euo pipefail
    vars=(AWS_ENDPOINT_URL_S3 AWS_REGION AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY E2E_S3_BUCKET)
    missing=()
    for v in "${vars[@]}"; do [ -n "${!v:-}" ] || missing+=("$v"); done
    if [ "${#missing[@]}" -eq "${#vars[@]}" ]; then
      echo "note: no AWS variables set, so files stay on local disk" >&2
      exit 0
    fi
    if [ "${#missing[@]}" -gt 0 ]; then
      echo "Image storage is half-configured. Not set: ${missing[*]}" >&2
      exit 1
    fi
    q() { printf '%q' "$1"; }
    echo "export DAMSON_STORAGE_MODE=s3"
    echo "export DAMSON_S3_ENDPOINT=$(q "$AWS_ENDPOINT_URL_S3")"
    echo "export DAMSON_S3_REGION=$(q "$AWS_REGION")"
    echo "export DAMSON_S3_BUCKET=$(q "$E2E_S3_BUCKET")"
    echo "export DAMSON_S3_ACCESS_KEY=$(q "$AWS_ACCESS_KEY_ID")"
    echo "export DAMSON_S3_SECRET_KEY=$(q "$AWS_SECRET_ACCESS_KEY")"
    echo "export DAMSON_S3_PREFIX=$(q "damson/{{ DEV_DB }}")"

# Task ran this as a `status:` guard on every frontend recipe. `just` always runs
# a recipe, and a module recipe cannot depend on a root one, so the frontend
# recipes call this by name as their first line instead.

# Install frontend packages if they are missing
[private]
_web-deps:
    [ -d web/node_modules ] || (cd web && pnpm install)
