package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestGetAssets_FiltersByTag(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	a1 := seedAsset(t, db, lib.ID, "with-tag")
	_ = seedAsset(t, db, lib.ID, "without-tag")

	tag := models.Tag{LibraryID: lib.ID, Path: "music"}
	_ = db.Create(&tag)
	_ = db.Create(&models.AssetTag{AssetID: a1.ID, TagID: tag.ID})

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?tag_id=1")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].ID != a1.ID {
		t.Fatalf("filter returned wrong rows: %+v", body.Assets)
	}
}

func TestGetAssets_FiltersByQuery(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "fantasy-kingdom")
	seedAsset(t, db, lib.ID, "sci-fi-pack")

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?q=fantasy")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].Name != "fantasy-kingdom" {
		t.Fatalf("query didn't filter: %+v", body.Assets)
	}
}

func TestGetAssets_FiltersByPartialQuery(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "fantasy-kingdom")
	seedAsset(t, db, lib.ID, "sci-fi-pack")

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?q=fant")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].Name != "fantasy-kingdom" {
		t.Fatalf("partial query didn't filter: %+v", body.Assets)
	}
}

func TestGetAssets_ComposedFilters(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	a1 := seedAsset(t, db, lib.ID, "music-pack")
	_ = seedAsset(t, db, lib.ID, "untouched")

	tag := models.Tag{LibraryID: lib.ID, Path: "audio"}
	_ = db.Create(&tag)
	_ = db.Create(&models.AssetTag{AssetID: a1.ID, TagID: tag.ID})
	a1.NeedsReview = false
	_ = db.Save(a1)

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?tag_id=1&needs_review=false")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].ID != a1.ID {
		t.Fatalf("composed filter wrong: %+v", body.Assets)
	}
}
