// Package uploads holds the background maintenance for resumable upload
// sessions. The HTTP surface lives in internal/handlers/api.
package uploads

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/storage"
)

// Janitor expires abandoned upload sessions. Their backing multipart uploads
// accrue storage that no bucket listing reveals, so nothing else reclaims them.
type Janitor struct {
	DB       *gorm.DB
	Storage  storage.Service
	Interval time.Duration
}

// Run blocks until ctx is cancelled, sweeping on every Interval tick. The
// first sweep happens after one Interval. Failures are logged; the loop never
// aborts.
func (j *Janitor) Run(ctx context.Context) {
	if j.Interval <= 0 {
		slog.Warn("upload janitor: non-positive interval, worker disabled", "interval", j.Interval)
		return
	}
	if j.DB == nil || j.Storage == nil {
		slog.Warn("upload janitor: no database or storage, worker disabled")
		return
	}
	t := time.NewTicker(j.Interval)
	defer t.Stop()
	slog.Info("upload janitor started", "interval", j.Interval)
	for {
		select {
		case <-ctx.Done():
			slog.Info("upload janitor stopping")
			return
		case <-t.C:
			if _, err := SweepOnce(ctx, j.DB, j.Storage, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("upload janitor sweep failed", "err", err)
			}
		}
	}
}

// SweepOnce aborts the backing multipart upload of every open session past
// its expiry and marks it expired. Returns how many sessions were swept.
//
// A session whose multipart will not abort is still marked expired: the status
// is what stops the client resuming, and the leftover objects belong to the
// orphan sweep.
func SweepOnce(ctx context.Context, db *gorm.DB, store storage.Service, now time.Time) (int, error) {
	var stale []models.UploadSession
	if err := db.WithContext(ctx).
		Where("status = ? AND expires_at < ?", models.UploadOpen, now).
		Find(&stale).Error; err != nil {
		return 0, err
	}

	swept := 0
	for _, s := range stale {
		if err := ctx.Err(); err != nil {
			return swept, err
		}
		up, err := store.ResumeMultipart(ctx, storage.Prefix(s.ObjectPrefix), s.Sha256, s.BackendUploadID)
		if err != nil {
			slog.Warn("upload janitor: could not reattach to abort",
				"err", err, "upload_id", s.ID, "backend_upload_id", s.BackendUploadID)
		} else if err := up.Abort(ctx); err != nil {
			slog.Warn("upload janitor: multipart left behind",
				"err", err, "upload_id", s.ID, "backend_upload_id", s.BackendUploadID)
		}
		if err := db.WithContext(ctx).Model(&models.UploadSession{}).
			Where("id = ? AND status = ?", s.ID, models.UploadOpen).
			Updates(map[string]any{"status": models.UploadExpired, "updated_at": now}).Error; err != nil {
			slog.Warn("upload janitor: could not mark expired", "err", err, "upload_id", s.ID)
			continue
		}
		slog.Info("upload session expired",
			"upload_id", s.ID, "library_id", s.LibraryID,
			"received_bytes", s.ReceivedBytes, "size_bytes", s.SizeBytes)
		swept++
	}
	if swept > 0 {
		slog.Info("upload janitor sweep complete", "expired", swept)
	}
	return swept, nil
}
