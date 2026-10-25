package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/ingest"
	"github.com/thomas-elliott/damson/internal/services/ingest/ingesttest"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/preview/previewtest"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// testChunkBytes is deliberately tiny so a handful of bytes exercises the
// multi-chunk path the 32 MiB default would need gigabytes to reach.
const testChunkBytes = 8

func newChunkedHandler(t *testing.T) (*Handler, *gorm.DB, storage.Service) {
	t.Helper()
	db := newTestDB(t)
	store := newTestStorage(t)
	bc := ingest.NewBroadcaster()
	pbc := preview.NewBroadcaster()
	previewPool := previewtest.NewSync(preview.Deps{DB: db, Storage: store, Broadcaster: pbc})
	pool := ingesttest.NewSync(ingest.Deps{DB: db, Storage: store, Broadcaster: bc, Previews: previewPool})
	h := New(Deps{
		DB:                 db,
		AuthMode:           config.AuthModeOIDC,
		Storage:            store,
		ImportPool:         pool,
		Broadcaster:        bc,
		PreviewPool:        previewPool,
		PreviewBroadcaster: pbc,
		UploadChunkBytes:   testChunkBytes,
		OIDCAuthority:      "https://auth.example.com",
		OIDCClientID:       "damson",
	})
	return h, db, store
}

