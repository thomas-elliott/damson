package preview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"regexp"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"github.com/thomas-elliott/damson/internal/services/storage"
)

var hexKey = regexp.MustCompile(`^[0-9a-f]{64}$`)

func newTestStorage(t *testing.T) storage.Service {
	t.Helper()
	s, err := storage.NewLocalFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	return s
}

func TestSupports(t *testing.T) {
	for _, name := range []string{"Tex/Albedo.bmp", "scan.TIFF", "art.psd"} {
		if !Supports(name, "") {
			t.Errorf("%s should be supported", name)
		}
	}
	if Supports("Player.cs", "") || Supports("clip.wav", "") {
		t.Error("code/audio should not be backend-previewable")
	}
	// PDFs keep pdf.js in the panel and their icon in the grid: no poster is
	// rendered (rejected; see the 2026-10-10 doc-reconciliation spec).
	if Supports("manual.pdf", "") {
		t.Error("a PDF should get no backend preview")
	}
}

func TestGenerate_BMP_OpaqueIsJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for x := 0; x < 24; x++ {
		for y := 0; y < 16; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 120, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatalf("encode bmp: %v", err)
	}

	g := &Generator{Storage: newTestStorage(t)}
	res, err := g.Generate(context.Background(), Input{Reader: &buf, Filename: "x.bmp"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.ContentType != "image/jpeg" {
		t.Errorf("opaque image should encode as JPEG, got %q", res.ContentType)
	}
	if !hexKey.MatchString(res.ObjectKey) {
		t.Errorf("object key should be sha256 hex, got %q", res.ObjectKey)
	}
}

func TestGenerate_TIFF_AlphaIsPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 128}) // partial alpha
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode tiff: %v", err)
	}

	g := &Generator{Storage: newTestStorage(t)}
	res, err := g.Generate(context.Background(), Input{Reader: &buf, Filename: "x.tiff"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.ContentType != "image/png" {
		t.Errorf("image with alpha should encode as PNG, got %q", res.ContentType)
	}
}

func TestGenerate_SVG(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">` +
		`<rect width="100" height="50" fill="#caa84a"/></svg>`
	g := &Generator{Storage: newTestStorage(t)}
	res, err := g.Generate(context.Background(), Input{
		Reader:   bytes.NewReader([]byte(svg)),
		Filename: "logo.svg",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.ContentType != "image/png" {
		t.Errorf("svg should rasterise to PNG, got %q", res.ContentType)
	}
	if !hexKey.MatchString(res.ObjectKey) {
		t.Errorf("svg should produce a derivative, got key %q", res.ObjectKey)
	}
}

func TestGenerate_Unsupported(t *testing.T) {
	g := &Generator{Storage: newTestStorage(t)}
	res, err := g.Generate(context.Background(), Input{Reader: bytes.NewReader([]byte("x")), Filename: "lib.dll"})
	if err != nil {
		t.Fatalf("unsupported should be a no-op, not an error: %v", err)
	}
	if res.ObjectKey != "" {
		t.Errorf("unsupported type should produce no derivative, got %q", res.ObjectKey)
	}
}
