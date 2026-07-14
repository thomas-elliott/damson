package imports

import (
	"bytes"
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

// maxGalleryImages caps how many gallery images one fetch/enrich stores.
const maxGalleryImages = 8

// imageFetcher is the download surface gallery storage needs — the minimal
// slice of the metadata service both callers already provide.
type imageFetcher interface {
	FetchImage(ctx context.Context, rawURL string) ([]byte, string, error)
}

// StoreGalleryImages downloads listing gallery images, stores each as a raster
// thumbnail blob, and writes asset_images rows — skipping any source URL
// already stored for the asset. It is best-effort: an individual download or
// render failure is logged and skipped, never returned.
func StoreGalleryImages(ctx context.Context, db *gorm.DB, store storage.Service, fetch imageFetcher, assetID uint, urls []string) {
	if db == nil || store == nil || fetch == nil || len(urls) == 0 {
		return
	}

	var existing []string
	if err := db.WithContext(ctx).Model(&models.AssetImage{}).
		Where("asset_id = ?", assetID).Pluck("source_url", &existing).Error; err != nil {
		slog.Warn("gallery: load existing", "err", err, "asset_id", assetID)
		return
	}
	seen := make(map[string]bool, len(existing))
	for _, u := range existing {
		seen[u] = true
	}
	order := len(existing)

	gen := &thumbnail.Generator{Storage: store}
	stored := 0
	for _, u := range urls {
		if stored >= maxGalleryImages {
			break
		}
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true

		imgBytes, contentType, err := fetch.FetchImage(ctx, u)
		if err != nil {
			slog.Warn("gallery: fetch image", "err", err, "url", u)
			continue
		}
		res, err := gen.Generate(ctx, thumbnail.Input{
			Reader:      bytes.NewReader(imgBytes),
			Filename:    "gallery",
			ContentType: contentType,
		}, thumbnail.Toggles{Raster: true})
		if err != nil || res.ObjectKey == "" {
			if err != nil {
				slog.Warn("gallery: render image", "err", err, "url", u)
			}
			continue
		}
		row := models.AssetImage{
			AssetID:     assetID,
			SourceURL:   u,
			ObjectKey:   res.ObjectKey,
			ContentType: res.ContentType,
			SortOrder:   order,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			slog.Warn("gallery: persist image", "err", err, "url", u)
			continue
		}
		order++
		stored++
	}
}
