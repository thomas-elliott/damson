package db

import (
	"strings"
	"testing"
)

func TestDSNFromEnv(t *testing.T) {
	t.Setenv("PGHOST", "pg.example")
	t.Setenv("PGPORT", "5434")
	t.Setenv("PGUSER", "e2e")
	t.Setenv("PGSSLMODE", "require")
	t.Setenv("PGDATABASE", "damson_test_x")
	t.Setenv("PGPASSWORD", "never-in-the-url")

	dsn, err := DSNFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if want := "postgres://e2e@pg.example:5434/damson_test_x?sslmode=require"; dsn != want {
		t.Fatalf("dsn = %q, want %q", dsn, want)
	}
}

func TestDSNFromEnv_NamesMissingVariables(t *testing.T) {
	for _, v := range PGVariables {
		t.Setenv(v, "set")
	}
	t.Setenv("PGHOST", "")
	t.Setenv("PGSSLMODE", "")

	_, err := DSNFromEnv()
	if err == nil {
		t.Fatal("want an error when PGHOST and PGSSLMODE are unset")
	}
	if !strings.Contains(err.Error(), "not set: PGHOST, PGSSLMODE") {
		t.Fatalf("error does not name the missing variables: %v", err)
	}
}
