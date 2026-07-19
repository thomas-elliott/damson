package api

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestPostImport_RejectsOversizeUpload(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	_ = seedLibraryAndSwitch(t, h, u)

	// Small server-wide cap so the test stays cheap. Non-zero MaxUploadBytes is
	// stored verbatim (the 2 GiB default only applies to a zero value). The
	// migrations already seed server_settings(id=1), so update rather than insert.
	if err := db.Model(&models.ServerSettings{}).Where("id = ?", 1).
		Update("max_upload_bytes", 2<<20).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	// 4 MiB payload exceeds the 2 MiB cap plus the 1 MiB multipart headroom.
	big := bytes.Repeat([]byte("a"), 4<<20)
	body, ctype := buildMultipart(t, "file", "big.bin", big)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusRequestEntityTooLarge)
}

func TestPostImport_AcceptsUploadWithinLimit(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	_ = seedLibraryAndSwitch(t, h, u)

	if err := db.Model(&models.ServerSettings{}).Where("id = ?", 1).
		Update("max_upload_bytes", 2<<20).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	small := bytes.Repeat([]byte("a"), 64<<10) // 64 KiB, well under the cap
	body, ctype := buildMultipart(t, "file", "small.bin", small)
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
}
