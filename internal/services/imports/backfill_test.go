package imports

import (
	"bytes"
	"context"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imaging/imagingtest"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/preview/previewtest"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

// Files imported before tier 2 have no cover and an unsupported preview. The
// boot-time backfill gives them both, without a re-import.
func TestBackfill_Tier2FilesImportedEarlierGainACoverAndAPreview(t *testing.T) {
	for name, data := range imagingtest.Files(t) {
		t.Run(name, func(t *testing.T) {
			db := enrichTestDB(t)
			ctx := context.Background()
			store, err := storage.NewLocalFS(t.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}
			asset := seedEnrichAsset(t, db, "Old import", false)
			key, _, err := store.PutBlob(ctx, bytes.NewReader(data), storage.ObjectMeta{})
			if err != nil {
				t.Fatal(err)
			}
			version := models.AssetVersion{AssetID: asset.ID, PackageKind: "loose", OriginalPackageObjectKey: key}
			if err := db.Create(&version).Error; err != nil {
				t.Fatal(err)
			}
			file := models.AssetFile{AssetVersionID: version.ID, RelativePath: name, ObjectKey: key, ContentHash: key, PreviewStatus: preview.StatusUnsupported}
			if err := db.Create(&file).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(asset).Update("current_version_id", version.ID).Error; err != nil {
				t.Fatal(err)
			}

			// What the server runs at boot.
			if _, err := preview.QueueNewlySupported(ctx, db); err != nil {
				t.Fatalf("queue previews: %v", err)
			}
			pool := previewtest.NewSync(preview.Deps{DB: db, Storage: store})
			if _, err := preview.ResumeInFlight(ctx, db, pool); err != nil {
				t.Fatalf("resume previews: %v", err)
			}
			if _, err := BackfillCovers(ctx, db, store); err != nil {
				t.Fatalf("backfill covers: %v", err)
			}

			var a models.Asset
			db.First(&a, asset.ID)
			if a.ThumbnailObjectKey == "" || (a.ThumbnailContentType != "image/png" && a.ThumbnailContentType != "image/jpeg") {
				t.Errorf("cover = %q (%s), want a PNG or JPEG", a.ThumbnailObjectKey, a.ThumbnailContentType)
			}
			var f models.AssetFile
			db.First(&f, file.ID)
			if f.PreviewStatus != preview.StatusDone || f.PreviewObjectKey == "" {
				t.Errorf("preview = %q (key %q), want done", f.PreviewStatus, f.PreviewObjectKey)
			}
		})
	}
}

// The backfill must not hand an archive a cover its import rules refuse: a
// Unity package's cover only ever comes from its preview.png, under the
// library's setting, and a zip takes no image over the 16 MiB cover cap.
func TestBackfillCovers_RespectsArchiveCoverRules(t *testing.T) {
	cases := []struct {
		name, kind string
		size       int64
	}{
		{"unity package", "unitypackage", 1024},
		{"zip image over the cover cap", "zip", 17 << 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := enrichTestDB(t)
			ctx := context.Background()
			store, err := storage.NewLocalFS(t.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}
			asset := seedEnrichAsset(t, db, "Archive", false)
			key, _, err := store.PutBlob(ctx, bytes.NewReader(imagingtest.TGA(t)), storage.ObjectMeta{})
			if err != nil {
				t.Fatal(err)
			}
			version := models.AssetVersion{AssetID: asset.ID, PackageKind: tc.kind}
			if err := db.Create(&version).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&models.AssetFile{AssetVersionID: version.ID, RelativePath: "Assets/tex.tga", SizeBytes: tc.size, ObjectKey: key, ContentHash: key}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(asset).Update("current_version_id", version.ID).Error; err != nil {
				t.Fatal(err)
			}

			if _, err := BackfillCovers(ctx, db, store); err != nil {
				t.Fatal(err)
			}
			var a models.Asset
			db.First(&a, asset.ID)
			if a.ThumbnailObjectKey != "" {
				t.Fatalf("backfill gave the %s a cover its import would not", tc.name)
			}
		})
	}
}

// A zip's import takes a cover-like name before archive order, so the
// backfill must pick the same image rather than the first by path.
func TestBackfillCovers_PrefersACoverLikeNameAsAZipImportDoes(t *testing.T) {
	db := enrichTestDB(t)
	ctx := context.Background()
	store, err := storage.NewLocalFS(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	asset := seedEnrichAsset(t, db, "Zip", false)
	version := models.AssetVersion{AssetID: asset.ID, PackageKind: "zip"}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	images := &pngFetcher{} // each call returns a differently coloured PNG
	var coverPNG []byte
	for _, path := range []string{"a/first.png", "z/cover.png"} {
		data, _, _ := images.FetchImage(ctx, path)
		if path == "z/cover.png" {
			coverPNG = data
		}
		key, _, err := store.PutBlob(ctx, bytes.NewReader(data), storage.ObjectMeta{})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.AssetFile{AssetVersionID: version.ID, RelativePath: path, SizeBytes: int64(len(data)), ObjectKey: key, ContentHash: key}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(asset).Update("current_version_id", version.ID).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := BackfillCovers(ctx, db, store); err != nil {
		t.Fatal(err)
	}

	want, err := (&thumbnail.Generator{Storage: store}).Generate(ctx, thumbnail.Input{Reader: bytes.NewReader(coverPNG), Filename: "cover.png"}, thumbnail.Toggles{Raster: true})
	if err != nil {
		t.Fatal(err)
	}
	var a models.Asset
	db.First(&a, asset.ID)
	if a.ThumbnailObjectKey != want.ObjectKey {
		t.Errorf("cover = %q, want the one rendered from z/cover.png (%q)", a.ThumbnailObjectKey, want.ObjectKey)
	}
}
