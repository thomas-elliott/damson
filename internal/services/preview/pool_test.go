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
	key, _, err := store.Put(context.Background(), storage.Blobs, bytes.NewReader(buf.Bytes()), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	ensureLibrary(t, db, libraryID)
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

// inlineEnqueuer processes each job on the calling goroutine.
type inlineEnqueuer struct{ deps Deps }

func (e inlineEnqueuer) Enqueue(_ *gorm.DB, job Job) error {
	_ = ProcessVersion(context.Background(), e.deps, job.LibraryID, job.VersionID, job.FileID)
	return nil
}

func TestResumeInFlight_RecoversCrashedProcessing(t *testing.T) {
	db := newProcessDB(t)
	store := newTestStorage(t)
	// A file left in 'processing' by a crash — ProcessVersion only picks up
	// 'queued', so without the reset this would never complete.
	versionID := seedRenderableFile(t, db, store, 5, StatusProcessing)

	n, err := ResumeInFlight(context.Background(), db, inlineEnqueuer{Deps{DB: db, Storage: store}})
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

// Only files a renderer now exists for are queued, by extension in any case,
// and only in libraries with previews on.
func TestQueueNewlySupported_QueuesOnlyRenderableFilesWherePreviewsAreOn(t *testing.T) {
	db := newProcessDB(t)
	seedFiles := func(libraryID uint, paths ...string) {
		ensureLibrary(t, db, libraryID)
		asset := models.Asset{LibraryID: libraryID, Name: "a"}
		if err := db.Create(&asset).Error; err != nil {
			t.Fatal(err)
		}
		version := models.AssetVersion{AssetID: asset.ID}
		if err := db.Create(&version).Error; err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			f := models.AssetFile{AssetVersionID: version.ID, RelativePath: p, ObjectKey: p, ContentHash: p, PreviewStatus: StatusUnsupported}
			if err := db.Create(&f).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	seedFiles(1, "Textures/Sky.EXR", "Scripts/Player.cs", "Icons/logo.svg")
	seedFiles(2, "Textures/ground.tga")
	if err := db.Create(&models.LibrarySettings{LibraryID: 2}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.LibrarySettings{}).Where("library_id = ?", 2).Update("previews_enabled", false).Error; err != nil {
		t.Fatal(err)
	}

	n, err := QueueNewlySupported(context.Background(), db)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if n != 2 {
		t.Errorf("queued %d, want 2", n)
	}
	want := map[string]string{
		"Textures/Sky.EXR":    StatusQueued,
		"Icons/logo.svg":      StatusQueued,
		"Scripts/Player.cs":   StatusUnsupported,
		"Textures/ground.tga": StatusUnsupported,
	}
	var files []models.AssetFile
	if err := db.Find(&files).Error; err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.PreviewStatus != want[f.RelativePath] {
			t.Errorf("%s: status %q, want %q", f.RelativePath, f.PreviewStatus, want[f.RelativePath])
		}
	}
}
