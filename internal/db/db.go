// Package db wires up the Gorm connection and migration runner.
package db

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
		Logger: quietLogger{logger.NewSlogLogger(slog.Default(), logger.Config{
			LogLevel:                  logger.Warn,
			SlowThreshold:             slowQuery,
			ParameterizedQueries:      true,
			IgnoreRecordNotFoundError: true,
		})},
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

// quietLogger drops the failures callers expect and handle: a unique
// violation, which handlers answer with 409, and a query cut short because
// its request went away. A slow one still logs at warn.
type quietLogger struct{ logger.Interface }

func (l quietLogger) LogMode(level logger.LogLevel) logger.Interface {
	return quietLogger{l.Interface.LogMode(level)}
}

func (l quietLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if expectedErr(err) {
		err = nil
	}
	l.Interface.Trace(ctx, begin, fc, err)
}

// ParamsFilter keeps parameter values out of the line; without it the
// wrapper would hide the slog logger's own filter from GORM.
func (quietLogger) ParamsFilter(ctx context.Context, sql string, params ...any) (string, []any) {
	return dropParams(ctx, sql, params...)
}

func expectedErr(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, context.Canceled) || (errors.As(err, &pgErr) && pgErr.Code == "23505")
}
