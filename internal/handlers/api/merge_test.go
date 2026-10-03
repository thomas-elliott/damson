package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestPostAssetMerge_FoldsSourceIntoTarget(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	target := seedAsset(t, db, lib.ID, "Fantasy Pack")
	source := seedAsset(t, db, lib.ID, "Fantasy Pack (dup)")

	// Target has 1.0; source has 1.0 (duplicate → dropped) and 2.0 (moved).
	tv := models.AssetVersion{AssetID: target.ID, StoreVersion: "1.0"}
	sv1 := models.AssetVersion{AssetID: source.ID, StoreVersion: "1.0"}
	sv2 := models.AssetVersion{AssetID: source.ID, StoreVersion: "2.0"}
	for _, v := range []*models.AssetVersion{&tv, &sv1, &sv2} {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Source has a distinct tag and a project pin on its duplicate 1.0.
	tag := models.Tag{LibraryID: lib.ID, Path: "rpg"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AssetTag{AssetID: source.ID, TagID: tag.ID}).Error; err != nil {
		t.Fatal(err)
	}
	proj := models.Project{LibraryID: lib.ID, Name: "Game"}
	if err := db.Create(&proj).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ProjectAssetPin{ProjectID: proj.ID, AssetID: source.ID, PinnedVersionID: &sv1.ID}).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(target.ID)+"/merge",
		map[string]any{"source_asset_id": source.ID})
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[mergeResponse](t, w)

	// Only 2.0 moved; 1.0 was a duplicate and dropped.
	if resp.MergedVersions != 1 {
		t.Fatalf("expected 1 merged version, got %d", resp.MergedVersions)
	}
	if resp.Adoptable.Name != "Fantasy Pack (dup)" {
		t.Fatalf("adoptable name: got %q", resp.Adoptable.Name)
	}

	// Source is gone.
	var srcCount int64
	db.Model(&models.Asset{}).Where("id = ?", source.ID).Count(&srcCount)
	if srcCount != 0 {
		t.Fatalf("source asset should be deleted")
	}

	// Target has exactly 1.0 (its own) + 2.0 (moved).
	var stores []string
	db.Model(&models.AssetVersion{}).Where("asset_id = ?", target.ID).Order("store_version").Pluck("store_version", &stores)
	if len(stores) != 2 || stores[0] != "1.0" || stores[1] != "2.0" {
		t.Fatalf("target versions: got %v", stores)
	}

	// Source's tag moved to target.
	var tagLinks int64
	db.Model(&models.AssetTag{}).Where("asset_id = ? AND tag_id = ?", target.ID, tag.ID).Count(&tagLinks)
	if tagLinks != 1 {
		t.Fatalf("tag not moved to target")
	}

	// The pin moved to target and was repointed to target's 1.0 (its pinned
	// source version was the dropped duplicate).
	var pin models.ProjectAssetPin
	if err := db.Where("project_id = ?", proj.ID).First(&pin).Error; err != nil {
		t.Fatal(err)
	}
	if pin.AssetID != target.ID {
		t.Fatalf("pin not moved to target: asset_id=%d", pin.AssetID)
	}
	if pin.PinnedVersionID == nil || *pin.PinnedVersionID != tv.ID {
		t.Fatalf("pin should be repointed to target's 1.0 (%d), got %d", tv.ID, pin.PinnedVersionID)
	}
}

func TestPostAssetMerge_RejectsSelfMerge(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(asset.ID)+"/merge",
		map[string]any{"source_asset_id": asset.ID})
	requireStatus(t, w, http.StatusBadRequest)
}
