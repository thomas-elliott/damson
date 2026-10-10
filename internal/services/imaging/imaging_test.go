package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/thomas-elliott/damson/internal/services/imaging/imagingtest"
)

func TestDecode_Tier2Formats(t *testing.T) {
	for name, data := range imagingtest.Files(t) {
		t.Run(name, func(t *testing.T) {
			img, _, err := Decode(bytes.NewReader(data), name)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if b := img.Bounds(); b.Dx() != imagingtest.Width || b.Dy() != imagingtest.Height {
				t.Fatalf("bounds %v", b)
			}
			r, g, b, _ := img.At(3, 3).RGBA()
			r, g, b = r>>8, g>>8, b>>8
			switch name {
			case "sky.exr", "sky.hdr":
				// Linear mid-grey must come out a visible mid-grey: neither
				// crushed to black nor clipped to white.
				if r != g || g != b || r < 64 || r > 192 {
					t.Fatalf("tone-mapped mid-grey came out (%d, %d, %d)", r, g, b)
				}
			default:
				if r != 255 || g != 0 || b != 0 {
					t.Fatalf("colour (%d, %d, %d), want red", r, g, b)
				}
			}
		})
	}
}

// Every format's header is checked against its ceiling before its pixels are
// allocated: MaxPixels, or less for a format that decodes to float pixels.
func TestDecode_RefusesHeadersOverTheCeiling(t *testing.T) {
	// An 18-byte header, then room for the 26-byte footer TGA readers look for.
	tgaHeader := make([]byte, 18+26)
	tgaHeader[2] = 2 // uncompressed true-colour
	binary.LittleEndian.PutUint16(tgaHeader[12:], 20_000)
	binary.LittleEndian.PutUint16(tgaHeader[14:], 20_000)
	tgaHeader[16] = 24

	for name, data := range map[string][]byte{
		"huge.psd": imagingtest.PSD(t, 20_000, 20_000),
		"huge.tga": tgaHeader,
		"huge.hdr": []byte("#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y 20000 +X 20000\n"),
		// 36 megapixels: under MaxPixels, over the float formats' ceilings.
		"big.hdr": []byte("#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y 6000 +X 6000\n"),
		"big.exr": exrDeclaring(t, 6000, 6000),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Decode(bytes.NewReader(data), name); !errors.Is(err, ErrTooLarge) {
				t.Fatalf("got %v, want ErrTooLarge", err)
			}
		})
	}
}

// exrDeclaring returns the EXR fixture with its data window widened to w×h,
// padded so the file can hold the chunk table that window implies.
func exrDeclaring(t *testing.T, w, h uint32) []byte {
	t.Helper()
	data := imagingtest.EXR(t)
	attr := []byte("dataWindow\x00box2i\x00")
	i := bytes.Index(data, attr)
	if i < 0 {
		t.Fatal("fixture has no dataWindow")
	}
	box := i + len(attr) + 4 // past the attribute's size
	binary.LittleEndian.PutUint32(data[box+8:], w-1)
	binary.LittleEndian.PutUint32(data[box+12:], h-1)
	return append(data, make([]byte, 8*int(h))...)
}
