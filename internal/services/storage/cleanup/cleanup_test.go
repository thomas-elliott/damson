package cleanup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/testdb"
)

func newEnv(t *testing.T) (*storage.LocalFS, *gorm.DB, context.Context) {
	t.Helper()
	db := testdb.NewPostgres(t)
	store, err := storage.NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatalf("new local storage: %v", err)
	}
	return store, db, context.Background()
}

// seedParents creates the library → asset → version chain the FK constraints
// require, returning the ids so callers can attach asset_files / asset_versions
// to a real parent. The orphan logic only cares that a row references a blob
// key, not which parent it hangs off, so one shared chain suffices.
func seedParents(t *testing.T, db *gorm.DB) (libID, assetID, versionID uint) {
	t.Helper()
	lib := models.Library{Name: "lib"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	asset := models.Asset{LibraryID: lib.ID, Name: "seed"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	ver := models.AssetVersion{AssetID: asset.ID, VersionLabel: "seed"}
	if err := db.Create(&ver).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}
	return lib.ID, asset.ID, ver.ID
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
	libID, assetID, versionID := seedParents(t, db)

	// Referenced blob (has an asset_files row) — must be spared.
	refBlob, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("referenced")), storage.ObjectMeta{})
	db.Create(&models.AssetFile{AssetVersionID: versionID, RelativePath: "a", SizeBytes: 1, ContentHash: refBlob, ObjectKey: refBlob})

	// Orphan blob (no row) with metadata — must be found, meta surfaced.
	orphanMeta := storage.ObjectMeta{Filename: "lost.psd", LibraryID: 9, Kind: "loose"}
	orphanBlob, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("orphan-blob")), orphanMeta)

	// Referenced thumb (asset thumbnail) vs orphan thumb.
	refThumb := putThumb(t, store, ctx, "ref-thumb")
	db.Create(&models.Asset{LibraryID: libID, Name: "x", ThumbnailObjectKey: refThumb})
	orphanThumb := putThumb(t, store, ctx, "orphan-thumb")

	// Live archive original (purged=false) spared; purged original found; loose original found.
	liveOrig, _, _ := store.PutOriginal(ctx, bytes.NewReader([]byte("live-archive")), storage.ObjectMeta{})
	db.Create(&models.AssetVersion{AssetID: assetID, VersionLabel: "1", OriginalPackageObjectKey: liveOrig, PackageKind: "unitypackage", OriginalPackagePurged: false})
	purgedOrig, _, _ := store.PutOriginal(ctx, bytes.NewReader([]byte("purged-archive")), storage.ObjectMeta{})
	db.Create(&models.AssetVersion{AssetID: assetID, VersionLabel: "2", OriginalPackageObjectKey: purgedOrig, PackageKind: "unitypackage", OriginalPackagePurged: true})
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
	_, _, versionID := seedParents(t, db)
	future := time.Now().Add(time.Hour)

	deletable, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("delete-me")), storage.ObjectMeta{})
	raced, _, _ := store.PutBlob(ctx, bytes.NewReader([]byte("raced")), storage.ObjectMeta{})
	// Simulate a dedup re-use committed between scan and delete.
	db.Create(&models.AssetFile{AssetVersionID: versionID, RelativePath: "b", SizeBytes: 1, ContentHash: raced, ObjectKey: raced})
	missing := hex.EncodeToString(sha256.New().Sum([]byte("never-stored"))[:32])

	keys := []Key{
		{Prefix: "blobs", Key: deletable},
		{Prefix: "blobs", Key: raced},
		{Prefix: "blobs", Key: missing},
		{Prefix: "evil", Key: deletable},
	}
	user := models.User{Email: "actor@example.com", DisplayName: "actor", Role: "user"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed actor: %v", err)
	}
	actor := user.ID
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

// The sweep keeps only what models.ObjectKeyColumns lists, so an object key
// column missing from the list would have its objects deleted.
func TestObjectKeyColumns_ListsEveryKeyColumn(t *testing.T) {
	_, db, _ := newEnv(t)
	var inSchema []string
	if err := db.Raw(`SELECT table_name || '.' || column_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND column_name LIKE '%object\_key'
		ORDER BY 1`).Scan(&inSchema).Error; err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, col := range models.ObjectKeyColumns {
		listed[col.Table+"."+col.Column] = true
		if !isSweptPrefix(col.Prefix) {
			t.Errorf("%s.%s maps to %q, which the sweep does not list", col.Table, col.Column, col.Prefix)
		}
	}
	for _, c := range inSchema {
		if !listed[c] {
			t.Errorf("%s holds object keys but is not in models.ObjectKeyColumns", c)
		}
		delete(listed, c)
	}
	for c := range listed {
		t.Errorf("models.ObjectKeyColumns lists %s, which is not in the schema", c)
	}
}
