package thumbnail

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"path/filepath"
	"strings"

	gomp3 "github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
)

// Waveform image dimensions — 512x128 fills the thumbnail cell without
// looking cramped; the aspect ratio differs from raster thumbs on purpose
// so the waveform reads as an audio preview.
const (
	waveformWidth  = 512
	waveformHeight = 128
	waveformBins   = waveformWidth
)

// renderAudio streams the input through a decoder into a fixed-size peak
// envelope and renders it as a centred strip. Memory is bounded by the
// waveform's width, not the file's length, so an hour of audio costs what a
// second does. Supported decoders: WAV (in-tree), MP3 (hajimehoshi/go-mp3),
// OGG Vorbis (jfreymuth/oggvorbis). Anything else returns an error so the
// caller falls back to an icon.
func (g *Generator) renderAudio(ctx context.Context, in Input) (Result, error) {
	env := &envelope{block: 1}
	if err := decodePCM(in, env.add); err != nil {
		return Result{}, fmt.Errorf("audio decode: %w", err)
	}
	peaks := env.bins()
	if len(peaks) == 0 {
		return Result{}, fmt.Errorf("audio decode: empty PCM")
	}
	img := drawWaveform(peaks)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return Result{}, fmt.Errorf("waveform encode: %w", err)
	}
	key, err := g.putThumbBytes(ctx, buf.Bytes())
	if err != nil {
		return Result{}, err
	}
	return Result{ObjectKey: key, ContentType: "image/png", Category: "audio"}, nil
}

// envelope keeps a fixed-size peak envelope of a sample stream of unknown
// length: the peak of each block of samples, with neighbouring peaks merged
// pairwise, and the block doubled, whenever there are twice as many as the
// waveform has bins.
type envelope struct {
	peaks []float32
	block int // samples per peak
	cur   float32
	n     int // samples in the current block
}

func (e *envelope) add(s float32) {
	e.cur = max(e.cur, float32(math.Abs(float64(s))))
	if e.n++; e.n < e.block {
		return
	}
	e.peaks = append(e.peaks, e.cur)
	e.cur, e.n = 0, 0
	if len(e.peaks) == 2*waveformBins {
		for i := range waveformBins {
			e.peaks[i] = max(e.peaks[2*i], e.peaks[2*i+1])
		}
		e.peaks = e.peaks[:waveformBins]
		e.block *= 2
	}
}

// bins returns the envelope as at most waveformBins peaks.
func (e *envelope) bins() []float32 {
	if e.n > 0 {
		e.peaks = append(e.peaks, e.cur)
		e.cur, e.n = 0, 0
	}
	return binPeaks(e.peaks, waveformBins)
}

// decodePCM picks a decoder by content type / extension and streams mono
// samples in [-1, 1] to add. Multi-channel inputs are collapsed by averaging
// across channels — waveform rendering only cares about amplitude envelope,
// not spatial information.
func decodePCM(in Input, add func(float32)) error {
	ext := strings.ToLower(filepath.Ext(in.Filename))
	ct := strings.ToLower(in.ContentType)

	switch {
	case ext == ".wav" || strings.Contains(ct, "wav"):
		return decodeWAV(in.Reader, add)
	case ext == ".mp3" || strings.Contains(ct, "mpeg"):
		return decodeMP3(in.Reader, add)
	case ext == ".ogg" || ext == ".oga" || strings.Contains(ct, "ogg") || strings.Contains(ct, "vorbis"):
		return decodeOGG(in.Reader, add)
	}
	return fmt.Errorf("unsupported audio type: ext=%q ct=%q", ext, ct)
}

// maxWAVFmtChunk bounds the one chunk decodeWAV reads whole; a real `fmt `
// chunk is 16 to 40 bytes.
const maxWAVFmtChunk = 1 << 10

// decodeWAV parses RIFF/WAVE with PCM format 1 (int) and format 3 (float),
// streaming the data chunk. We don't attempt to support compressed codecs
// embedded in WAV — those are rare in game audio and would pull in larger
// decoders. `fmt ` must precede `data`, as it does in a normal WAV.
func decodeWAV(r io.Reader, add func(float32)) error {
	br := bufio.NewReader(r)
	var riff [12]byte
	if _, err := io.ReadFull(br, riff[:]); err != nil {
		return fmt.Errorf("wav too short")
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return fmt.Errorf("not a RIFF/WAVE")
	}

	var format, channels, bits uint16
	haveFmt := false
	for {
		var hdr [8]byte
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			return fmt.Errorf("wav missing fmt or data chunk")
		}
		id := string(hdr[0:4])
		size := int64(binary.LittleEndian.Uint32(hdr[4:8]))
		switch {
		case id == "fmt ":
			if size < 16 || size > maxWAVFmtChunk {
				return fmt.Errorf("wav fmt chunk size %d", size)
			}
			body := make([]byte, size)
			if _, err := io.ReadFull(br, body); err != nil {
				return fmt.Errorf("wav fmt chunk: %w", err)
			}
			format = binary.LittleEndian.Uint16(body[0:2])
			channels = binary.LittleEndian.Uint16(body[2:4])
			bits = binary.LittleEndian.Uint16(body[14:16])
			haveFmt = true
		case id == "data" && haveFmt:
			return streamPCM(io.LimitReader(br, size), format, bits, int(max(channels, 1)), add)
		default:
			if _, err := io.CopyN(io.Discard, br, size); err != nil {
				return fmt.Errorf("wav chunk overflows")
			}
		}
		if size%2 == 1 {
			_, _ = br.Discard(1) // pad byte
		}
	}
}

