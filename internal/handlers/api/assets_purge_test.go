package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestImport_PurgesOriginalWhenRetentionOff(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	// GORM treats a zero-value bool as "use default" (retain defaults true), so
	// create with defaults then explicitly turn retention off.
	s := models.LibrarySettings{LibraryID: lib.ID, GridDensityDefault: "medium"}
	if err := db.Create(&s).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&s).Update("retain_original_packages", false).Error; err != nil {
		t.Fatal(err)
	}

	pkg := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x07), pathname: "Assets/y.bin", content: []byte("y")},
	})
	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "Ret.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var v models.AssetVersion
	if err := db.Where("asset_id = ?", resp.AssetID).First(&v).Error; err != nil {
		t.Fatal(err)
	}
	if !v.OriginalPackagePurged {
		t.Fatalf("expected original_package_purged=true after retain-off import")
	}
	if v.OriginalPackageObjectKey == "" {
		t.Fatalf("object key should be retained on the row (D3-B), got empty")
	}
	if _, ok, _ := h.storage.HeadOriginal(context.Background(), v.OriginalPackageObjectKey); ok {
		t.Fatalf("original archive should have been deleted from storage")
	}
}

func TestImport_KeepsOriginalWhenRetentionOn(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u) // no settings row → retain defaults on

	pkg := buildUnityPackage(t, []unityPkgEntry{
		{guid: testGUID(0x08), pathname: "Assets/z.bin", content: []byte("z")},
	})
	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "Keep.unitypackage", pkg)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var v models.AssetVersion
	if err := db.Where("asset_id = ?", resp.AssetID).First(&v).Error; err != nil {
		t.Fatal(err)
	}
	if v.OriginalPackagePurged {
		t.Fatalf("original should not be purged when retention is on")
	}
	if _, ok, _ := h.storage.HeadOriginal(context.Background(), v.OriginalPackageObjectKey); !ok {
		t.Fatalf("original archive should be retained in storage")
	}
}
