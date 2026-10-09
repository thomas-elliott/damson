package api

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

// TestPostImport_GeneratesRasterThumbnail asserts the M6 thumbnail pipeline
// runs synchronously inside the import handler when the upload is a raster
// image and the per-library toggle is on (the default).
func TestPostImport_GeneratesRasterThumbnail(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)

	pngBytes := makeTestPNG(t, 800, 600)
	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "tile.png", pngBytes)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.ThumbnailObjectKey == "" {
		t.Fatalf("expected thumbnail_object_key to be set, got empty")
	}
	if a.ThumbnailContentType != "image/png" && a.ThumbnailContentType != "image/jpeg" {
		t.Fatalf("unexpected content type: %q", a.ThumbnailContentType)
	}
}

// TestPostImport_RasterToggleOff asserts disabling the raster toggle on the
// library settings prevents thumbnail generation. We pre-create the row so
// loadOrCreateLibrarySettings finds it and respects the false toggle.
func TestPostImport_RasterToggleOff(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	// GORM's default-value semantics treat zero-value bools as "use default",
	// so we Create the row with defaults then Update the toggle to false.
	s := models.LibrarySettings{
		LibraryID:          lib.ID,
		ThemeDefault:       "dark",
		GridDensityDefault: "medium",
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&s).Update("thumbnails_raster_enabled", false).Error; err != nil {
		t.Fatal(err)
	}

	pngBytes := makeTestPNG(t, 32, 32)
	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "tile.png", pngBytes)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.ThumbnailObjectKey != "" {
		t.Fatalf("expected no thumbnail when raster toggle is off, got %q", a.ThumbnailObjectKey)
	}
}

func makeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 0x40, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A storefront cover outlives later extractions: a new version of an asset
// whose cover came from the store must not replace it, as rescan and
// re-extract already don't.
func TestPostImport_NewVersionKeepsTheStorefrontCover(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	body, ctype := buildMultipart(t, "file", "tile.png", makeTestPNG(t, 64, 64))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	first := jsonResponse[importEnqueueResponse](t, w)

	if err := db.Model(&models.Asset{}).Where("id = ?", first.AssetID).Updates(map[string]any{
		"thumbnail_object_key": "store-cover", "thumbnail_content_type": "image/jpeg", "thumbnail_from_store": true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	body, ctype = buildMultipartWithFields(t, "file", "tile v2.png", makeTestPNG(t, 96, 96), map[string]string{
		"asset_id": fmt.Sprint(first.AssetID), "store_version": "2",
	})
	w = postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)

	var a models.Asset
	if err := db.First(&a, first.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.ThumbnailObjectKey != "store-cover" || !a.ThumbnailFromStore {
		t.Fatalf("store cover replaced: key %q, from store %v", a.ThumbnailObjectKey, a.ThumbnailFromStore)
	}
}
