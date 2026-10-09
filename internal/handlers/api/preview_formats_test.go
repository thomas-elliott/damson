package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imaging/imagingtest"
)

// Tier-2 formats get both a grid cover and a panel preview, from the same
// decoders, in formats a browser shows.
func TestPostImport_Tier2FormatsGetACoverAndAPreview(t *testing.T) {
	for name, data := range imagingtest.Files(t) {
		t.Run(name, func(t *testing.T) {
			h, db := newTestHandler(t)
			u := seedUser(t, db, "librarian@example.com")
			seedLibraryAndSwitch(t, h, u)

			body, ctype := buildMultipart(t, "file", name, data)
			w := postMultipart(setupRouter(h, u), "/api/imports", body, ctype)
			requireStatus(t, w, http.StatusAccepted)

			var a models.Asset
			if err := db.First(&a, jsonResponse[importEnqueueResponse](t, w).AssetID).Error; err != nil {
				t.Fatal(err)
			}
			if a.ThumbnailObjectKey == "" || (a.ThumbnailContentType != "image/png" && a.ThumbnailContentType != "image/jpeg") {
				t.Fatalf("grid cover = %q (%s), want a PNG or JPEG", a.ThumbnailObjectKey, a.ThumbnailContentType)
			}
			var f models.AssetFile
			if err := db.Where("asset_version_id = ?", *a.CurrentVersionID).First(&f).Error; err != nil {
				t.Fatal(err)
			}
			if f.PreviewStatus != "done" || f.PreviewObjectKey == "" {
				t.Fatalf("panel preview = %q (key %q), want done", f.PreviewStatus, f.PreviewObjectKey)
			}
		})
	}
}
