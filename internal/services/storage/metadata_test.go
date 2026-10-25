package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	s, err := NewLocalFS(dir)
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

func TestMetaFromS3RoundTrip(t *testing.T) {
	meta := ObjectMeta{
		Filename:  "Grässe Pack.unitypackage",
		LibraryID: 3,
		AssetID:   42,
		VersionID: 7,
		Kind:      "unitypackage",
	}
	if back := metaFromS3(metaToS3(meta)); back != meta {
		t.Fatalf("round-trip: got %+v, want %+v", back, meta)
	}

	// Empty S3 metadata decodes to the zero value.
	if back := metaFromS3(nil); back != (ObjectMeta{}) {
		t.Fatalf("nil metadata: got %+v, want zero", back)
	}

	// Malformed numeric fields decode to zero rather than erroring — the
	// metadata is a best-effort fallback.
	got := metaFromS3(map[string]string{"library-id": "notanumber", "kind": "zip"})
	if got.LibraryID != 0 || got.Kind != "zip" {
		t.Fatalf("malformed numeric: got %+v", got)
	}
}

func TestLocalFS_ListStatDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	meta := ObjectMeta{Filename: "a.png", LibraryID: 1, AssetID: 2, VersionID: 3, Kind: "loose"}
	withMeta, _, err := s.PutBlob(ctx, bytes.NewReader([]byte("with-meta")), meta)
	if err != nil {
		t.Fatal(err)
	}
	noMeta, _, err := s.PutBlob(ctx, bytes.NewReader([]byte("no-meta")), ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	thumbData := []byte("thumb bytes")
	sum := sha256.Sum256(thumbData)
	thumbKey := hex.EncodeToString(sum[:])
	if err := s.PutThumb(ctx, thumbKey, bytes.NewReader(thumbData)); err != nil {
		t.Fatal(err)
	}

	// List returns both blobs (with correct sizes), and does not leak across prefixes.
	blobs, err := s.List(ctx, "blobs")
	if err != nil {
		t.Fatalf("list blobs: %v", err)
	}
	found := map[string]int64{}
	for _, o := range blobs {
		found[o.Key] = o.Size
	}
	if found[withMeta] != int64(len("with-meta")) || found[noMeta] != int64(len("no-meta")) {
		t.Fatalf("list blobs: got %+v", found)
	}
	if _, leaked := found[thumbKey]; leaked {
		t.Fatalf("thumb key leaked into blobs listing")
	}
	if thumbs, _ := s.List(ctx, "thumbs"); len(thumbs) != 1 || thumbs[0].Key != thumbKey {
		t.Fatalf("list thumbs: got %+v", thumbs)
	}

	// StatMeta returns the stored meta, and empty (no error) when none was written.
	if got, err := s.StatMeta(ctx, "blobs", withMeta); err != nil || got != meta {
		t.Fatalf("statmeta with meta: got %+v err=%v", got, err)
	}
	if got, err := s.StatMeta(ctx, "blobs", noMeta); err != nil || got != (ObjectMeta{}) {
		t.Fatalf("statmeta no meta: got %+v err=%v, want zero/nil", got, err)
	}

	// Delete removes the object; a second delete of a missing key is not an error.
	if err := s.Delete(ctx, "blobs", withMeta); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, exists, _ := s.HeadBlob(ctx, withMeta); exists {
		t.Fatalf("blob still present after delete")
	}
	if err := s.Delete(ctx, "blobs", withMeta); err != nil {
		t.Fatalf("delete missing key should be nil, got %v", err)
	}
	after, _ := s.List(ctx, "blobs")
	if len(after) != 1 || after[0].Key != noMeta {
		t.Fatalf("after delete, list blobs: got %+v", after)
	}
}

func TestLocalFS_PutBlob_EmptyMetaWritesNoSidecar(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFS(dir)
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
