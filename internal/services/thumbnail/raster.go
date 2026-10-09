package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"

	"github.com/thomas-elliott/damson/internal/services/imaging"
)

// resampleMaxEdge is the long-edge size of generated thumbnails. 512 matches
// docs/archive/technical-design.md §11.1.
const resampleMaxEdge = 512

// jpegQuality is the encoder quality for photographic thumbnails; PNG is
// lossless so this only applies to the JPEG branch.
const jpegQuality = 85

// renderRaster decodes the input through the shared imaging decoders, which
// refuse an image over imaging.MaxPixels, resamples to `resampleMaxEdge` on the longer edge, and
// re-encodes. Per §11.1 we pick JPEG for photographic content (clean RGBA
// with no palette, full alpha or transparency) and PNG for everything else.
// The call sites that hit us — raster uploads from a user — won't know
// what they've dropped, so we inspect the decoded image rather than trust
// the input content type.
func (g *Generator) renderRaster(ctx context.Context, in Input) (Result, error) {
	src, fmtName, err := imaging.Decode(in.Reader)
	if errors.Is(err, imaging.ErrTooLarge) {
		return Result{}, nil // no thumbnail: the placeholder shows
	}
	if err != nil {
		return Result{}, fmt.Errorf("raster decode: %w", err)
	}

	scaled := resample(src, resampleMaxEdge)
	useJPEG := pickJPEG(scaled, fmtName)

	var buf bytes.Buffer
	var contentType string
	if useJPEG {
		if err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return Result{}, fmt.Errorf("jpeg encode: %w", err)
		}
		contentType = "image/jpeg"
	} else {
		if err := png.Encode(&buf, scaled); err != nil {
			return Result{}, fmt.Errorf("png encode: %w", err)
		}
		contentType = "image/png"
	}

	key, err := g.putThumbBytes(ctx, buf.Bytes())
	if err != nil {
		return Result{}, err
	}
	return Result{ObjectKey: key, ContentType: contentType, Category: "raster"}, nil
}

// resample downscales src so its longer edge equals max, preserving aspect
// ratio. If src is already smaller we leave it alone — upscaling a tiny
// icon produces a blurry mess, and the frontend is fine with whatever ratio
// we hand back.
func resample(src image.Image, max int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return src
	}
	var tw, th int
	if w >= h {
		tw = max
		th = h * max / w
	} else {
		th = max
		tw = w * max / h
	}
	if tw <= 0 {
		tw = 1
	}
	if th <= 0 {
		th = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// pickJPEG decides JPEG vs PNG for the encoded thumbnail. JPEG is reserved
// for photographic-like content: fully-opaque RGB sampled from a format
// that's typically photographic (JPEG / WebP). PNG is the default for
// anything that might have alpha, a palette, or rendered text.
func pickJPEG(img image.Image, srcFmt string) bool {
	if srcFmt != "jpeg" && srcFmt != "webp" {
		return false
	}
	// Reject anything with an alpha channel: the decoded image may still be
	// RGBA but the encode target determines whether alpha is preserved. A
	// quick transparency scan lets us catch PNG-masqueraded-as-WebP cases.
	return !hasTransparency(img)
}

// hasTransparency scans every Nth pixel looking for a non-opaque sample.
// Sampling is fine — a thumbnail-sized image has >= 500 pixels per row and
// transparent content tends to be obvious. The loop bails on the first hit.
func hasTransparency(img image.Image) bool {
	b := img.Bounds()
	step := 4
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xff00 { // tolerate tiny rounding below full opacity
				return true
			}
		}
	}
	return false
}
