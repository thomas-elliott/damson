package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// LocalFS implements Service against a local directory. Layout mirrors the
// S3 convention (`blobs/<hh>/<hash>`, `thumbs/<hh>/<hash>`) so operators can
// later migrate a deployment between backends with a plain copy.
type LocalFS struct {
	root    string
	baseURL string // used to build presigned-equivalent URLs served by the server
}

// NewLocalFS creates (if necessary) the root directory and returns a
// filesystem-backed Service. baseURL is the public base URL of the server —
// presigned URLs point back at the server's /api/files endpoint since local
// storage has no native signed-URL concept.
func NewLocalFS(root, baseURL string) (*LocalFS, error) {
	if root == "" {
		return nil, errors.New("local storage: root path required")
	}
	for _, sub := range []string{"blobs", "thumbs"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}
	return &LocalFS{root: root, baseURL: baseURL}, nil
}

func (l *LocalFS) PutBlob(ctx context.Context, r io.Reader) (string, int64, error) {
	return l.putHashed(ctx, "blobs", r)
}

func (l *LocalFS) GetBlob(ctx context.Context, key string) (io.ReadCloser, error) {
	return l.openHashed("blobs", key)
}

func (l *LocalFS) HeadBlob(ctx context.Context, key string) (int64, bool, error) {
	return l.statHashed("blobs", key)
}

func (l *LocalFS) PutThumb(ctx context.Context, key string, r io.Reader) error {
	path, err := l.pathFor("thumbs", key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, r)
}

func (l *LocalFS) GetThumb(ctx context.Context, key string) (io.ReadCloser, error) {
	return l.openHashed("thumbs", key)
}

func (l *LocalFS) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	// Local storage has no native signing; hand back a URL that routes to the
	// server's blob handler. The TTL is advisory.
	u := l.baseURL + "/api/files/" + url.PathEscape(key)
	return u, nil
}

// putHashed streams r to a temp file, computes its SHA-256, renames into the
// final two-char-sharded path. If the file already exists we delete the temp
// and return the existing key — same contract as the S3 backend.
func (l *LocalFS) putHashed(_ context.Context, prefix string, r io.Reader) (string, int64, error) {
	tmp, err := os.CreateTemp(l.root, "upload-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		cleanup()
		return "", 0, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}

	key := hex.EncodeToString(h.Sum(nil))
	finalPath, err := l.pathFor(prefix, key)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}

	if _, err := os.Stat(finalPath); err == nil {
		// Already present — drop the temp and return the existing key.
		_ = os.Remove(tmpPath)
		return key, n, nil
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}
	return key, n, nil
}

func (l *LocalFS) openHashed(prefix, key string) (io.ReadCloser, error) {
	p, err := l.pathFor(prefix, key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (l *LocalFS) statHashed(prefix, key string) (int64, bool, error) {
	p, err := l.pathFor(prefix, key)
	if err != nil {
		return 0, false, err
	}
	info, err := os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return info.Size(), true, nil
}

func (l *LocalFS) pathFor(prefix, key string) (string, error) {
	if len(key) < 2 || !isHexKey(key) {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	return filepath.Join(l.root, prefix, key[:2], key), nil
}

func isHexKey(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// writeAtomic writes r to a temp file in the destination's dir and renames
// into place.
func writeAtomic(dst string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, dst)
}
