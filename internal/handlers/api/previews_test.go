package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/preview"
)

// spyPreviewEnqueuer is a PreviewEnqueuer that records calls instead of running
// a pool. It is deliberately not a *preview.Pool, so a passing test proves the
// handler routes through the ImportEnqueuer/PreviewEnqueuer seam rather than a
// concrete pool type.
type spyPreviewEnqueuer struct {
	jobCalls []preview.Job
}

func (s *spyPreviewEnqueuer) Enqueue(j preview.Job) { s.jobCalls = append(s.jobCalls, j) }

// TestPreviewRequeue_RoutesThroughEnqueuerSeam injects a non-pool enqueuer and
// asserts the requeue handler enqueues the version through the interface.
func TestPreviewRequeue_RoutesThroughEnqueuerSeam(t *testing.T) {
	db := newTestDB(t)
	store := newTestStorage(t)
	spy := &spyPreviewEnqueuer{}
	h := New(Deps{DB: db, AuthMode: config.AuthModeOIDC, Storage: store, PreviewPool: spy})

	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "pack"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	asset.CurrentVersionID = &v.ID
	if err := db.Save(&asset).Error; err != nil {
		t.Fatal(err)
	}
	f := models.AssetFile{AssetVersionID: v.ID, RelativePath: "tex.bmp", ObjectKey: "k", ContentHash: "k"}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(asset.ID)+"/previews/requeue", nil)
	requireStatus(t, w, http.StatusAccepted)

	if len(spy.jobCalls) != 1 {
		t.Fatalf("expected exactly one Enqueue call through the seam, got %d", len(spy.jobCalls))
	}
	if want := (preview.Job{LibraryID: lib.ID, VersionID: v.ID}); spy.jobCalls[0] != want {
		t.Fatalf("Enqueue args: got %+v, want %+v", spy.jobCalls[0], want)
	}
}

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

func TestGetPreviewQueue_ListsFinished(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	// A version whose only renderable file finished recently: not in flight,
	// but retained in the finished tail.
	asset := models.Asset{LibraryID: lib.ID, Name: "done-pack"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	f := models.AssetFile{
		AssetVersionID: v.ID, RelativePath: "a.bmp", ObjectKey: "k", ContentHash: "k",
		PreviewStatus: preview.StatusDone, PreviewObjectKey: "p", PreviewUpdatedAt: &now,
	}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}

	w := performGET(r, "/api/previews")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[previewQueueResponse](t, w)
	if body.Total != 0 || len(body.Previews) != 0 {
		t.Fatalf("expected nothing in flight, got total=%d rows=%d", body.Total, len(body.Previews))
	}
	if len(body.Finished) != 1 || body.Finished[0].Done != 1 || body.Finished[0].AssetID != asset.ID {
		t.Fatalf("expected the completed rollup in finished, got %+v", body.Finished)
	}

	// A rollup finished outside the retention window drops off.
	stale := now.Add(-8 * 24 * time.Hour)
	if err := db.Model(&models.AssetFile{}).Where("id = ?", f.ID).
		Update("preview_updated_at", stale).Error; err != nil {
		t.Fatal(err)
	}
	w = performGET(r, "/api/previews")
	body = jsonResponse[previewQueueResponse](t, w)
	if len(body.Finished) != 0 {
		t.Fatalf("expected the stale rollup to age out of finished, got %+v", body.Finished)
	}
}

func TestPostAssetRescan_RegeneratesThumbnailAndPreviews(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	assetID, f := importBMP(t, h, u, r)

	// Knock the preview back so the rescan has something to requeue.
	if err := db.Model(&models.AssetFile{}).Where("id = ?", f.ID).
		Updates(map[string]any{"preview_status": preview.StatusFailed, "preview_object_key": ""}).Error; err != nil {
		t.Fatal(err)
	}

	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(assetID)+"/rescan", nil)
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[rescanResponse](t, w)
	if !resp.ThumbnailFresh {
		t.Errorf("expected a regenerated thumbnail for a raster asset")
	}
	if resp.PreviewsQueued != 1 {
		t.Errorf("expected 1 preview queued, got %d", resp.PreviewsQueued)
	}

	// The synchronous preview pool should have driven the requeued file back to done.
	var got models.AssetFile
	if err := db.First(&got, f.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.PreviewStatus != preview.StatusDone || got.PreviewObjectKey == "" {
		t.Fatalf("file not re-rendered after rescan: %+v", got)
	}

	var asset models.Asset
	if err := db.First(&asset, assetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.ThumbnailObjectKey == "" {
		t.Fatal("expected the asset thumbnail_object_key to be set after rescan")
	}
}

// TestPostAssetRescan_KeepsStoreCover asserts a storefront-sourced cover is
// never regenerated over by a rescan: the store cover always wins.
func TestPostAssetRescan_KeepsStoreCover(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	assetID, _ := importBMP(t, h, u, r)

	if err := db.Model(&models.Asset{}).Where("id = ?", assetID).
		Updates(map[string]any{
			"thumbnail_object_key":   "store-cover",
			"thumbnail_content_type": "image/png",
			"thumbnail_from_store":   true,
		}).Error; err != nil {
		t.Fatal(err)
	}

	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(assetID)+"/rescan", nil)
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[rescanResponse](t, w)
	if resp.ThumbnailFresh {
		t.Error("expected the store cover to be preserved, not regenerated")
	}

	var asset models.Asset
	if err := db.First(&asset, assetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.ThumbnailObjectKey != "store-cover" {
		t.Fatalf("store cover overwritten: got %q, want store-cover", asset.ThumbnailObjectKey)
	}
	if !asset.ThumbnailFromStore {
		t.Error("expected thumbnail_from_store to remain true after rescan")
	}
}

func TestPostAssetRescan_404CrossLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	other := seedLibrary(t, db, "Other")
	outside := models.Asset{LibraryID: other.ID, Name: "outside"}
	if err := db.Create(&outside).Error; err != nil {
		t.Fatal(err)
	}
	w := performRequest(r, http.MethodPost, "/api/assets/"+uintStr(outside.ID)+"/rescan", nil)
	requireStatus(t, w, http.StatusNotFound)
}
