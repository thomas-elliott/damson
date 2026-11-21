package db

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// PGVariables are the libpq variables that name the Postgres server the test
// suites and the e2e server run against. Every dev box and CI sets them.
var PGVariables = []string{"PGHOST", "PGPORT", "PGUSER", "PGSSLMODE", "PGDATABASE"}

// DSNFromEnv builds a connection URL from the PG* variables. It leaves the
// password out: pgx reads PGPASSWORD or ~/.pgpass itself, so the URL is safe
// to log. A missing variable is an error that names it, never a fallback to
// localhost.
func DSNFromEnv() (string, error) {
	var missing []string
	for _, v := range PGVariables {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("postgres is named by %s, with the password in ~/.pgpass or PGPASSWORD; not set: %s",
			strings.Join(PGVariables, ", "), strings.Join(missing, ", "))
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.User(os.Getenv("PGUSER")),
		Host:     os.Getenv("PGHOST") + ":" + os.Getenv("PGPORT"),
		Path:     "/" + os.Getenv("PGDATABASE"),
		RawQuery: url.Values{"sslmode": {os.Getenv("PGSSLMODE")}}.Encode(),
	}
	return u.String(), nil
}
