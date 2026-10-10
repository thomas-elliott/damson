package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
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
	arc := buildZip(t, []zentry{
		{name: "pack/readme.txt", data: []byte("hello")},
		{name: "pack/model.fbx", data: []byte("fbx-bytes")},
		{name: "pack/cover.png", data: solidPNG(t, 64, 64, color.RGBA{B: 255, A: 255})},
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
	if _, ok := fs.thumbs[res.ThumbObjectKey]; !ok {
		t.Fatalf("cover not stored under thumbs")
	}
}

// solidPNG encodes a w×h PNG filled with c.
func solidPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A zip's cover is a thumbnail like any other: the first image that decodes,
// trying cover-like names first, resized and in a format a browser shows.
func TestZipExtractor_CoverIsTheFirstDecodableImageResized(t *testing.T) {
	blue := color.RGBA{B: 255, A: 255}
	arc := buildZip(t, []zentry{
		{name: "pack/a_broken.png", data: []byte("\x89PNG\r\n\x1a\nnot really")},
		{name: "pack/b_texture.png", data: solidPNG(t, 32, 32, color.RGBA{R: 255, A: 255})},
		{name: "pack/z_Cover.png", data: solidPNG(t, 1024, 768, blue)},
	})
	fs := newFakeStorage()
	res, err := (&ZipExtractor{Storage: fs}).Extract(context.Background(), bytes.NewReader(arc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if res.ThumbContentType != "image/png" && res.ThumbContentType != "image/jpeg" {
		t.Fatalf("cover content type %q is not one a browser shows", res.ThumbContentType)
	}
	img, _, err := image.Decode(bytes.NewReader(fs.thumbs[res.ThumbObjectKey]))
	if err != nil {
		t.Fatalf("stored cover does not decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 384 {
		t.Fatalf("cover is %v, want the 1024×768 cover resized to 512×384", b)
	}
	if r, g, b, _ := img.At(10, 10).RGBA(); r != 0 || g != 0 || b < 0xf000 {
		t.Fatalf("cover was not taken from the cover-named image")
	}
}

func TestZipExtractor_NoDecodableImageMeansNoCover(t *testing.T) {
	arc := buildZip(t, []zentry{
		{name: "pack/cover.png", data: []byte("\x89PNG\r\n\x1a\nnot really")},
		{name: "pack/art.psd", data: []byte("8BPS garbage")},
	})
	res, err := (&ZipExtractor{Storage: newFakeStorage()}).Extract(context.Background(), bytes.NewReader(arc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if res.ThumbObjectKey != "" {
		t.Fatalf("a zip with no decodable image got cover %q", res.ThumbObjectKey)
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
