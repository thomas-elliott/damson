// Package imaging decodes the raster images Damson renders, for both the grid
// thumbnail and the per-file preview, so the two agree on which files are
// images. Every decode is refused above MaxPixels, judged from the image's
// header before its pixels are allocated. Pure Go: no cgo, no external binary.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // gif decode registration
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/blezek/tga"
	"github.com/mdouchement/hdr"
	"github.com/mdouchement/hdr/codec/rgbe"
	"github.com/mrjoshuak/go-openexr/exr"
	"github.com/oov/psd"
	_ "golang.org/x/image/bmp" // bmp decode registration
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// MaxPixels is the decompression-bomb ceiling: an image declaring more pixels
// than this is not decoded, and gets the placeholder.
const MaxPixels = 100 * 1000 * 1000

// ErrTooLarge reports an image over MaxPixels.
var ErrTooLarge = errors.New("image exceeds the pixel ceiling")

// codec decodes one format that is chosen by extension rather than sniffed:
// TGA has no magic bytes, and the others are kept off image.Decode so a new
// format never changes what an existing file sniffs as. config reads only
// what it needs to report the dimensions.
type codec struct {
	config func(io.Reader) (image.Config, error)
	decode func(io.Reader) (image.Image, error)
}

var byExtension = map[string]codec{
	".psd": {psdConfig, psdDecode},
	".tga": {tga.DecodeConfig, tga.Decode},
	".exr": {exrConfig, exrDecode},
	".hdr": {rgbe.DecodeConfig, hdrDecode},
}

// sniffed lists the formats decoded through image.Decode's registry.
var sniffed = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".bmp": true, ".tif": true, ".tiff": true,
}

// browserNative lists the formats a browser shows as they are.
var browserNative = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

// IsImage reports whether filename names a raster format Decode handles.
func IsImage(filename string) bool {
	ext := extension(filename)
	_, ok := byExtension[ext]
	return ok || sniffed[ext]
}

// NeedsConversion reports whether filename is an image a browser cannot show
// as it is, so it needs a rendered preview.
func NeedsConversion(filename string) bool {
	return IsImage(filename) && !browserNative[extension(filename)]
}

func extension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

// Decode reads an image and returns it with its format name. The format comes
// from filename's extension where the extension needs its own decoder, and is
// sniffed otherwise. The header is read first, so an image over MaxPixels
// returns ErrTooLarge without its pixels ever being allocated.
func Decode(r io.Reader, filename string) (image.Image, string, error) {
	c, named := byExtension[extension(filename)]
	if !named {
		c = codec{
			config: func(r io.Reader) (image.Config, error) {
				cfg, _, err := image.DecodeConfig(r)
				return cfg, err
			},
		}
	}

	// The header is read through a tee so the full decode can replay it.
	var head bytes.Buffer
	cfg, err := c.config(io.TeeReader(r, &head))
	if err != nil {
		return nil, "", fmt.Errorf("decode config: %w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, "", ErrTooLarge
	}
	full := io.MultiReader(&head, r)
	if !named {
		return image.Decode(full)
	}
	img, err := c.decode(full)
	return img, strings.TrimPrefix(extension(filename), "."), err
}

func psdConfig(r io.Reader) (image.Config, error) {
	cfg, _, err := psd.DecodeConfig(r)
	return image.Config{Width: cfg.Rect.Dx(), Height: cfg.Rect.Dy()}, err
}

// psdDecode returns a PSD's merged composite, the image as Photoshop last
// saved it flattened.
func psdDecode(r io.Reader) (image.Image, error) {
	p, _, err := psd.Decode(r, &psd.DecodeOptions{SkipLayerImage: true})
	if err != nil {
		return nil, err
	}
	if p.Picker == nil {
		return nil, errors.New("psd: no merged composite")
	}
	return p.Picker, nil
}

// exrConfig reads the whole file: EXR is read through a ReaderAt, and the
// data window that gives the dimensions sits in a header of variable length.
func exrConfig(r io.Reader) (image.Config, error) {
	f, err := openEXR(r)
	if err != nil {
		return image.Config{}, err
	}
	dw := f.Header(0).DataWindow()
	return image.Config{Width: int(dw.Max.X-dw.Min.X) + 1, Height: int(dw.Max.Y-dw.Min.Y) + 1}, nil
}

func openEXR(r io.Reader) (*exr.File, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return exr.OpenReader(bytes.NewReader(data), int64(len(data)))
}

func exrDecode(r io.Reader) (image.Image, error) {
	f, err := openEXR(r)
	if err != nil {
		return nil, err
	}
	in, err := exr.NewRGBAInputFile(f)
	if err != nil {
		return nil, err
	}
	img, err := in.ReadRGBA()
	if err != nil {
		return nil, err
	}
	return toneMapped(img.Rect, func(x, y int) (r, g, b, a float64) {
		fr, fg, fb, fa := img.RGBA(x, y)
		return float64(fr), float64(fg), float64(fb), float64(fa)
	}), nil
}

func hdrDecode(r io.Reader) (image.Image, error) {
	img, err := rgbe.Decode(r)
	if err != nil {
		return nil, err
	}
	h, ok := img.(hdr.Image)
	if !ok {
		return nil, errors.New("hdr: not a high-dynamic-range image")
	}
	return toneMapped(h.Bounds(), func(x, y int) (r, g, b, a float64) {
		return h.HDRAt(x, y).HDRRGBA()
	}), nil
}

// exposure scales linear light before tone mapping. It is fixed: a preview
// gives the user no exposure control.
const exposure = 1.0

// toneMapped renders linear high-dynamic-range light as an 8-bit image, so
// highlights roll off instead of clipping. at returns linear, premultiplied
// RGBA, as EXR stores it.
func toneMapped(bounds image.Rectangle, at func(x, y int) (r, g, b, a float64)) *image.NRGBA {
	out := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := at(x, y)
			a = min(max(a, 0), 1)
			if a > 0 {
				r, g, b = r/a, g/a, b/a
			}
			i := out.PixOffset(x, y)
			out.Pix[i+0] = toneMap(r)
			out.Pix[i+1] = toneMap(g)
			out.Pix[i+2] = toneMap(b)
			out.Pix[i+3] = uint8(math.Round(a * 255))
		}
	}
	return out
}

// toneMap maps one linear channel to 8 bits: the Reinhard operator v/(1+v),
// then a 2.2 display gamma.
func toneMap(v float64) uint8 {
	if !(v > 0) { // also catches NaN
		return 0
	}
	v *= exposure
	return uint8(math.Round(math.Pow(v/(1+v), 1/2.2) * 255))
}
