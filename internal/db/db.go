// Package db wires up the Gorm connection and migration runner.
package db

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Pool limits for the GORM pool. River keeps its own pgx pool beside it.
const (
	maxOpenConns    = 20
	maxIdleConns    = 10
	connMaxLifetime = 30 * time.Minute
	connMaxIdleTime = 5 * time.Minute
)

// slowQuery is the duration above which a query is logged at warn.
const slowQuery = 200 * time.Millisecond

// Open connects to Postgres. GORM logs through slog.Default, so its lines are
// JSON like the rest, and with placeholders in place of parameter values,
// which can be emails, token digests or anything a user typed.
func Open(dsn string) (*gorm.DB, error) {
	// Scan swaps in GORM's trace recorder, which ignores ParameterizedQueries
	// and filters through this package-level hook instead.
	logger.RecorderParamsFilter = dropParams
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.NewSlogLogger(slog.Default(), logger.Config{
			LogLevel:                  logger.Warn,
			SlowThreshold:             slowQuery,
			ParameterizedQueries:      true,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	return gormDB, nil
}

func dropParams(_ context.Context, sql string, _ ...any) (string, []any) {
	return sql, nil
}
