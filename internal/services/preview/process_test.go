package preview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"sync"
	"testing"

	"golang.org/x/image/bmp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// capturePublisher records every rollup the processor emits so tests can assert
// the live-update sequence.
type capturePublisher struct {
	mu     sync.Mutex
	events []Event
}

func (c *capturePublisher) Publish(_ uint, ev Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
}

func (c *capturePublisher) last() Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) == 0 {
		return Event{}
	}
	return c.events[len(c.events)-1]
}

func newProcessDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Asset{}, &models.AssetVersion{}, &models.AssetFile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestProcessVersion_DrivesPerFileStatus(t *testing.T) {
	ctx := context.Background()
	db := newProcessDB(t)
	store := newTestStorage(t)

	// A renderable BMP, an unsupported .dll, and a .psd with garbage bytes that
	// classifies as supported but fails to decode.
	var bmpBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 80, A: 255})
		}
	}
	if err := bmp.Encode(&bmpBuf, img); err != nil {
		t.Fatalf("encode bmp: %v", err)
	}
	bmpKey, _, err := store.PutBlob(ctx, bytes.NewReader(bmpBuf.Bytes()), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put bmp: %v", err)
	}
	psdKey, _, err := store.PutBlob(ctx, bytes.NewReader([]byte("not a real psd")), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put psd: %v", err)
	}
	dllKey, _, err := store.PutBlob(ctx, bytes.NewReader([]byte("MZ binary")), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put dll: %v", err)
	}

	asset := models.Asset{LibraryID: 7, Name: "pack"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	version := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	files := []models.AssetFile{
		{AssetVersionID: version.ID, RelativePath: "tex.bmp", ObjectKey: bmpKey, ContentHash: bmpKey, SizeBytes: 1},
		{AssetVersionID: version.ID, RelativePath: "art.psd", ObjectKey: psdKey, ContentHash: psdKey, SizeBytes: 1},
		{AssetVersionID: version.ID, RelativePath: "lib.dll", ObjectKey: dllKey, ContentHash: dllKey, SizeBytes: 1},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatal(err)
	}

	// Enqueue: supported files (bmp, psd) become queued; the dll stays unsupported.
	queued, err := MarkVersionQueued(ctx, db, version.ID, 0)
	if err != nil {
		t.Fatalf("mark queued: %v", err)
	}
	if queued != 2 {
		t.Fatalf("queued count: got %d, want 2", queued)
	}

	// The version is in-flight before processing, gone after.
	if inflight, _ := InFlight(ctx, db, 7); len(inflight) != 1 {
		t.Fatalf("expected 1 in-flight version before processing, got %d", len(inflight))
	}

	pub := &capturePublisher{}
	deps := Deps{DB: db, Storage: store, Broadcaster: pub}
	if err := ProcessVersion(ctx, deps, 7, version.ID, 0); err != nil {
		t.Fatalf("process: %v", err)
	}

	statusByPath := map[string]string{}
	keyByPath := map[string]string{}
	var got []models.AssetFile
	if err := db.Where("asset_version_id = ?", version.ID).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		statusByPath[f.RelativePath] = f.PreviewStatus
		keyByPath[f.RelativePath] = f.PreviewObjectKey
	}
	if statusByPath["tex.bmp"] != StatusDone {
		t.Errorf("bmp status: got %q, want done", statusByPath["tex.bmp"])
	}
	if keyByPath["tex.bmp"] == "" {
		t.Error("bmp should have a preview_object_key after a successful render")
	}
	if statusByPath["art.psd"] != StatusFailed {
		t.Errorf("garbage psd status: got %q, want failed", statusByPath["art.psd"])
	}
	if statusByPath["lib.dll"] != StatusUnsupported {
		t.Errorf("dll status: got %q, want unsupported", statusByPath["lib.dll"])
	}

	// Nothing left in flight, and the final rollup is terminal with the counts.
	if inflight, _ := InFlight(ctx, db, 7); len(inflight) != 0 {
		t.Fatalf("expected 0 in-flight versions after processing, got %d", len(inflight))
	}
	final := pub.last()
	if final.Status != StatusDone || final.Done != 1 || final.Failed != 1 || final.Unsupported != 1 || final.Total != 3 {
		t.Errorf("final rollup: %+v", final)
	}
}

func TestMarkVersionQueued_SingleFile(t *testing.T) {
	ctx := context.Background()
	db := newProcessDB(t)

	asset := models.Asset{LibraryID: 1, Name: "a"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	version := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	a := models.AssetFile{AssetVersionID: version.ID, RelativePath: "a.bmp", ObjectKey: "k1", ContentHash: "k1"}
	b := models.AssetFile{AssetVersionID: version.ID, RelativePath: "b.bmp", ObjectKey: "k2", ContentHash: "k2"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}

	// Targeting one file id must queue only that file.
	queued, err := MarkVersionQueued(ctx, db, version.ID, a.ID)
	if err != nil {
		t.Fatalf("mark queued: %v", err)
	}
	if queued != 1 {
		t.Fatalf("queued: got %d, want 1", queued)
	}
	var reloadB models.AssetFile
	if err := db.First(&reloadB, b.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloadB.PreviewStatus != StatusUnsupported {
		t.Errorf("untargeted file should stay unsupported, got %q", reloadB.PreviewStatus)
	}
}
