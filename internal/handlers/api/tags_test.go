package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestPostTag_CreatesHierarchy(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/tags",
		map[string]any{"path": "environment/trees/deciduous"})
	requireStatus(t, w, http.StatusCreated)

	var count int64
	_ = db.Model(&models.Tag{}).Count(&count)
	if count != 3 {
		t.Fatalf("expected 3 ancestor rows, got %d", count)
	}
	var audits int64
	_ = db.Model(&models.AuditLog{}).Where("action_type = ?", "tag.create").Count(&audits)
	if audits != 3 {
		t.Fatalf("expected 3 audit rows for tag creates, got %d", audits)
	}
}

func TestPostAssetTag_CreatesAndLinks(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset-one")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/tags",
		map[string]any{"path": "music/ambient"})
	requireStatus(t, w, http.StatusCreated)

	// Idempotent: same call again attaches the same tag.
	w2 := performRequest(r, http.MethodPost, "/api/assets/1/tags",
		map[string]any{"path": "music/ambient"})
	if w2.Code != http.StatusCreated && w2.Code != http.StatusOK {
		t.Fatalf("second tag call failed: %d %s", w2.Code, w2.Body.String())
	}

	var linkCount int64
	_ = db.Model(&models.AssetTag{}).Where("asset_id = ?", asset.ID).Count(&linkCount)
	if linkCount != 1 {
		t.Fatalf("expected 1 tag link, got %d", linkCount)
	}
}

func TestGetTags_ReturnsCounts(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset-one")
	tag := models.Tag{LibraryID: lib.ID, Path: "environment"}
	_ = db.Create(&tag)
	_ = db.Create(&models.AssetTag{AssetID: asset.ID, TagID: tag.ID})

	r := setupRouter(h, u)
	w := performGET(r, "/api/tags")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[struct {
		Tags []tagView `json:"tags"`
	}](t, w)
	if len(body.Tags) != 1 || body.Tags[0].AssetCount != 1 {
		t.Fatalf("tags wrong: %+v", body.Tags)
	}
}

func TestDeleteTag_RemovesDescendants(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	parent := models.Tag{LibraryID: lib.ID, Path: "env"}
	_ = db.Create(&parent)
	child := models.Tag{LibraryID: lib.ID, Path: "env/trees", ParentID: &parent.ID}
	_ = db.Create(&child)

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodDelete, "/api/tags/1", nil)
	requireStatus(t, w, http.StatusNoContent)

	var remaining int64
	_ = db.Model(&models.Tag{}).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected 0 tags, got %d", remaining)
	}
}
