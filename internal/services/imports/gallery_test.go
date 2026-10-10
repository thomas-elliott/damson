package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// pngFetcher answers every gallery URL with a small PNG.
type pngFetcher struct{ calls []string }

func (f *pngFetcher) FetchImage(_ context.Context, url string) ([]byte, string, error) {
	f.calls = append(f.calls, url)
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: uint8(len(f.calls)), A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

// The orphan sweep used to delete gallery images. Their rows still point at
// the missing objects; the repair drops those rows and fetches the images
// again from the stored listing, leaving intact rows alone.
func TestRepairGallery_RefetchesImagesWhoseObjectIsGone(t *testing.T) {
	db := enrichTestDB(t)
	ctx := context.Background()
	store, err := storage.NewLocalFS(t.TempDir(), "")
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	asset := seedEnrichAsset(t, db, "Pack", false)
	rec, _ := json.Marshal(metadata.SourceRecord{Metadata: &metadata.Metadata{
		ImageURLs: []string{"https://store.example/kept.png", "https://store.example/swept.png"},
	}})
	if err := db.Model(asset).Update("source_metadata_json", rec).Error; err != nil {
		t.Fatalf("seed source record: %v", err)
	}

	kept := "1111111111111111111111111111111111111111111111111111111111111111"
	if err := store.PutThumb(ctx, kept, bytes.NewReader([]byte("kept"))); err != nil {
		t.Fatalf("put kept: %v", err)
	}
	swept := "2222222222222222222222222222222222222222222222222222222222222222"
	for i, row := range []models.AssetImage{
		{AssetID: asset.ID, SourceURL: "https://store.example/kept.png", ObjectKey: kept, SortOrder: 0},
		{AssetID: asset.ID, SourceURL: "https://store.example/swept.png", ObjectKey: swept, SortOrder: 1},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	fetch := &pngFetcher{}
	if err := RepairGallery(ctx, db, store, fetch); err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(fetch.calls) != 1 || fetch.calls[0] != "https://store.example/swept.png" {
		t.Fatalf("fetched %v, want only the swept image", fetch.calls)
	}
	var rows []models.AssetImage
	db.Where("asset_id = ?", asset.ID).Order("source_url").Find(&rows)
	if len(rows) != 2 {
		t.Fatalf("got %d gallery rows, want 2", len(rows))
	}
	for _, r := range rows {
		if _, exists, _ := store.Stat(ctx, "thumbs", r.ObjectKey); !exists {
			t.Errorf("gallery row %s still points at a missing object", r.SourceURL)
		}
	}
	if rows[0].ObjectKey != kept {
		t.Errorf("the intact row was rewritten")
	}
}

// Started against empty storage (a forgotten volume, a new bucket), every
// cover is missing as well as every gallery image. The repair must leave the
// rows alone rather than drop galleries the storefront may no longer serve.
func TestRepairGallery_LeavesRowsAloneWhenStorageIsNotTheDatabases(t *testing.T) {
	db := enrichTestDB(t)
	ctx := context.Background()
	store, err := storage.NewLocalFS(t.TempDir(), "")
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	asset := seedEnrichAsset(t, db, "Pack", false)
	cover := "3333333333333333333333333333333333333333333333333333333333333333"
	if err := db.Model(asset).Update("thumbnail_object_key", cover).Error; err != nil {
		t.Fatalf("seed cover: %v", err)
	}
	gallery := "4444444444444444444444444444444444444444444444444444444444444444"
	row := models.AssetImage{AssetID: asset.ID, SourceURL: "https://store.example/a.png", ObjectKey: gallery}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	fetch := &pngFetcher{}
	if err := RepairGallery(ctx, db, store, fetch); err != nil {
		t.Fatalf("repair: %v", err)
	}

	if len(fetch.calls) != 0 {
		t.Errorf("fetched %v, want nothing", fetch.calls)
	}
	var kept models.AssetImage
	if err := db.First(&kept, row.ID).Error; err != nil || kept.ObjectKey != gallery {
		t.Errorf("gallery row was dropped or rewritten: %v", err)
	}
}