// streamPCM reads interleaved PCM frames from r and passes each frame's
// channel average to add.
func streamPCM(r io.Reader, format, bits uint16, channels int, add func(float32)) error {
	sample, ok := pcmSampleDecoder(format, bits)
	if !ok {
		return fmt.Errorf("wav format=%d bits=%d not supported", format, bits)
	}
	width := int(bits / 8)
	frame := width * channels
	buf := make([]byte, frame*4096)
	for {
		n, err := io.ReadFull(r, buf)
		for off := 0; off+frame <= n; off += frame {
			var sum float32
			for c := range channels {
				sum += sample(buf[off+c*width:])
			}
			add(sum / float32(channels))
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// pcmSampleDecoder returns the function that reads one little-endian sample
// of the given WAV format and bit depth as a float in [-1, 1].
func pcmSampleDecoder(format, bits uint16) (func([]byte) float32, bool) {
	switch {
	case format == 1 && bits == 8:
		// 8-bit WAV is unsigned [0,255] with 128 as silence.
		return func(b []byte) float32 { return float32(int(b[0])-128) / 128 }, true
	case format == 1 && bits == 16:
		return func(b []byte) float32 { return float32(int16(binary.LittleEndian.Uint16(b))) / 32768 }, true
	case format == 1 && bits == 24:
		return func(b []byte) float32 {
			v := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16
			if v&0x800000 != 0 {
				v |= ^0xFFFFFF
			}
			return float32(v) / 8388608
		}, true
	case format == 1 && bits == 32:
		return func(b []byte) float32 { return float32(int32(binary.LittleEndian.Uint32(b))) / 2147483648 }, true
	case format == 3 && bits == 32:
		return func(b []byte) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b)) }, true
	}
	return nil, false
}

// decodeMP3 streams the decoder's output, which go-mp3 always emits as
// 16-bit little-endian interleaved stereo. A reader that is not a seeker is
// read straight through, never buffered.
func decodeMP3(r io.Reader, add func(float32)) error {
	dec, err := gomp3.NewDecoder(r)
	if err != nil {
		return err
	}
	return streamPCM(dec, 1, 16, 2, add)
}

func decodeOGG(r io.Reader, add func(float32)) error {
	dec, err := oggvorbis.NewReader(r)
	if err != nil {
		return err
	}
	ch := max(dec.Channels(), 1)
	// oggvorbis hands us float32 interleaved; average channels to mono.
	tmp := make([]float32, 4096*ch)
	for {
		n, err := dec.Read(tmp)
		for i := 0; i+ch <= n; i += ch {
			var sum float32
			for c := range ch {
				sum += tmp[i+c]
			}
			add(sum / float32(ch))
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// binPeaks buckets samples into `bins` groups and returns the peak absolute
// amplitude per bucket. Chosen over RMS because peaks read better at
// thumbnail scale — the eye is drawn to the loudest moments, not the
// average.
func binPeaks(samples []float32, bins int) []float32 {
	if bins <= 0 || len(samples) == 0 {
		return nil
	}
	if len(samples) <= bins {
		// Too short to bin: one column per sample.
		bins = len(samples)
	}
	out := make([]float32, bins)
	per := float64(len(samples)) / float64(bins)
	if per < 1 {
		per = 1
	}
	for i := 0; i < bins; i++ {
		start := int(float64(i) * per)
		end := int(float64(i+1) * per)
		if end > len(samples) {
			end = len(samples)
		}
		var peak float32
		for _, s := range samples[start:end] {
			v := s
			if v < 0 {
				v = -v
			}
			if v > peak {
				peak = v
			}
		}
		if peak > 1 {
			peak = 1
		}
		out[i] = peak
	}
	return out
}

// drawWaveform renders peaks as a centred amplitude strip. Colour palette:
// dark background + accent-leaning warm yellow, matching the design-system
// tokens without hard-coding CSS (we can't query them from Go).
func drawWaveform(peaks []float32) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, waveformWidth, waveformHeight))
	bg := color.RGBA{R: 0x16, G: 0x16, B: 0x18, A: 0xff}
	fg := color.RGBA{R: 0xe0, G: 0xb8, B: 0x38, A: 0xff}
	// fill background
	for y := 0; y < waveformHeight; y++ {
		for x := 0; x < waveformWidth; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	mid := waveformHeight / 2
	for x, p := range peaks {
		h := int(float32(mid) * p)
		if h < 1 {
			h = 1
		}
		for y := mid - h; y < mid+h; y++ {
			img.SetRGBA(x, y, fg)
		}
	}
	return img
}
