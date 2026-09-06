# Read-only static analysis.
#
# `[working-directory]` in a module is relative to THIS file, so the frontend is
# `../web`; it cannot be combined with `[no-cd]`.

set shell := ["bash", "-euo", "pipefail", "-c"]

# go vet plus the frontend gate
[default]
[no-cd]
all: backend frontend

# go vet
[no-cd]
backend:
    go vet ./...

# The design-token guardrail (web/scripts/check-tokens.mjs) and the Tailwind
# `@source` check run here too, ahead of ESLint — see docs/design-system.md.

# ESLint, the token guardrail and the Tailwind source check
[working-directory('../web')]
frontend:
    just _web-deps
    pnpm run lint
