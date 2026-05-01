// Package testdb provides shared helpers for tests that need a real database
// instead of the in-memory SQLite default. Currently Postgres-only.
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

// PostgresDSNEnv is the env var that points integration tests at a real
// Postgres instance. Empty / unset → tests skip with t.Skip.
const PostgresDSNEnv = "DAMSON_TEST_POSTGRES_DSN"

// NewPostgres opens a Postgres connection from $DAMSON_TEST_POSTGRES_DSN,
// creates a fresh per-test schema (test_<random>), runs all SQL migrations
// against that schema, and registers a Cleanup to drop it. Each test gets an
// isolated namespace on the same Postgres instance — no per-test container.
//
// Callers should additionally guard the test file with
// `//go:build postgres_integration` so default `go test ./...` ignores it.
// This helper enforces the env-var skip as a second line of defence.
func NewPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(PostgresDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set; skipping Postgres integration test", PostgresDSNEnv)
	}
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
