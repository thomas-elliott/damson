package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestGetImports_FiltersDoneOut(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibraryAndSwitch(t, h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "x", NeedsReview: true}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	rows := []models.AssetImport{
		{AssetID: asset.ID, Stage: "done", UpdatedAt: time.Now().UTC()},
		{AssetID: asset.ID, Stage: "failed", Error: "boom", UpdatedAt: time.Now().UTC()},
	}
	for _, r := range rows {
		if err := db.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/imports")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[importListResponse](t, w)
	if body.Total != 1 || body.Imports[0].Stage != "failed" {
		t.Fatalf("expected the failed row only, got %+v", body)
	}

	// stage=all returns everything.
	w = performGET(r, "/api/imports?stage=all")
	requireStatus(t, w, http.StatusOK)
	body = jsonResponse[importListResponse](t, w)
	if body.Total != 2 {
		t.Fatalf("expected 2 imports with stage=all, got %d", body.Total)
	}
}

func TestPostImportRetry_RequiresFailedStage(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibraryAndSwitch(t, h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "x", NeedsReview: true}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	imp := models.AssetImport{AssetID: asset.ID, Stage: "done", UpdatedAt: time.Now().UTC()}
	if err := db.Create(&imp).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/imports/1/retry", nil)
	requireStatus(t, w, http.StatusConflict)
}

func TestPostImportRetry_NoSourceBlob(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibraryAndSwitch(t, h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "x", NeedsReview: true}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	imp := models.AssetImport{AssetID: asset.ID, Stage: "failed", Error: "x", UpdatedAt: time.Now().UTC()}
	if err := db.Create(&imp).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/imports/1/retry", nil)
	requireStatus(t, w, http.StatusConflict) // no source blob → 409
}

func TestPostImport_RecordsFailedRowOnExtractFailure(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "broken.unitypackage", []byte("not gzipped tar"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusBadRequest)

	// One asset + one failed import row should exist now.
	var assetCount int64
	if err := db.Model(&models.Asset{}).Count(&assetCount).Error; err != nil {
		t.Fatal(err)
	}
	if assetCount != 1 {
		t.Fatalf("expected 1 asset stub, got %d", assetCount)
	}
	var failed int64
	if err := db.Model(&models.AssetImport{}).Where("stage = ?", "failed").Count(&failed).Error; err != nil {
		t.Fatal(err)
	}
	if failed != 1 {
		t.Fatalf("expected 1 failed import row, got %d", failed)
	}
}
