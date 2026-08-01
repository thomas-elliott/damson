package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// chunkServer is a fake damson upload-session endpoint. It records what
// arrived so a test can assert the transport, not just the outcome.
type chunkServer struct {
	mu        sync.Mutex
	chunkSize int64
	opened    []map[string]any
	chunks    []int
	assembled []byte
	completed bool
	// failChunkOnce makes that chunk index fail with 503 the first time.
	failChunkOnce int
	failed        map[int]bool
	// dedupe answers the open call with a deduped session.
	dedupe bool
	// resumeState, when non-nil, is what GET on the session returns.
	resumeState map[string]any
}

func newChunkServer(chunkSize int64) *chunkServer {
	return &chunkServer{chunkSize: chunkSize, failChunkOnce: -1, failed: map[int]bool{}}
}

func (s *chunkServer) session(over map[string]any) map[string]any {
	out := map[string]any{
		"upload_id":      7,
		"status":         "open",
		"chunk_size":     s.chunkSize,
		"received_bytes": int64(len(s.assembled)),
		"next_chunk":     len(s.chunks),
		"deduped":        false,
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (s *chunkServer) mount(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/imports/uploads", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.opened = append(s.opened, body)
		dedupe := s.dedupe
		s.mu.Unlock()
		if dedupe {
			writeJSON(w, http.StatusAccepted, s.session(map[string]any{
				"deduped": true, "status": "completed", "asset_id": 21, "import_id": 22,
			}))
			return
		}
		writeJSON(w, http.StatusCreated, s.session(nil))
	})

	mux.HandleFunc("GET /api/imports/uploads/{id}", func(w http.ResponseWriter, r *http.Request) {
		if s.resumeState == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "upload session not found"})
			return
		}
		writeJSON(w, http.StatusOK, s.session(s.resumeState))
	})

	mux.HandleFunc("PUT /api/imports/uploads/{id}/chunks/{n}", func(w http.ResponseWriter, r *http.Request) {
		var index int
		_, _ = fmt.Sscanf(r.PathValue("n"), "%d", &index)
		s.mu.Lock()
		defer s.mu.Unlock()
		if index == s.failChunkOnce && !s.failed[index] {
			s.failed[index] = true
			_, _ = io.Copy(io.Discard, r.Body)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backend unavailable"})
			return
		}
		body, _ := io.ReadAll(r.Body)
		if r.ContentLength != int64(len(body)) {
			t.Errorf("chunk %d: Content-Length %d but %d bytes arrived", index, r.ContentLength, len(body))
		}
		s.chunks = append(s.chunks, index)
		s.assembled = append(s.assembled, body...)
		writeJSON(w, http.StatusOK, s.session(map[string]any{
			"received_bytes": int64(len(s.assembled)),
			"next_chunk":     index + 1,
		}))
	})

	mux.HandleFunc("POST /api/imports/uploads/{id}/complete", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.completed = true
		s.mu.Unlock()
		writeJSON(w, http.StatusAccepted, s.session(map[string]any{
			"status": "completed", "asset_id": 3, "import_id": 4,
		}))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func chunkedClient(t *testing.T, base string) *Client {
	t.Helper()
	isolate(t)
	c, err := NewClient(base, testToken, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func payloadFile(t *testing.T, body string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Pack.unitypackage")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fh.Close() })
	return fh
}

// TestUploadChunked_SendsEveryByteInOrder is the transport contract: the file
// arrives whole, in bounded pieces, and is only completed once.
func TestUploadChunked_SendsEveryByteInOrder(t *testing.T) {
	body := "chunked payload that spans four pieces exactly ok"
	srv := newChunkServer(16)
	c := chunkedClient(t, srv.mount(t).URL)

	var progress []int64
	resp, err := c.UploadChunked(context.Background(), "Pack.unitypackage", payloadFile(t, body),
		int64(len(body)), importOptions{}, "key-1", func(sent int64) { progress = append(progress, sent) })
	if err != nil {
		t.Fatal(err)
	}

	if got := string(srv.assembled); got != body {
		t.Errorf("assembled = %q, want the whole payload", got)
	}
	wantChunks := (len(body) + 15) / 16
	if len(srv.chunks) != wantChunks {
		t.Errorf("chunks = %v, want %d of them", srv.chunks, wantChunks)
	}
	for i, n := range srv.chunks {
		if n != i {
			t.Errorf("chunk %d arrived as index %d — the loop is not sequential", i, n)
		}
	}
	if !srv.completed {
		t.Error("the upload was never completed")
	}
	if resp.AssetID != 3 || resp.ImportID != 4 {
		t.Errorf("resp = %+v, want the ids from the complete call", resp)
	}
	if progress[len(progress)-1] != int64(len(body)) {
		t.Errorf("final progress = %d, want %d", progress[len(progress)-1], len(body))
	}
}

// TestUploadChunked_DeclaresTheContentHash is what buys dedupe: the digest the
// server matches against has to be of the bytes actually on disk.
func TestUploadChunked_DeclaresTheContentHash(t *testing.T) {
	body := "hash me"
	srv := newChunkServer(4)
	c := chunkedClient(t, srv.mount(t).URL)

	if _, err := c.UploadChunked(context.Background(), "Pack.unitypackage", payloadFile(t, body),
		int64(len(body)), importOptions{assetID: "12", storeVersion: "3.1"}, "", nil); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256([]byte(body))
	opened := srv.opened[0]
	if got := opened["sha256"]; got != hex.EncodeToString(sum[:]) {
		t.Errorf("sha256 = %v, want %s", got, hex.EncodeToString(sum[:]))
	}
	if got := opened["asset_id"]; got != float64(12) {
		t.Errorf("asset_id = %v (%T), want the numeric 12", got, got)
	}
	if got := opened["store_version"]; got != "3.1" {
		t.Errorf("store_version = %v, want 3.1", got)
	}
	if got := opened["size_bytes"]; got != float64(len(body)) {
		t.Errorf("size_bytes = %v, want %d", got, len(body))
	}
}

// TestUploadChunked_SkipsTheTransferWhenDeduped proves the payoff: content the
// server already holds costs one round trip, not a re-upload.
func TestUploadChunked_SkipsTheTransferWhenDeduped(t *testing.T) {
	srv := newChunkServer(4)
	srv.dedupe = true
	c := chunkedClient(t, srv.mount(t).URL)

	resp, err := c.UploadChunked(context.Background(), "Pack.unitypackage",
		payloadFile(t, "already stored"), 14, importOptions{}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.AssetID != 21 || resp.ImportID != 22 {
		t.Errorf("resp = %+v, want the ids from the deduped open", resp)
	}
	if len(srv.chunks) != 0 {
		t.Errorf("chunks = %v, want none sent", srv.chunks)
	}
	if srv.completed {
		t.Error("a deduped upload has nothing to complete")
	}
}

// TestUploadChunked_RetriesATransientChunkFailure covers the reliability claim:
// a blip costs one chunk, not the whole transfer.
func TestUploadChunked_RetriesATransientChunkFailure(t *testing.T) {
	body := "aaaabbbbcccc"
	srv := newChunkServer(4)
	srv.failChunkOnce = 1
	c := chunkedClient(t, srv.mount(t).URL)

	if _, err := c.UploadChunked(context.Background(), "Pack.unitypackage", payloadFile(t, body),
		int64(len(body)), importOptions{}, "", nil); err != nil {
		t.Fatal(err)
	}
	if got := string(srv.assembled); got != body {
		t.Errorf("assembled = %q, want %q — the retry must resend the same bytes", got, body)
	}
}

// TestUploadChunked_GivesUpOnARejectedChunk: a 400 means the body is wrong, so
// resending it four more times only delays the error.
func TestUploadChunked_GivesUpOnARejectedChunk(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/imports/uploads", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{
			"upload_id": 7, "status": "open", "chunk_size": 4,
			"received_bytes": 0, "next_chunk": 0, "deduped": false,
		})
	})
	mux.HandleFunc("PUT /api/imports/uploads/{id}/chunks/{n}", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		_, _ = io.Copy(io.Discard, r.Body)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "chunk 0 must be 4 bytes"})
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	c := chunkedClient(t, s.URL)

	_, err := c.UploadChunked(context.Background(), "Pack.unitypackage",
		payloadFile(t, "abcdefgh"), 8, importOptions{}, "", nil)
	if err == nil {
		t.Fatal("a rejected chunk must fail the upload")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 — a 400 is not retryable", attempts)
	}
	if !strings.Contains(err.Error(), "4 bytes") {
		t.Errorf("err = %v, want the server's reason", err)
	}
}