func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func putChunk(r http.Handler, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// sendChunks pushes payload through the chunk endpoint testChunkBytes at a
// time and returns the last response.
func sendChunks(t *testing.T, r http.Handler, uploadID uint, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	var last *httptest.ResponseRecorder
	for i, off := 0, 0; off < len(payload); i, off = i+1, off+testChunkBytes {
		end := min(off+testChunkBytes, len(payload))
		last = putChunk(r, chunkPath(uploadID, i), payload[off:end])
		if last.Code != http.StatusOK {
			t.Fatalf("chunk %d: status %d: %s", i, last.Code, last.Body.String())
		}
	}
	return last
}

func chunkPath(uploadID uint, n int) string {
	return "/api/imports/uploads/" + itoa(uploadID) + "/chunks/" + itoa(uint(n))
}

func openSession(t *testing.T, r http.Handler, body map[string]any) (*httptest.ResponseRecorder, uploadSessionResponse) {
	t.Helper()
	w := performRequest(r, http.MethodPost, "/api/imports/uploads", body)
	if w.Code != http.StatusCreated && w.Code != http.StatusAccepted {
		return w, uploadSessionResponse{}
	}
	return w, jsonResponse[uploadSessionResponse](t, w)
}

// TestChunkedUpload_RoundTrip is the whole feature end to end: a body larger
// than one chunk arrives in pieces, is verified against the declared hash, and
// lands as an ordinary import with the bytes intact.
func TestChunkedUpload_RoundTrip(t *testing.T) {
	h, db, store := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	payload := []byte("chunked upload payload spanning several chunks")
	digest := hexSum(payload)

	w, session := openSession(t, r, map[string]any{
		"filename":   "Big Pack.unitypackage",
		"size_bytes": len(payload),
		"sha256":     digest,
	})
	requireStatus(t, w, http.StatusCreated)
	if session.ChunkSize != testChunkBytes {
		t.Fatalf("chunk_size = %d, want the server-advertised %d", session.ChunkSize, testChunkBytes)
	}
	if session.Deduped {
		t.Fatal("a body storage has never seen must not report as deduped")
	}
	wantChunks := (len(payload) + testChunkBytes - 1) / testChunkBytes
	if session.TotalChunks != wantChunks {
		t.Errorf("total_chunks = %d, want %d", session.TotalChunks, wantChunks)
	}

	last := sendChunks(t, r, session.UploadID, payload)
	state := jsonResponse[uploadSessionResponse](t, last)
	if state.ReceivedBytes != int64(len(payload)) {
		t.Errorf("received_bytes = %d, want %d", state.ReceivedBytes, len(payload))
	}
	if state.NextChunk != wantChunks {
		t.Errorf("next_chunk = %d, want %d", state.NextChunk, wantChunks)
	}

	w = performRequest(r, http.MethodPost, "/api/imports/uploads/"+itoa(session.UploadID)+"/complete", nil)
	requireStatus(t, w, http.StatusAccepted)
	done := jsonResponse[uploadSessionResponse](t, w)
	if done.AssetID == 0 || done.ImportID == 0 {
		t.Fatalf("complete returned asset=%d import=%d, want both set", done.AssetID, done.ImportID)
	}

	rc, err := store.Get(t.Context(), storage.Originals, digest)
	if err != nil {
		t.Fatalf("stored object: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, payload) {
		t.Errorf("stored object = %q, want the uploaded bytes", got)
	}

	var version models.AssetVersion
	if err := db.Where("asset_id = ?", done.AssetID).First(&version).Error; err != nil {
		t.Fatal(err)
	}
	if version.OriginalPackageObjectKey != digest {
		t.Errorf("version object key = %q, want the content hash %q", version.OriginalPackageObjectKey, digest)
	}

	var session2 models.UploadSession
	if err := db.First(&session2, session.UploadID).Error; err != nil {
		t.Fatal(err)
	}
	if session2.Status != models.UploadCompleted {
		t.Errorf("session status = %q, want completed", session2.Status)
	}
}

// TestChunkedUpload_DedupesWithoutTransfer is the payoff of a client-declared
// hash: re-uploading content the library already holds must not move a byte.
func TestChunkedUpload_DedupesWithoutTransfer(t *testing.T) {
	h, db, store := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	payload := []byte("already in the bucket")
	digest := hexSum(payload)
	if _, _, err := store.Put(t.Context(), storage.Originals, bytes.NewReader(payload), storage.ObjectMeta{}); err != nil {
		t.Fatal(err)
	}

	w, resp := openSession(t, r, map[string]any{
		"filename":   "Dupe.unitypackage",
		"size_bytes": len(payload),
		"sha256":     digest,
	})
	requireStatus(t, w, http.StatusAccepted)
	if !resp.Deduped {
		t.Error("deduped = false, want true — the object was already stored")
	}
	if resp.ImportID == 0 || resp.AssetID == 0 {
		t.Errorf("dedupe must run the import immediately; got asset=%d import=%d", resp.AssetID, resp.ImportID)
	}
	if resp.UploadID != 0 {
		t.Errorf("upload_id = %d, want none — there is nothing to resume", resp.UploadID)
	}

	var sessions int64
	if err := db.Model(&models.UploadSession{}).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Errorf("want no session row for a deduped upload, got %d", sessions)
	}

	var version models.AssetVersion
	if err := db.Where("asset_id = ?", resp.AssetID).First(&version).Error; err != nil {
		t.Fatal(err)
	}
	if version.OriginalPackageObjectKey != digest {
		t.Errorf("version object key = %q, want %q", version.OriginalPackageObjectKey, digest)
	}
}

// TestChunkedUpload_RejectsHashMismatch is the reason the server re-hashes:
// a client that declares one digest and sends different bytes must not get
// its content published under the declared key.
func TestChunkedUpload_RejectsHashMismatch(t *testing.T) {
	h, db, store := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	claimed := hexSum([]byte("what the client says it is sending"))
	payload := []byte("what it actually sent")

	w, session := openSession(t, r, map[string]any{
		"filename":   "Liar.unitypackage",
		"size_bytes": len(payload),
		"sha256":     claimed,
	})
	requireStatus(t, w, http.StatusCreated)
	sendChunks(t, r, session.UploadID, payload)

	w = performRequest(r, http.MethodPost, "/api/imports/uploads/"+itoa(session.UploadID)+"/complete", nil)
	requireStatus(t, w, http.StatusBadRequest)

	if _, ok, err := store.Stat(t.Context(), storage.Originals, claimed); err != nil || ok {
		t.Errorf("object present = %v (err %v), want nothing published at the declared key", ok, err)
	}
	var assets int64
	if err := db.Model(&models.Asset{}).Count(&assets).Error; err != nil {
		t.Fatal(err)
	}
	if assets != 0 {
		t.Errorf("want no asset created for a rejected upload, got %d", assets)
	}
	var s models.UploadSession
	if err := db.First(&s, session.UploadID).Error; err != nil {
		t.Fatal(err)
	}
	if s.Status != models.UploadAborted {
		t.Errorf("session status = %q, want aborted", s.Status)
	}
}

// TestChunkedUpload_ChunkOrdering covers the three answers the chunk endpoint
// owes a retrying client: too far ahead, already applied, and just right.
func TestChunkedUpload_ChunkOrdering(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	payload := []byte("aaaaaaaabbbbbbbbcccc")
	w, session := openSession(t, r, map[string]any{
		"filename":   "Order.unitypackage",
		"size_bytes": len(payload),
		"sha256":     hexSum(payload),
	})
	requireStatus(t, w, http.StatusCreated)

	t.Run("out of order is refused", func(t *testing.T) {
		w := putChunk(r, chunkPath(session.UploadID, 2), payload[16:20])
		requireStatus(t, w, http.StatusConflict)
		if !strings.Contains(w.Body.String(), "send chunk 0 next") {
			t.Errorf("body = %s, want it to name the expected chunk", w.Body.String())
		}
	})

	t.Run("in order is accepted", func(t *testing.T) {
		w := putChunk(r, chunkPath(session.UploadID, 0), payload[0:8])
		requireStatus(t, w, http.StatusOK)
		got := jsonResponse[uploadSessionResponse](t, w)
		if got.NextChunk != 1 || got.ReceivedBytes != 8 {
			t.Errorf("next_chunk=%d received=%d, want 1 and 8", got.NextChunk, got.ReceivedBytes)
		}
	})

	t.Run("replaying an accepted chunk does not double count", func(t *testing.T) {
		w := putChunk(r, chunkPath(session.UploadID, 0), payload[0:8])
		requireStatus(t, w, http.StatusOK)
		got := jsonResponse[uploadSessionResponse](t, w)
		if got.ReceivedBytes != 8 {
			t.Errorf("received_bytes = %d, want 8 — a replayed chunk must not advance the count", got.ReceivedBytes)
		}
	})

	t.Run("a chunk of the wrong size is refused", func(t *testing.T) {
		w := putChunk(r, chunkPath(session.UploadID, 1), payload[8:12])
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("resume reports where to continue", func(t *testing.T) {
		w := performGET(r, "/api/imports/uploads/"+itoa(session.UploadID))
		requireStatus(t, w, http.StatusOK)
		got := jsonResponse[uploadSessionResponse](t, w)
		if got.NextChunk != 1 || got.ReceivedBytes != 8 {
			t.Errorf("resume point = chunk %d / %d bytes, want 1 / 8", got.NextChunk, got.ReceivedBytes)
		}
	})

	t.Run("completing early is refused", func(t *testing.T) {
		w := performRequest(r, http.MethodPost, "/api/imports/uploads/"+itoa(session.UploadID)+"/complete", nil)
		requireStatus(t, w, http.StatusConflict)
	})
}

// TestChunkedUpload_AbortReleasesTheSession pins that a cancelled upload stops
// accepting chunks rather than silently resuming.
func TestChunkedUpload_AbortReleasesTheSession(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	payload := []byte("abandoned midway")
	_, session := openSession(t, r, map[string]any{
		"filename":   "Gone.unitypackage",
		"size_bytes": len(payload),
		"sha256":     hexSum(payload),
	})
	putChunk(r, chunkPath(session.UploadID, 0), payload[0:8])

	w := performRequest(r, http.MethodDelete, "/api/imports/uploads/"+itoa(session.UploadID), nil)
	requireStatus(t, w, http.StatusNoContent)

	w = putChunk(r, chunkPath(session.UploadID, 1), payload[8:16])
	requireStatus(t, w, http.StatusConflict)

	var s models.UploadSession
	if err := db.First(&s, session.UploadID).Error; err != nil {
		t.Fatal(err)
	}
	if s.Status != models.UploadAborted {
		t.Errorf("status = %q, want aborted", s.Status)
	}
}

// TestChunkedUpload_ExpiredSessionIsGone distinguishes "wrong request" from
// "too late", so a client that slept through the TTL restarts instead of
// retrying a chunk forever.
func TestChunkedUpload_ExpiredSessionIsGone(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	payload := []byte("too slow!")
	_, session := openSession(t, r, map[string]any{
		"filename":   "Slow.unitypackage",
		"size_bytes": len(payload),
		"sha256":     hexSum(payload),
	})
	if err := db.Model(&models.UploadSession{}).Where("id = ?", session.UploadID).
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}

	w := putChunk(r, chunkPath(session.UploadID, 0), payload[0:8])
	requireStatus(t, w, http.StatusGone)
}

// TestChunkedUpload_ValidatesTheOpenRequest keeps a malformed session from
// reserving a multipart upload nobody can ever complete.
func TestChunkedUpload_ValidatesTheOpenRequest(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"no hash", map[string]any{"filename": "a.zip", "size_bytes": 10}, http.StatusBadRequest},
		{"short hash", map[string]any{"filename": "a.zip", "size_bytes": 10, "sha256": "abc"}, http.StatusBadRequest},
		{"non-hex hash", map[string]any{"filename": "a.zip", "size_bytes": 10, "sha256": strings.Repeat("z", 64)}, http.StatusBadRequest},
		{"zero size", map[string]any{"filename": "a.zip", "size_bytes": 0, "sha256": hexSum(nil)}, http.StatusBadRequest},
		{"over the cap", map[string]any{"filename": "a.zip", "size_bytes": 1 << 50, "sha256": hexSum(nil)}, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := performRequest(r, http.MethodPost, "/api/imports/uploads", tc.body)
			requireStatus(t, w, tc.want)
		})
	}

	var sessions int64
	if err := db.Model(&models.UploadSession{}).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Errorf("a rejected open must not leave a session row; got %d", sessions)
	}
}

