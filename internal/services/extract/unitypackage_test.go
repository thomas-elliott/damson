package extract

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/services/storage"
)

// fakeStorage implements storage.Service with in-memory maps for tests.
type fakeStorage struct {
	blobs     map[string][]byte
	thumbs    map[string][]byte
	originals map[string][]byte
	meta      map[string]storage.ObjectMeta
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		blobs:     map[string][]byte{},
		thumbs:    map[string][]byte{},
		originals: map[string][]byte{},
		meta:      map[string]storage.ObjectMeta{},
	}
}

func (f *fakeStorage) PutBlob(_ context.Context, r io.Reader, meta storage.ObjectMeta) (string, int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(data)
	key := hex.EncodeToString(sum[:])
	f.blobs[key] = data
	f.meta[key] = meta
	return key, int64(len(data)), nil
}

func (f *fakeStorage) GetBlob(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.blobs[key])), nil
}

func (f *fakeStorage) HeadBlob(_ context.Context, key string) (int64, bool, error) {
	b, ok := f.blobs[key]
	return int64(len(b)), ok, nil
}

func (f *fakeStorage) PutOriginal(_ context.Context, r io.Reader, meta storage.ObjectMeta) (string, int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(data)
	key := hex.EncodeToString(sum[:])
	f.originals[key] = data
	f.meta[key] = meta
	return key, int64(len(data)), nil
}

func (f *fakeStorage) GetOriginal(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.originals[key])), nil
}

func (f *fakeStorage) HeadOriginal(_ context.Context, key string) (int64, bool, error) {
	b, ok := f.originals[key]
	return int64(len(b)), ok, nil
}

func (f *fakeStorage) DeleteOriginal(_ context.Context, key string) error {
	delete(f.originals, key)
	return nil
}

func (f *fakeStorage) bucket(prefix string) map[string][]byte {
	switch prefix {
	case "thumbs":
		return f.thumbs
	case "originals":
		return f.originals
	default:
		return f.blobs
	}
}

func (f *fakeStorage) List(_ context.Context, prefix string) ([]storage.ObjectInfo, error) {
	var out []storage.ObjectInfo
	for key, data := range f.bucket(prefix) {
		out = append(out, storage.ObjectInfo{Key: key, Size: int64(len(data))})
	}
	return out, nil
}

func (f *fakeStorage) StatMeta(_ context.Context, _ string, key string) (storage.ObjectMeta, error) {
	return f.meta[key], nil
}

func (f *fakeStorage) Delete(_ context.Context, prefix, key string) error {
	delete(f.bucket(prefix), key)
	return nil
}

func (f *fakeStorage) PutThumb(_ context.Context, key string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.thumbs[key] = data
	return nil
}

func (f *fakeStorage) GetThumb(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.thumbs[key])), nil
}

func (f *fakeStorage) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "http://fake/" + key, nil
}

// buildPackage assembles a .unitypackage-shaped tar.gz from a set of entries.
type entry struct {
	guid string
	leaf string // "asset" | "pathname" | "preview.png" | "asset.meta"
	data []byte
}

func buildPackage(t *testing.T, entries []entry) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.guid + "/" + e.leaf,
			Size:     int64(len(e.data)),
			Mode:     0o644,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// guidN generates a deterministic 32-hex-char GUID for tests.
func guidN(n byte) string {
	b := bytes.Repeat([]byte{n}, 16)
	return hex.EncodeToString(b)
}

func TestUnityExtractor_HappyPath(t *testing.T) {
	g1, g2 := guidN(0x11), guidN(0x22)
	pkg := buildPackage(t, []entry{
		// Deliberately out-of-order leaves within each GUID.
		{guid: g1, leaf: "pathname", data: []byte("Assets/Textures/grass.png\n")},
		{guid: g1, leaf: "asset", data: []byte("fake-grass-png-bytes")},
		{guid: g1, leaf: "asset.meta", data: []byte("fileFormatVersion: 2\n")},
		{guid: g2, leaf: "asset", data: []byte("model-bytes")},
		{guid: g2, leaf: "pathname", data: []byte("Assets/Models/Tree.fbx")},
		// A preview on guid2.
		{guid: g2, leaf: "preview.png", data: []byte("\x89PNG\r\n\x1a\nfake")},
	})

	fs := newFakeStorage()
	ex := &UnityExtractor{Storage: fs}
	res, err := ex.Extract(context.Background(), bytes.NewReader(pkg))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(res.Files) != 2 {
		t.Fatalf("expected 2 files, got %d: %+v", len(res.Files), res.Files)
	}

	byPath := map[string]UnityFile{}
	for _, f := range res.Files {
		byPath[f.RelativePath] = f
	}

	tex, ok := byPath["Assets/Textures/grass.png"]
	if !ok {
		t.Fatalf("grass.png not extracted: %+v", res.Files)
	}
	if tex.GUID != g1 || tex.SizeBytes != int64(len("fake-grass-png-bytes")) {
		t.Fatalf("grass.png row wrong: %+v", tex)
	}
	if _, ok := fs.blobs[tex.ObjectKey]; !ok {
		t.Fatalf("grass blob not in storage")
	}

	tree, ok := byPath["Assets/Models/Tree.fbx"]
	if !ok {
		t.Fatalf("tree.fbx not extracted: %+v", res.Files)
	}
	if tree.GUID != g2 {
		t.Fatalf("tree guid: got %q, want %q", tree.GUID, g2)
	}

	if res.ThumbObjectKey == "" || res.ThumbContentType != "image/png" {
		t.Fatalf("expected preview harvested, got %+v", res)
	}
	if _, ok := fs.thumbs[res.ThumbObjectKey]; !ok {
		t.Fatalf("preview blob not stored under thumbs")
	}
}

