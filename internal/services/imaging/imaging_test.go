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

// Every format's header is checked against MaxPixels before its pixels are
// allocated.
func TestDecode_RefusesHeadersOver100Megapixels(t *testing.T) {
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
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Decode(bytes.NewReader(data), name); !errors.Is(err, ErrTooLarge) {
				t.Fatalf("got %v, want ErrTooLarge", err)
			}
		})
	}
}
