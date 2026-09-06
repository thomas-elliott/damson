# Building the binaries and the SPA the server embeds.
#
# `[working-directory]` in a module is relative to THIS file, so the frontend is
# `../web`; it cannot be combined with `[no-cd]`.

set shell := ["bash", "-euo", "pipefail", "-c"]

# `damson.go` embeds `web/static` into the binary, so a bare `just build backend`
# ships whatever that directory already holds — the committed `.gitkeep`
# placeholder on a fresh clone. The full build is the only one that produces a
# serving binary.

# The SPA, then the binary that embeds it
[default]
[no-cd]
all: frontend backend

# Production binary (embeds web/static as it stands)
[no-cd]
backend:
    CGO_ENABLED=0 go build -o damson ./cmd/server

# Build the SPA into web/static (tsc -b + vite build)
[working-directory('../web')]
frontend:
    just _web-deps
    pnpm run build

# `dmsn` is a second binary, not a configuration of the first — neither `build`
# nor `build backend` produces it — so it earns a name rather than a scope word.

# Build the dmsn CLI for this machine (no cross-compile matrix, no release artefacts)
[no-cd]
cli:
    CGO_ENABLED=0 go build -o dmsn ./cmd/dmsn
    @echo "built ./dmsn — run ./dmsn setup to install the Claude Code skill"
