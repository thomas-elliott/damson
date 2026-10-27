# Formatting, applied and checked.
#
# `[working-directory]` in a module is relative to THIS file, so the frontend is
# `../web`; it cannot be combined with `[no-cd]`.

set shell := ["bash", "-euo", "pipefail", "-c"]

# gofmt and Prettier, writing
[default]
[no-cd]
all: backend frontend

# go fmt
[no-cd]
backend:
    go fmt ./...

# prettier --write
[working-directory('../web')]
frontend:
    just _web-deps
    pnpm run format

# The Go half is CI's gofmt step; the Prettier half is the `web` job's
# format:check.

# Fail on an unformatted file, Go or frontend
[no-cd]
check: check-backend check-frontend

# gofmt -l (fails on unformatted files)
[no-cd]
check-backend:
    #!/usr/bin/env bash
    set -euo pipefail
    out=$(gofmt -l .)
    if [ -n "$out" ]; then
      echo "unformatted Go files:" >&2
      echo "$out" >&2
      exit 1
    fi

# prettier --check
[working-directory('../web')]
check-frontend:
    just _web-deps
    pnpm run format:check
