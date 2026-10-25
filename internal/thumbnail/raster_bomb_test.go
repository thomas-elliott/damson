package thumbnail

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

// bombPNG is a valid w×h greyscale PNG of zeros: a few hundred KiB on disk
// that decodes to w×h bytes. It is written row by row so the test itself
// never holds the canvas.
func bombPNG(t *testing.T, w, h uint32) []byte {
	t.Helper()
	var idat bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&idat, zlib.BestSpeed)
	row := make([]byte, 1+w) // filter byte, then the row
	for range h {
		if _, err := zw.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(kind string, data []byte) {
		_ = binary.Write(&out, binary.BigEndian, uint32(len(data)))
		body := append([]byte(kind), data...)
		out.Write(body)
		_ = binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(body))
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8] = 8 // bit depth; colour type 0 (greyscale) and the rest stay zero
	chunk("IHDR", ihdr)
	chunk("IDAT", idat.Bytes())
	chunk("IEND", nil)
	return out.Bytes()
}

// An image over 100 megapixels gets the placeholder, not a thumbnail, and is
// refused before its pixels are allocated.
func TestGenerator_Raster_RefusesImagesOver100Megapixels(t *testing.T) {
	g := &Generator{Storage: newTestStorage(t)}
	var res Result
	var err error
	grew := peakHeap(func() {
		res, err = g.Generate(context.Background(), Input{
			Reader: bytes.NewReader(bombPNG(t, 20_000, 20_000)), Filename: "huge.png", ContentType: "image/png",
		}, Toggles{Raster: true})
	})
	if err != nil || res.ObjectKey != "" {
		t.Fatalf("got %+v, %v; want no thumbnail and no error, so the placeholder shows", res, err)
	}
	if grew > 64<<20 {
		t.Fatalf("live heap rose %d MiB: the canvas was allocated", grew>>20)
	}
}
