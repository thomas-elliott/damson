# Your dev database, and the test databases, on the server the PG* variables
# name. There is no `db up` / `db down`: that server is shared, not something
# this repo starts and stops. psql reads PGHOST, PGPORT, PGUSER, PGSSLMODE and
# ~/.pgpass itself, so nothing here handles a password.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Drop your dev database and create it empty; the next `just dev` migrates it
[no-cd]
reset:
    #!/usr/bin/env bash
    set -euo pipefail
    just _pg-required
    db="$(just _dev-db)"
    dropdb --if-exists "$db"
    createdb "$db"
    echo "reset $db"

# Open psql on your dev database
[no-cd]
psql:
    #!/usr/bin/env bash
    set -euo pipefail
    just _pg-required
    exec psql "$(just _dev-db)"

# Each test run drops its own database on exit, so this is only needed after a
# run was killed. It drops only test databases the current PGUSER owns and that
# nothing is connected to; the nightly pg-e2e-sweep handles the rest.

# Drop the test databases a killed run left behind
[no-cd]
drop-leaked:
    #!/usr/bin/env bash
    set -euo pipefail
    just _pg-required
    leaked="$(psql -XAt <<'SQL'
    select d.datname
    from pg_database d
    join pg_roles r on r.oid = d.datdba
    where r.rolname = current_user
      and d.datname like 'damson\_test\_%'
      and not exists (select 1 from pg_stat_activity a where a.datname = d.datname)
    SQL
    )"
    if [ -z "$leaked" ]; then
      echo "no leaked test databases"
      exit 0
    fi
    while read -r db; do
      dropdb "$db"
      echo "dropped $db"
    done <<< "$leaked"
