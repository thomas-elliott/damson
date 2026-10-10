package thumbnail

import (
	"context"
	"encoding/binary"
	"io"
	"runtime"
	"testing"
	"time"
)

// patternReader yields n bytes of a repeating pattern without holding them,
// so a test can feed a very large file through a decoder cheaply.
type patternReader struct {
	pattern []byte
	n, off  int64
}

func (p *patternReader) Read(b []byte) (int, error) {
	if p.off >= p.n {
		return 0, io.EOF
	}
	if rem := p.n - p.off; int64(len(b)) > rem {
		b = b[:rem]
	}
	for i := range b {
		b[i] = p.pattern[(p.off+int64(i))%int64(len(p.pattern))]
	}
	p.off += int64(len(b))
	return len(b), nil
}

// largeWAV streams a 16-bit stereo WAV of size bytes of a square wave.
func largeWAV(size int64) io.Reader { return wavWithChannels(2, size) }

// wavWithChannels streams a 16-bit PCM WAV header claiming channels,
// followed by size bytes of a square wave.
func wavWithChannels(channels uint16, size int64) io.Reader {
	hdr := make([]byte, 44)
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+size))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:], channels)
	binary.LittleEndian.PutUint32(hdr[24:], 44100)
	binary.LittleEndian.PutUint32(hdr[28:], 44100*4)
	binary.LittleEndian.PutUint16(hdr[32:], 4)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(size))
	square := []byte{0x00, 0x40, 0x00, 0x40, 0x00, 0xc0, 0x00, 0xc0}
	return io.MultiReader(bytesReader(hdr), &patternReader{pattern: square, n: size})
}

// largeMP3 streams n silent MPEG-1 Layer III frames (128 kbps, 44.1 kHz):
// a frame header and zeroed side info and main data, which decodes to
// silence.
func largeMP3(frames int64) io.Reader {
	frame := make([]byte, 417)
	copy(frame, []byte{0xff, 0xfb, 0x90, 0x00})
	return &patternReader{pattern: frame, n: frames * int64(len(frame))}
}

func bytesReader(b []byte) io.Reader { return &patternReader{pattern: b, n: int64(len(b))} }

// peakHeap runs fn while sampling the live heap, and returns how far it rose
// above where it started.
func peakHeap(fn func()) uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	base, peak := ms.HeapAlloc, ms.HeapAlloc
	done := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		var m runtime.MemStats
		for {
			select {
			case <-done:
				return
			case <-time.After(2 * time.Millisecond):
				runtime.ReadMemStats(&m)
				peak = max(peak, m.HeapAlloc)
			}
		}
	}()
	fn()
	close(done)
	<-sampled
	return peak - base
}

// A long audio file is rendered without holding its samples: the live memory
// a waveform takes must not grow with the file. Before streaming, a 256 MiB
// WAV held over 700 MiB at once.
func TestGenerator_Audio_LargeFilesStayWithinMemory(t *testing.T) {
	const budget = 64 << 20
	cases := []struct {
		name string
		in   func() io.Reader
		file string
	}{
		{"256 MiB WAV", func() io.Reader { return largeWAV(256 << 20) }, "long.wav"},
		{"3 minutes of MP3", func() io.Reader { return largeMP3(7_500) }, "long.mp3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &Generator{Storage: newTestStorage(t)}
			var res Result
			var err error
			grew := peakHeap(func() {
				res, err = g.Generate(context.Background(), Input{Reader: tc.in(), Filename: tc.file}, Toggles{Audio: true})
			})
			if err != nil || res.ObjectKey == "" {
				t.Fatalf("no waveform: %+v, %v", res, err)
			}
			if grew > budget {
				t.Fatalf("live heap rose %d MiB while rendering, budget %d MiB", grew>>20, budget>>20)
			}
		})
	}
}

// The read buffer is sized from the header's channel count, which the
// uploader controls: a tiny WAV claiming 65535 channels must not make the
// worker reserve a gigabyte.
func TestGenerator_Audio_AHeaderClaimingHugeChannelsStaysWithinMemory(t *testing.T) {
	const budget = 64 << 20
	g := &Generator{Storage: newTestStorage(t)}
	// Total allocated, not the live heap: the buffer is short-lived, so a
	// sampler can miss it.
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, _ = g.Generate(context.Background(), Input{Reader: wavWithChannels(65535, 8), Filename: "bomb.wav"}, Toggles{Audio: true})
	runtime.ReadMemStats(&after)
	if grew := after.TotalAlloc - before.TotalAlloc; grew > budget {
		t.Fatalf("allocated %d MiB, budget %d MiB", grew>>20, budget>>20)
	}
}
