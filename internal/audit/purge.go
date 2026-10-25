package audit

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// Purger sweeps audit_log against per-library
// `library_settings.audit_retention_days`. Server-level events
// (`library_id IS NULL`) are deliberately untouched — `server_settings` has
// no retention column yet (see PER-73).
type Purger struct {
	DB       *gorm.DB
	Interval time.Duration
}

// Run blocks until ctx is cancelled, calling PurgeOnce when it starts and
// then on every Interval tick, so a restart applies retention without
// waiting an Interval. Failures are logged; the loop never aborts.
func (p *Purger) Run(ctx context.Context) {
	if p.Interval <= 0 {
		slog.Warn("audit purge worker: non-positive interval, worker disabled", "interval", p.Interval)
		return
	}
	t := time.NewTicker(p.Interval)
	defer t.Stop()
	slog.Info("audit purge worker started", "interval", p.Interval)
	for {
		if _, err := PurgeOnce(ctx, p.DB, time.Now()); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("audit purge sweep failed", "err", err)
		}
		select {
		case <-ctx.Done():
			slog.Info("audit purge worker stopping")
			return
		case <-t.C:
		}
	}
}

// PurgeOnce performs a single sweep: for every library with a positive
// `audit_retention_days`, deletes audit_log rows older than that window.
// Returns the total number of rows deleted across all libraries.
//
// One DELETE per library avoids holding a single statement open across
// thousands of libraries.
func PurgeOnce(ctx context.Context, db *gorm.DB, now time.Time) (int64, error) {
	type rule struct {
		LibraryID uint
		Days      int
	}
	var rules []rule
	if err := db.WithContext(ctx).
		Table("library_settings").
		Select("library_id, audit_retention_days AS days").
		Where("audit_retention_days IS NOT NULL AND audit_retention_days > 0").
		Scan(&rules).Error; err != nil {
		return 0, err
	}

	var total int64
	for _, r := range rules {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		cutoff := now.Add(-time.Duration(r.Days) * 24 * time.Hour)
		res := db.WithContext(ctx).
			Where("library_id = ? AND created_at < ?", r.LibraryID, cutoff).
			Delete(&models.AuditLog{})
		if res.Error != nil {
			slog.Warn("audit purge: delete failed",
				"library_id", r.LibraryID,
				"err", res.Error)
			continue
		}
		if res.RowsAffected > 0 {
			slog.Info("audit purge: library swept",
				"library_id", r.LibraryID,
				"deleted", res.RowsAffected,
				"older_than_days", r.Days)
			total += res.RowsAffected
		}
	}
	if total > 0 {
		slog.Info("audit purge sweep complete", "total_deleted", total, "libraries", len(rules))
	}
	return total, nil
}
