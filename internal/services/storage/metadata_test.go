package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMetaToS3(t *testing.T) {
	// Populated meta: filename URL-encoded, IDs formatted, kind verbatim.
	got := metaToS3(ObjectMeta{
		Filename:  "Grässe Pack.unitypackage",
		LibraryID: 3,
		AssetID:   42,
		VersionID: 7,
		Kind:      "unitypackage",
	})
	want := map[string]string{
		"filename":   "Gr%C3%A4sse+Pack.unitypackage",
		"library-id": "3",
		"asset-id":   "42",
		"version-id": "7",
		"kind":       "unitypackage",
	}
	if len(got) != len(want) {
		t.Fatalf("key count: got %d (%v), want %d", len(got), got, len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("meta[%q]: got %q, want %q", k, got[k], v)
		}
	}

	// Empty meta must produce nil so PutObject omits the metadata block — a
	// non-nil empty map would be a behaviour change for every existing caller.
	if m := metaToS3(ObjectMeta{}); m != nil {
		t.Fatalf("empty meta: got %v, want nil", m)
	}
}

func TestLocalFS_Original_RoundTripSidecarDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFS(dir, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	payload := []byte("archive bytes")
	meta := ObjectMeta{Filename: "Pack.unitypackage", LibraryID: 1, AssetID: 2, VersionID: 3, Kind: "unitypackage"}
	key, _, err := s.PutOriginal(ctx, bytes.NewReader(payload), meta)
	if err != nil {
		t.Fatalf("put original: %v", err)
	}

	// Lives under originals/, and is NOT reachable via the blobs/ prefix.
	if sz, exists, err := s.HeadOriginal(ctx, key); err != nil || !exists || sz != int64(len(payload)) {
		t.Fatalf("head original: size=%d exists=%v err=%v", sz, exists, err)
	}
	if _, exists, _ := s.HeadBlob(ctx, key); exists {
		t.Fatalf("original key leaked into blobs/ prefix")
	}

	// Sidecar records the metadata verbatim.
	sidecar := filepath.Join(dir, "originals", key[:2], key+metaSuffix)
	raw, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var back ObjectMeta
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal sidecar: %v", err)
	}
	if back != meta {
		t.Fatalf("sidecar meta: got %+v, want %+v", back, meta)
	}

	// Delete removes both object and sidecar.
	if err := s.DeleteOriginal(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, exists, _ := s.HeadOriginal(ctx, key); exists {
		t.Fatalf("original still present after delete")
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("sidecar still present after delete: err=%v", err)
	}
}

func TestLocalFS_PutBlob_EmptyMetaWritesNoSidecar(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFS(dir, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := s.PutBlob(context.Background(), bytes.NewReader([]byte("x")), ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(dir, "blobs", key[:2], key+metaSuffix)
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("empty meta should write no sidecar, but found one: err=%v", err)
	}
}
