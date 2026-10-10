package imports

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// ResumeInFlight scans asset_imports for rows in a non-terminal stage and
// re-enqueues them through enq. Used at server startup to reconcile work
// interrupted by a crash or a deploy, and to enqueue rows that predate River.
// Rows whose source blob can't be located (e.g. the version row never received
// an OriginalPackageObjectKey) are marked failed so they don't sit forever.
//
// With the River enqueuer, re-enqueue is deduped by args so running this every
// boot (and alongside the handler enqueue paths) is safe. The pipeline itself is
// idempotent — re-running against an existing row re-extracts/re-thumbnails into
// the same content-addressed blobs and rewrites asset_files for the version.
func ResumeInFlight(ctx context.Context, db *gorm.DB, enq interface{ Enqueue(Job) }) (int, error) {
	type joined struct {
		ImportID                 uint
		LibraryID                uint
		OriginalPackageObjectKey string
	}
	var rows []joined
	err := db.WithContext(ctx).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Joins("LEFT JOIN asset_versions v ON v.id = i.asset_version_id").
		Where("i.stage NOT IN ?", []string{StageDone, StageFailed}).
		Select("i.id AS import_id, a.library_id, v.original_package_object_key").
		Order("i.id ASC").
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}

	enqueued := 0
	for _, r := range rows {
		if r.OriginalPackageObjectKey == "" {
			if err := MarkImportFailed(db.WithContext(ctx), r.ImportID, "server restart: source blob unavailable"); err != nil {
				slog.Warn("resume: mark failed", "import_id", r.ImportID, "err", err)
			}
			continue
		}
		enq.Enqueue(Job{ImportID: r.ImportID, LibraryID: r.LibraryID})
		enqueued++
	}
	return enqueued, nil
}

type eventRow struct {
	ID          uint
	AssetID     uint
	AssetName   string
	Stage       string
	Error       string
	UpdatedAt   time.Time
	CreatedAt   time.Time
	NeedsReview bool
}

func (r eventRow) toEvent() Event {
	return Event{
		ID:          r.ID,
		AssetID:     r.AssetID,
		AssetName:   r.AssetName,
		Stage:       r.Stage,
		Error:       r.Error,
		UpdatedAt:   r.UpdatedAt,
		CreatedAt:   r.CreatedAt,
		NeedsReview: r.NeedsReview,
	}
}

// ReplaySnapshot returns the current non-terminal imports for the given
// library, packaged as Events. SSE handlers call this on subscribe so a
// fresh client paints the live state without waiting for the next stage
// transition.
func ReplaySnapshot(ctx context.Context, db *gorm.DB, libraryID uint) ([]Event, error) {
	var rows []eventRow
	err := db.WithContext(ctx).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("a.library_id = ? AND i.stage NOT IN ?", libraryID, []string{StageDone, StageFailed}).
		Select("i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at, a.created_at, a.needs_review").
		Order("i.updated_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toEvent())
	}
	return out, nil
}

// LoadEvent fetches the wire-shape Event for one import id, library-gated.
// Returns ok=false if the row doesn't exist or belongs to another library.
func LoadEvent(ctx context.Context, db *gorm.DB, libraryID, importID uint) (Event, bool, error) {
	var r eventRow
	err := db.WithContext(ctx).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("i.id = ? AND a.library_id = ?", importID, libraryID).
		Select("i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at, a.created_at, a.needs_review").
		Limit(1).
		Scan(&r).Error
	if err != nil {
		return Event{}, false, err
	}
	if r.ID == 0 {
		return Event{}, false, nil
	}
	return r.toEvent(), true, nil
}
