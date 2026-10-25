package extract

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/services/storage"
)

// fakeStorage implements storage.Service with in-memory maps for tests.
type fakeStorage struct {
	blobs     map[string][]byte
	thumbs    map[string][]byte
	originals map[string][]byte
	meta      map[string]storage.ObjectMeta
	uploads   map[string]*fakeMultipart
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		blobs:     map[string][]byte{},
		thumbs:    map[string][]byte{},
		originals: map[string][]byte{},
		meta:      map[string]storage.ObjectMeta{},
		uploads:   map[string]*fakeMultipart{},
	}
}

func (f *fakeStorage) bucket(prefix storage.Prefix) map[string][]byte {
	switch prefix {
	case storage.Thumbs:
		return f.thumbs
	case storage.Originals:
		return f.originals
	default:
		return f.blobs
	}
}

func (f *fakeStorage) Put(_ context.Context, prefix storage.Prefix, r io.Reader, meta storage.ObjectMeta) (string, int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(data)
	key := hex.EncodeToString(sum[:])
	f.bucket(prefix)[key] = data
	f.meta[key] = meta
	return key, int64(len(data)), nil
}

func (f *fakeStorage) Get(_ context.Context, prefix storage.Prefix, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.bucket(prefix)[key])), nil
}

func (f *fakeStorage) List(_ context.Context, prefix storage.Prefix) ([]storage.ObjectInfo, error) {
	var out []storage.ObjectInfo
	for key, data := range f.bucket(prefix) {
		out = append(out, storage.ObjectInfo{Key: key, Size: int64(len(data))})
	}
	return out, nil
}

func (f *fakeStorage) StatMeta(_ context.Context, _ storage.Prefix, key string) (storage.ObjectMeta, error) {
	return f.meta[key], nil
}

func (f *fakeStorage) Stat(_ context.Context, prefix storage.Prefix, key string) (storage.ObjectInfo, bool, error) {
	data, ok := f.bucket(prefix)[key]
	return storage.ObjectInfo{Key: key, Size: int64(len(data))}, ok, nil
}

func (f *fakeStorage) Touch(_ context.Context, prefix storage.Prefix, key string) (bool, error) {
	_, ok := f.bucket(prefix)[key]
	return ok, nil
}

func (f *fakeStorage) Delete(_ context.Context, prefix storage.Prefix, key string) error {
	delete(f.bucket(prefix), key)
	return nil
}

func (f *fakeStorage) BeginMultipart(_ context.Context, prefix storage.Prefix, key string, meta storage.ObjectMeta) (storage.MultipartUpload, error) {
	id := fmt.Sprintf("mpu-%d", len(f.uploads)+1)
	u := &fakeMultipart{s: f, id: id, prefix: prefix, key: key, meta: meta, parts: map[int][]byte{}}
	f.uploads[id] = u
	return u, nil
}

func (f *fakeStorage) ResumeMultipart(_ context.Context, _ storage.Prefix, _, uploadID string) (storage.MultipartUpload, error) {
	u, ok := f.uploads[uploadID]
	if !ok {
		return nil, fmt.Errorf("no such upload %q", uploadID)
	}
	return u, nil
}

type fakeMultipart struct {
	s      *fakeStorage
	id     string
	prefix storage.Prefix
	key    string
	meta   storage.ObjectMeta
	parts  map[int][]byte
}

func (m *fakeMultipart) ID() string { return m.id }

func (m *fakeMultipart) UploadPart(_ context.Context, n int, size int64, r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return "", err
	}
	m.parts[n] = data
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (m *fakeMultipart) Complete(_ context.Context, parts []storage.Part) (string, error) {
	numbers := make([]int, 0, len(parts))
	for _, p := range parts {
		numbers = append(numbers, p.Number)
	}
	sort.Ints(numbers)
	var body []byte
	for _, n := range numbers {
		body = append(body, m.parts[n]...)
	}
	m.s.bucket(m.prefix)[m.key] = body
	m.s.meta[m.key] = m.meta
	delete(m.s.uploads, m.id)
	return m.key, nil
}

func (m *fakeMultipart) Abort(_ context.Context) error {
	delete(m.s.uploads, m.id)
	return nil
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

// TestUnityExtractor_StripsPathnameFlagLine covers the real Unity pathname
// sidecar shape: the asset path on line 1 followed by a trailing flag line
// (commonly "00"). Only the first line is the path; the flag must not leak
// into the extracted file name or every extension-based lookup (previews,
// type detection) breaks.
func TestUnityExtractor_StripsPathnameFlagLine(t *testing.T) {
	g := guidN(0x88)
	pkg := buildPackage(t, []entry{
		{guid: g, leaf: "pathname", data: []byte("Assets/Textures/grass.png\n00\n")},
		{guid: g, leaf: "asset", data: []byte("grass-bytes")},
	})
	res, err := (&UnityExtractor{Storage: newFakeStorage()}).
		Extract(context.Background(), bytes.NewReader(pkg))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("expected 1 file, got %d: %+v", len(res.Files), res.Files)
	}
	if got := res.Files[0].RelativePath; got != "Assets/Textures/grass.png" {
		t.Fatalf("pathname flag line leaked: got %q, want %q", got, "Assets/Textures/grass.png")
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

// TestUnityExtractor_PrefersImagePathnamePreview proves the cover is taken
// from the preview whose bundle is an actual image asset, even when a
// non-image asset's preview appears first in tar order.
func TestUnityExtractor_PrefersImagePathnamePreview(t *testing.T) {
	gVideo, gImage := guidN(0xa1), guidN(0xa2)
	videoPrev := []byte("video-frame-preview")
	imagePrev := []byte("real-texture-preview")
	pkg := buildPackage(t, []entry{
		// Non-image asset (a video) with a preview, first in tar order.
		{guid: gVideo, leaf: "preview.png", data: videoPrev},
		{guid: gVideo, leaf: "asset", data: []byte("mp4-bytes")},
		{guid: gVideo, leaf: "pathname", data: []byte("Assets/Clips/trailer.mp4")},
		// Image asset with a preview, later in tar order.
		{guid: gImage, leaf: "preview.png", data: imagePrev},
		{guid: gImage, leaf: "asset", data: []byte("png-bytes")},
		{guid: gImage, leaf: "pathname", data: []byte("Assets/Textures/hero.png")},
	})
	fs := newFakeStorage()
	res, err := (&UnityExtractor{Storage: fs}).
		Extract(context.Background(), bytes.NewReader(pkg))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	wantSum := sha256.Sum256(imagePrev)
	wantKey := hex.EncodeToString(wantSum[:])
	if res.ThumbObjectKey != wantKey {
		t.Fatalf("cover = %q, want the image-asset preview %q (not the earlier video preview)", res.ThumbObjectKey, wantKey)
	}
	videoSum := sha256.Sum256(videoPrev)
	if res.ThumbObjectKey == hex.EncodeToString(videoSum[:]) {
		t.Fatalf("cover wrongly taken from the video-asset preview")
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
