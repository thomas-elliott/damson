package thumbnail

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/storage"
)

func newTestStorage(t *testing.T) storage.Service {
	t.Helper()
	s, err := storage.NewLocalFS(t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	return s
}

func TestGenerator_Raster_PNG(t *testing.T) {
	store := newTestStorage(t)
	g := &Generator{Storage: store}

	src := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 0x40, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	res, err := g.Generate(context.Background(), Input{
		Reader:      &buf,
		Filename:    "x.png",
		ContentType: "image/png",
	}, Toggles{Raster: true})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.ObjectKey == "" || res.Category != "raster" {
		t.Fatalf("expected raster result, got %+v", res)
	}
	if res.ContentType != "image/png" {
		t.Fatalf("expected PNG (alpha-tolerant), got %q", res.ContentType)
	}
	// Verify the stored thumb is decodable and downscaled.
	rc, err := store.Get(context.Background(), storage.Thumbs, res.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	img, _, err := image.Decode(rc)
	if err != nil {
		t.Fatalf("decode thumb: %v", err)
	}
	if img.Bounds().Dx() != 512 {
		t.Fatalf("expected 512px wide thumb, got %d", img.Bounds().Dx())
	}
}

func TestGenerator_Raster_JPEG_PicksJPEG(t *testing.T) {
	store := newTestStorage(t)
	g := &Generator{Storage: store}

	// Photographic-style JPEG → expect JPEG output.
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x80, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	res, err := g.Generate(context.Background(), Input{
		Reader:      &buf,
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
	}, Toggles{Raster: true})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.ContentType != "image/jpeg" {
		t.Fatalf("expected jpeg output, got %q", res.ContentType)
	}
}

func TestGenerator_TogglesOff(t *testing.T) {
	store := newTestStorage(t)
	g := &Generator{Storage: store}

	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	res, err := g.Generate(context.Background(), Input{
		Reader:      &buf,
		Filename:    "x.png",
		ContentType: "image/png",
	}, Toggles{}) // all toggles off
	if err != nil {
		t.Fatal(err)
	}
	if res.ObjectKey != "" {
		t.Fatalf("expected zero result with toggles off, got %+v", res)
	}
}

func TestGenerator_Audio_WAV(t *testing.T) {
	store := newTestStorage(t)
	g := &Generator{Storage: store}

	wav := makeSineWAV(44100, 440, 1.0)
	res, err := g.Generate(context.Background(), Input{
		Reader:      bytes.NewReader(wav),
		Filename:    "tone.wav",
		ContentType: "audio/wav",
	}, Toggles{Audio: true})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.Category != "audio" || res.ContentType != "image/png" {
		t.Fatalf("expected audio waveform PNG, got %+v", res)
	}
	rc, err := store.Get(context.Background(), storage.Thumbs, res.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	img, _, err := image.Decode(rc)
	if err != nil {
		t.Fatalf("decode waveform: %v", err)
	}
	if img.Bounds().Dx() != waveformWidth || img.Bounds().Dy() != waveformHeight {
		t.Fatalf("waveform dims wrong: %v", img.Bounds())
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"image/png":           "raster",
		"image/jpeg":          "raster",
		"audio/wav":           "audio",
		"audio/mpeg":          "audio",
		"font/ttf":            "font",
		"application/x-thing": "",
	}
	for ct, want := range cases {
		if got := classify(Input{ContentType: ct}); got != want {
			t.Errorf("classify(%q) = %q, want %q", ct, got, want)
		}
	}
	// Extension fallback when content type is empty.
	if got := classify(Input{Filename: "x.WAV"}); got != "audio" {
		t.Errorf("ext fallback wav: got %q", got)
	}
	if got := classify(Input{Filename: "x.OTF"}); got != "font" {
		t.Errorf("ext fallback otf: got %q", got)
	}
}

func TestGenerator_UnknownCategory(t *testing.T) {
	g := &Generator{Storage: newTestStorage(t)}
	res, err := g.Generate(context.Background(), Input{
		Reader:      strings.NewReader("hello"),
		Filename:    "notes.txt",
		ContentType: "text/plain",
	}, Toggles{Raster: true, Audio: true, Font: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.ObjectKey != "" {
		t.Fatalf("expected zero result for txt, got %+v", res)
	}
}

func TestGenerator_Raster_DecodeError(t *testing.T) {
	g := &Generator{Storage: newTestStorage(t)}
	_, err := g.Generate(context.Background(), Input{
		Reader:      strings.NewReader("not an image"),
		Filename:    "broken.png",
		ContentType: "image/png",
	}, Toggles{Raster: true})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

// makeSineWAV builds a 16-bit mono PCM WAV containing a sine wave at the
// supplied frequency for `seconds`. Used to feed the audio worker without
// shipping binary fixtures.
func makeSineWAV(sampleRate int, freq, seconds float64) []byte {
	frames := int(float64(sampleRate) * seconds)
	pcm := make([]byte, 0, frames*2)
	for i := 0; i < frames; i++ {
		t := float64(i) / float64(sampleRate)
		v := math.Sin(2 * math.Pi * freq * t)
		s := int16(v * 0x7fff)
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(s))
		pcm = append(pcm, b[:]...)
	}

	dataSize := uint32(len(pcm))
	chunkSize := 36 + dataSize

	var w bytes.Buffer
	w.WriteString("RIFF")
	_ = binary.Write(&w, binary.LittleEndian, chunkSize)
	w.WriteString("WAVE")
	w.WriteString("fmt ")
	_ = binary.Write(&w, binary.LittleEndian, uint32(16)) // PCM fmt size
	_ = binary.Write(&w, binary.LittleEndian, uint16(1))  // format = PCM
	_ = binary.Write(&w, binary.LittleEndian, uint16(1))  // mono
	_ = binary.Write(&w, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&w, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	_ = binary.Write(&w, binary.LittleEndian, uint16(2))            // block align
	_ = binary.Write(&w, binary.LittleEndian, uint16(16))           // bits per sample
	w.WriteString("data")
	_ = binary.Write(&w, binary.LittleEndian, dataSize)
	w.Write(pcm)
	return w.Bytes()
}

// Compile-time check that we satisfy io.Reader implicitly.
var _ io.Reader = (*bytes.Buffer)(nil)
