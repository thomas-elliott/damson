package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestPostAssetVersionSubset_CreatesNewVersionWithSubset(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibraryAndSwitch(t, h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "x", NeedsReview: true}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1", PackageKind: "loose"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	files := []models.AssetFile{
		{AssetVersionID: v.ID, RelativePath: "a.png", SizeBytes: 1, ContentHash: "aa", ObjectKey: "aa"},
		{AssetVersionID: v.ID, RelativePath: "b.png", SizeBytes: 1, ContentHash: "bb", ObjectKey: "bb"},
		{AssetVersionID: v.ID, RelativePath: "c.png", SizeBytes: 1, ContentHash: "cc", ObjectKey: "cc"},
	}
	for i := range files {
		if err := db.Create(&files[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	asset.CurrentVersionID = &v.ID
	if err := db.Save(&asset).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/versions/1/subset", subsetRequest{
		FileIDs: []uint{files[0].ID, files[2].ID},
	})
	requireStatus(t, w, http.StatusCreated)

	// New version exists with two files.
	var versions []models.AssetVersion
	if err := db.Where("asset_id = ?", asset.ID).Find(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	var newVersion models.AssetVersion
	for _, vv := range versions {
		if vv.ID != v.ID {
			newVersion = vv
		}
	}
	var newFiles []models.AssetFile
	if err := db.Where("asset_version_id = ?", newVersion.ID).Find(&newFiles).Error; err != nil {
		t.Fatal(err)
	}
	if len(newFiles) != 2 {
		t.Fatalf("expected 2 files in subset version, got %d", len(newFiles))
	}
	// asset.current_version_id flipped to the new version.
	var refreshed models.Asset
	if err := db.First(&refreshed, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.CurrentVersionID == nil || *refreshed.CurrentVersionID != newVersion.ID {
		t.Fatalf("current_version_id not updated: %+v", refreshed.CurrentVersionID)
	}

	// Audit row written.
	var aud models.AuditLog
	if err := db.Where("action_type = ?", "asset.version.subset").First(&aud).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPostAssetVersionSubset_RejectsCrossVersionFiles(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibraryAndSwitch(t, h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "x", NeedsReview: true}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	v1 := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1", PackageKind: "loose"}
	if err := db.Create(&v1).Error; err != nil {
		t.Fatal(err)
	}
	v2 := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v2", PackageKind: "loose"}
	if err := db.Create(&v2).Error; err != nil {
		t.Fatal(err)
	}
	other := models.AssetFile{AssetVersionID: v2.ID, RelativePath: "z.png", ContentHash: "zz", ObjectKey: "zz"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/versions/1/subset", subsetRequest{
		FileIDs: []uint{other.ID}, // belongs to v2, not v1
	})
	requireStatus(t, w, http.StatusBadRequest)
}
