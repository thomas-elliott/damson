// Package imaging decodes the raster images Damson renders, for both the grid
// thumbnail and the per-file preview, so the two agree on which files are
// images. Every decode is refused above MaxPixels, judged from the image's
// header before its pixels are allocated.
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

	_ "golang.org/x/image/bmp" // bmp decode registration
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// MaxPixels is the decompression-bomb ceiling: an image declaring more pixels
// than this is not decoded, and gets the placeholder.
const MaxPixels = 100 * 1000 * 1000

// ErrTooLarge reports an image over MaxPixels.
var ErrTooLarge = errors.New("image exceeds the pixel ceiling")

// Decode reads an image and returns it with its format name. The header is
// read first, so an image over MaxPixels returns ErrTooLarge without its
// pixels ever being allocated.
func Decode(r io.Reader) (image.Image, string, error) {
	// The header is read through a tee so the full decode can replay it.
	var head bytes.Buffer
	cfg, _, err := image.DecodeConfig(io.TeeReader(r, &head))
	if err != nil {
		return nil, "", fmt.Errorf("decode config: %w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, "", ErrTooLarge
	}
	return image.Decode(io.MultiReader(&head, r))
}
