# The test suites.
#
# Every Go-side recipe resolves its database through `just _test-dsn` (see the
# root justfile) rather than hardcoding a DSN, so an exported
# DAMSON_TEST_POSTGRES_DSN always wins and the local default gets its database
# created on first use.
#
# `[working-directory]` in a module is relative to THIS file, so the frontend is
# `../web`; it cannot be combined with `[no-cd]`.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Everything — the unit suites, then Playwright
[default]
[no-cd]
all: unit e2e

# Go tests plus the Vitest unit project
[no-cd]
unit: backend frontend

# ARGS narrows to a package or a test:
# `just test backend ./internal/handlers/api -run '^TestName$/SubtestName$'`.

# Go tests (Postgres-backed)
[no-cd]
backend *ARGS:
    #!/usr/bin/env bash
    set -euo pipefail
    export DAMSON_TEST_POSTGRES_DSN="$(just _test-dsn)"
    race=-race
    if ! command -v "$(go env CC)" >/dev/null; then
      echo "warning: no C compiler ($(go env CC)), so running without -race" >&2
      race=
    fi
    if [ -z "{{ ARGS }}" ]; then
      go test -count=1 $race ./...
    else
      go test -count=1 $race {{ ARGS }}
    fi

# FILE runs one test file; bare runs the whole unit project.

# Vitest unit project (jsdom)
[working-directory('../web')]
frontend FILE="":
    #!/usr/bin/env bash
    set -euo pipefail
    just _web-deps
    if [ -z "{{ FILE }}" ]; then
      pnpm run test -- --run
    else
      pnpm run test -- "{{ FILE }}" --run
    fi

# ARGS goes straight through to Playwright, which is why there is no separate
# name for the interactive runner: `just test e2e --ui`. Playwright launches the
# e2e server (built by the dependency below) and Vite itself — see
# web/playwright.config.ts.

# Playwright end-to-end tests
[working-directory('../web')]
e2e *ARGS: _e2e-server
    #!/usr/bin/env bash
    set -euo pipefail
    just _web-deps
    export DAMSON_TEST_POSTGRES_DSN="$(just _test-dsn)"
    if [ -z "{{ ARGS }}" ]; then
      pnpm run e2e
    else
      pnpm run e2e -- {{ ARGS }}
    fi

# Every *.stories.tsx as a test in real Chromium, with axe-core a11y checks, in both themes
[working-directory('../web')]
stories:
    just _web-deps
    pnpm run test:stories -- --run
    STORYBOOK_THEME=light pnpm run test:stories -- --run

# Vitest watch mode (unit project)
[working-directory('../web')]
watch:
    just _web-deps
    pnpm run test

# Go coverage report → coverage.html
[no-cd]
coverage:
    DAMSON_TEST_POSTGRES_DSN="$(just _test-dsn)" go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Only `e2e` uses this, so it is a dependency rather than a name of its own.

# Build the e2e test server into web/e2e/.bin
[no-cd]
[private]
_e2e-server:
    mkdir -p web/e2e/.bin
    go build -o web/e2e/.bin/damson-e2e ./cmd/e2e-server
