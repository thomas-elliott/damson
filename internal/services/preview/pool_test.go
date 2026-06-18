package preview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/bmp"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// seedRenderableFile stores a small opaque BMP and creates an asset/version/file
// for it with the given starting preview status. Returns the version id.
func seedRenderableFile(t *testing.T, db *gorm.DB, store storage.Service, libraryID uint, status string) uint {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 30, G: 90, B: 200, A: 255})
		}
	}
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatalf("encode bmp: %v", err)
	}
	key, _, err := store.PutBlob(context.Background(), bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	asset := models.Asset{LibraryID: libraryID, Name: "a"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	version := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	f := models.AssetFile{
		AssetVersionID: version.ID,
		RelativePath:   "tex.bmp",
		ObjectKey:      key,
		ContentHash:    key,
		PreviewStatus:  status,
	}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	return version.ID
}

func TestSyncPool_ProcessesEnqueuedVersion(t *testing.T) {
	db := newProcessDB(t)
	store := newTestStorage(t)
	versionID := seedRenderableFile(t, db, store, 3, StatusQueued)

	pool := NewSync(Deps{DB: db, Storage: store})
	pool.EnqueueVersion(3, versionID)

	var f models.AssetFile
	if err := db.Where("asset_version_id = ?", versionID).First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if f.PreviewStatus != StatusDone {
		t.Fatalf("status after sync enqueue: got %q, want done", f.PreviewStatus)
	}
}

func TestResumeInFlight_RecoversCrashedProcessing(t *testing.T) {
	db := newProcessDB(t)
	store := newTestStorage(t)
	// A file left in 'processing' by a crash — ProcessVersion only picks up
	// 'queued', so without the reset this would never complete.
	versionID := seedRenderableFile(t, db, store, 5, StatusProcessing)

	pool := NewSync(Deps{DB: db, Storage: store})
	n, err := ResumeInFlight(context.Background(), db, pool)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if n != 1 {
		t.Fatalf("resumed versions: got %d, want 1", n)
	}
	var f models.AssetFile
	if err := db.Where("asset_version_id = ?", versionID).First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if f.PreviewStatus != StatusDone {
		t.Fatalf("crashed processing file not recovered: got %q, want done", f.PreviewStatus)
	}
}
