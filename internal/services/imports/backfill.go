package imports

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imaging"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

// browserCoverTypes are the cover content types a browser shows.
var browserCoverTypes = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}

// BackfillCovers gives a grid cover to assets that have none a browser can
// show but hold an image the decoders now read: files imported before the
// tier-2 formats, and zips whose cover was once stored raw (a PSD, say). The
// cover is rendered from the first image file, by path, in the asset's
// current version, as an import would. A store cover is never touched. Safe
// to run on every boot; an image that still fails to decode is tried again
// on the next one.
func BackfillCovers(ctx context.Context, db *gorm.DB, store storage.Service) (int, error) {
	type candidate struct {
		AssetID      uint
		RelativePath string
		ObjectKey    string
	}
	var rows []candidate
	err := db.WithContext(ctx).
		Table("assets AS a").
		Joins("JOIN asset_files f ON f.asset_version_id = a.current_version_id").
		Joins("LEFT JOIN library_settings s ON s.library_id = a.library_id").
		Where("NOT a.is_reference AND NOT a.thumbnail_from_store").
		Where("a.thumbnail_object_key = '' OR a.thumbnail_content_type NOT IN ?", browserCoverTypes).
		Where("COALESCE(s.thumbnails_raster_enabled, true)").
		Order("a.id, f.relative_path").
		Select("a.id AS asset_id, f.relative_path, f.object_key").
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}

	gen := &thumbnail.Generator{Storage: store}
	covered := 0
	done := map[uint]bool{}
	for _, r := range rows {
		if done[r.AssetID] || !imaging.IsImage(r.RelativePath) {
			continue
		}
		res, err := renderCover(ctx, gen, store, r.RelativePath, r.ObjectKey)
		if err != nil {
			slog.Warn("cover backfill: render", "err", err, "asset_id", r.AssetID, "path", r.RelativePath)
			continue
		}
		if res.ObjectKey == "" {
			continue
		}
		done[r.AssetID] = true
		err = db.WithContext(ctx).Model(&models.Asset{}).
			Where("id = ? AND NOT thumbnail_from_store", r.AssetID).
			Updates(map[string]any{"thumbnail_object_key": res.ObjectKey, "thumbnail_content_type": res.ContentType}).Error
		if err != nil {
			slog.Warn("cover backfill: save", "err", err, "asset_id", r.AssetID)
			continue
		}
		covered++
	}
	return covered, nil
}

func renderCover(ctx context.Context, gen *thumbnail.Generator, store storage.Service, path, key string) (thumbnail.Result, error) {
	blob, err := store.GetBlob(ctx, key)
	if err != nil {
		return thumbnail.Result{}, err
	}
	defer blob.Close()
	return gen.Generate(ctx, thumbnail.Input{Reader: blob, Filename: path}, thumbnail.Toggles{Raster: true})
}
