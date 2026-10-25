package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// LookupFetcher looks a store link up for a background lookup, waiting out
// any pause the store asks for, and downloads the listing's images.
type LookupFetcher interface {
	FetchWaiting(ctx context.Context, rawURL string) (*metadata.Metadata, error)
	ImageFetcher
}

// Lookups runs queued store lookups, the store_lookups rows.
type Lookups struct {
	DB       *gorm.DB
	Storage  storage.Service
	Metadata LookupFetcher
}

// Run runs one queued lookup: it looks the asset's store link up, applies
// what it found, saves the source record and records the outcome on the row,
// all in one transaction. A refresh may replace fields a person hasn't
// changed, and marks the asset Needs review when it changes one; any other
// lookup fills blanks only. An error is returned only for what a retry could
// fix; a lookup the store can't answer is recorded as failed.
func (l *Lookups) Run(ctx context.Context, lookupID uint) error {
	db := l.DB.WithContext(ctx)
	var lk models.StoreLookup
	if err := db.First(&lk, lookupID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // the asset or batch was deleted since
		}
		return err
	}
	if lk.Status != models.StoreLookupQueued {
		return nil
	}
	var batch models.StoreLookupBatch
	if err := db.First(&batch, lk.BatchID).Error; err != nil {
		return err
	}
	var asset models.Asset
	if err := db.Where("id = ? AND library_id = ?", lk.AssetID, lk.LibraryID).First(&asset).Error; err != nil {
		return err
	}
	if asset.SourceURL == "" {
		return finishLookup(db, &lk, models.StoreLookupFailed, "", nil, nil, "the asset has no store link")
	}
	if l.Metadata == nil {
		return fmt.Errorf("store lookup %d: no metadata service", lk.ID)
	}

	meta, err := l.Metadata.FetchWaiting(ctx, asset.SourceURL)
	if err != nil {
		if ctx.Err() != nil {
			return err // shut down or timed out: River tries again
		}
		slog.Warn("store lookup failed", "lookup_id", lk.ID, "asset_id", asset.ID, "url", asset.SourceURL, "err", err)
		return finishLookup(db, &lk, models.StoreLookupFailed, "", nil, nil, err.Error())
	}

	// A generic-scraper answer is not the store's details, so even a refresh
	// only fills blanks with it, and it never replaces a record a store lookup
	// left: the next refresh would read the store's values as a person's.
	plan, err := Prepare(ctx, Deps{DB: l.DB, Storage: l.Storage, Images: l.Metadata}, &asset, meta, Options{
		Refresh: batch.Refresh && !meta.StoreLookupFailed,
		UserID:  batch.CreatedByUserID,
		By:      licence.Automatic,
	})
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		res, err := plan.Write(tx)
		if err != nil {
			return err
		}
		if err := plan.SaveRecord(tx, asset.SourceURL, res.Applied, ConflictFields(res.Conflicts)); err != nil {
			return err
		}
		result := models.StoreLookupUnchanged
		switch {
		case meta.StoreLookupFailed:
			result = models.StoreLookupNoStoreData
		case len(res.Applied) > 0:
			result = models.StoreLookupChanged
		}
		return finishLookup(tx, &lk, models.StoreLookupDone, result, res.Applied, res.Conflicts, "")
	})
}

func finishLookup(tx *gorm.DB, lk *models.StoreLookup, status, result string, applied []string, conflicts []Conflict, errMsg string) error {
	if applied == nil {
		applied = []string{}
	}
	if conflicts == nil {
		conflicts = []Conflict{}
	}
	appliedJSON, err := json.Marshal(applied)
	if err != nil {
		return err
	}
	conflictsJSON, err := json.Marshal(conflicts)
	if err != nil {
		return err
	}
	return tx.Model(lk).Updates(map[string]any{
		"status":    status,
		"result":    result,
		"applied":   json.RawMessage(appliedJSON),
		"conflicts": json.RawMessage(conflictsJSON),
		"error":     errMsg,
	}).Error
}

// Fail records a still-queued lookup failed, for a job River has given up
// retrying.
func (l *Lookups) Fail(ctx context.Context, lookupID uint, reason string) error {
	return l.DB.WithContext(ctx).Model(&models.StoreLookup{}).
		Where("id = ? AND status = ?", lookupID, models.StoreLookupQueued).
		Updates(map[string]any{"status": models.StoreLookupFailed, "error": reason}).Error
}

// Resume re-enqueues every lookup still queued, for one whose enqueue failed
// or whose job was lost. River dedupes by args, so this is safe on every boot.
func (l *Lookups) Resume(ctx context.Context, enq interface{ EnqueueStoreLookup(uint) }) (int, error) {
	var ids []uint
	if err := l.DB.WithContext(ctx).Model(&models.StoreLookup{}).
		Where("status = ?", models.StoreLookupQueued).Order("id").Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	for _, id := range ids {
		enq.EnqueueStoreLookup(id)
	}
	return len(ids), nil
}
