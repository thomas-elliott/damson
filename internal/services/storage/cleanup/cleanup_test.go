package cleanup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

func newEnv(t *testing.T) (*storage.LocalFS, *gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Asset{}, &models.AssetVersion{}, &models.AssetFile{}, &models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store, err := storage.NewLocalFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatalf("new local storage: %v", err)
	}
	return store, db, context.Background()
}

func putThumb(t *testing.T, store *storage.LocalFS, ctx context.Context, payload string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(payload))
	key := hex.EncodeToString(sum[:])
	if err := store.PutThumb(ctx, key, bytes.NewReader([]byte(payload))); err != nil {
		t.Fatalf("put thumb: %v", err)
	}
	return key
}

func candidateSet(cands []Candidate) map[string]bool {
	set := map[string]bool{}
	for _, c := range cands {
		set[c.Prefix+"|"+c.Key] = true
	}
	return set
}

func TestFindOrphans(t *testing.T) {
	store, db, ctx := newEnv(t)

	// Referenced blob (has an asset_files row) — must be spared.
	refBlob, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("referenced")), storage.ObjectMeta{})
	db.Create(&models.AssetFile{AssetVersionID: 1, RelativePath: "a", SizeBytes: 1, ContentHash: refBlob, ObjectKey: refBlob})

	// Orphan blob (no row) with metadata — must be found, meta surfaced.
	orphanMeta := storage.ObjectMeta{Filename: "lost.psd", LibraryID: 9, Kind: "loose"}
	orphanBlob, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("orphan-blob")), orphanMeta)

	// Referenced thumb (asset thumbnail) vs orphan thumb.
	refThumb := putThumb(t, store, ctx, "ref-thumb")
	db.Create(&models.Asset{LibraryID: 1, Name: "x", ThumbnailObjectKey: refThumb})
	orphanThumb := putThumb(t, store, ctx, "orphan-thumb")

	// Live archive original (purged=false) spared; purged original found; loose original found.
	liveOrig, _, _ := store.PutOriginal(ctx, bytes.NewReader([]byte("live-archive")), storage.ObjectMeta{})
	db.Create(&models.AssetVersion{AssetID: 1, VersionLabel: "1", OriginalPackageObjectKey: liveOrig, PackageKind: "unitypackage", OriginalPackagePurged: false})
	purgedOrig, _, _ := store.PutOriginal(ctx, bytes.NewReader([]byte("purged-archive")), storage.ObjectMeta{})
	db.Create(&models.AssetVersion{AssetID: 1, VersionLabel: "2", OriginalPackageObjectKey: purgedOrig, PackageKind: "unitypackage", OriginalPackagePurged: true})
	orphanOrig, _, _ := store.PutOriginal(ctx, bytes.NewReader([]byte("orphan-archive")), storage.ObjectMeta{})

	// minAge 0, now in the future → every object clears the age gate.
	future := time.Now().Add(time.Hour)
	cands, err := FindOrphans(ctx, store, db, 0, future)
	if err != nil {
		t.Fatalf("find orphans: %v", err)
	}
	set := candidateSet(cands)

	wantOrphan := []string{"blobs|" + orphanBlob, "thumbs|" + orphanThumb, "originals|" + purgedOrig, "originals|" + orphanOrig}
	for _, w := range wantOrphan {
		if !set[w] {
			t.Fatalf("expected orphan %s missing from %v", w, set)
		}
	}
	wantSpared := []string{"blobs|" + refBlob, "thumbs|" + refThumb, "originals|" + liveOrig}
	for _, s := range wantSpared {
		if set[s] {
			t.Fatalf("referenced object %s was flagged as orphan", s)
		}
	}

	// Metadata is surfaced on candidates.
	for _, c := range cands {
		if c.Prefix == "blobs" && c.Key == orphanBlob && c.Meta != orphanMeta {
			t.Fatalf("orphan meta: got %+v want %+v", c.Meta, orphanMeta)
		}
	}

	// Age guard: with a 24h minimum and now = now, freshly-written objects are
	// all too new — nothing is a candidate.
	fresh, err := FindOrphans(ctx, store, db, 24*time.Hour, time.Now())
	if err != nil {
		t.Fatalf("find orphans (age): %v", err)
	}
	if len(fresh) != 0 {
		t.Fatalf("age guard: expected no candidates, got %d", len(fresh))
	}
}

func TestDeleteOrphans(t *testing.T) {
	store, db, ctx := newEnv(t)
	future := time.Now().Add(time.Hour)

	deletable, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("delete-me")), storage.ObjectMeta{})
	raced, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("raced")), storage.ObjectMeta{})
	// Simulate a dedup re-use committed between scan and delete.
	db.Create(&models.AssetFile{AssetVersionID: 1, RelativePath: "b", SizeBytes: 1, ContentHash: raced, ObjectKey: raced})
	missing := hex.EncodeToString(sha256.New().Sum([]byte("never-stored"))[:32])

	keys := []Key{
		{Prefix: "blobs", Key: deletable},
		{Prefix: "blobs", Key: raced},
		{Prefix: "blobs", Key: missing},
		{Prefix: "evil", Key: deletable},
	}
	actor := uint(5)
	res, err := DeleteOrphans(ctx, store, db, keys, 0, future, &actor)
	if err != nil {
		t.Fatalf("delete orphans: %v", err)
	}

	if len(res.Deleted) != 1 || res.Deleted[0].Key != deletable {
		t.Fatalf("deleted: got %+v, want only %s", res.Deleted, deletable)
	}
	if res.FreedBytes != int64(len("delete-me")) {
		t.Fatalf("freed bytes: got %d, want %d", res.FreedBytes, len("delete-me"))
	}
	reasons := map[string]string{}
	for _, s := range res.Skipped {
		reasons[s.Key+"|"+s.Prefix] = s.Reason
	}
	if reasons[raced+"|blobs"] != "now referenced" {
		t.Fatalf("raced key: got reason %q", reasons[raced+"|blobs"])
	}
	if reasons[missing+"|blobs"] != "not found" {
		t.Fatalf("missing key: got reason %q", reasons[missing+"|blobs"])
	}
	if reasons[deletable+"|evil"] != "invalid prefix" {
		t.Fatalf("evil prefix: got reason %q", reasons[deletable+"|evil"])
	}

	// The deletable object is gone; the raced one survives.
	if _, exists, _ := store.HeadBlob(ctx, deletable); exists {
		t.Fatalf("deletable object still present")
	}
	if _, exists, _ := store.HeadBlob(ctx, raced); !exists {
		t.Fatalf("raced object was wrongly deleted")
	}

	// Exactly one audit row, for the one deletion.
	var auditCount int64
	db.Model(&models.AuditLog{}).Where("action_type = ?", "admin.storage.orphan_delete").Count(&auditCount)
	if auditCount != 1 {
		t.Fatalf("audit rows: got %d, want 1", auditCount)
	}
}
