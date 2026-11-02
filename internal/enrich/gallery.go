package enrich

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/thumbnail"
)

// maxGalleryImages caps how many gallery images one asset holds.
const maxGalleryImages = 8

// fetchGallery downloads the listing's gallery images the asset doesn't hold
// yet, up to maxGalleryImages in all, and stores each as a raster thumbnail.
// It returns the asset_images rows to write. A download or render that fails
// is logged and skipped.
func fetchGallery(ctx context.Context, deps Deps, assetID uint, urls []string) ([]models.AssetImage, error) {
	if deps.Storage == nil || deps.Images == nil || len(urls) == 0 {
		return nil, nil
	}
	var existing []string
	if err := deps.DB.WithContext(ctx).Model(&models.AssetImage{}).
		Where("asset_id = ?", assetID).Pluck("source_url", &existing).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(existing))
	for _, u := range existing {
		seen[u] = true
	}

	gen := &thumbnail.Generator{Storage: deps.Storage}
	var rows []models.AssetImage
	for _, u := range urls {
		if len(existing)+len(rows) >= maxGalleryImages {
			break
		}
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true

		imgBytes, contentType, err := deps.Images.FetchImage(ctx, u)
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
		rows = append(rows, models.AssetImage{
			AssetID:     assetID,
			SourceURL:   u,
			ObjectKey:   res.ObjectKey,
			ContentType: res.ContentType,
			SortOrder:   len(existing) + len(rows),
		})
	}
	return rows, nil
}
