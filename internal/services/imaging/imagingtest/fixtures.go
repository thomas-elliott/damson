// Package imagingtest builds small images in the formats imaging decodes, for
// tests in any package. Each is a solid colour, so a test can check what a
// decoder or tone map made of it.
package imagingtest

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"testing"

	"github.com/blezek/tga"
	"github.com/mdouchement/hdr"
	"github.com/mdouchement/hdr/codec/rgbe"
	"github.com/mdouchement/hdr/hdrcolor"
	"github.com/mrjoshuak/go-openexr/exr"
	"golang.org/x/image/tiff"
)

// Width and Height are every fixture's dimensions.
const Width, Height = 64, 48

// Red is the colour of the 8-bit fixtures.
var Red = color.RGBA{R: 255, A: 255}

// Linear is the linear-light value of every channel in the HDR fixtures
// (EXR, HDR): mid-grey, before tone mapping.
const Linear = 0.18

// Files returns one fixture per tier-2 format, keyed by file name.
func Files(t *testing.T) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"texture.psd": PSD(t, Width, Height),
		"texture.tga": TGA(t),
		"texture.tif": TIFF(t),
		"sky.exr":     EXR(t),
		"sky.hdr":     HDR(t),
	}
}

func solid() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(Red), image.Point{}, draw.Src)
	return img
}

func TGA(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := tga.Encode(&buf, solid()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TIFF(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, solid(), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// PSD hand-writes a flat 8-bit RGB Photoshop file of w×h red pixels: the
// header, three empty sections, and raw (uncompressed) channel planes. It
// writes only the header when w×h is too large to hold, which is all a
// pixel-ceiling check reads.
func PSD(t *testing.T, w, h uint32) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("8BPS")
	be := func(v any) { _ = binary.Write(&b, binary.BigEndian, v) }
	be(uint16(1)) // version
	b.Write(make([]byte, 6))
	be(uint16(3)) // channels
	be(h)
	be(w)
	be(uint16(8)) // bits per channel
	be(uint16(3)) // colour mode: RGB
	be(uint32(0)) // colour mode data
	be(uint32(0)) // image resources
	be(uint32(0)) // layer and mask information
	be(uint16(0)) // image data: raw
	if uint64(w)*uint64(h) > 1<<20 {
		return b.Bytes()
	}
	plane := int(w * h)
	b.Write(bytes.Repeat([]byte{255}, plane)) // R
	b.Write(make([]byte, 2*plane))            // G, B
	return b.Bytes()
}

func EXR(t *testing.T) []byte {
	t.Helper()
	img := exr.NewRGBAImage(image.Rect(0, 0, Width, Height))
	for y := range Height {
		for x := range Width {
			img.SetRGBA(x, y, Linear, Linear, Linear, 1)
		}
	}
	// exr.Encode needs a seeker, so it writes to a file.
	path := filepath.Join(t.TempDir(), "fixture.exr")
	if err := exr.EncodeFile(path, img); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func HDR(t *testing.T) []byte {
	t.Helper()
	img := hdr.NewRGB(image.Rect(0, 0, Width, Height))
	for y := range Height {
		for x := range Width {
			img.SetRGB(x, y, hdrcolor.RGB{R: Linear, G: Linear, B: Linear})
		}
	}
	var buf bytes.Buffer
	if err := rgbe.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
