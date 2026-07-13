package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestGetImports_ReviewBacklogAndFinished(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibraryAndSwitch(t, h, u)

	// review needs an eye: a done import whose asset still needs review, plus a
	// failed import. Both belong in the active queue.
	review := models.Asset{LibraryID: lib.ID, Name: "needs-review", NeedsReview: true}
	if err := db.Create(&review).Error; err != nil {
		t.Fatal(err)
	}
	// cleared was reviewed and its import finished — it belongs in the finished
	// tail, not the active queue.
	cleared := models.Asset{LibraryID: lib.ID, Name: "cleared", NeedsReview: false}
	if err := db.Create(&cleared).Error; err != nil {
		t.Fatal(err)
	}
	// NeedsReview has a DB default of true, so a zero-value false is ignored on
	// insert — force it off explicitly.
	if err := db.Model(&cleared).Update("needs_review", false).Error; err != nil {
		t.Fatal(err)
	}
	rows := []models.AssetImport{
		{AssetID: review.ID, Stage: "done", UpdatedAt: time.Now().UTC()},
		{AssetID: review.ID, Stage: "failed", Error: "boom", UpdatedAt: time.Now().UTC()},
		{AssetID: cleared.ID, Stage: "done", UpdatedAt: time.Now().UTC()},
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
	// Active queue: the review asset's done + failed rows (needs_review keeps the
	// done row visible); the cleared asset's done row is finished, not active.
	if body.Total != 2 {
		t.Fatalf("expected 2 active imports (review backlog), got %d: %+v", body.Total, body.Imports)
	}
	for _, imp := range body.Imports {
		if imp.AssetID == cleared.ID {
			t.Fatalf("cleared asset should not be in the active queue: %+v", imp)
		}
	}
	if len(body.Finished) != 1 || body.Finished[0].AssetID != cleared.ID {
		t.Fatalf("expected the cleared done import in finished, got %+v", body.Finished)
	}

	// stage=all returns everything, with no finished tail.
	w = performGET(r, "/api/imports?stage=all")
	requireStatus(t, w, http.StatusOK)
	body = jsonResponse[importListResponse](t, w)
	if body.Total != 3 {
		t.Fatalf("expected 3 imports with stage=all, got %d", body.Total)
	}
	if body.Finished != nil {
		t.Fatalf("stage=all should not carry a finished tail, got %+v", body.Finished)
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

func TestPostImport_MarksFailedOnExtractError(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "broken.unitypackage", []byte("not gzipped tar"))
	w := postMultipart(r, "/api/imports", body, ctype)
	// The handler accepts the upload; the worker (sync mode) fails the
	// extract and writes the failure to the import row.
	requireStatus(t, w, http.StatusAccepted)

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
