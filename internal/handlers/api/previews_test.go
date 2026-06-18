package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/preview"
)

// importBMP uploads a real BMP and returns the created asset id and its single
// asset_files row, after the synchronous import + preview handoff has run.
func importBMP(t *testing.T, h *Handler, u *models.User, r http.Handler) (uint, models.AssetFile) {
	t.Helper()
	body, ctype := buildMultipart(t, "file", "tex.bmp", realBMP(t))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	var f models.AssetFile
	if err := h.db.Where("relative_path = ?", "tex.bmp").First(&f).Error; err != nil {
		t.Fatal(err)
	}
	return resp.AssetID, f
}

func TestPostAssetPreviewRequeue_RegeneratesSupportedFiles(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	assetID, f := importBMP(t, h, u, r)

	// Simulate a bad/failed preview, then requeue and expect it back to done.
	if err := db.Model(&models.AssetFile{}).Where("id = ?", f.ID).
		Updates(map[string]any{"preview_status": preview.StatusFailed, "preview_object_key": ""}).Error; err != nil {
		t.Fatal(err)
	}

	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(assetID)+"/previews/requeue", nil)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[struct {
		Queued int `json:"queued"`
	}](t, w)
	if resp.Queued != 1 {
		t.Fatalf("queued: got %d, want 1", resp.Queued)
	}

	var got models.AssetFile
	if err := db.First(&got, f.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.PreviewStatus != preview.StatusDone {
		t.Fatalf("status after requeue: got %q, want done", got.PreviewStatus)
	}
	if got.PreviewObjectKey == "" {
		t.Fatal("expected a fresh preview key after requeue")
	}
}

func TestPostAssetPreviewRequeue_409WhenPreviewsDisabled(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)
	assetID, _ := importBMP(t, h, u, r)

	// The import handoff already created the settings row (previews on); flip it.
	if err := db.Model(&models.LibrarySettings{}).Where("library_id = ?", lib.ID).
		Update("previews_enabled", false).Error; err != nil {
		t.Fatal(err)
	}

	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(assetID)+"/previews/requeue", nil)
	requireStatus(t, w, http.StatusConflict)
}

func TestPostAssetPreviewRequeue_404CrossLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	// An asset that lives in a different library must not be requeue-able.
	other := seedLibrary(t, db, "Other")
	outside := models.Asset{LibraryID: other.ID, Name: "outside"}
	if err := db.Create(&outside).Error; err != nil {
		t.Fatal(err)
	}

	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(outside.ID)+"/previews/requeue", nil)
	requireStatus(t, w, http.StatusNotFound)
}

func TestPostFilePreviewRequeue_409Unsupported(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	// Import an unsupported file (.txt) — its single row has no backend renderer.
	body, ctype := buildMultipart(t, "file", "notes.txt", []byte("text"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)
	var f models.AssetFile
	if err := db.Where("relative_path = ?", "notes.txt").First(&f).Error; err != nil {
		t.Fatal(err)
	}

	w = performRequest(r, http.MethodPost,
		"/api/assets/"+uintStr(resp.AssetID)+"/files/"+uintStr(f.ID)+"/previews/requeue", nil)
	requireStatus(t, w, http.StatusConflict)
}

func TestGetPreviewQueue_ListsInFlight(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	// Seed a version with a queued file directly — the snapshot should surface it.
	asset := models.Asset{LibraryID: lib.ID, Name: "pack"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	f := models.AssetFile{AssetVersionID: v.ID, RelativePath: "a.bmp", ObjectKey: "k", ContentHash: "k", PreviewStatus: preview.StatusQueued}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}

	w := performGET(r, "/api/previews")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[previewQueueResponse](t, w)
	if body.Total != 1 || len(body.Previews) != 1 {
		t.Fatalf("expected 1 in-flight rollup, got total=%d rows=%d", body.Total, len(body.Previews))
	}
	if body.Previews[0].Status != preview.StatusQueued || body.Previews[0].Queued != 1 {
		t.Fatalf("rollup: %+v", body.Previews[0])
	}
}
