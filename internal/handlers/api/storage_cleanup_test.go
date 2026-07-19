package api

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

func TestStorageCleanup_RequiresSysadmin(t *testing.T) {
	h, db := newTestHandler(t)
	user := seedUser(t, db, "member@example.com") // role "user"
	r := setupRouter(h, user)

	w := performRequest(r, http.MethodPost, "/api/admin/storage/cleanup/scan", nil)
	requireStatus(t, w, http.StatusForbidden)
}

func TestStorageCleanup_ScanAndDelete(t *testing.T) {
	h, db := newTestHandler(t)
	// Negative min-age → cutoff in the future, so freshly-written test objects
	// clear the age gate deterministically regardless of clock timing.
	h.orphanMinAge = -time.Hour

	admin := seedUser(t, db, "admin@example.com")
	admin.Role = "sysadmin"
	ctx := context.Background()

	orphan, _, err := h.storage.PutBlob(ctx, bytes.NewReader([]byte("orphan")), storage.ObjectMeta{Filename: "lost.bin"})
	if err != nil {
		t.Fatalf("put orphan: %v", err)
	}
	referenced, _, err := h.storage.PutBlob(ctx, bytes.NewReader([]byte("kept")), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put referenced: %v", err)
	}
	lib := seedLibrary(t, db, "Personal")
	_, v := seedAssetWithVersion(t, db, lib.ID, "pack", "v1")
	db.Create(&models.AssetFile{AssetVersionID: v.ID, RelativePath: "a", SizeBytes: 4, ContentHash: referenced, ObjectKey: referenced})

	r := setupRouter(h, admin)

	// Scan surfaces the orphan (with metadata) and spares the referenced blob.
	w := performRequest(r, http.MethodPost, "/api/admin/storage/cleanup/scan", nil)
	requireStatus(t, w, http.StatusOK)
	scan := jsonResponse[storageCleanupScanView](t, w)
	var foundOrphan bool
	for _, cand := range scan.Candidates {
		if cand.Key == referenced {
			t.Fatalf("referenced blob surfaced as orphan")
		}
		if cand.Prefix == "blobs" && cand.Key == orphan {
			foundOrphan = true
			if cand.Filename != "lost.bin" {
				t.Fatalf("orphan metadata not surfaced: %+v", cand)
			}
		}
	}
	if !foundOrphan {
		t.Fatalf("orphan not found in scan: %+v", scan.Candidates)
	}

	// Delete the orphan; a bogus prefix is skipped, not deleted.
	body := storageCleanupDeleteRequest{}
	body.Keys = append(body.Keys, struct {
		Prefix string `json:"prefix"`
		Key    string `json:"key"`
	}{Prefix: "blobs", Key: orphan}, struct {
		Prefix string `json:"prefix"`
		Key    string `json:"key"`
	}{Prefix: "evil", Key: orphan})

	w = performRequest(r, http.MethodPost, "/api/admin/storage/cleanup/delete", body)
	requireStatus(t, w, http.StatusOK)
	del := jsonResponse[storageCleanupDeleteView](t, w)
	if del.DeletedCount != 1 || del.FreedBytes != int64(len("orphan")) {
		t.Fatalf("delete result: %+v", del)
	}
	var sawEvilSkip bool
	for _, s := range del.Skipped {
		if s.Prefix == "evil" && s.Reason == "invalid prefix" {
			sawEvilSkip = true
		}
	}
	if !sawEvilSkip {
		t.Fatalf("evil prefix not skipped: %+v", del.Skipped)
	}

	if _, exists, _ := h.storage.HeadBlob(ctx, orphan); exists {
		t.Fatalf("orphan still present after delete")
	}
	if _, exists, _ := h.storage.HeadBlob(ctx, referenced); !exists {
		t.Fatalf("referenced blob was wrongly deleted")
	}
}
