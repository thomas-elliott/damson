package uploads

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/testdb"
)

const testKey = "aa" + "bbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

func newEnv(t *testing.T) (*gorm.DB, *storage.LocalFS) {
	t.Helper()
	db := testdb.NewPostgres(t)
	store, err := storage.NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return db, store
}

func seedLibraryAndUser(t *testing.T, db *gorm.DB) (uint, uint) {
	t.Helper()
	lib := models.Library{Name: "Personal"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	u := models.User{Email: "librarian@example.com", DisplayName: "Thomas"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	return lib.ID, u.ID
}

func seedSession(t *testing.T, db *gorm.DB, store *storage.LocalFS, libID, userID uint, expiresAt time.Time) models.UploadSession {
	t.Helper()
	up, err := store.BeginMultipart(context.Background(), "originals", testKey, storage.ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := up.UploadPart(context.Background(), 1, 4, strings.NewReader("abcd")); err != nil {
		t.Fatal(err)
	}
	s := models.UploadSession{
		LibraryID:       libID,
		UserID:          userID,
		Status:          models.UploadOpen,
		Filename:        "Pack.unitypackage",
		SizeBytes:       64,
		Sha256:          testKey,
		ChunkSize:       32,
		ObjectPrefix:    "originals",
		BackendUploadID: up.ID(),
		PartsJSON:       []byte("[]"),
		ReceivedBytes:   4,
		NextChunk:       1,
		ExpiresAt:       expiresAt,
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatal(err)
	}
	return s
}

// TestSweepOnce_ExpiresAbandonedSessions is the reason the janitor exists: an
// abandoned session's multipart upload holds storage that no bucket listing
// reveals, so nothing else would ever reclaim it.
func TestSweepOnce_ExpiresAbandonedSessions(t *testing.T) {
	db, store := newEnv(t)
	libID, userID := seedLibraryAndUser(t, db)
	now := time.Now().UTC()

	stale := seedSession(t, db, store, libID, userID, now.Add(-time.Minute))
	fresh := seedSession(t, db, store, libID, userID, now.Add(time.Hour))

	swept, err := SweepOnce(context.Background(), db, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if swept != 1 {
		t.Fatalf("swept = %d, want only the expired session", swept)
	}

	var got models.UploadSession
	if err := db.First(&got, stale.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != models.UploadExpired {
		t.Errorf("stale session status = %q, want expired", got.Status)
	}
	if _, err := store.ResumeMultipart(context.Background(), "originals", testKey, stale.BackendUploadID); err == nil {
		t.Error("the expired session's multipart upload is still open")
	}

	var live models.UploadSession
	if err := db.First(&live, fresh.ID).Error; err != nil {
		t.Fatal(err)
	}
	if live.Status != models.UploadOpen {
		t.Errorf("live session status = %q, want it untouched", live.Status)
	}
	if _, err := store.ResumeMultipart(context.Background(), "originals", testKey, fresh.BackendUploadID); err != nil {
		t.Errorf("a live session's multipart must survive the sweep: %v", err)
	}
}

// TestSweepOnce_LeavesTerminalSessionsAlone keeps a completed upload from
// being re-aborted, which would target whatever multipart id it recorded.
func TestSweepOnce_LeavesTerminalSessionsAlone(t *testing.T) {
	db, store := newEnv(t)
	libID, userID := seedLibraryAndUser(t, db)
	now := time.Now().UTC()

	done := seedSession(t, db, store, libID, userID, now.Add(-time.Hour))
	if err := db.Model(&models.UploadSession{}).Where("id = ?", done.ID).
		Update("status", models.UploadCompleted).Error; err != nil {
		t.Fatal(err)
	}

	swept, err := SweepOnce(context.Background(), db, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if swept != 0 {
		t.Errorf("swept = %d, want 0 — only open sessions expire", swept)
	}
	var got models.UploadSession
	if err := db.First(&got, done.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != models.UploadCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
}

// TestSweepOnce_MarksExpiredEvenWhenTheAbortFails: the status is what stops a
// client resuming, so a storage failure must not leave the row resumable.
func TestSweepOnce_MarksExpiredEvenWhenTheAbortFails(t *testing.T) {
	db, store := newEnv(t)
	libID, userID := seedLibraryAndUser(t, db)
	now := time.Now().UTC()

	s := seedSession(t, db, store, libID, userID, now.Add(-time.Minute))
	if err := db.Model(&models.UploadSession{}).Where("id = ?", s.ID).
		Update("backend_upload_id", "gone").Error; err != nil {
		t.Fatal(err)
	}

	swept, err := SweepOnce(context.Background(), db, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if swept != 1 {
		t.Fatalf("swept = %d, want 1", swept)
	}
	var got models.UploadSession
	if err := db.First(&got, s.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != models.UploadExpired {
		t.Errorf("status = %q, want expired despite the failed abort", got.Status)
	}
}
