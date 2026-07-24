package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// seedLibraryAndSwitch creates a library, membership, and switches the user
// to it. Matches the older helper in assets_test.go but lives here so new
// tests don't depend on import-specific seed code.
func seedLibraryForUser(t *testing.T, db *gorm.DB, u *models.User, name string) *models.Library {
	t.Helper()
	lib := seedLibrary(t, db, name)
	seedMembership(t, db, lib.ID, u.ID, "admin")
	u.CurrentLibraryID = &lib.ID
	if err := db.Save(u).Error; err != nil {
		t.Fatalf("set current lib: %v", err)
	}
	return lib
}

func seedAsset(t *testing.T, db *gorm.DB, libID uint, name string) *models.Asset {
	t.Helper()
	a := models.Asset{LibraryID: libID, Name: name, NeedsReview: true}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return &a
}

func TestPatchAsset_UpdatesFieldsAndAudits(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "original-name")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/assets/1",
		map[string]any{"name": "new-name", "description": "has a description"})
	requireStatus(t, w, http.StatusOK)

	var refreshed models.Asset
	if err := db.First(&refreshed, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Name != "new-name" || refreshed.Description != "has a description" {
		t.Fatalf("asset not updated: %+v", refreshed)
	}
	var auditCount int64
	if err := db.Model(&models.AuditLog{}).
		Where("action_type = ? AND target_type = ? AND target_id = ?", "asset.update", "asset", asset.ID).
		Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit row, got %d", auditCount)
	}
}

func TestPatchAssetVersion_UpdatesStoreVersionAndAudits(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/assets/1/versions/1",
		map[string]any{"store_version": "v2.0"})
	requireStatus(t, w, http.StatusOK)

	var refreshed models.AssetVersion
	if err := db.First(&refreshed, v.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.StoreVersion != "2.0" {
		t.Fatalf("store version not normalised/updated: %q", refreshed.StoreVersion)
	}
	var auditCount int64
	if err := db.Model(&models.AuditLog{}).
		Where("action_type = ?", "asset.version.update").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit row, got %d", auditCount)
	}
}

func TestPatchAssetVersion_RejectsInvalid(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1", StoreVersion: "1.0"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/assets/1/versions/1",
		map[string]any{"store_version": "abc"})
	requireStatus(t, w, http.StatusBadRequest)

	var refreshed models.AssetVersion
	if err := db.First(&refreshed, v.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.StoreVersion != "1.0" {
		t.Fatalf("store version should be unchanged, got %q", refreshed.StoreVersion)
	}
}

func TestPatchAssetVersion_EditsLabelAndBlocksDuplicateStore(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")
	v1 := models.AssetVersion{AssetID: asset.ID, StoreVersion: "1.0"}
	v2 := models.AssetVersion{AssetID: asset.ID, StoreVersion: "2.0"}
	if err := db.Create(&v1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&v2).Error; err != nil {
		t.Fatal(err)
	}
	r := setupRouter(h, u)

	// The label is editable.
	lw := performRequest(r, http.MethodPatch, "/api/assets/1/versions/"+uintStr(v1.ID),
		map[string]any{"version_label": "release candidate"})
	requireStatus(t, lw, http.StatusOK)
	var relabelled models.AssetVersion
	if err := db.First(&relabelled, v1.ID).Error; err != nil {
		t.Fatal(err)
	}
	if relabelled.VersionLabel != "release candidate" {
		t.Fatalf("label not updated: %q", relabelled.VersionLabel)
	}

	// Setting v2's store version to v1's is blocked (identity must be unique).
	dw := performRequest(r, http.MethodPatch, "/api/assets/1/versions/"+uintStr(v2.ID),
		map[string]any{"store_version": "1.0"})
	requireStatus(t, dw, http.StatusConflict)
	var unchanged models.AssetVersion
	if err := db.First(&unchanged, v2.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.StoreVersion != "2.0" {
		t.Fatalf("store version should be unchanged after conflict, got %q", unchanged.StoreVersion)
	}
}

func TestGetAssetDetail_OrdersVersionsBySemver(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")
	for _, sv := range []string{"1.1", "1.10", "1.9"} {
		v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v" + sv, StoreVersion: sv}
		if err := db.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := setupRouter(h, u)
	w := performRequest(r, http.MethodGet, "/api/assets/1", nil)
	requireStatus(t, w, http.StatusOK)

	var resp struct {
		Versions []struct {
			StoreVersion string `json:"store_version"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	want := []string{"1.10", "1.9", "1.1"}
	if len(resp.Versions) != len(want) {
		t.Fatalf("expected %d versions, got %d", len(want), len(resp.Versions))
	}
	for i, exp := range want {
		if resp.Versions[i].StoreVersion != exp {
			t.Fatalf("version order = %v, want %v", resp.Versions, want)
		}
	}
}

func TestPatchAsset_RejectsBlankName(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "x")
	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/assets/1",
		map[string]any{"name": "   "})
	requireStatus(t, w, http.StatusBadRequest)
}

func TestDeleteAsset_CascadesVersionsAndAudits(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "to-delete")
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodDelete, "/api/assets/1", nil)
	requireStatus(t, w, http.StatusNoContent)

	var assetCount int64
	_ = db.Model(&models.Asset{}).Count(&assetCount)
	if assetCount != 0 {
		t.Fatalf("asset not deleted")
	}
	var versionCount int64
	_ = db.Model(&models.AssetVersion{}).Count(&versionCount)
	if versionCount != 0 {
		t.Fatalf("version not cascaded")
	}
	var auditCount int64
	_ = db.Model(&models.AuditLog{}).Where("action_type = ?", "asset.delete").Count(&auditCount)
	if auditCount != 1 {
		t.Fatalf("expected 1 asset.delete audit row, got %d", auditCount)
	}
}

func TestPostReview_ClearsFlag(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "review-me")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/review", nil)
	requireStatus(t, w, http.StatusNoContent)

	var refreshed models.Asset
	_ = db.First(&refreshed, asset.ID)
	if refreshed.NeedsReview {
		t.Fatalf("needs_review not cleared")
	}
}

func TestGetAssetDetail_ReturnsTagsVersionsAndLicence(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset-one")
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	asset.CurrentVersionID = &v.ID
	_ = db.Save(asset)

	tag := models.Tag{LibraryID: lib.ID, Path: "environment"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AssetTag{AssetID: asset.ID, TagID: tag.ID}).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets/1")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetDetailView](t, w)
	if len(body.Versions) != 1 || !body.Versions[0].IsCurrent {
		t.Fatalf("versions wrong: %+v", body.Versions)
	}
	if len(body.Tags) != 1 || body.Tags[0].Path != "environment" {
		t.Fatalf("tags wrong: %+v", body.Tags)
	}
}
