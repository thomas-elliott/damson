package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/thomas-elliott/damson/internal/storage"
	"github.com/thomas-elliott/damson/internal/thumbnail"
)

// MaxZipCoverBytes caps the size of a zip entry considered for the cover.
const MaxZipCoverBytes = 16 << 20

type ZipExtractor struct {
	Storage storage.Service

	EntryMeta storage.ObjectMeta

	MaxEntrySize  int64
	MaxTotalBytes int64
	MaxEntries    int
}

func (e *ZipExtractor) Extract(ctx context.Context, r io.Reader) (*UnityResult, error) {
	if e.Storage == nil {
		return nil, errors.New("extract: storage not configured")
	}

	maxEntry := e.MaxEntrySize
	if maxEntry <= 0 {
		maxEntry = defaultMaxEntrySize
	}
	maxTotal := e.MaxTotalBytes
	if maxTotal <= 0 {
		maxTotal = defaultMaxTotalBytes
	}
	maxEntries := e.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}

	tmp, err := os.CreateTemp("", "damson-zip-*")
	if err != nil {
		return nil, fmt.Errorf("zip temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	size, err := io.Copy(tmp, r)
	if err != nil {
		return nil, fmt.Errorf("zip spool: %w", err)
	}

	zr, err := zip.NewReader(tmp, size)
	if err != nil {
		return nil, fmt.Errorf("zip open: %w", err)
	}

	if len(zr.File) > maxEntries {
		return nil, fmt.Errorf("extract: too many entries (>%d)", maxEntries)
	}

	files := make([]*zip.File, len(zr.File))
	copy(files, zr.File)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	var (
		result     UnityResult
		totalBytes int64
		covers     []*zip.File
		seen       = map[string]bool{}
	)

	for _, f := range files {
		if f.FileInfo().IsDir() || !f.Mode().IsRegular() {
			continue
		}
		rel, perr := cleanRelPath(f.Name)
		if perr != nil {
			continue
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		usize := int64(f.UncompressedSize64)
		if usize > maxEntry {
			return nil, fmt.Errorf("extract: entry %q exceeds per-entry cap", f.Name)
		}
		totalBytes += usize
		if totalBytes > maxTotal {
			return nil, fmt.Errorf("extract: total size exceeds cap (>%d bytes)", maxTotal)
		}

		rc, oerr := f.Open()
		if oerr != nil {
			return nil, fmt.Errorf("open entry %q: %w", f.Name, oerr)
		}
		key, wsize, serr := e.Storage.Put(ctx, storage.Blobs, io.LimitReader(rc, maxEntry+1), e.EntryMeta)
		rc.Close()
		if serr != nil {
			return nil, fmt.Errorf("put blob %s: %w: %w", f.Name, ErrStorage, serr)
		}
		// UncompressedSize64 above is only the declared size; the streamed
		// bytes are the real guard against a lying central directory (zip bomb).
		if wsize > maxEntry {
			return nil, fmt.Errorf("extract: entry %q exceeds per-entry cap", f.Name)
		}

		result.Files = append(result.Files, UnityFile{
			RelativePath: rel,
			SizeBytes:    wsize,
			ContentHash:  key,
			ObjectKey:    key,
		})

		if isImagePathname(rel) && usize > 0 && usize <= MaxZipCoverBytes {
			covers = append(covers, f)
		}
	}

	result.ThumbObjectKey, result.ThumbContentType = e.pickCover(ctx, covers)

	return &result, nil
}

func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return readCapped(rc, limit)
}

// pickCover renders the zip's cover from the first image entry that decodes,
// trying cover-like names first and archive order after that. It goes through
// the thumbnail generator, so the cover is resized and in a format a browser
// shows, like any other thumbnail. No entry decoding means no cover.
func (e *ZipExtractor) pickCover(ctx context.Context, candidates []*zip.File) (string, string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		return isCoverName(candidates[i].Name) && !isCoverName(candidates[j].Name)
	})
	gen := &thumbnail.Generator{Storage: e.Storage}
	for _, f := range candidates {
		data, err := readZipEntry(f, MaxZipCoverBytes)
		if err != nil {
			continue
		}
		res, err := gen.Generate(ctx, thumbnail.Input{Reader: bytes.NewReader(data), Filename: f.Name}, thumbnail.Toggles{Raster: true})
		if err == nil && res.ObjectKey != "" {
			return res.ObjectKey, res.ContentType
		}
		// Not a decodable image, or the write failed: try the next. A cover
		// is best-effort, like every thumbnail.
	}
	return "", ""
}

// isCoverName reports whether a file name reads like a cover image. A zip's
// import tries these first for its cover.
func isCoverName(name string) bool {
	base := strings.ToLower(path.Base(name))
	for _, word := range []string{"cover", "preview", "thumbnail"} {
		if strings.Contains(base, word) {
			return true
		}
	}
	return false
}
