package preview

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// Publisher receives a per-version rollup whenever a file's preview status
// changes. The Broadcaster (pool.go) implements it; a nil Publisher disables
// live updates without affecting persistence.
type Publisher interface {
	Publish(libraryID uint, ev Event)
}

// Deps are the collaborators preview processing needs. Broadcaster is optional.
type Deps struct {
	DB          *gorm.DB
	Storage     storage.Service
	Broadcaster Publisher
}

// MarkVersionQueued classifies a version's files and writes their starting
// preview status: supported files become 'queued', everything else
// 'unsupported'. With fileID != 0 only that file is (re)classified. Returns the
// number of files left queued so callers can skip enqueuing a no-op job. Used
// by both the import handoff and the requeue endpoints, so the supported-set
// rule lives in exactly one place.
func MarkVersionQueued(ctx context.Context, db *gorm.DB, versionID, fileID uint) (int, error) {
	q := db.WithContext(ctx).Where("asset_version_id = ?", versionID)
	if fileID != 0 {
		q = q.Where("id = ?", fileID)
	}
	var files []models.AssetFile
	if err := q.Find(&files).Error; err != nil {
		return 0, err
	}
	queued := 0
	for _, f := range files {
		status := StatusUnsupported
		if Supports(f.RelativePath, f.DetectedType) {
			status = StatusQueued
			queued++
		}
		if err := db.WithContext(ctx).Model(&models.AssetFile{}).
			Where("id = ?", f.ID).
			Updates(map[string]any{
				"preview_status":     status,
				"preview_updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return queued, err
		}
	}
	return queued, nil
}

// ProcessVersion renders previews for a version's queued files, driving each
// from queued → processing → done/failed and publishing a rollup after every
// transition. With fileID != 0 only that file is processed (single-file
// requeue). Best-effort per file: a render error marks the file failed and
// moves on, never aborting the batch.
func ProcessVersion(ctx context.Context, deps Deps, libraryID, versionID, fileID uint) error {
	gen := &Generator{Storage: deps.Storage}

	q := deps.DB.WithContext(ctx).
		Where("asset_version_id = ? AND preview_status = ?", versionID, StatusQueued)
	if fileID != 0 {
		q = q.Where("id = ?", fileID)
	}
	var files []models.AssetFile
	if err := q.Find(&files).Error; err != nil {
		return err
	}
	if len(files) == 0 {
		// Nothing queued (e.g. all unsupported, or already processed). Still
		// emit a rollup so a subscriber sees the terminal state.
		deps.publish(ctx, libraryID, versionID)
		return nil
	}

	for _, f := range files {
		// Only flip the status while rendering — leave any existing derivative
		// key in place so a requeue doesn't flash the UI to "no preview".
		deps.setStatus(ctx, f.ID, StatusProcessing)
		deps.publish(ctx, libraryID, versionID)

		key, ctype, ok := render(ctx, gen, deps.Storage, f)
		if ok {
			deps.setDone(ctx, f.ID, key, ctype)
		} else {
			deps.setStatus(ctx, f.ID, StatusFailed)
		}
		deps.publish(ctx, libraryID, versionID)
	}
	return nil
}

// render fetches a file's bytes and produces its derivative. Returns ok=false
// (the caller marks the file failed) on any fetch/render error or when the
// renderer declines to produce output (e.g. dimensions over the bomb ceiling).
func render(ctx context.Context, gen *Generator, store storage.Service, f models.AssetFile) (key, contentType string, ok bool) {
	blob, err := store.GetBlob(ctx, f.ObjectKey)
	if err != nil {
		slog.Warn("preview: fetch blob", "err", err, "object_key", f.ObjectKey)
		return "", "", false
	}
	defer blob.Close()

	res, err := gen.Generate(ctx, Input{
		Reader:       blob,
		Filename:     f.RelativePath,
		DetectedType: f.DetectedType,
	})
	if err != nil {
		slog.Warn("preview: generate", "err", err, "path", f.RelativePath)
		return "", "", false
	}
	if res.ObjectKey == "" {
		slog.Warn("preview: no derivative produced", "path", f.RelativePath)
		return "", "", false
	}
	return res.ObjectKey, res.ContentType, true
}

// setStatus persists a file's preview status without touching its derivative
// key — used for the processing and failed transitions, so an existing preview
// survives a requeue until a fresh one overwrites it on 'done'.
func (d Deps) setStatus(ctx context.Context, fileID uint, status string) {
	if err := d.DB.WithContext(ctx).Model(&models.AssetFile{}).
		Where("id = ?", fileID).
		Updates(map[string]any{
			"preview_status":     status,
			"preview_updated_at": time.Now().UTC(),
		}).Error; err != nil {
		slog.Warn("preview: persist status", "err", err, "file_id", fileID, "status", status)
	}
}

// setDone records a successful render: status plus the new derivative key and
// content type, in one write.
func (d Deps) setDone(ctx context.Context, fileID uint, key, contentType string) {
	if err := d.DB.WithContext(ctx).Model(&models.AssetFile{}).
		Where("id = ?", fileID).
		Updates(map[string]any{
			"preview_status":       StatusDone,
			"preview_object_key":   key,
			"preview_content_type": contentType,
			"preview_updated_at":   time.Now().UTC(),
		}).Error; err != nil {
		slog.Warn("preview: persist done", "err", err, "file_id", fileID)
	}
}

// publish computes the version's current rollup and forwards it to the
// broadcaster. No-op when no broadcaster is wired.
func (d Deps) publish(ctx context.Context, libraryID, versionID uint) {
	if d.Broadcaster == nil {
		return
	}
	ev, err := computeRollup(ctx, d.DB, libraryID, versionID)
	if err != nil {
		slog.Warn("preview: compute rollup", "err", err, "version_id", versionID)
		return
	}
	d.Broadcaster.Publish(libraryID, ev)
}

// rollupRow is the aggregate-query projection shared by computeRollup and
// InFlight.
type rollupRow struct {
	VersionID   uint
	AssetID     uint
	AssetName   string
	Total       int
	Done        int
	Failed      int
	Processing  int
	Queued      int
	Unsupported int
	CreatedAt   time.Time
	// UpdatedAt is MAX(preview_updated_at) over the version's files. NullTime
	// because the aggregate is NULL when no file has been touched yet.
	UpdatedAt sql.NullTime
}

func (r rollupRow) toEvent() Event {
	updated := time.Now().UTC()
	if r.UpdatedAt.Valid && !r.UpdatedAt.Time.IsZero() {
		updated = r.UpdatedAt.Time
	}
	return Event{
		VersionID:   r.VersionID,
		AssetID:     r.AssetID,
		AssetName:   r.AssetName,
		Status:      rollupStatus(r.Processing, r.Queued),
		Total:       r.Total,
		Done:        r.Done,
		Failed:      r.Failed,
		Processing:  r.Processing,
		Queued:      r.Queued,
		Unsupported: r.Unsupported,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   updated,
	}
}

// previewCountsSelect builds the per-status COUNT/SUM projection used by both
// the single-version rollup and the library-wide snapshot.
const previewCountsSelect = `v.id AS version_id, a.id AS asset_id, a.name AS asset_name, a.created_at,
	COUNT(*) AS total,
	MAX(f.preview_updated_at) AS updated_at,
	SUM(CASE WHEN f.preview_status = 'done' THEN 1 ELSE 0 END) AS done,
	SUM(CASE WHEN f.preview_status = 'failed' THEN 1 ELSE 0 END) AS failed,
	SUM(CASE WHEN f.preview_status = 'processing' THEN 1 ELSE 0 END) AS processing,
	SUM(CASE WHEN f.preview_status = 'queued' THEN 1 ELSE 0 END) AS queued,
	SUM(CASE WHEN f.preview_status = 'unsupported' THEN 1 ELSE 0 END) AS unsupported`

// computeRollup aggregates one version's per-file statuses into an Event,
// library-gated. A version with no files matches no rows, leaving a zero-count
// Event — still tagged with versionID so a terminal "all done" rollup is
// addressable.
func computeRollup(ctx context.Context, db *gorm.DB, libraryID, versionID uint) (Event, error) {
	var row rollupRow
	err := db.WithContext(ctx).
		Table("asset_files AS f").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("a.library_id = ? AND v.id = ?", libraryID, versionID).
		Group("v.id, a.id, a.name, a.created_at").
		Select(previewCountsSelect).
		Scan(&row).Error
	if err != nil {
		return Event{}, err
	}
	row.VersionID = versionID
	return row.toEvent(), nil
}

// InFlight returns the rollups for a library's versions that still have queued
// or processing files — the live processing queue. Powers the snapshot fetch
// and the SSE replay.
func InFlight(ctx context.Context, db *gorm.DB, libraryID uint) ([]Event, error) {
	var rows []rollupRow
	err := db.WithContext(ctx).
		Table("asset_files AS f").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("a.library_id = ?", libraryID).
		Group("v.id, a.id, a.name, a.created_at").
		Having("SUM(CASE WHEN f.preview_status IN ('queued','processing') THEN 1 ELSE 0 END) > 0").
		Select(previewCountsSelect).
		Order("a.created_at DESC").
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

// Finished returns the rollups for a library's versions whose preview work is
// complete — no queued or processing files, but at least one file that reached
// a terminal render (done or failed). Retained for the processing queue's
// "finished" section: only rows touched since `since`, newest-finished first,
// capped at `limit`. Pure-unsupported versions (nothing ever rendered) are
// excluded so the section shows real completed work, not inert manifests.
func Finished(ctx context.Context, db *gorm.DB, libraryID uint, since time.Time, limit int) ([]Event, error) {
	var rows []rollupRow
	err := db.WithContext(ctx).
		Table("asset_files AS f").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("a.library_id = ?", libraryID).
		Group("v.id, a.id, a.name, a.created_at").
		Having("SUM(CASE WHEN f.preview_status IN ('queued','processing') THEN 1 ELSE 0 END) = 0 "+
			"AND SUM(CASE WHEN f.preview_status IN ('done','failed') THEN 1 ELSE 0 END) > 0 "+
			"AND MAX(f.preview_updated_at) >= ?", since).
		Select(previewCountsSelect).
		Order("MAX(f.preview_updated_at) DESC").
		Limit(limit).
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
