package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

type zentry struct {
	name string
	data []byte
}

func buildZip(t *testing.T, entries []zentry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestZipExtractor_HappyPath(t *testing.T) {
	cover := []byte("\x89PNG\r\n\x1a\nfake-image")
	arc := buildZip(t, []zentry{
		{name: "pack/readme.txt", data: []byte("hello")},
		{name: "pack/model.fbx", data: []byte("fbx-bytes")},
		{name: "pack/cover.png", data: cover},
	})

	fs := newFakeStorage()
	res, err := (&ZipExtractor{Storage: fs}).Extract(context.Background(), bytes.NewReader(arc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(res.Files) != 3 {
		t.Fatalf("expected 3 files, got %d: %+v", len(res.Files), res.Files)
	}
	byPath := map[string]UnityFile{}
	for _, f := range res.Files {
		byPath[f.RelativePath] = f
		if _, ok := fs.blobs[f.ObjectKey]; !ok {
			t.Fatalf("blob for %q not stored", f.RelativePath)
		}
	}
	if _, ok := byPath["pack/model.fbx"]; !ok {
		t.Fatalf("model.fbx not extracted: %+v", res.Files)
	}

	// The image entry is harvested as the cover thumbnail.
	if res.ThumbObjectKey == "" {
		t.Fatalf("expected a cover thumbnail to be harvested")
	}
	if res.ThumbContentType != "image/png" {
		t.Fatalf("cover content type: got %q, want image/png", res.ThumbContentType)
	}
	wantKey := sha256.Sum256(cover)
	if res.ThumbObjectKey != hex.EncodeToString(wantKey[:]) {
		t.Fatalf("cover key mismatch: got %q", res.ThumbObjectKey)
	}
	if _, ok := fs.thumbs[res.ThumbObjectKey]; !ok {
		t.Fatalf("cover not stored under thumbs")
	}
}

func TestZipExtractor_RejectsZipSlip(t *testing.T) {
	arc := buildZip(t, []zentry{
		{name: "../../etc/passwd", data: []byte("root:x:0:0")},
		{name: "safe.txt", data: []byte("ok")},
	})
	res, err := (&ZipExtractor{Storage: newFakeStorage()}).
		Extract(context.Background(), bytes.NewReader(arc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0].RelativePath != "safe.txt" {
		t.Fatalf("zip-slip entry not rejected: %+v", res.Files)
	}
}

func TestZipExtractor_EntryCountCap(t *testing.T) {
	arc := buildZip(t, []zentry{
		{name: "a.txt", data: []byte("a")},
		{name: "b.txt", data: []byte("b")},
	})
	_, err := (&ZipExtractor{Storage: newFakeStorage(), MaxEntries: 1}).
		Extract(context.Background(), bytes.NewReader(arc))
	if err == nil || !strings.Contains(err.Error(), "too many entries") {
		t.Fatalf("expected entry-count cap error, got %v", err)
	}
}

func TestZipExtractor_PerEntrySizeCap(t *testing.T) {
	arc := buildZip(t, []zentry{
		{name: "big.bin", data: bytes.Repeat([]byte{'x'}, 256)},
	})
	_, err := (&ZipExtractor{Storage: newFakeStorage(), MaxEntrySize: 128}).
		Extract(context.Background(), bytes.NewReader(arc))
	if err == nil || !strings.Contains(err.Error(), "per-entry cap") {
		t.Fatalf("expected per-entry cap error, got %v", err)
	}
}

func TestZipExtractor_DedupesDuplicatePaths(t *testing.T) {
	// A zip may legally carry two entries with the same name; only one
	// asset_files row may exist per (version, relative_path).
	arc := buildZip(t, []zentry{
		{name: "dup.txt", data: []byte("first")},
		{name: "dup.txt", data: []byte("second")},
	})
	res, err := (&ZipExtractor{Storage: newFakeStorage()}).
		Extract(context.Background(), bytes.NewReader(arc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("expected duplicate paths collapsed to 1 file, got %d: %+v", len(res.Files), res.Files)
	}
}
