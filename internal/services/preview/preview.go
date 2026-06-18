// Package preview renders per-file preview derivatives: web-friendly,
// safe-to-serve artifacts (flattened or rasterised images) for file types a
// browser can't open directly. It mirrors services/thumbnail but produces one
// derivative per asset_files row (a larger image than the per-asset thumbnail)
// and stores it content-addressed in the same thumb namespace.
//
// Stage 4 seeds this with pure-Go TIFF/BMP; later stages add PSD/EXR/TGA (raster
// family), SVG (rasterise), and PDF (first-page poster). A zero Result means
// "no preview for this input" — callers leave the file's preview unset and the
// SPA falls back to the type icon.
package preview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/ftrvxmtrx/tga"
	"github.com/oov/psd"
	_ "golang.org/x/image/bmp" // bmp decode registration
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff" // tiff decode registration

	"github.com/thomas-elliott/damson/internal/services/storage"
)

const (
	// previewMaxEdge is the long-edge cap for a rasterised preview — larger than
	// the 512px thumbnail so the detail panel shows real detail, but bounded so
	// a derivative can't balloon.
	previewMaxEdge = 1600
	// maxPixels guards against decompression bombs: DecodeConfig is read first
	// and anything whose declared dimensions exceed this is skipped.
	maxPixels   = 100 * 1000 * 1000 // 100 MP
	jpegQuality = 85
)

// Result is a generated derivative. A zero Result means the input had no
// preview renderer (not an error).
type Result struct {
	ObjectKey   string
	ContentType string
}

// Input carries what a renderer needs about a source file. The reader is
// consumed; callers supply a fresh stream per call.
type Input struct {
	Reader       io.Reader
	Filename     string
	DetectedType string
}

// Generator renders a derivative and stores it via the supplied storage.
type Generator struct {
	Storage storage.Service
}

// Supports reports whether a file has a backend preview renderer, so the
// pipeline can skip fetching blobs it can't use.
func Supports(filename, detectedType string) bool {
	return classify(filename, detectedType) != ""
}

// Generate renders the preview derivative for an input, or a zero Result when
// the type isn't supported.
func (g *Generator) Generate(ctx context.Context, in Input) (Result, error) {
	if g.Storage == nil {
		return Result{}, fmt.Errorf("preview: storage not configured")
	}
	switch classify(in.Filename, in.DetectedType) {
	case "raster":
		return g.renderRaster(ctx, in)
	}
	return Result{}, nil
}

func classify(filename, detectedType string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".tiff", ".tif", ".bmp", ".tga", ".psd":
		return "raster"
	}
	return ""
}

// decodeRaster decodes a browser-unopenable raster by extension. TGA has no
// magic bytes (so image.Decode can't sniff it) and PSD is read for its merged
// composite; TIFF/BMP go through the registered x/image decoders.
func decodeRaster(raw []byte, ext string) (image.Image, error) {
	switch ext {
	case ".tga":
		return tga.Decode(bytes.NewReader(raw))
	case ".psd":
		p, _, err := psd.Decode(bytes.NewReader(raw), nil)
		if err != nil {
			return nil, err
		}
		if p.Picker == nil {
			return nil, fmt.Errorf("psd: no merged composite")
		}
		return p.Picker, nil
	default:
		img, _, err := image.Decode(bytes.NewReader(raw))
		return img, err
	}
}

// renderRaster decodes a (browser-unopenable) raster, downsamples it, and
// re-encodes to a web-safe image: JPEG for opaque content, PNG otherwise.
func (g *Generator) renderRaster(ctx context.Context, in Input) (Result, error) {
	raw, err := io.ReadAll(in.Reader)
	if err != nil {
		return Result{}, fmt.Errorf("preview read: %w", err)
	}
	// Cheap pre-decode bomb guard for formats that expose a config without
	// decoding the pixels (TIFF/BMP/PSD). TGA has no such header, so it relies
	// on the post-decode bounds check below.
	if cfg, _, cErr := image.DecodeConfig(bytes.NewReader(raw)); cErr == nil {
		if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
			return Result{}, nil // too large — skip, fall back to icon
		}
	}
	src, err := decodeRaster(raw, strings.ToLower(filepath.Ext(in.Filename)))
	if err != nil {
		return Result{}, fmt.Errorf("preview decode: %w", err)
	}
	if b := src.Bounds(); int64(b.Dx())*int64(b.Dy()) > maxPixels {
		return Result{}, nil // decoded larger than the ceiling — skip
	}

	scaled := resample(src, previewMaxEdge)
	var buf bytes.Buffer
	var contentType string
	if isOpaque(scaled) {
		if err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return Result{}, fmt.Errorf("preview jpeg encode: %w", err)
		}
		contentType = "image/jpeg"
	} else {
		if err := png.Encode(&buf, scaled); err != nil {
			return Result{}, fmt.Errorf("preview png encode: %w", err)
		}
		contentType = "image/png"
	}

	key, err := g.putBytes(ctx, buf.Bytes())
	if err != nil {
		return Result{}, err
	}
	return Result{ObjectKey: key, ContentType: contentType}, nil
}

// resample scales src down so its longer edge is at most maxEdge, preserving
// aspect ratio. Images already within bounds pass through untouched.
func resample(src image.Image, maxEdge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxEdge && h <= maxEdge {
		return src
	}
	scale := float64(maxEdge) / float64(max(w, h))
	dst := image.NewRGBA(image.Rect(0, 0, int(float64(w)*scale), int(float64(h)*scale)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}

// putBytes writes derivative bytes content-addressed (sha256) into the thumb
// namespace, so retries and duplicates collapse onto the same object.
func (g *Generator) putBytes(ctx context.Context, b []byte) (string, error) {
	sum := sha256.Sum256(b)
	key := hex.EncodeToString(sum[:])
	if err := g.Storage.PutThumb(ctx, key, bytes.NewReader(b)); err != nil {
		return "", fmt.Errorf("preview put: %w", err)
	}
	return key, nil
}
