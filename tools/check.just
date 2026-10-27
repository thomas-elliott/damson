# The preflight gate.
#
# `just check` mirrors what CI gates on (.github/workflows/ci.yml), so passing it locally
# is what makes a push safe. `just check fast` is the read-only subset that needs
# no database and no browser.

set shell := ["bash", "-euo", "pipefail", "-c"]

# `build frontend` is in the sweep because `pnpm run build` is `tsc -b && vite
# build` — it is CI's only type check, and a type error passes every other gate
# here.

# Full preflight — read-only gates, the frontend build, then every test
[default]
[no-cd]
all: fast
    just build frontend
    just test unit
    just test stories
    just test e2e

# Read-only, no database — lint, formatting, module tidiness
[no-cd]
fast:
    just lint
    just format check
    just check::tidy

# Run this whenever an import lands — especially when a previously-indirect
# dependency becomes a direct require — because CI runs `go mod tidy` then
# `git diff --exit-code` and fails the build on any drift.

# Verify go.mod / go.sum are tidy (matches CI)
[no-cd]
tidy:
    #!/usr/bin/env bash
    set -euo pipefail
    cp go.mod go.mod.bak
    cp go.sum go.sum.bak
    trap 'mv go.mod.bak go.mod; mv go.sum.bak go.sum' EXIT
    go mod tidy
    if ! cmp -s go.mod go.mod.bak || ! cmp -s go.sum go.sum.bak; then
      echo "go.mod / go.sum are not tidy. Run: go mod tidy && git add go.mod go.sum" >&2
      diff -u go.mod.bak go.mod >&2 || true
      exit 1
    fi
