package imports

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"testing/iotest"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imaging/imagingtest"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// Retrying an older version's failed import, after a newer version became
// current, must leave the newer version and its cover in place.
func TestRun_OlderVersionImportDoesNotRollBackCurrentVersion(t *testing.T) {
	db := enrichTestDB(t)
	ctx := context.Background()
	store, err := storage.NewLocalFS(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	asset := seedEnrichAsset(t, db, "Texture", false)

	key, _, err := store.PutBlob(ctx, bytes.NewReader(imagingtest.TGA(t)), storage.ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	older := models.AssetVersion{AssetID: asset.ID, ImportDate: time.Now().Add(-time.Hour), PackageKind: "loose", OriginalPackageObjectKey: key}
	newer := models.AssetVersion{AssetID: asset.ID, ImportDate: time.Now(), PackageKind: "loose", OriginalPackageObjectKey: key}
	for _, v := range []*models.AssetVersion{&older, &newer} {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
		f := models.AssetFile{AssetVersionID: v.ID, RelativePath: "texture.tga", ObjectKey: key, ContentHash: key}
		if err := db.Create(&f).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(asset).Updates(map[string]any{
		"current_version_id": newer.ID, "thumbnail_object_key": "newer-cover", "thumbnail_content_type": "image/png",
	}).Error; err != nil {
		t.Fatal(err)
	}
	imp := models.AssetImport{AssetID: asset.ID, AssetVersionID: &older.ID, Stage: StageFailed}
	if err := db.Create(&imp).Error; err != nil {
		t.Fatal(err)
	}

	if err := Run(ctx, Deps{DB: db, Storage: store}, Job{ImportID: imp.ID, LibraryID: asset.LibraryID}); err != nil {
		t.Fatalf("run: %v", err)
	}

	var a models.Asset
	if err := db.First(&a, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if a.CurrentVersionID == nil || *a.CurrentVersionID != newer.ID {
		t.Errorf("current version = %v, want the newer %d", a.CurrentVersionID, newer.ID)
	}
	if a.ThumbnailObjectKey != "newer-cover" {
		t.Errorf("cover = %q, want the newer version's", a.ThumbnailObjectKey)
	}
}

// flakyStore fails opening or reading originals, or writing blobs, as an
// object store does when it is briefly unreachable.
type flakyStore struct {
	storage.Service
	failGetOriginal, failReadOriginal, failPutBlob bool
}

func (s flakyStore) GetOriginal(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.failGetOriginal {
		return nil, errors.New("connection reset")
	}
	rc, err := s.Service.GetOriginal(ctx, key)
	if err != nil || !s.failReadOriginal {
		return rc, err
	}
	rc.Close()
	return io.NopCloser(iotest.ErrReader(errors.New("connection reset"))), nil
}

func (s flakyStore) PutBlob(ctx context.Context, r io.Reader, meta storage.ObjectMeta) (string, int64, error) {
	if s.failPutBlob {
		return "", 0, errors.New("connection reset")
	}
	return s.Service.PutBlob(ctx, r, meta)
}

// seedZipImport stores payload as a zip original and creates the import for
// it, returning the store and the import id.
func seedZipImport(t *testing.T, payload []byte) (*gorm.DB, storage.Service, models.AssetImport) {
	t.Helper()
	db := enrichTestDB(t)
	store, err := storage.NewLocalFS(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := store.PutOriginal(context.Background(), bytes.NewReader(payload), storage.ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	asset := seedEnrichAsset(t, db, "Pack", false)
	v := models.AssetVersion{AssetID: asset.ID, PackageKind: "zip", OriginalPackageObjectKey: key}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	imp := models.AssetImport{AssetID: asset.ID, AssetVersionID: &v.ID, Stage: StageUploaded}
	if err := db.Create(&imp).Error; err != nil {
		t.Fatal(err)
	}
	return db, store, imp
}

func oneFileZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A storage error is transient: the import must stay retryable rather than
// be marked failed, which makes the queue cancel it.
func TestRun_StorageErrorLeavesImportRetryable(t *testing.T) {
	cases := map[string]flakyStore{
		"opening the original": {failGetOriginal: true},
		"reading the original": {failReadOriginal: true},
		"writing a blob":       {failPutBlob: true},
	}
	for name, flaky := range cases {
		t.Run(name, func(t *testing.T) {
			db, store, imp := seedZipImport(t, oneFileZip(t))
			flaky.Service = store

			err := Run(context.Background(), Deps{DB: db, Storage: flaky}, Job{ImportID: imp.ID})
			if err == nil {
				t.Fatal("run: want an error")
			}
			if IsFailed(context.Background(), db, imp.ID) {
				t.Error("import marked failed, so the queue would cancel instead of retrying")
			}
		})
	}
}

// A missing original never comes back, so retrying it only delays the failure.
func TestRun_MissingOriginalFailsTheImport(t *testing.T) {
	db, store, imp := seedZipImport(t, oneFileZip(t))
	var v models.AssetVersion
	if err := db.First(&v, *imp.AssetVersionID).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteOriginal(context.Background(), v.OriginalPackageObjectKey); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), Deps{DB: db, Storage: store}, Job{ImportID: imp.ID}); err == nil {
		t.Fatal("run: want an error")
	}
	if !IsFailed(context.Background(), db, imp.ID) {
		t.Error("import not marked failed")
	}
}

// A corrupt archive will not get better on a retry, so it stays terminal.
func TestRun_CorruptArchiveFailsTheImport(t *testing.T) {
	db, store, imp := seedZipImport(t, []byte("not a zip"))

	if err := Run(context.Background(), Deps{DB: db, Storage: store}, Job{ImportID: imp.ID}); err == nil {
		t.Fatal("run: want an error")
	}
	if !IsFailed(context.Background(), db, imp.ID) {
		t.Error("import not marked failed")
	}
}
