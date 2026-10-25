# Read-only static analysis.
#
# `[working-directory]` in a module is relative to THIS file, so the frontend is
# `../web`; it cannot be combined with `[no-cd]`.

set shell := ["bash", "-euo", "pipefail", "-c"]

# go vet and golangci-lint, plus the frontend gate
[default]
[no-cd]
all: backend frontend

# golangci-lint is pinned as a Go tool in go.mod, so it needs no install; its
# linters are in .golangci.yml.

# go vet and golangci-lint
[no-cd]
backend:
    go vet ./...
    go tool golangci-lint run ./...

# The design-token guardrail (web/scripts/check-tokens.mjs) and the Tailwind
# `@source` check run here too, ahead of ESLint — see docs/design-system.md.

# ESLint, the token guardrail and the Tailwind source check
[working-directory('../web')]
frontend:
    just _web-deps
    pnpm run lint
