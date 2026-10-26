# The local Postgres from compose.yaml, for `just dev` and the test suites.
#
# Each checkout gets its own server: the compose project is named after a hash
# of the checkout's path, and Docker picks a free loopback port, so clones and
# worktrees never share or take over another checkout's database. Recipes ask
# `just db dsn` for the address rather than assuming a port.

set shell := ["bash", "-euo", "pipefail", "-c"]

export COMPOSE_PROJECT_NAME := "damson-" + replace_regex(sha256(justfile_directory()), '^(.{8}).*$', '$1')

# Start this checkout's Postgres and wait until it accepts connections
[no-cd]
up:
    docker compose up -d --wait postgres

# Stop Postgres, keeping its data
[no-cd]
down:
    docker compose down

# Delete the dev and test databases and start again from empty
[no-cd]
reset:
    docker compose down --volumes
    docker compose up -d --wait postgres

# Open psql on the dev database
[no-cd]
psql:
    docker compose exec postgres psql -U damson damson

# DB is `damson` for `just dev` or `damson_test` for the test suites.

# Print the DSN for DB on this checkout's Postgres, starting it if needed
[no-cd]
dsn DB="damson":
    #!/usr/bin/env bash
    set -euo pipefail
    docker compose up -d --wait postgres >&2
    addr="$(docker compose port postgres 5432)"
    echo "postgres://damson:damson@${addr}/{{ DB }}?sslmode=disable"

# Delete the Postgres, data included, of every checkout whose directory is gone
[no-cd]
prune:
    #!/usr/bin/env bash
    set -euo pipefail
    docker ps -a --filter label=com.docker.compose.service=postgres \
      --format '{{{{.Label "com.docker.compose.project"}}	{{{{.Label "com.docker.compose.project.working_dir"}}' |
      sort -u |
      while IFS=$'\t' read -r project dir; do
        [[ "$project" == damson-* && ! -d "$dir" ]] || continue
        echo "removing $project (was $dir)"
        docker compose -p "$project" down --volumes
      done
