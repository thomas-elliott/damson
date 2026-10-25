package preview

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imaging"
)

// Job is the unit of work for the preview pool: a version to (re)process, and
// optionally a single file within it (file-level requeue). FileID == 0 means
// every queued file in the version.
type Job struct {
	LibraryID uint
	VersionID uint
	FileID    uint
}

// ResumeInFlight recovers preview work interrupted by a crash or deploy. Files
// stuck in 'processing' are reset to 'queued' (the worker died mid-render), then
// every version with queued files is re-enqueued through enq. ProcessVersion is
// idempotent — content-addressed derivatives mean a re-run is cheap and safe;
// the River enqueuer additionally dedupes so this is safe to run every boot.
func ResumeInFlight(ctx context.Context, db *gorm.DB, enq Enqueuer) (int, error) {
	if err := db.WithContext(ctx).Model(&models.AssetFile{}).
		Where("preview_status = ?", StatusProcessing).
		Update("preview_status", StatusQueued).Error; err != nil {
		return 0, err
	}

	type ref struct {
		VersionID uint
		LibraryID uint
	}
	var refs []ref
	err := db.WithContext(ctx).
		Table("asset_files AS f").
		Joins("JOIN asset_versions v ON v.id = f.asset_version_id").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("f.preview_status = ?", StatusQueued).
		Group("v.id, a.library_id").
		Select("v.id AS version_id, a.library_id AS library_id").
		Scan(&refs).Error
	if err != nil {
		return 0, err
	}
	for _, r := range refs {
		if err := enq.Enqueue(db.WithContext(ctx), Job{LibraryID: r.LibraryID, VersionID: r.VersionID}); err != nil {
			return 0, err
		}
	}
	return len(refs), nil
}

// QueueNewlySupported queues the files an earlier build marked unsupported
// that a renderer now exists for, such as the tier-2 image formats, in
// libraries with previews on. Run it before ResumeInFlight, which enqueues
// them. Once queued a file is no longer unsupported, so a second run finds
// nothing.
func QueueNewlySupported(ctx context.Context, db *gorm.DB) (int, error) {
	// The extensions classify renders, as a Postgres regex: \.(svg|psd|...)$.
	// They are letters and digits, so none needs escaping.
	var exts []string
	for _, ext := range append([]string{".svg"}, imaging.ConvertedExtensions()...) {
		exts = append(exts, strings.TrimPrefix(ext, "."))
	}
	pattern := `\.(` + strings.Join(exts, "|") + `)$`

	previewsOn := db.Table("asset_versions AS v").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Joins("LEFT JOIN library_settings s ON s.library_id = a.library_id").
		Where("COALESCE(s.previews_enabled, true)").
		Select("v.id")
	res := db.WithContext(ctx).Model(&models.AssetFile{}).
		Where("preview_status = ? AND lower(relative_path) ~ ?", StatusUnsupported, pattern).
		Where("asset_version_id IN (?)", previewsOn).
		Updates(map[string]any{"preview_status": StatusQueued, "preview_updated_at": time.Now().UTC()})
	return int(res.RowsAffected), res.Error
}
