package db_test

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// TestMigrate_AppliesAllSqlFiles is a sanity check that every file under
// database/migrations/ applies cleanly against a fresh Postgres schema. The
// SQLite AutoMigrate path used by unit tests doesn't exercise these files at
// all, so Postgres-only syntax (tsvector columns, plpgsql triggers, GIN
// indexes) only fails here.
func TestMigrate_AppliesAllSqlFiles(t *testing.T) {
	gdb := testdb.NewPostgres(t)

	var count int64
	if err := gdb.Raw(`SELECT count(1) FROM schema_migrations`).Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected schema_migrations rows after Migrate, got 0")
	}

	// Idempotent: a second Migrate against the same schema is a no-op.
	if err := db.Migrate(context.Background(), gdb); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var count2 int64
	if err := gdb.Raw(`SELECT count(1) FROM schema_migrations`).Scan(&count2).Error; err != nil {
		t.Fatalf("count2: %v", err)
	}
	if count2 != count {
		t.Fatalf("schema_migrations row count drifted: %d -> %d", count, count2)
	}
}

// TestMigrate_AdvisoryLockContention spawns two goroutines that call Migrate
// concurrently against the same schema. The pg_advisory_lock(4242424242) call
// must serialise them so each migration row is inserted exactly once — no
// duplicate-PK errors on schema_migrations, no partial application.
func TestMigrate_AdvisoryLockContention(t *testing.T) {
	dsn := testdb.RequireDSN(t)
	schema := testdb.NewSchemaName(t)

	// First connection (migrate=true) creates the schema and registers the
	// drop-on-cleanup hook. We immediately tear down the schema contents so
	// both racers have work to do.
	a := testdb.OpenSchema(t, dsn, schema, true)
	if err := a.Exec(`DROP TABLE IF EXISTS schema_migrations CASCADE`).Error; err != nil {
		t.Fatalf("drop schema_migrations: %v", err)
	}
	for _, table := range []string{
		"audit_log", "asset_tags", "tags", "asset_files", "asset_versions",
		"asset_imports", "project_asset_pins", "projects", "collection_assets",
		"collections", "library_settings", "library_memberships", "libraries",
		"licences", "assets", "users", "server_settings",
	} {
		if err := a.Exec(`DROP TABLE IF EXISTS ` + table + ` CASCADE`).Error; err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}

	// Second connection: migrate=false, no cleanup — same schema, separate
	// session so the advisory lock has something to contend with.
	b := testdb.OpenSchema(t, dsn, schema, false)

	conns := []*gorm.DB{a, b}
	errs := make([]error, len(conns))
	var wg sync.WaitGroup
	wg.Add(len(conns))
	for i, conn := range conns {
		i, conn := i, conn
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			errs[i] = db.Migrate(ctx, conn)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}

	var total, distinct int64
	if err := a.Raw(`SELECT count(1) FROM schema_migrations`).Scan(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if err := a.Raw(`SELECT count(DISTINCT filename) FROM schema_migrations`).Scan(&distinct).Error; err != nil {
		t.Fatalf("distinct count: %v", err)
	}
	if total == 0 || total != distinct {
		t.Fatalf("schema_migrations integrity broken: %d rows, %d distinct filenames", total, distinct)
	}
}

// The advisory lock is session-level, so it must be released on the session
// that took it. A pooled *gorm.DB can run the unlock on another connection,
// where it is a no-op and the lock stays held until that connection closes —
// blocking River's migrator, which takes the same key. This test forces that
// interleaving: just before the unlock, it checks out whichever idle
// connection the pool would hand next (the one that took the lock, if the
// migrator was not pinned) and holds it.
func TestMigrate_ReleasesTheAdvisoryLockItTook(t *testing.T) {
	gdb := testdb.NewPostgres(t)
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}

	var held *sql.Conn
	if err := gdb.Callback().Raw().Before("gorm:raw").Register("test:hold_idle_conn", func(tx *gorm.DB) {
		if held == nil && strings.Contains(tx.Statement.SQL.String(), "pg_advisory_unlock") {
			held, _ = sqlDB.Conn(context.Background())
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}

	if err := db.Migrate(context.Background(), gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if held == nil {
		t.Fatal("the migrator never unlocked")
	}
	defer held.Close()

	// pg_advisory_unlock reports whether this session held the lock.
	var leaked bool
	if err := held.QueryRowContext(context.Background(), `SELECT pg_advisory_unlock($1)`, db.AdvisoryLockKey).Scan(&leaked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if leaked {
		t.Fatal("Migrate returned with the advisory lock still held by a pooled connection")
	}
}
