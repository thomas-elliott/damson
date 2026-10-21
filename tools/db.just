# The local Postgres from compose.yaml, for `just dev` and the test suites.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Start Postgres on :55432 and wait until it accepts connections
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
