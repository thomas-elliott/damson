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

const advisoryLockKey int64 = 4242424242

// Migrate applies any pending SQL migrations from database/migrations/ in
// lexicographic order. Applied filenames are tracked in schema_migrations;
// a Postgres advisory lock serialises concurrent startups.
func Migrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	if err := db.WithContext(ctx).Exec(`SELECT pg_advisory_lock(?)`, advisoryLockKey).Error; err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		_ = db.Exec(`SELECT pg_advisory_unlock(?)`, advisoryLockKey).Error
	}()

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
		if err := db.WithContext(ctx).
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
		if err := db.WithContext(ctx).Exec(string(body)).Error; err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if err := db.WithContext(ctx).
			Exec(`INSERT INTO schema_migrations (filename) VALUES (?)`, name).Error; err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}

	return nil
}