func TestUnityExtractor_SkipsIncompleteBundles(t *testing.T) {
	g := guidN(0x33)
	// Only pathname, no `asset` entry — should be silently skipped.
	pkg := buildPackage(t, []entry{
		{guid: g, leaf: "pathname", data: []byte("Assets/foo.txt")},
		{guid: g, leaf: "asset.meta", data: []byte("y: 1")},
	})
	res, err := (&UnityExtractor{Storage: newFakeStorage()}).
		Extract(context.Background(), bytes.NewReader(pkg))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(res.Files) != 0 {
		t.Fatalf("expected no files, got %+v", res.Files)
	}
}

func TestUnityExtractor_RejectsPathTraversal(t *testing.T) {
	g := guidN(0x44)
	pkg := buildPackage(t, []entry{
		{guid: g, leaf: "pathname", data: []byte("../../etc/passwd\n")},
		{guid: g, leaf: "asset", data: []byte("nope")},
	})
	res, err := (&UnityExtractor{Storage: newFakeStorage()}).
		Extract(context.Background(), bytes.NewReader(pkg))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Bundle dropped because its pathname was rejected.
	if len(res.Files) != 0 {
		t.Fatalf("traversal path should have been skipped: %+v", res.Files)
	}
}

func TestUnityExtractor_FirstPreviewWins(t *testing.T) {
	g1, g2 := guidN(0x55), guidN(0x66)
	first := []byte("preview-one")
	second := []byte("preview-two")
	pkg := buildPackage(t, []entry{
		{guid: g1, leaf: "preview.png", data: first},
		{guid: g1, leaf: "asset", data: []byte("a")},
		{guid: g1, leaf: "pathname", data: []byte("a")},
		{guid: g2, leaf: "preview.png", data: second},
		{guid: g2, leaf: "asset", data: []byte("b")},
		{guid: g2, leaf: "pathname", data: []byte("b")},
	})
	fs := newFakeStorage()
	res, err := (&UnityExtractor{Storage: fs}).
		Extract(context.Background(), bytes.NewReader(pkg))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	wantSum := sha256.Sum256(first)
	wantKey := hex.EncodeToString(wantSum[:])
	if res.ThumbObjectKey != wantKey {
		t.Fatalf("wrong preview: got %q, want %q (first)", res.ThumbObjectKey, wantKey)
	}
}

func TestUnityExtractor_TotalBytesCap(t *testing.T) {
	g := guidN(0x77)
	pkg := buildPackage(t, []entry{
		{guid: g, leaf: "pathname", data: []byte("Assets/big.bin")},
		{guid: g, leaf: "asset", data: bytes.Repeat([]byte{'x'}, 256)},
	})
	ex := &UnityExtractor{Storage: newFakeStorage(), MaxTotalBytes: 128}
	_, err := ex.Extract(context.Background(), bytes.NewReader(pkg))
	if err == nil || !strings.Contains(err.Error(), "total size") {
		t.Fatalf("expected total-size cap error, got %v", err)
	}
}

func TestUnityExtractor_RejectsGarbageInput(t *testing.T) {
	// Neither gzip nor valid tar — should surface a tar error rather than
	// succeeding silently.
	_, err := (&UnityExtractor{Storage: newFakeStorage()}).
		Extract(context.Background(), bytes.NewReader([]byte("not an archive at all")))
	if err == nil {
		t.Fatalf("expected error for garbage input")
	}
}

// TestUnityExtractor_AcceptsRawTar covers .unitypackage files that are plain
// POSIX tars without gzip framing — rarer than the tar.gz variant but present
// in real-world Asset Store downloads.
func TestUnityExtractor_AcceptsRawTar(t *testing.T) {
	g1 := guidN(0x33)
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	for _, e := range []entry{
		{guid: g1, leaf: "pathname", data: []byte("Assets/Props/chip.fbx")},
		{guid: g1, leaf: "asset", data: []byte("chip-bytes")},
	} {
		hdr := &tar.Header{
			Name:     e.guid + "/" + e.leaf,
			Size:     int64(len(e.data)),
			Mode:     0o644,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	fs := newFakeStorage()
	res, err := (&UnityExtractor{Storage: fs}).
		Extract(context.Background(), bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("extract raw tar: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0].RelativePath != "Assets/Props/chip.fbx" {
		t.Fatalf("unexpected files: %+v", res.Files)
	}
}
