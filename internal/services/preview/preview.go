// Package preview renders per-file preview derivatives: web-friendly,
// safe-to-serve artifacts (flattened or rasterised images) for file types a
// browser can't open directly. It mirrors services/thumbnail but produces one
// derivative per asset_files row (a larger image than the per-asset thumbnail)
// and stores it content-addressed in the same thumb namespace.
//
// Rasters decode through services/imaging, the same decoders the grid
// thumbnail uses; SVG is rasterised here. A zero Result means
// "no preview for this input" — callers leave the file's preview unset and the
// SPA falls back to the type icon.
package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/draw"

	"github.com/thomas-elliott/damson/internal/services/imaging"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

const (
	// previewMaxEdge is the long-edge cap for a rasterised preview — larger than
	// the 512px thumbnail so the detail panel shows real detail, but bounded so
	// a derivative can't balloon.
	previewMaxEdge = 1600
	jpegQuality    = 85
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
	case "svg":
		return g.renderSVG(ctx, in)
	}
	return Result{}, nil
}

func classify(filename, detectedType string) string {
	switch {
	case strings.ToLower(filepath.Ext(filename)) == ".svg":
		return "svg"
	case imaging.NeedsConversion(filename):
		return "raster"
	}
	return ""
}

// renderRaster decodes a browser-unopenable raster through the shared imaging
// decoders, downsamples it, and re-encodes to a web-safe image: JPEG for
// opaque content, PNG otherwise. An image over the imaging pixel ceiling
// gets no preview.
func (g *Generator) renderRaster(ctx context.Context, in Input) (Result, error) {
	src, _, err := imaging.Decode(in.Reader, in.Filename)
	if errors.Is(err, imaging.ErrTooLarge) {
		return Result{}, nil // too large — skip, fall back to icon
	}
	if err != nil {
		return Result{}, fmt.Errorf("preview decode: %w", err)
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

// renderSVG rasterises an SVG to a PNG on the backend, so no active content
// (script, external refs) ever reaches the browser — matching the hostile-blob
// posture that keeps SVG out of inline preview. oksvg supports a useful subset;
// complex SVGs render approximately, and a parse failure leaves the file
// download-only (zero Result).
func (g *Generator) renderSVG(ctx context.Context, in Input) (Result, error) {
	icon, err := oksvg.ReadIconStream(in.Reader)
	if err != nil {
		return Result{}, fmt.Errorf("preview svg parse: %w", err)
	}
	w, h := svgTarget(icon.ViewBox.W, icon.ViewBox.H, previewMaxEdge)
	if int64(w)*int64(h) > imaging.MaxPixels {
		return Result{}, nil
	}
	icon.SetTarget(0, 0, float64(w), float64(h))

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, img, img.Bounds())
	raster := rasterx.NewDasher(w, h, scanner)
	icon.Draw(raster, 1.0)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return Result{}, fmt.Errorf("preview svg encode: %w", err)
	}
	key, err := g.putBytes(ctx, buf.Bytes())
	if err != nil {
		return Result{}, err
	}
	return Result{ObjectKey: key, ContentType: "image/png"}, nil
}

// svgTarget renders so the longer edge is maxEdge (vectors upscale crisply),
// preserving the viewBox aspect ratio. A missing/zero viewBox renders square.
func svgTarget(vbW, vbH float64, maxEdge int) (int, int) {
	if vbW <= 0 || vbH <= 0 {
		return maxEdge, maxEdge
	}
	if vbW >= vbH {
		return maxEdge, int(float64(maxEdge) * vbH / vbW)
	}
	return int(float64(maxEdge) * vbW / vbH), maxEdge
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
	key, _, err := g.Storage.Put(ctx, storage.Thumbs, bytes.NewReader(b), storage.ObjectMeta{})
	if err != nil {
		return "", fmt.Errorf("preview put: %w", err)
	}
	return key, nil
}
