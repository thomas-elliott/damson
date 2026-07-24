package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/thomas-elliott/damson/internal/services/storage"
)

const maxZipCoverBytes = 16 << 20

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
		coverPick  *zip.File
		coverPath  string
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
		key, wsize, serr := e.Storage.PutBlob(ctx, io.LimitReader(rc, maxEntry+1), e.EntryMeta)
		rc.Close()
		if serr != nil {
			return nil, fmt.Errorf("put blob %s: %w", f.Name, serr)
		}
		if wsize > maxEntry {
			return nil, fmt.Errorf("extract: entry %q exceeds per-entry cap", f.Name)
		}

		result.Files = append(result.Files, UnityFile{
			RelativePath: rel,
			SizeBytes:    wsize,
			ContentHash:  key,
			ObjectKey:    key,
		})

		if coverPick == nil && isImagePathname(rel) && usize > 0 && usize <= maxZipCoverBytes {
			coverPick = f
			coverPath = rel
		}
	}

	if coverPick != nil {
		if data, rerr := readZipEntry(coverPick, maxZipCoverBytes); rerr == nil {
			sum := sha256.Sum256(data)
			key := hex.EncodeToString(sum[:])
			if perr := e.Storage.PutThumb(ctx, key, bytes.NewReader(data)); perr != nil {
				return nil, fmt.Errorf("put thumb: %w", perr)
			}
			result.ThumbObjectKey = key
			result.ThumbContentType = coverContentType(coverPath)
		}
	}

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

func coverContentType(p string) string {
	if ct := mime.TypeByExtension(strings.ToLower(path.Ext(p))); ct != "" {
		if i := strings.IndexByte(ct, ';'); i >= 0 {
			ct = ct[:i]
		}
		return strings.TrimSpace(ct)
	}
	return "application/octet-stream"
}
