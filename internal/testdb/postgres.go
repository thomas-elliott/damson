// Package testdb provides shared helpers for tests that need a database.
// Postgres is the only supported backend: every test runs against a real
// Postgres instance, each in its own throwaway schema (test_<random>).
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/db"
)

// PostgresDSNEnv is the env var that points the test suite at a Postgres
// instance. It is required: with it unset the suite fails fast rather than
// silently skipping, because Postgres is the only supported backend.
const PostgresDSNEnv = "DAMSON_TEST_POSTGRES_DSN"

// RequireDSN returns the Postgres DSN from the environment, failing the test
// if it is unset. Use it in tests that open their own connection(s) rather
// than going through NewPostgres.
func RequireDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(PostgresDSNEnv)
	if dsn == "" {
		t.Fatalf("%s must be set: the test suite requires a Postgres instance", PostgresDSNEnv)
	}
	return dsn
}

// NewPostgres opens a Postgres connection from $DAMSON_TEST_POSTGRES_DSN,
// creates a fresh per-test schema (test_<random>), runs all SQL migrations
// against that schema, and registers a Cleanup to drop it. Each test gets an
// isolated namespace on the same Postgres instance — no per-test container.
//
// The DSN is mandatory; an unset value is a hard failure so a run without a
// database is never mistaken for a passing run.
func NewPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := RequireDSN(t)
	schema := newSchemaName(t)
	return OpenSchema(t, dsn, schema, true)
}

// OpenSchema opens a fresh GORM connection against `dsn` with `search_path`
// pointing at `schema`. If `migrate` is true the SQL migrations are applied
// and a Cleanup hook drops the schema at test end. If `migrate` is false the
// schema is left untouched — useful for the second connection in
// migration-contention tests, which races against the first on the advisory
// lock and must not pre-create the schema or cleanup-drop it.
func OpenSchema(t *testing.T, dsn, schema string, migrate bool) *gorm.DB {
	t.Helper()

	if migrate {
		bootstrap, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{})
		if err != nil {
			t.Fatalf("open bootstrap connection: %v", err)
		}
		if err := bootstrap.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %q`, schema)).Error; err != nil {
			t.Fatalf("create schema %s: %v", schema, err)
		}
		if sql, _ := bootstrap.DB(); sql != nil {
			_ = sql.Close()
		}
		t.Cleanup(func() {
			cleanup, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{})
			if err != nil {
				t.Logf("cleanup open: %v", err)
				return
			}
			defer func() {
				if sql, _ := cleanup.DB(); sql != nil {
					_ = sql.Close()
				}
			}()
			if err := cleanup.Exec(fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema)).Error; err != nil {
				t.Logf("drop schema %s: %v", schema, err)
			}
		})
	}

	scopedDSN := withSearchPath(dsn, schema)
	gdb, err := gorm.Open(postgres.New(postgres.Config{DSN: scopedDSN}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open scoped connection: %v", err)
	}

	// Bound and close the per-test pool: the suite opens one pool per test, so
	// without a cap the many concurrent schemas exhaust Postgres max_connections.
	if sql, err := gdb.DB(); err == nil {
		sql.SetMaxOpenConns(4)
		sql.SetMaxIdleConns(2)
		t.Cleanup(func() { _ = sql.Close() })
	}

	if migrate {
		if err := db.Migrate(context.Background(), gdb); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return gdb
}

// NewSchemaName returns a fresh schema identifier suitable for an integration
// test that wants to coordinate two connections (e.g. the migration
// contention test). Most tests should use NewPostgres instead.
func NewSchemaName(t *testing.T) string { return newSchemaName(t) }

func newSchemaName(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "test_" + hex.EncodeToString(b[:])
}

func withSearchPath(dsn, schema string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema + ",public"
}
