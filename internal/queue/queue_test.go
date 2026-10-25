package queue

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
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

	store, err := storage.NewLocalFS(t.TempDir())
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
	imp := models.AssetImport{AssetID: asset.ID, AssetVersionID: &v.ID, Stage: imports.StageUploaded}
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

// TestQueue_PreviewJobRunsToDone proves the full preview path: rivermigrate
// creates the schema, the client starts, Enqueue inserts a river_job,
// and the preview worker drives the file to done. A broken enqueue or worker
// wiring leaves the file queued and the wait times out.
func TestQueue_PreviewJobRunsToDone(t *testing.T) {
	q, gdb, store, _ := newQueueEnv(t)
	libID, versionID, fileID := seedQueuedFile(t, gdb, store, renderableBMP(t), "tex.bmp")

	q.Previews().Enqueue(preview.Job{LibraryID: libID, VersionID: versionID})

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

// concurrencyRunner records how many lookups run at once.
type concurrencyRunner struct {
	mu      sync.Mutex
	running int
	most    int
	ran     []uint
}

func (r *concurrencyRunner) Run(_ context.Context, id uint) error {
	r.mu.Lock()
	r.running++
	r.most = max(r.most, r.running)
	r.mu.Unlock()
	time.Sleep(50 * time.Millisecond)
	r.mu.Lock()
	r.running--
	r.ran = append(r.ran, id)
	r.mu.Unlock()
	return nil
}

func (r *concurrencyRunner) Fail(context.Context, uint, string) error { return nil }

// failingRunner fails every lookup and records which it was told were failed.
type failingRunner struct{ failed []uint }

func (r *failingRunner) Run(context.Context, uint) error {
	return errors.New("database unreachable")
}

func (r *failingRunner) Fail(_ context.Context, id uint, _ string) error {
	r.failed = append(r.failed, id)
	return nil
}

func TestStoreLookupWorker_LastAttemptMarksTheLookupFailed(t *testing.T) {
	runner := &failingRunner{}
	w := &storeLookupWorker{run: runner}
	job := func(attempt int) *river.Job[StoreLookupArgs] {
		return &river.Job[StoreLookupArgs]{
			JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: 5},
			Args:   StoreLookupArgs{LookupID: 7},
		}
	}

	if err := w.Work(context.Background(), job(4)); err == nil || len(runner.failed) != 0 {
		t.Fatalf("attempt 4: err %v, failed %v; want the error returned for a retry and nothing marked", err, runner.failed)
	}
	if err := w.Work(context.Background(), job(5)); err == nil || len(runner.failed) != 1 || runner.failed[0] != 7 {
		t.Fatalf("attempt 5: err %v, failed %v; want lookup 7 marked failed", err, runner.failed)
	}
}

func (r *concurrencyRunner) snapshot() (most int, ran int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.most, len(r.ran)
}

// However many store lookups are queued, the store sees one at a time.
func TestStoreLookups_RunOneAtATime(t *testing.T) {
	dsn := testdb.RequireDSN(t)
	schema := testdb.NewSchemaName(t)
	gdb := testdb.OpenSchema(t, dsn, schema, true)
	runner := &concurrencyRunner{}
	q, err := New(context.Background(), Config{
		DSN: dsn, Schema: schema, ImportWorkers: 4, PreviewWorkers: 4,
		ImportDeps: imports.Deps{DB: gdb}, PreviewDeps: preview.Deps{DB: gdb}, StoreLookups: runner,
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

	for id := uint(1); id <= 5; id++ {
		q.StoreLookups().EnqueueStoreLookup(id)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		most, ran := runner.snapshot()
		if ran == 5 {
			if most != 1 {
				t.Fatalf("%d store lookups ran at once, want 1", most)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of 5 store lookups ran", ran)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A job that times out on its last attempt arrives with an expired context.
// The row must still be marked failed, or it stays in flight until the next
// start.
func TestImportWorker_TimeoutOnLastAttemptMarksTheImportFailed(t *testing.T) {
	gdb := testdb.NewPostgres(t)
	importID, libID := seedImportRow(t, gdb, true)
	store, err := storage.NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	events := imports.NewBroadcaster()
	ch, unsub := events.Subscribe(libID)
	defer unsub()
	w := &importWorker{deps: imports.Deps{DB: gdb, Storage: store, Broadcaster: events}}

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	<-ctx.Done()
	job := &river.Job[ImportArgs]{
		JobRow: &rivertype.JobRow{Attempt: 5, MaxAttempts: 5},
		Args:   ImportArgs{ImportID: importID, LibraryID: libID},
	}
	if err := w.Work(ctx, job); err == nil {
		t.Fatal("work: want the timeout returned")
	}

	var imp models.AssetImport
	if err := gdb.First(&imp, importID).Error; err != nil {
		t.Fatal(err)
	}
	if imp.Stage != imports.StageFailed {
		t.Fatalf("stage = %q, want %q", imp.Stage, imports.StageFailed)
	}
	// An open page learns the import stopped only from the stream.
	select {
	case ev := <-ch:
		if ev.Stage != imports.StageFailed {
			t.Errorf("event stage = %q, want %q", ev.Stage, imports.StageFailed)
		}
	default:
		t.Error("no failed event published")
	}
}
