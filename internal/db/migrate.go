package db

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"gorm.io/gorm"

	damson "github.com/thomas-elliott/damson"
)

// AdvisoryLockKey is the Postgres advisory-lock key that serialises schema
// migrations across concurrent startups. Exported so other migrators (River's)
// can take the same lock and serialise against this one.
const AdvisoryLockKey int64 = 4242424242

// Migrate applies any pending SQL migrations from database/migrations/ in
// lexicographic order. Applied filenames are tracked in schema_migrations;
// a Postgres advisory lock serialises concurrent startups.
func Migrate(ctx context.Context, db *gorm.DB) error {
	// The advisory lock is session-level, so the lock, every migration and the
	// unlock must share one connection. On the pool, the unlock can land on
	// another connection, where it is a no-op and the lock stays held.
	return db.WithContext(ctx).Connection(migrateLocked)
}

func migrateLocked(conn *gorm.DB) error {
	// Acquire the advisory lock first — even before creating
	// schema_migrations. Two concurrent backends running CREATE TABLE IF
	// NOT EXISTS race on pg_type/pg_class inserts and one gets a duplicate-
	// key error on `pg_type_typname_nsp_index`. The lock takes a plain
	// bigint key and doesn't need any table to exist, so it's safe to take
	// here.
	if err := conn.Exec(`SELECT pg_advisory_lock(?)`, AdvisoryLockKey).Error; err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		_ = conn.Exec(`SELECT pg_advisory_unlock(?)`, AdvisoryLockKey).Error
	}()

	if err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(damson.MigrationsFS, "database/migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var count int64
		if err := conn.
			Raw(`SELECT count(1) FROM schema_migrations WHERE filename = ?`, name).
			Scan(&count).Error; err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if count > 0 {
			continue
		}

		slog.Info("applying migration", "name", name)
		body, err := fs.ReadFile(damson.MigrationsFS, "database/migrations/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := conn.Exec(string(body)).Error; err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if err := conn.
			Exec(`INSERT INTO schema_migrations (filename) VALUES (?)`, name).Error; err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}

	return nil
}
