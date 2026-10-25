// Package thumbnail generates preview images for imported assets. One worker
// per input category:
//
//   - Raster (image/*, or any format imaging decodes): decode + resample to
//     512px long edge + re-encode as JPEG (photographic) or PNG
//     (palette/alpha/rendered).
//   - Audio (audio/*): decode PCM, downsample to waveform bins, render a
//     waveform strip as PNG.
//   - Font (font/* or .ttf/.otf): render a sample string as PNG.
//
// The .unitypackage preview-harvest path lives in internal/extract — it fires
// before this package runs and sets `thumbnail_object_key` directly from the
// embedded preview.png.
//
// Failures are not fatal to the enclosing import: callers log at warn and
// leave `thumbnail_object_key` unset; the SPA falls back to a file-type icon.
package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/thomas-elliott/damson/internal/imaging"
	"github.com/thomas-elliott/damson/internal/storage"
)

// Result is what Generate returns on success. A zero Result means the input
// wasn't a category we handle; callers should treat that as "no thumbnail"
// rather than an error.
type Result struct {
	ObjectKey   string
	ContentType string
	Category    string // "raster" | "audio" | "font"
}

// Input carries what the workers need about the source file. The reader is
// consumed by the matching worker; callers are responsible for supplying a
// fresh stream (e.g. by re-fetching from storage).
type Input struct {
	Reader      io.Reader
	Filename    string
	ContentType string // best-effort mime from upload; may be empty
}

// Toggles mirror the per-category switches on library_settings. When a
// toggle is false the matching worker short-circuits and Generate returns a
// zero Result.
type Toggles struct {
	Raster bool
	Audio  bool
	Font   bool
}

// Generator renders a thumbnail and stores it via the supplied storage
// service. Construct per-import (it's a thin struct; share if you need to).
type Generator struct {
	Storage storage.Service
}

// Generate picks a worker based on content type / filename extension and
// runs it. Returns a zero Result for categories we don't cover or when the
// matching toggle is off — callers treat that as a no-op, not an error.
func (g *Generator) Generate(ctx context.Context, in Input, t Toggles) (Result, error) {
	if g.Storage == nil {
		return Result{}, fmt.Errorf("thumbnail: storage not configured")
	}
	cat := classify(in)
	switch cat {
	case "raster":
		if !t.Raster {
			return Result{}, nil
		}
		return g.renderRaster(ctx, in)
	case "audio":
		if !t.Audio {
			return Result{}, nil
		}
		return g.renderAudio(ctx, in)
	case "font":
		if !t.Font {
			return Result{}, nil
		}
		return g.renderFont(ctx, in)
	}
	return Result{}, nil
}

// classify returns one of "raster" | "audio" | "font" | "" for the input.
// Content type wins when present and recognisable; filename extension is a
// fallback for uploads that arrived without a type.
func classify(in Input) string {
	ct := strings.ToLower(strings.TrimSpace(in.ContentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch {
	case strings.HasPrefix(ct, "image/"):
		return "raster"
	case strings.HasPrefix(ct, "audio/"):
		return "audio"
	case strings.HasPrefix(ct, "font/") || ct == "application/font-sfnt" ||
		ct == "application/x-font-ttf" || ct == "application/x-font-otf":
		return "font"
	}
	if imaging.IsImage(in.Filename) {
		return "raster"
	}
	ext := strings.ToLower(filepath.Ext(in.Filename))
	switch ext {
	case ".wav", ".mp3", ".ogg", ".oga":
		return "audio"
	case ".ttf", ".otf":
		return "font"
	}
	return ""
}

// putThumbBytes writes thumb bytes to storage with a content-addressed key
// so retries + duplicates collapse onto the same object. Keeps the serving
// handler's long-cache headers safe.
func (g *Generator) putThumbBytes(ctx context.Context, b []byte) (string, error) {
	key, _, err := g.Storage.Put(ctx, storage.Thumbs, bytes.NewReader(b), storage.ObjectMeta{})
	if err != nil {
		return "", fmt.Errorf("put thumb: %w", err)
	}
	return key, nil
}
