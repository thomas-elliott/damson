// Package extract handles reading archive formats into flat file manifests
// suitable for storage as asset_files. The first entry is .unitypackage;
// .zip support lives alongside it when added.
//
// See docs/technical-design.md §9.2 for the format description.
package extract

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/thomas-elliott/damson/internal/services/storage"
)

// UnityFile is one extracted file ready to be written as an asset_files row.
// ObjectKey == ContentHash (SHA-256 hex) — content-addressed.
type UnityFile struct {
	GUID         string
	RelativePath string
	SizeBytes    int64
	ContentHash  string
	ObjectKey    string
}

// UnityResult is the outcome of unitypackage extraction.
type UnityResult struct {
	Files            []UnityFile
	ThumbObjectKey   string // sha256 key of the first harvested preview.png; "" if none
	ThumbContentType string // "image/png" when a preview was harvested
}

// UnityExtractor extracts .unitypackage archives.
type UnityExtractor struct {
	Storage storage.Service

	// MaxEntrySize caps the uncompressed size of any single tar entry.
	// A zero value uses the default.
	MaxEntrySize int64

	// MaxTotalBytes caps the sum of all extracted `asset` entry sizes.
	MaxTotalBytes int64

	// MaxEntries caps the total number of tar entries processed.
	MaxEntries int
}

// Defaults tuned to cover large real-world packages without making zip-bomb
// mitigation toothless. Callers can override per-extractor.
const (
	defaultMaxEntrySize  = 2 << 30 // 2 GiB per file
	defaultMaxTotalBytes = 8 << 30 // 8 GiB per package
	defaultMaxEntries    = 200_000
	previewContentType   = "image/png"
)

var guidPrefix = regexp.MustCompile(`^([0-9a-f]{32})(?:/|$)`)

// Extract streams r (a .unitypackage — either a gzipped tar or a plain tar)
// and produces a UnityResult. The format is detected from the first two
// bytes: `1f 8b` is gzip; anything else is handed straight to tar.NewReader,
// which covers the rarer raw-tar packages that appear in the wild (Unity
// Asset Store packages from older exports, some third-party tooling).
//
// Files whose GUID bundle is incomplete (missing pathname or asset) are
// skipped rather than erroring — real packages occasionally carry meta-only
// stubs, and the UI surfaces the discrepancy downstream if it matters.
//
// Extraction is streaming in intent but not fully streaming in implementation:
// `asset` entries are streamed straight to storage, but small sidecar files
// (pathname, preview.png) are buffered per-GUID until the bundle is complete.
// Per-entry and total-size caps keep this safe against archive bombs.
func (e *UnityExtractor) Extract(ctx context.Context, r io.Reader) (*UnityResult, error) {
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

	br := bufio.NewReader(r)
	head, err := br.Peek(2)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("peek header: %w", err)
	}

	var tarSrc io.Reader = br
	if len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b {
		gzr, gerr := gzip.NewReader(br)
		if gerr != nil {
			return nil, fmt.Errorf("gzip: %w", gerr)
		}
		defer gzr.Close()
		tarSrc = gzr
	}

	tr := tar.NewReader(tarSrc)

	type bundle struct {
		guid        string
		pathname    string
		contentHash string
		objectKey   string
		sizeBytes   int64
		haveContent bool
	}
	bundles := map[string]*bundle{}
	get := func(guid string) *bundle {
		b, ok := bundles[guid]
		if !ok {
			b = &bundle{guid: guid}
			bundles[guid] = b
		}
		return b
	}

	var (
		result      UnityResult
		totalBytes  int64
		entries     int
		previewSeen bool
	)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar next: %w", err)
		}

		entries++
		if entries > maxEntries {
			return nil, fmt.Errorf("extract: too many entries (>%d)", maxEntries)
		}

		// Only plain files carry content. Directory / symlink / etc. entries
		// are ignored — Unity packages do include explicit dir entries.
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		guid, leaf := splitUnityEntry(hdr.Name)
		if guid == "" {
			continue
		}

		switch leaf {
		case "asset":
			if hdr.Size > maxEntry {
				return nil, fmt.Errorf("extract: entry %q exceeds per-entry cap", hdr.Name)
			}
			totalBytes += hdr.Size
			if totalBytes > maxTotal {
				return nil, fmt.Errorf("extract: total size exceeds cap (>%d bytes)", maxTotal)
			}
			key, size, perr := e.Storage.PutBlob(ctx, io.LimitReader(tr, maxEntry+1))
			if perr != nil {
				return nil, fmt.Errorf("put blob %s: %w", hdr.Name, perr)
			}
			if size > maxEntry {
				return nil, fmt.Errorf("extract: entry %q exceeds per-entry cap", hdr.Name)
			}
			b := get(guid)
			b.objectKey = key
			b.contentHash = key
			b.sizeBytes = size
			b.haveContent = true

		case "pathname":
			// Small text file. Read with a modest cap.
			data, rerr := readCapped(tr, 8<<10) // 8 KiB is plenty; Unity paths are short
			if rerr != nil {
				return nil, fmt.Errorf("read pathname %s: %w", hdr.Name, rerr)
			}
			p, perr := cleanUnityPath(string(data))
			if perr != nil {
				// Unsafe path — skip the bundle silently; other entries with
				// the same GUID may be safe but without a legal pathname we
				// can't place the file anyway.
				continue
			}
			get(guid).pathname = p

		case "preview.png":
			if previewSeen {
				// First preview wins; drain & continue.
				if _, derr := io.Copy(io.Discard, io.LimitReader(tr, 1<<20)); derr != nil {
					return nil, fmt.Errorf("drain preview: %w", derr)
				}
				continue
			}
			data, rerr := readCapped(tr, 1<<20) // 1 MiB cap for a Unity preview
			if rerr != nil {
				return nil, fmt.Errorf("read preview %s: %w", hdr.Name, rerr)
			}
			sum := sha256.Sum256(data)
			key := hex.EncodeToString(sum[:])
			if perr := e.Storage.PutThumb(ctx, key, bytes.NewReader(data)); perr != nil {
				return nil, fmt.Errorf("put thumb: %w", perr)
			}
			result.ThumbObjectKey = key
			result.ThumbContentType = previewContentType
			previewSeen = true

		default:
			// `asset.meta` and anything else is ignored for v1.
		}
	}

	// Assemble files from bundles that have both a pathname and content.
	// Ordering is stable (by guid) so tests and audit diffs are deterministic.
	guids := make([]string, 0, len(bundles))
	for g := range bundles {
		guids = append(guids, g)
	}
	sort.Strings(guids)
	for _, g := range guids {
		b := bundles[g]
		if !b.haveContent || b.pathname == "" {
			continue
		}
		result.Files = append(result.Files, UnityFile{
			GUID:         b.guid,
			RelativePath: b.pathname,
			SizeBytes:    b.sizeBytes,
			ContentHash:  b.contentHash,
			ObjectKey:    b.objectKey,
		})
	}
	return &result, nil
}