// TestChunkedUpload_ConflictsBeforeTheBytesMove is why the duplicate check runs
// at open as well as at complete: discovering it after 20 GB is useless.
func TestChunkedUpload_ConflictsBeforeTheBytesMove(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "Existing Pack"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	version := models.AssetVersion{AssetID: asset.ID, StoreVersion: "2.0", ImportDate: time.Now().UTC()}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}

	w := performRequest(r, http.MethodPost, "/api/imports/uploads", map[string]any{
		"filename":      "Existing Pack.unitypackage",
		"size_bytes":    64,
		"sha256":        hexSum([]byte("something new")),
		"asset_id":      asset.ID,
		"store_version": "2.0",
	})
	requireStatus(t, w, http.StatusConflict)

	var sessions int64
	if err := db.Model(&models.UploadSession{}).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Errorf("a conflicting open must not reserve a session; got %d", sessions)
	}
}

// TestChunkedUpload_TargetsAnExistingAsset proves the session carries the same
// import hints the one-shot form fields do.
func TestChunkedUpload_TargetsAnExistingAsset(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "Existing Pack"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}

	payload := []byte("a new version of an existing asset")
	_, session := openSession(t, r, map[string]any{
		"filename":      "Existing Pack.unitypackage",
		"size_bytes":    len(payload),
		"sha256":        hexSum(payload),
		"asset_id":      asset.ID,
		"store_version": "v4.2",
	})
	sendChunks(t, r, session.UploadID, payload)
	w := performRequest(r, http.MethodPost, "/api/imports/uploads/"+itoa(session.UploadID)+"/complete", nil)
	requireStatus(t, w, http.StatusAccepted)

	done := jsonResponse[uploadSessionResponse](t, w)
	if done.AssetID != asset.ID {
		t.Errorf("asset_id = %d, want the targeted asset %d", done.AssetID, asset.ID)
	}
	var versions []models.AssetVersion
	if err := db.Where("asset_id = ?", asset.ID).Find(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].StoreVersion != "4.2" {
		t.Fatalf("versions = %+v, want one at store version 4.2", versions)
	}
	var assets int64
	if err := db.Model(&models.Asset{}).Count(&assets).Error; err != nil {
		t.Fatal(err)
	}
	if assets != 1 {
		t.Errorf("want no new asset, got %d in the library", assets)
	}
}

