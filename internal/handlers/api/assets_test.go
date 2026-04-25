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
	requireStatus(t, w, http.StatusCreated)

	resp := jsonResponse[importResponse](t, w)
	if resp.AssetID == 0 || resp.AssetVersionID == 0 {
		t.Fatalf("missing ids in response: %+v", resp)
	}
	if resp.PackageKind != "loose" || resp.FileCount != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	// Asset row is library-scoped and has current_version_id set.
	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.LibraryID != lib.ID {
		t.Fatalf("library_id: got %d, want %d", a.LibraryID, lib.ID)
	}
	if a.CurrentVersionID == nil || *a.CurrentVersionID != resp.AssetVersionID {
		t.Fatalf("current_version_id not set: %+v", a.CurrentVersionID)
	}
	if a.Name != "grass-texture" {
		t.Fatalf("name: got %q, want %q", a.Name, "grass-texture")
	}

	// File row carries the content hash as its object key.
	var f models.AssetFile
	if err := db.Where("asset_version_id = ?", resp.AssetVersionID).First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if f.ContentHash == "" || f.ContentHash != f.ObjectKey {
		t.Fatalf("hash/key mismatch: %+v", f)
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