// splitUnityEntry returns (guid, leafName) for a Unity-style tar entry name
// like "1a2b.../asset" or "./1a2b.../pathname" (both appear in the wild).
// Returns ("","") for anything else.
func splitUnityEntry(name string) (string, string) {
	clean := strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./")
	m := guidPrefix.FindStringSubmatch(clean)
	if m == nil {
		return "", ""
	}
	rest := strings.TrimPrefix(clean[len(m[1]):], "/")
	if rest == "" {
		return "", ""
	}
	// Only consider immediate children of the GUID dir — leaves like
	// `asset`, `asset.meta`, `pathname`, `preview.png`.
	if strings.Contains(rest, "/") {
		return "", ""
	}
	return m[1], rest
}

// cleanUnityPath validates and normalises a pathname value from a Unity
// package. Rejects absolute paths, drive letters, and `..` traversal.
func cleanUnityPath(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("empty pathname")
	}
	s = strings.ReplaceAll(s, "\\", "/")
	// Strip a leading "./" for normalisation only.
	s = strings.TrimPrefix(s, "./")
	if strings.HasPrefix(s, "/") {
		return "", errors.New("absolute pathname")
	}
	if len(s) >= 2 && s[1] == ':' {
		return "", errors.New("drive-letter pathname")
	}
	cleaned := path.Clean(s)
	if cleaned == "." || cleaned == "" {
		return "", errors.New("empty cleaned pathname")
	}
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", errors.New("traversal pathname")
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".." {
			return "", errors.New("traversal pathname")
		}
	}
	return cleaned, nil
}

// readCapped reads all of r up to limit+1 bytes and errors if the limit is
// exceeded. Prevents unexpectedly large sidecar files from being loaded.
func readCapped(r io.Reader, limit int64) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > limit {
		return nil, fmt.Errorf("entry exceeds %d byte cap", limit)
	}
	return buf, nil
}
