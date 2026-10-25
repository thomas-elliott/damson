package preview

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"sync"
	"testing"

	"golang.org/x/image/bmp"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/testdb"
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
	return testdb.NewPostgres(t)
}

// ensureLibrary seeds a library row with an explicit id so the FK from assets
// is satisfied. Safe with a fixed id because these tests never auto-insert
// libraries, so the bigserial sequence is never contended.
func ensureLibrary(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	var n int64
	if err := db.Model(&models.Library{}).Where("id = ?", id).Count(&n).Error; err != nil {
		t.Fatalf("count library %d: %v", id, err)
	}
	if n > 0 {
		return
	}
	if err := db.Create(&models.Library{ID: id, Name: "lib"}).Error; err != nil {
		t.Fatalf("seed library %d: %v", id, err)
	}
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
	bmpKey, _, err := store.Put(ctx, storage.Blobs, bytes.NewReader(bmpBuf.Bytes()), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put bmp: %v", err)
	}
	psdKey, _, err := store.Put(ctx, storage.Blobs, bytes.NewReader([]byte("not a real psd")), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put psd: %v", err)
	}
	dllKey, _, err := store.Put(ctx, storage.Blobs, bytes.NewReader([]byte("MZ binary")), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put dll: %v", err)
	}

	ensureLibrary(t, db, 7)
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
	queued, err := Queue(ctx, db, nil, Job{LibraryID: 7, VersionID: version.ID})
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

func TestQueue_SingleFile(t *testing.T) {
	ctx := context.Background()
	db := newProcessDB(t)

	ensureLibrary(t, db, 1)
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
	queued, err := Queue(ctx, db, nil, Job{LibraryID: 1, VersionID: version.ID, FileID: a.ID})
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

type jobRecorder struct{ jobs []Job }

func (r *jobRecorder) Enqueue(_ *gorm.DB, j Job) error {
	r.jobs = append(r.jobs, j)
	return nil
}

// Every hand-off obeys the library's previews toggle: with previews off,
// Queue marks nothing and enqueues nothing.
func TestQueue_ObeysTheLibrarysPreviewsToggle(t *testing.T) {
	ctx := context.Background()
	db := newProcessDB(t)
	ensureLibrary(t, db, 1)
	asset := models.Asset{LibraryID: 1, Name: "a"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	version := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	f := models.AssetFile{AssetVersionID: version.ID, RelativePath: "a.bmp", ObjectKey: "k1", ContentHash: "k1"}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	job := Job{LibraryID: 1, VersionID: version.ID}

	if err := db.Create(&models.LibrarySettings{LibraryID: 1, GridDensityDefault: "medium"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.LibrarySettings{}).Where("library_id = 1").Update("previews_enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	rec := &jobRecorder{}
	if _, err := Queue(ctx, db, rec, job); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Queue with previews off: err %v, want ErrDisabled", err)
	}
	var got models.AssetFile
	db.First(&got, f.ID)
	if got.PreviewStatus != StatusUnsupported || len(rec.jobs) != 0 {
		t.Fatalf("status %q, jobs %v: previews off must leave the file alone", got.PreviewStatus, rec.jobs)
	}

	db.Model(&models.LibrarySettings{}).Where("library_id = 1").Update("previews_enabled", true)
	if n, err := Queue(ctx, db, rec, job); err != nil || n != 1 {
		t.Fatalf("Queue with previews on: %d, %v; want 1 file queued", n, err)
	}
	db.First(&got, f.ID)
	if got.PreviewStatus != StatusQueued || len(rec.jobs) != 1 || rec.jobs[0] != job {
		t.Fatalf("status %q, jobs %v: want the file queued and the job enqueued", got.PreviewStatus, rec.jobs)
	}
}

// Once the queue gives up on a job, the files it left in flight are failed,
// and files it finished are left alone.
func TestFailJob_FailsOnlyTheFilesLeftInFlight(t *testing.T) {
	ctx := context.Background()
	db := newProcessDB(t)
	ensureLibrary(t, db, 1)
	asset := models.Asset{LibraryID: 1, Name: "a"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	version := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	files := []models.AssetFile{
		{AssetVersionID: version.ID, RelativePath: "a.bmp", ObjectKey: "k1", ContentHash: "k1", PreviewStatus: StatusQueued},
		{AssetVersionID: version.ID, RelativePath: "b.bmp", ObjectKey: "k2", ContentHash: "k2", PreviewStatus: StatusProcessing},
		{AssetVersionID: version.ID, RelativePath: "c.bmp", ObjectKey: "k3", ContentHash: "k3", PreviewStatus: StatusDone},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatal(err)
	}

	if err := FailJob(ctx, db, Job{LibraryID: 1, VersionID: version.ID}); err != nil {
		t.Fatal(err)
	}
	want := []string{StatusFailed, StatusFailed, StatusDone}
	for i, f := range files {
		var got models.AssetFile
		db.First(&got, f.ID)
		if got.PreviewStatus != want[i] {
			t.Errorf("%s: status %q, want %q", f.RelativePath, got.PreviewStatus, want[i])
		}
	}
}
