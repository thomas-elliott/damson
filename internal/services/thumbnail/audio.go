package thumbnail

import (
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

// renderAudio decodes the input to mono-float32 PCM, downsamples into
// `waveformBins` peak-amplitude bins, and renders a centred strip.
// Supported decoders: WAV (in-tree), MP3 (hajimehoshi/go-mp3), OGG Vorbis
// (jfreymuth/oggvorbis). Anything else returns a zero Result so the caller
// falls back to an icon.
func (g *Generator) renderAudio(ctx context.Context, in Input) (Result, error) {
	samples, err := decodePCM(in)
	if err != nil {
		return Result{}, fmt.Errorf("audio decode: %w", err)
	}
	if len(samples) == 0 {
		return Result{}, fmt.Errorf("audio decode: empty PCM")
	}
	peaks := binPeaks(samples, waveformBins)
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

// decodePCM picks a decoder by content type / extension and returns a
// mono-float32 PCM slice in [-1, 1]. Multi-channel inputs are collapsed by
// averaging across channels — waveform rendering only cares about
// amplitude envelope, not spatial information.
func decodePCM(in Input) ([]float32, error) {
	ext := strings.ToLower(filepath.Ext(in.Filename))
	ct := strings.ToLower(in.ContentType)

	switch {
	case ext == ".wav" || strings.Contains(ct, "wav"):
		return decodeWAV(in.Reader)
	case ext == ".mp3" || strings.Contains(ct, "mpeg"):
		return decodeMP3(in.Reader)
	case ext == ".ogg" || ext == ".oga" || strings.Contains(ct, "ogg") || strings.Contains(ct, "vorbis"):
		return decodeOGG(in.Reader)
	}
	return nil, fmt.Errorf("unsupported audio type: ext=%q ct=%q", ext, ct)
}

// decodeWAV parses RIFF/WAVE with PCM format 1 (int) and format 3 (float).
// We don't attempt to support compressed codecs embedded in WAV — those are
// rare in game audio and would pull in larger decoders.
func decodeWAV(r io.Reader) ([]float32, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(buf) < 44 {
		return nil, fmt.Errorf("wav too short")
	}
	if string(buf[0:4]) != "RIFF" || string(buf[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a RIFF/WAVE")
	}

	var (
		audioFormat   uint16
		numChannels   uint16
		bitsPerSample uint16
		dataOffset    int
		dataSize      int
	)

	// Walk chunks until we find `fmt ` and `data`. `fmt ` must precede
	// `data` for a normal WAV; if the file is badly ordered we give up.
	pos := 12
	haveFmt := false
	for pos+8 <= len(buf) {
		id := string(buf[pos : pos+4])
		sz := int(binary.LittleEndian.Uint32(buf[pos+4 : pos+8]))
		if pos+8+sz > len(buf) {
			return nil, fmt.Errorf("wav chunk overflows")
		}
		body := buf[pos+8 : pos+8+sz]
		switch id {
		case "fmt ":
			if len(body) < 16 {
				return nil, fmt.Errorf("wav fmt chunk too small")
			}
			audioFormat = binary.LittleEndian.Uint16(body[0:2])
			numChannels = binary.LittleEndian.Uint16(body[2:4])
			bitsPerSample = binary.LittleEndian.Uint16(body[14:16])
			haveFmt = true
		case "data":
			dataOffset = pos + 8
			dataSize = sz
		}
		pos += 8 + sz
		if sz%2 == 1 {
			pos++ // pad byte
		}
		if haveFmt && dataSize > 0 {
			break
		}
	}
	if !haveFmt || dataSize == 0 {
		return nil, fmt.Errorf("wav missing fmt or data chunk")
	}

	data := buf[dataOffset : dataOffset+dataSize]
	ch := int(numChannels)
	if ch == 0 {
		ch = 1
	}

	switch audioFormat {
	case 1: // PCM int
		switch bitsPerSample {
		case 8:
			return wavInt8ToFloat(data, ch), nil
		case 16:
			return wavInt16ToFloat(data, ch), nil
		case 24:
			return wavInt24ToFloat(data, ch), nil
		case 32:
			return wavInt32ToFloat(data, ch), nil
		}
	case 3: // IEEE float
		if bitsPerSample == 32 {
			return wavFloat32(data, ch), nil
		}
	}
	return nil, fmt.Errorf("wav format=%d bits=%d not supported", audioFormat, bitsPerSample)
}

func wavInt8ToFloat(data []byte, ch int) []float32 {
	// 8-bit WAV is unsigned [0,255] with 128 as silence.
	frames := len(data) / ch
	out := make([]float32, frames)
	for i := 0; i < frames; i++ {
		var sum int
		for c := 0; c < ch; c++ {
			sum += int(data[i*ch+c]) - 128
		}
		out[i] = float32(sum) / float32(ch) / 128
	}
	return out
}

func wavInt16ToFloat(data []byte, ch int) []float32 {
	stride := 2 * ch
	frames := len(data) / stride
	out := make([]float32, frames)
	for i := 0; i < frames; i++ {
		var sum int32
		for c := 0; c < ch; c++ {
			off := i*stride + c*2
			sum += int32(int16(binary.LittleEndian.Uint16(data[off : off+2])))
		}
		out[i] = float32(sum) / float32(ch) / 32768
	}
	return out
}

func wavInt24ToFloat(data []byte, ch int) []float32 {
	stride := 3 * ch
	frames := len(data) / stride
	out := make([]float32, frames)
	for i := 0; i < frames; i++ {
		var sum int64
		for c := 0; c < ch; c++ {
			off := i*stride + c*3
			// little-endian 24-bit signed
			v := int32(data[off]) | int32(data[off+1])<<8 | int32(data[off+2])<<16
			if v&0x800000 != 0 {
				v |= ^0xFFFFFF
			}
			sum += int64(v)
		}
		out[i] = float32(sum) / float32(ch) / 8388608
	}
	return out
}

func wavInt32ToFloat(data []byte, ch int) []float32 {
	stride := 4 * ch
	frames := len(data) / stride
	out := make([]float32, frames)
	for i := 0; i < frames; i++ {
		var sum int64
		for c := 0; c < ch; c++ {
			off := i*stride + c*4
			sum += int64(int32(binary.LittleEndian.Uint32(data[off : off+4])))
		}
		out[i] = float32(sum) / float32(ch) / 2147483648
	}
	return out
}

func wavFloat32(data []byte, ch int) []float32 {
	stride := 4 * ch
	frames := len(data) / stride
	out := make([]float32, frames)
	for i := 0; i < frames; i++ {
		var sum float32
		for c := 0; c < ch; c++ {
			off := i*stride + c*4
			bits := binary.LittleEndian.Uint32(data[off : off+4])
			sum += math.Float32frombits(bits)
		}
		out[i] = sum / float32(ch)
	}
	return out
}

func decodeMP3(r io.Reader) ([]float32, error) {
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		// go-mp3 wants a ReadSeeker; buffer the whole file. Audio imports
		// are capped by the upload limit, so this is bounded.
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		rs = bytes.NewReader(buf)
	}
	dec, err := gomp3.NewDecoder(rs)
	if err != nil {
		return nil, err
	}
	// go-mp3 emits 16-bit stereo, little-endian, interleaved.
	raw, err := io.ReadAll(dec)
	if err != nil {
		return nil, err
	}
	return wavInt16ToFloat(raw, 2), nil
}

func decodeOGG(r io.Reader) ([]float32, error) {
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		rs = bytes.NewReader(buf)
	}
	dec, err := oggvorbis.NewReader(rs)
	if err != nil {
		return nil, err
	}
	ch := dec.Channels()
	if ch <= 0 {
		ch = 1
	}
	// oggvorbis hands us float32 interleaved; average channels to mono.
	out := make([]float32, 0, 48000*8)
	tmp := make([]float32, 4096*ch)
	for {
		n, err := dec.Read(tmp)
		if n > 0 {
			frames := n / ch
			for i := 0; i < frames; i++ {
				var sum float32
				for c := 0; c < ch; c++ {
					sum += tmp[i*ch+c]
				}
				out = append(out, sum/float32(ch))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// binPeaks buckets samples into `bins` groups and returns the peak absolute
// amplitude per bucket. Chosen over RMS because peaks read better at
// thumbnail scale — the eye is drawn to the loudest moments, not the
// average.
func binPeaks(samples []float32, bins int) []float32 {
	if bins <= 0 || len(samples) == 0 {
		return nil
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
