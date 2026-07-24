package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

type bulkResult struct {
	Affected int `json:"affected"`
}

func TestBulkAssets_Delete(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	a1 := seedAsset(t, db, lib.ID, "a1")
	a2 := seedAsset(t, db, lib.ID, "a2")
	a3 := seedAsset(t, db, lib.ID, "a3")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/bulk", map[string]any{
		"action":    "delete",
		"asset_ids": []uint{a1.ID, a2.ID},
	})
	requireStatus(t, w, http.StatusOK)
	if got := jsonResponse[bulkResult](t, w).Affected; got != 2 {
		t.Fatalf("affected: got %d, want 2", got)
	}

	var count int64
	db.Model(&models.Asset{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 asset left, got %d", count)
	}
	var remaining int64
	db.Model(&models.Asset{}).Where("id = ?", a3.ID).Count(&remaining)
	if remaining != 1 {
		t.Fatalf("the unselected asset should survive")
	}
}

func TestBulkAssets_Review(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	a1 := seedAsset(t, db, lib.ID, "a1")
	a2 := seedAsset(t, db, lib.ID, "a2")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/bulk", map[string]any{
		"action":    "review",
		"asset_ids": []uint{a1.ID, a2.ID},
	})
	requireStatus(t, w, http.StatusOK)
	if got := jsonResponse[bulkResult](t, w).Affected; got != 2 {
		t.Fatalf("affected: got %d, want 2", got)
	}

	var stillNeedsReview int64
	db.Model(&models.Asset{}).
		Where("id IN ? AND needs_review = ?", []uint{a1.ID, a2.ID}, true).
		Count(&stillNeedsReview)
	if stillNeedsReview != 0 {
		t.Fatalf("expected both assets marked reviewed, %d still need review", stillNeedsReview)
	}
}

func TestBulkAssets_AddTag(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	a1 := seedAsset(t, db, lib.ID, "a1")
	a2 := seedAsset(t, db, lib.ID, "a2")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/bulk", map[string]any{
		"action":    "add_tag",
		"asset_ids": []uint{a1.ID, a2.ID},
		"path":      "environment/rocks",
	})
	requireStatus(t, w, http.StatusOK)
	if got := jsonResponse[bulkResult](t, w).Affected; got != 2 {
		t.Fatalf("affected: got %d, want 2", got)
	}

	var linked int64
	db.Table("asset_tags AS at").
		Joins("JOIN tags t ON t.id = at.tag_id").
		Where("t.path = ? AND at.asset_id IN ?", "environment/rocks", []uint{a1.ID, a2.ID}).
		Count(&linked)
	if linked != 2 {
		t.Fatalf("expected the tag linked to both assets, got %d", linked)
	}
}

func TestBulkAssets_RejectsUnknownActionAndEmpty(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	a1 := seedAsset(t, db, lib.ID, "a1")

	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/assets/bulk", map[string]any{
		"action":    "frobnicate",
		"asset_ids": []uint{a1.ID},
	})
	requireStatus(t, w, http.StatusBadRequest)

	w = performRequest(r, http.MethodPost, "/api/assets/bulk", map[string]any{
		"action":    "delete",
		"asset_ids": []uint{},
	})
	requireStatus(t, w, http.StatusBadRequest)
}

func TestBulkAssets_ScopedToLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	// An asset in another library must not be reachable — the whole batch is
	// rejected and nothing is deleted.
	otherLib := seedLibrary(t, db, "Other")
	stranger := seedAsset(t, db, otherLib.ID, "stranger")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/bulk", map[string]any{
		"action":    "delete",
		"asset_ids": []uint{stranger.ID},
	})
	requireStatus(t, w, http.StatusNotFound)

	var survives int64
	db.Model(&models.Asset{}).Where("id = ?", stranger.ID).Count(&survives)
	if survives != 1 {
		t.Fatalf("cross-library asset must survive a scoped bulk delete")
	}
}
