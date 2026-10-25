package enrich

import (
	"bytes"
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
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

// RepairGallery drops gallery rows whose object is missing from storage and
// fetches those images again from each asset's stored listing. The orphan
// sweep once deleted gallery images (it did not count asset_images as a
// reference), leaving rows that serve nothing. Safe to run on every boot:
// with nothing missing it is one listing of thumbs/ and no fetches.
func RepairGallery(ctx context.Context, db *gorm.DB, store storage.Service, fetch ImageFetcher) error {
	// Rows first, then the listing: an image stored after the rows were read
	// is in the listing, so a row written in between is never counted missing.
	var rows []models.AssetImage
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}
	objs, err := store.List(ctx, storage.Thumbs)
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
		rec := SourceRecordOf(&asset)
		if rec == nil {
			slog.Warn("gallery repair: no readable listing to refetch from", "asset_id", assetID)
			continue
		}
		rows, err := fetchGallery(ctx, Deps{DB: db, Storage: store, Images: fetch}, assetID, rec.Metadata.ImageURLs)
		if err == nil && len(rows) > 0 {
			err = db.WithContext(ctx).Create(&rows).Error
		}
		if err != nil {
			slog.Warn("gallery repair: store images", "err", err, "asset_id", assetID)
		}
	}
	return nil
}