// TestChunkedUpload_SessionRoutesBeatTheImportIDRoute guards a routing trap:
// /imports/uploads/:id sits alongside /imports/:id, so a wrong precedence
// would silently send session reads to the import-status handler.
func TestChunkedUpload_SessionRoutesBeatTheImportIDRoute(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	payload := []byte("routing")
	_, session := openSession(t, r, map[string]any{
		"filename":   "Route.unitypackage",
		"size_bytes": len(payload),
		"sha256":     hexSum(payload),
	})

	w := performGET(r, "/api/imports/uploads/"+itoa(session.UploadID))
	requireStatus(t, w, http.StatusOK)
	got := jsonResponse[uploadSessionResponse](t, w)
	if got.UploadID != session.UploadID {
		t.Errorf("upload_id = %d, want %d — the request hit the wrong handler",
			got.UploadID, session.UploadID)
	}
	if got.ChunkSize == 0 {
		t.Error("no chunk_size in the response — this is an import status body, not a session")
	}
}

// TestChunkedUpload_OtherLibrariesCannotSeeTheSession keeps the session table
// inside the tenancy boundary every other library-scoped entity respects.
func TestChunkedUpload_OtherLibrariesCannotSeeTheSession(t *testing.T) {
	h, db, _ := newChunkedHandler(t)
	owner := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, owner)
	ownerRouter := setupRouter(h, owner)

	payload := []byte("private!")
	_, session := openSession(t, ownerRouter, map[string]any{
		"filename":   "Private.unitypackage",
		"size_bytes": len(payload),
		"sha256":     hexSum(payload),
	})

	outsider := seedUser(t, db, "someone@else.example")
	_ = seedLibraryAndSwitch(t, h, outsider)
	outsiderRouter := setupRouter(h, outsider)

	w := performGET(outsiderRouter, "/api/imports/uploads/"+itoa(session.UploadID))
	requireStatus(t, w, http.StatusNotFound)

	w = putChunk(outsiderRouter, chunkPath(session.UploadID, 0), payload)
	requireStatus(t, w, http.StatusNotFound)
}

