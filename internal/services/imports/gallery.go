package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

// maxGalleryImages caps how many gallery images one asset holds.
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
	stored := len(existing)
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

// RepairGallery drops gallery rows whose object is missing from storage and
// fetches those images again from each asset's stored listing. The orphan
// sweep once deleted gallery images (it did not count asset_images as a
// reference), leaving rows that serve nothing. Safe to run on every boot:
// with nothing missing it is one listing of thumbs/ and no fetches.
func RepairGallery(ctx context.Context, db *gorm.DB, store storage.Service, fetch imageFetcher) error {
	// Rows first, then the listing: an image stored after the rows were read
	// is in the listing, so a row written in between is never counted missing.
	var rows []models.AssetImage
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}
	objs, err := store.List(ctx, "thumbs")
	if err != nil {
		return err
	}
	present := make(map[string]bool, len(objs))
	for _, o := range objs {
		present[o.Key] = true
	}
	var missing []uint
	assets := map[uint]bool{}
	for _, r := range rows {
		if !present[r.ObjectKey] {
			missing = append(missing, r.ID)
			assets[r.AssetID] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}
	// The sweep never deleted asset covers, so most of them missing means this
	// storage is not the one the database describes — a forgotten /data
	// volume, a new bucket — not a swept gallery. Dropping rows then would
	// lose every gallery whose storefront link has since died.
	var covers []string
	if err := db.WithContext(ctx).Model(&models.Asset{}).
		Where("thumbnail_object_key <> ''").Pluck("thumbnail_object_key", &covers).Error; err != nil {
		return err
	}
	coversPresent := 0
	for _, k := range covers {
		if present[k] {
			coversPresent++
		}
	}
	if coversPresent*2 < len(covers) {
		slog.Warn("gallery repair: skipped, most asset covers are missing from storage too",
			"covers", len(covers), "covers_present", coversPresent, "gallery_rows_missing", len(missing))
		return nil
	}
	if err := db.WithContext(ctx).Delete(&models.AssetImage{}, missing).Error; err != nil {
		return err
	}
	slog.Info("gallery repair: dropped rows with a missing object", "rows", len(missing), "assets", len(assets))

	for assetID := range assets {
		var asset models.Asset
		if err := db.WithContext(ctx).Select("id", "source_metadata_json").First(&asset, assetID).Error; err != nil {
			slog.Warn("gallery repair: load asset", "err", err, "asset_id", assetID)
			continue
		}
		var rec metadata.SourceRecord
		if err := json.Unmarshal(asset.SourceMetadataJSON, &rec); err != nil || rec.Metadata == nil {
			slog.Warn("gallery repair: no readable listing to refetch from", "asset_id", assetID)
			continue
		}
		StoreGalleryImages(ctx, db, store, fetch, assetID, rec.Metadata.ImageURLs)
	}
	return nil
}
