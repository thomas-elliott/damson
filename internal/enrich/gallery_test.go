package enrich

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/thomas-elliott/damson/internal/metadata"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/storage"
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

// The cap is per asset: images already stored count towards it, so a later
// fetch tops the gallery up to the cap and no further.
func TestApply_GalleryCapCountsImagesAlreadyStored(t *testing.T) {
	db := enrichTestDB(t)
	ctx := context.Background()
	store, err := storage.NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	asset := seedEnrichAsset(t, db, "Pack", false)

	var urls []string
	for i := range 12 {
		urls = append(urls, fmt.Sprintf("https://store.example/%d.png", i))
	}
	deps := Deps{DB: db, Storage: store, Images: &pngFetcher{}}
	for _, batch := range [][]string{urls[:6], urls} {
		if _, err := Apply(ctx, deps, asset, &metadata.Metadata{ImageURLs: batch}, Options{}); err != nil {
			t.Fatal(err)
		}
	}

	var n int64
	if err := db.Model(&models.AssetImage{}).Where("asset_id = ?", asset.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != maxGalleryImages {
		t.Errorf("asset holds %d gallery images, want the cap of %d", n, maxGalleryImages)
	}
}