// TestResumeUploadHash_SurvivesTheRoundTrip is the mechanism that makes
// cross-request verification possible; without a marshallable digest the whole
// chunked design collapses to re-reading the object at complete.
func TestResumeUploadHash_SurvivesTheRoundTrip(t *testing.T) {
	first := []byte("first half ")
	second := []byte("second half")

	h1, err := resumeUploadHash(nil)
	if err != nil {
		t.Fatal(err)
	}
	h1.Write(first)
	state, err := marshalUploadHash(h1)
	if err != nil {
		t.Fatal(err)
	}
	if len(state) == 0 {
		t.Fatal("empty hash state — nothing would carry to the next request")
	}

	h2, err := resumeUploadHash(state)
	if err != nil {
		t.Fatal(err)
	}
	h2.Write(second)

	if got, want := hex.EncodeToString(h2.Sum(nil)), hexSum(append(append([]byte{}, first...), second...)); got != want {
		t.Errorf("resumed digest = %s, want %s", got, want)
	}

	if _, err := resumeUploadHash([]byte("not a digest")); err == nil {
		t.Error("corrupt hash state was accepted")
	}
}

func TestTotalChunks(t *testing.T) {
	cases := []struct {
		size, chunk int64
		want        int
	}{
		{0, 32, 0},
		{1, 32, 1},
		{32, 32, 1},
		{33, 32, 2},
		{64, 32, 2},
		{65, 32, 3},
		{10, 0, 0},
	}
	for _, tc := range cases {
		if got := totalChunks(tc.size, tc.chunk); got != tc.want {
			t.Errorf("totalChunks(%d, %d) = %d, want %d", tc.size, tc.chunk, got, tc.want)
		}
	}
}
