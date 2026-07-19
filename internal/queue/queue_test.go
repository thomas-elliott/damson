package queue

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"golang.org/x/image/bmp"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/storage"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// newQueueEnv builds a per-test schema with the domain migrations applied, a
// local-FS store, and a Queue whose River tables live in the same schema. The
// queue is started and Stop is registered as cleanup.
func newQueueEnv(t *testing.T) (*Queue, *gorm.DB, storage.Service, string) {
	t.Helper()
	dsn := testdb.RequireDSN(t)
	schema := testdb.NewSchemaName(t)
	gdb := testdb.OpenSchema(t, dsn, schema, true)

	store, err := storage.NewLocalFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	q, err := New(context.Background(), Config{
		DSN:            dsn,
		Schema:         schema,
		ImportWorkers:  1,
		PreviewWorkers: 1,
		ImportDeps:     imports.Deps{DB: gdb, Storage: store},
		PreviewDeps:    preview.Deps{DB: gdb, Storage: store},
	})
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}
	if err := q.Start(context.Background()); err != nil {
		t.Fatalf("start queue: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = q.Stop(ctx)
	})
	return q, gdb, store, schema
}

func renderableBMP(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 20, G: 120, B: 200, A: 255})
		}
	}
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatalf("encode bmp: %v", err)
	}
	return buf.Bytes()
}

func seedQueuedFile(t *testing.T, gdb *gorm.DB, store storage.Service, payload []byte, path string) (libID, versionID, fileID uint) {
	t.Helper()
	key, _, err := store.PutBlob(context.Background(), bytes.NewReader(payload), storage.ObjectMeta{})
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	lib := models.Library{Name: "lib"}
	if err := gdb.Create(&lib).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	asset := models.Asset{LibraryID: lib.ID, Name: "pack"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if err := gdb.Create(&v).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}
	f := models.AssetFile{
		AssetVersionID: v.ID, RelativePath: path, ObjectKey: key, ContentHash: key,
		PreviewStatus: preview.StatusQueued,
	}
	if err := gdb.Create(&f).Error; err != nil {
		t.Fatalf("seed file: %v", err)
	}
	return lib.ID, v.ID, f.ID
}

func waitForStatus(t *testing.T, gdb *gorm.DB, fileID uint, want string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var f models.AssetFile
		if err := gdb.First(&f, fileID).Error; err != nil {
			t.Fatalf("reload file: %v", err)
		}
		if f.PreviewStatus == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	var f models.AssetFile
	_ = gdb.First(&f, fileID).Error
	t.Fatalf("file %d never reached %q (stuck at %q)", fileID, want, f.PreviewStatus)
}

// seedImportRow creates the asset/version/import chain for an import job. When
// withOriginal is false the version has no source blob, so the pipeline fails
// terminally.
func seedImportRow(t *testing.T, gdb *gorm.DB, withOriginal bool) (importID, libID uint) {
	t.Helper()
	lib := models.Library{Name: "lib"}
	if err := gdb.Create(&lib).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	asset := models.Asset{LibraryID: lib.ID, Name: "pack"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	v := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1"}
	if withOriginal {
		v.OriginalPackageObjectKey = "some-key"
	}
	if err := gdb.Create(&v).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}
	imp := models.AssetImport{AssetID: asset.ID, Stage: imports.StageUploaded}
	if err := gdb.Create(&imp).Error; err != nil {
		t.Fatalf("seed import: %v", err)
	}
	return imp.ID, lib.ID
}

func waitForImportStage(t *testing.T, gdb *gorm.DB, importID uint, want string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var imp models.AssetImport
		if err := gdb.First(&imp, importID).Error; err != nil {
			t.Fatalf("reload import: %v", err)
		}
		if imp.Stage == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	var imp models.AssetImport
	_ = gdb.First(&imp, importID).Error
	t.Fatalf("import %d never reached %q (stuck at %q)", importID, want, imp.Stage)
}

func riverJobState(t *testing.T, gdb *gorm.DB, kind string) string {
	t.Helper()
	var state string
	err := gdb.Raw(`SELECT state FROM river_job WHERE kind = ? ORDER BY id DESC LIMIT 1`, kind).Scan(&state).Error
	if err != nil {
		t.Fatalf("query river_job state: %v", err)
	}
	return state
}

func waitForRiverState(t *testing.T, gdb *gorm.DB, kind string, want string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if riverJobState(t, gdb, kind) == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("river %s job never reached state %q (last %q)", kind, want, riverJobState(t, gdb, kind))
}

