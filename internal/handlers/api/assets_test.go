package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

// libraryContextShim attaches a library + membership to every request in the
// same way requireLibrary would for an authenticated user. Used for tests
// that skip the full chain.
func seedLibraryAndSwitch(t *testing.T, h *Handler, u *models.User) *models.Library {
	t.Helper()
	lib := seedLibrary(t, h.db, "Personal")
	seedMembership(t, h.db, lib.ID, u.ID, "admin")
	u.CurrentLibraryID = &lib.ID
	if err := h.db.Save(u).Error; err != nil {
		t.Fatal(err)
	}
	return lib
}

func buildMultipart(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return body, w.FormDataContentType()
}

func postMultipart(r http.Handler, path string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPostImport_StoresAssetAndVersion(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "grass-texture.png", []byte("fake png bytes"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)

	resp := jsonResponse[importEnqueueResponse](t, w)
	if resp.AssetID == 0 || resp.ImportID == 0 {
		t.Fatalf("missing ids in response: %+v", resp)
	}

	// Asset row is library-scoped and has current_version_id set after the
	// sync test pool runs the pipeline.
	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.LibraryID != lib.ID {
		t.Fatalf("library_id: got %d, want %d", a.LibraryID, lib.ID)
	}
	if a.CurrentVersionID == nil {
		t.Fatalf("current_version_id not set: %+v", a.CurrentVersionID)
	}
	if a.Name != "grass texture" {
		t.Fatalf("name: got %q, want %q", a.Name, "grass texture")
	}

	// File row carries the content hash as its object key.
	var f models.AssetFile
	if err := db.Where("asset_version_id = ?", *a.CurrentVersionID).First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if f.ContentHash == "" || f.ContentHash != f.ObjectKey {
		t.Fatalf("hash/key mismatch: %+v", f)
	}

	// Import row reaches the done stage.
	var imp models.AssetImport
	if err := db.First(&imp, resp.ImportID).Error; err != nil {
		t.Fatal(err)
	}
	if imp.Stage != "done" {
		t.Fatalf("stage: got %q, want done; err=%q", imp.Stage, imp.Error)
	}
}

func TestPostImport_RejectsWithoutLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	// No library membership, no current_library_id.

	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "file", "x.bin", []byte("x"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusForbidden)
}

func TestGetAssets_SizeSumsSameSizedFiles(t *testing.T) {
	h, db := newTestHandler(t)
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
	// Three files, two sharing a size. A SUM(DISTINCT size_bytes) would dedupe
	// to 100+50=150; the real total is 250 across 3 files.
	for i, size := range []int64{100, 100, 50} {
		f := models.AssetFile{
			AssetVersionID: v.ID,
			RelativePath:   "f" + uintStr(uint(i)),
			SizeBytes:      size,
			ContentHash:    "h",
			ObjectKey:      "o",
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(body.Assets))
	}
	if got := body.Assets[0].SizeBytes; got != 250 {
		t.Fatalf("size_bytes: got %d, want 250 (same-sized files must not dedupe)", got)
	}
	if got := body.Assets[0].FileCount; got != 3 {
		t.Fatalf("file_count: got %d, want 3", got)
	}
}

func TestAssetFile_PreviewStatusDefaultsUnsupported(t *testing.T) {
	_, db := newTestHandler(t)
	lib := seedLibrary(t, db, "Personal")
	_, v := seedAssetWithVersion(t, db, lib.ID, "pack", "v1")

	// A file row created without an explicit PreviewStatus must land on the
	// schema default ('unsupported'), not the Go empty string — the pipeline
	// and requeue rely on that to tell "no preview yet" from "queued".
	f := models.AssetFile{
		AssetVersionID: v.ID,
		RelativePath:   "tex.psd",
		SizeBytes:      10,
		ContentHash:    "h",
		ObjectKey:      "o",
	}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	var got models.AssetFile
	if err := db.First(&got, f.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.PreviewStatus != "unsupported" {
		t.Fatalf("preview_status default: got %q, want %q", got.PreviewStatus, "unsupported")
	}
}

func TestGetAssets_ListsOnlyCurrentLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	personal := seedLibraryAndSwitch(t, h, u)

	// Seed an asset in the current library and one in an unrelated library.
	inScope := models.Asset{LibraryID: personal.ID, Name: "in-scope"}
	if err := db.Create(&inScope).Error; err != nil {
		t.Fatal(err)
	}
	otherLib := seedLibrary(t, db, "Other")
	outOfScope := models.Asset{LibraryID: otherLib.ID, Name: "out-of-scope"}
	if err := db.Create(&outOfScope).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if body.Total != 1 || len(body.Assets) != 1 {
		t.Fatalf("expected 1 asset, got total=%d rows=%d", body.Total, len(body.Assets))
	}
	if body.Assets[0].Name != "in-scope" {
		t.Fatalf("wrong asset returned: %+v", body.Assets[0])
	}
}
