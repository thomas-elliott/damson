package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestCollections_CRUDAndMembership(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset, _ := seedAssetWithVersion(t, db, lib.ID, "audio", "v1")

	r := setupRouter(h, u)
	createW := performRequest(r, http.MethodPost, "/api/collections",
		map[string]any{"name": "Boss fight audio", "description": "battle cues"})
	requireStatus(t, createW, http.StatusCreated)
	created := jsonResponse[collectionView](t, createW)
	if created.Name != "Boss fight audio" {
		t.Fatalf("unexpected name: %q", created.Name)
	}

	// Duplicate name -> 409.
	dup := performRequest(r, http.MethodPost, "/api/collections",
		map[string]any{"name": "boss FIGHT audio"})
	requireStatus(t, dup, http.StatusConflict)

	// Add asset.
	addW := performRequest(r, http.MethodPost, "/api/collections/1/assets",
		map[string]any{"asset_id": asset.ID})
	requireStatus(t, addW, http.StatusNoContent)

	// Asset count is reflected.
	getW := performGET(r, "/api/collections/1")
	requireStatus(t, getW, http.StatusOK)
	body := jsonResponse[collectionView](t, getW)
	if body.AssetCount != 1 {
		t.Fatalf("expected 1 asset, got %d", body.AssetCount)
	}

	// Remove.
	delW := performRequest(r, http.MethodDelete, "/api/collections/1/assets/1", nil)
	requireStatus(t, delW, http.StatusNoContent)

	var linkCount int64
	_ = db.Model(&models.CollectionAsset{}).Count(&linkCount)
	if linkCount != 0 {
		t.Fatalf("expected 0 links, got %d", linkCount)
	}
}
