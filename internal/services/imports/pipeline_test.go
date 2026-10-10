package imports

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imaging/imagingtest"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// Retrying an older version's failed import, after a newer version became
// current, must leave the newer version and its cover in place.
func TestRun_OlderVersionImportDoesNotRollBackCurrentVersion(t *testing.T) {
	db := enrichTestDB(t)
	ctx := context.Background()
	store, err := storage.NewLocalFS(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	asset := seedEnrichAsset(t, db, "Texture", false)

	key, _, err := store.PutBlob(ctx, bytes.NewReader(imagingtest.TGA(t)), storage.ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	older := models.AssetVersion{AssetID: asset.ID, ImportDate: time.Now().Add(-time.Hour), PackageKind: "loose", OriginalPackageObjectKey: key}
	newer := models.AssetVersion{AssetID: asset.ID, ImportDate: time.Now(), PackageKind: "loose", OriginalPackageObjectKey: key}
	for _, v := range []*models.AssetVersion{&older, &newer} {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
		f := models.AssetFile{AssetVersionID: v.ID, RelativePath: "texture.tga", ObjectKey: key, ContentHash: key}
		if err := db.Create(&f).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(asset).Updates(map[string]any{
		"current_version_id": newer.ID, "thumbnail_object_key": "newer-cover", "thumbnail_content_type": "image/png",
	}).Error; err != nil {
		t.Fatal(err)
	}
	imp := models.AssetImport{AssetID: asset.ID, AssetVersionID: &older.ID, Stage: StageFailed}
	if err := db.Create(&imp).Error; err != nil {
		t.Fatal(err)
	}

	if err := Run(ctx, Deps{DB: db, Storage: store}, Job{ImportID: imp.ID, LibraryID: asset.LibraryID}); err != nil {
		t.Fatalf("run: %v", err)
	}

	var a models.Asset
	if err := db.First(&a, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if a.CurrentVersionID == nil || *a.CurrentVersionID != newer.ID {
		t.Errorf("current version = %v, want the newer %d", a.CurrentVersionID, newer.ID)
	}
	if a.ThumbnailObjectKey != "newer-cover" {
		t.Errorf("cover = %q, want the newer version's", a.ThumbnailObjectKey)
	}
}
