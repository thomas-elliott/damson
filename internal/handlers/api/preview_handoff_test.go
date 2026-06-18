package api

import (
	"bytes"
	"image"
	"image/color"
	"net/http"
	"testing"

	"golang.org/x/image/bmp"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/preview"
)

// realBMP returns the bytes of a small opaque BMP — a file type the backend
// preview renderer supports.
func realBMP(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 180, G: 90, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatalf("encode bmp: %v", err)
	}
	return buf.Bytes()
}

// TestPostImport_HandsOffToPreviewQueue proves the decoupled handoff: an import
// reaches `done` (no inline previewing stage), and a supported file is driven to
// preview_status=done with a derivative key by the (synchronous) preview pool.
func TestPostImport_HandsOffToPreviewQueue(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	body, ctype := buildMultipart(t, "file", "tex.bmp", realBMP(t))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[importEnqueueResponse](t, w)

	// Import is done — previewing no longer blocks it.
	var imp models.AssetImport
	if err := db.First(&imp, resp.ImportID).Error; err != nil {
		t.Fatal(err)
	}
	if imp.Stage != "done" {
		t.Fatalf("import stage: got %q, want done", imp.Stage)
	}

	// The preview pool rendered the supported file.
	var f models.AssetFile
	if err := db.Where("relative_path = ?", "tex.bmp").First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if f.PreviewStatus != preview.StatusDone {
		t.Fatalf("preview_status: got %q, want done", f.PreviewStatus)
	}
	if f.PreviewObjectKey == "" {
		t.Fatal("expected a preview_object_key after handoff")
	}
}

// TestPostImport_UnsupportedFileMarkedUnsupported proves the failure/terminal
// path: a file with no backend renderer ends 'unsupported', not stuck blank.
func TestPostImport_UnsupportedFileMarkedUnsupported(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	body, ctype := buildMultipart(t, "file", "notes.txt", []byte("just text"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)

	var f models.AssetFile
	if err := db.Where("relative_path = ?", "notes.txt").First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if f.PreviewStatus != preview.StatusUnsupported {
		t.Fatalf("preview_status: got %q, want unsupported", f.PreviewStatus)
	}
	if f.PreviewObjectKey != "" {
		t.Fatalf("unsupported file should have no preview key, got %q", f.PreviewObjectKey)
	}
}