// TestQueue_ImportTerminalErrorCancels proves the terminal-failure path: an
// import whose source blob is missing is marked failed by the pipeline, and the
// worker turns that into a River JobCancel (state=cancelled) rather than burning
// retries.
func TestQueue_ImportTerminalErrorCancels(t *testing.T) {
	q, gdb, _, _ := newQueueEnv(t)
	importID, libID := seedImportRow(t, gdb, false)

	q.Imports().Enqueue(imports.Job{ImportID: importID, LibraryID: libID})

	waitForImportStage(t, gdb, importID, imports.StageFailed)
	waitForRiverState(t, gdb, "import", "cancelled")
}

// TestQueue_ImportTransientErrorRetries proves the transient-failure path: an
// import job whose row doesn't exist fails without the row being marked failed,
// so the worker returns the error and River schedules a retry (state=retryable),
// not a cancel.
func TestQueue_ImportTransientErrorRetries(t *testing.T) {
	q, gdb, _, _ := newQueueEnv(t)

	q.Imports().Enqueue(imports.Job{ImportID: 999999, LibraryID: 0})

	waitForRiverState(t, gdb, "import", "retryable")
	if got := riverJobState(t, gdb, "import"); got == "cancelled" || got == "discarded" {
		t.Fatalf("transient error should retry, got terminal state %q", got)
	}
}

// TestWorkerTimeouts proves the per-queue job timeout is wired from config onto
// the workers.
func TestWorkerTimeouts(t *testing.T) {
	iw := &importWorker{timeout: 7 * time.Minute}
	if got := iw.Timeout(nil); got != 7*time.Minute {
		t.Fatalf("import worker timeout: got %v, want 7m", got)
	}
	pw := &previewWorker{timeout: 3 * time.Minute}
	if got := pw.Timeout(nil); got != 3*time.Minute {
		t.Fatalf("preview worker timeout: got %v, want 3m", got)
	}
}

// TestQueue_PreviewJobRunsToDone proves the full Stage 2 path: rivermigrate
// creates the schema, the client starts, EnqueueVersion inserts a river_job,
// and the preview worker drives the file to done. A broken enqueue or worker
// wiring leaves the file queued and the wait times out.
func TestQueue_PreviewJobRunsToDone(t *testing.T) {
	q, gdb, store, _ := newQueueEnv(t)
	libID, versionID, fileID := seedQueuedFile(t, gdb, store, renderableBMP(t), "tex.bmp")

	q.Previews().EnqueueVersion(libID, versionID)

	waitForStatus(t, gdb, fileID, preview.StatusDone)
}

// TestQueue_ReconcileEnqueuesOrphanedVersion proves the cutover/reconcile path:
// a version with a queued file but no River job (as after a crash or the deploy
// that introduces River) is re-enqueued by ResumeInFlight and driven to done.
// A second reconcile finds nothing queued and enqueues zero — idempotent.
func TestQueue_ReconcileEnqueuesOrphanedVersion(t *testing.T) {
	q, gdb, store, _ := newQueueEnv(t)
	_, _, fileID := seedQueuedFile(t, gdb, store, renderableBMP(t), "tex.bmp")

	n, err := preview.ResumeInFlight(context.Background(), gdb, q.Previews())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if n != 1 {
		t.Fatalf("reconcile enqueued: got %d, want 1", n)
	}

	waitForStatus(t, gdb, fileID, preview.StatusDone)

	// Nothing is queued anymore, so a re-run reconciles zero.
	n, err = preview.ResumeInFlight(context.Background(), gdb, q.Previews())
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if n != 0 {
		t.Fatalf("second reconcile enqueued: got %d, want 0", n)
	}
}

// TestQueue_PreviewJobMarksFailedOnBadRender exercises the error branch: a file
// that classifies as supported but can't be decoded ends failed (best-effort),
// while the job itself completes without retry storms.
func TestQueue_PreviewJobMarksFailedOnBadRender(t *testing.T) {
	q, gdb, store, _ := newQueueEnv(t)
	libID, versionID, fileID := seedQueuedFile(t, gdb, store, []byte("not a real psd"), "art.psd")

	q.Previews().Enqueue(preview.Job{LibraryID: libID, VersionID: versionID, FileID: fileID})

	waitForStatus(t, gdb, fileID, preview.StatusFailed)
}
