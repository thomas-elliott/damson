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
	if !Supports("Tex/Albedo.bmp", "") {
		t.Error("bmp should be supported")
	}
	if !Supports("scan.TIFF", "") {
		t.Error("tiff (any case) should be supported")
	}
	if Supports("Player.cs", "") || Supports("clip.wav", "") {
		t.Error("code/audio should not be backend-previewable")
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