// TestUploadChunked_ResumesFromRecordedState is what --resume is for: a rerun
// after an interrupted transfer picks up where it stopped.
func TestUploadChunked_ResumesFromRecordedState(t *testing.T) {
	body := "aaaabbbbcccc"
	srv := newChunkServer(4)
	srv.resumeState = map[string]any{"received_bytes": int64(8), "next_chunk": 2, "size_bytes": int64(len(body))}
	c := chunkedClient(t, srv.mount(t).URL)
	rememberUploadSession("resume-me", 7)

	if _, err := c.UploadChunked(context.Background(), "Pack.unitypackage", payloadFile(t, body),
		int64(len(body)), importOptions{}, "resume-me", nil); err != nil {
		t.Fatal(err)
	}
	if len(srv.opened) != 0 {
		t.Errorf("opened %d new sessions, want the recorded one resumed", len(srv.opened))
	}
	if len(srv.chunks) != 1 || srv.chunks[0] != 2 {
		t.Errorf("chunks = %v, want only the missing chunk 2", srv.chunks)
	}
	if got := string(srv.assembled); got != "cccc" {
		t.Errorf("assembled = %q, want only the tail", got)
	}
	if storedUploadSession("resume-me") != 0 {
		t.Error("a completed upload must not stay resumable")
	}
}

// TestUploadChunked_ForgetsAnUnusableSession keeps a stale record from wedging
// every future run of the same file.
func TestUploadChunked_ForgetsAnUnusableSession(t *testing.T) {
	body := "abcd"
	srv := newChunkServer(4)
	srv.resumeState = map[string]any{"status": "expired", "size_bytes": int64(len(body))}
	c := chunkedClient(t, srv.mount(t).URL)
	rememberUploadSession("stale", 7)

	if _, err := c.UploadChunked(context.Background(), "Pack.unitypackage", payloadFile(t, body),
		int64(len(body)), importOptions{}, "stale", nil); err != nil {
		t.Fatal(err)
	}
	if len(srv.opened) != 1 {
		t.Errorf("opened %d sessions, want a fresh one", len(srv.opened))
	}
	if storedUploadSession("stale") != 0 {
		t.Error("the stale record survived")
	}
}

func TestUploadResumeKey_DistinguishesRevisions(t *testing.T) {
	now := time.Unix(1700000000, 0)
	a := uploadResumeKey("/tmp/pack.unitypackage", 100, now)
	if a != uploadResumeKey("/tmp/pack.unitypackage", 100, now) {
		t.Error("the same file must produce the same key")
	}
	if a == uploadResumeKey("/tmp/pack.unitypackage", 101, now) {
		t.Error("a resized file must not reuse a session for the old bytes")
	}
	if a == uploadResumeKey("/tmp/pack.unitypackage", 100, now.Add(time.Second)) {
		t.Error("a rewritten file must not reuse a session for the old bytes")
	}
}
