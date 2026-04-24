package api

import (
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// seedAssetWithVersion creates an asset + one version and sets the version
// as current. Returns both for tests that need to pin versions.
func seedAssetWithVersion(t *testing.T, db *gorm.DB, libID uint, name, label string) (*models.Asset, *models.AssetVersion) {
	t.Helper()
	a := seedAsset(t, db, libID, name)
	v := models.AssetVersion{AssetID: a.ID, VersionLabel: label}
	if err := db.Create(&v).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}
	a.CurrentVersionID = &v.ID
	if err := db.Save(a).Error; err != nil {
		t.Fatalf("set current version: %v", err)
	}
	return a, &v
}

func TestPostProject_CreatesAndRejectsDuplicate(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/projects",
		map[string]any{"name": "Ashfall", "target_distribution": "commercial"})
	requireStatus(t, w, http.StatusCreated)

	dup := performRequest(r, http.MethodPost, "/api/projects",
		map[string]any{"name": "ashfall"})
	requireStatus(t, dup, http.StatusConflict)
}

func TestProjectPin_PinsCurrentVersionAndDetectsUpgrade(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset, v1 := seedAssetWithVersion(t, db, lib.ID, "texture-pack", "v1")

	r := setupRouter(h, u)
	createW := performRequest(r, http.MethodPost, "/api/projects",
		map[string]any{"name": "Ashfall"})
	requireStatus(t, createW, http.StatusCreated)
	proj := jsonResponse[projectView](t, createW)

	// Pin current.
	pinW := performRequest(r, http.MethodPost, "/api/projects/1/pins",
		map[string]any{"asset_id": asset.ID})
	requireStatus(t, pinW, http.StatusOK)

	// Create a newer version and point current_version at it.
	v2 := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v2"}
	if err := db.Create(&v2).Error; err != nil {
		t.Fatal(err)
	}
	asset.CurrentVersionID = &v2.ID
	_ = db.Save(asset)

	// Project detail should report updates_available=1.
	detailW := performGET(r, "/api/projects/1")
	requireStatus(t, detailW, http.StatusOK)
	body := jsonResponse[projectDetailView](t, detailW)
	if body.UpdatesAvailable != 1 {
		t.Fatalf("expected 1 update available, got %d", body.UpdatesAvailable)
	}
	if len(body.Pins) != 1 || !body.Pins[0].HasUpdate {
		t.Fatalf("pin upgrade flag not set: %+v", body.Pins)
	}

	// Upgrade: re-pin with VersionID=v2.
	upgradeW := performRequest(r, http.MethodPost, "/api/projects/1/pins",
		map[string]any{"asset_id": asset.ID, "version_id": v2.ID})
	requireStatus(t, upgradeW, http.StatusOK)

	// Audit row for upgrade.
	var upgradeAudits int64
	_ = db.Model(&models.AuditLog{}).
		Where("action_type = ?", "project.pin.upgrade").
		Count(&upgradeAudits)
	if upgradeAudits != 1 {
		t.Fatalf("expected 1 upgrade audit, got %d", upgradeAudits)
	}

	_ = v1 // silence
	_ = proj
}

func TestDeleteProject_CascadesPins(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset, _ := seedAssetWithVersion(t, db, lib.ID, "pack", "v1")

	r := setupRouter(h, u)
	_ = performRequest(r, http.MethodPost, "/api/projects",
		map[string]any{"name": "Ashfall"})
	_ = performRequest(r, http.MethodPost, "/api/projects/1/pins",
		map[string]any{"asset_id": asset.ID})

	delW := performRequest(r, http.MethodDelete, "/api/projects/1", nil)
	requireStatus(t, delW, http.StatusNoContent)

	var pinCount int64
	_ = db.Model(&models.ProjectAssetPin{}).Count(&pinCount)
	if pinCount != 0 {
		t.Fatalf("expected pins to cascade, got %d", pinCount)
	}
}
