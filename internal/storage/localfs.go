package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalFS implements Service against a local directory. Layout mirrors the
// S3 convention (`blobs/<hh>/<hash>`, `thumbs/<hh>/<hash>`) so operators can
// later migrate a deployment between backends with a plain copy.
type LocalFS struct {
	root string
}

// NewLocalFS creates (if necessary) the root directory and returns a
// filesystem-backed Service.
func NewLocalFS(root string) (*LocalFS, error) {
	if root == "" {
		return nil, errors.New("local storage: root path required")
	}
	for _, sub := range []string{string(Blobs), string(Thumbs), string(Originals), uploadsDir} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}
	return &LocalFS{root: root}, nil
}

func (l *LocalFS) Put(ctx context.Context, prefix Prefix, r io.Reader, meta ObjectMeta) (string, int64, error) {
	return l.putHashed(ctx, prefix, r, meta)
}

func (l *LocalFS) Get(_ context.Context, prefix Prefix, key string) (io.ReadCloser, error) {
	p, err := l.pathFor(prefix, key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (l *LocalFS) Delete(_ context.Context, prefix Prefix, key string) error {
	path, err := l.pathFor(prefix, key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(path + metaSuffix); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (l *LocalFS) List(_ context.Context, prefix Prefix) ([]ObjectInfo, error) {
	root := filepath.Join(l.root, string(prefix))
	var out []ObjectInfo
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || strings.HasSuffix(p, metaSuffix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, ObjectInfo{
			Key:          filepath.Base(p),
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (l *LocalFS) StatMeta(_ context.Context, prefix Prefix, key string) (ObjectMeta, error) {
	path, err := l.pathFor(prefix, key)
	if err != nil {
		return ObjectMeta{}, err
	}
	data, err := os.ReadFile(path + metaSuffix)
	if errors.Is(err, os.ErrNotExist) {
		return ObjectMeta{}, nil
	}
	if err != nil {
		return ObjectMeta{}, err
	}
	var m ObjectMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return ObjectMeta{}, err
	}
	return m, nil
}

func (l *LocalFS) Stat(_ context.Context, prefix Prefix, key string) (ObjectInfo, bool, error) {
	path, err := l.pathFor(prefix, key)
	if err != nil {
		return ObjectInfo{}, false, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ObjectInfo{}, false, nil
	}
	if err != nil {
		return ObjectInfo{}, false, err
	}
	return ObjectInfo{Key: key, Size: info.Size(), LastModified: info.ModTime()}, true, nil
}

func (l *LocalFS) Touch(_ context.Context, prefix Prefix, key string) (bool, error) {
	path, err := l.pathFor(prefix, key)
	if err != nil {
		return false, err
	}
	return touchFile(path)
}

// touchFile sets path's modification time to now, reporting false when the
// file does not exist.
func touchFile(path string) (bool, error) {
	now := time.Now()
	err := os.Chtimes(path, now, now)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// putHashed streams r to a temp file, computes its SHA-256, renames into the
// final two-char-sharded path. If the file already exists we delete the temp
// and return the existing key — same contract as the S3 backend.
func (l *LocalFS) putHashed(_ context.Context, prefix Prefix, r io.Reader, meta ObjectMeta) (string, int64, error) {
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

	if present, err := touchFile(finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	} else if present {
		// Already present — drop the temp and return the existing key, touched
		// so the orphan sweep sees it as new (Service.Touch).
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
	writeSidecar(finalPath, meta)
	return key, n, nil
}

const metaSuffix = ".meta.json"

// writeSidecar records ObjectMeta next to the blob as `<path>.meta.json`, the
// LocalFS equivalent of S3 user metadata. Best-effort: a sidecar write failure
// never fails the object write, since the metadata is a fallback aid only.
func writeSidecar(objectPath string, meta ObjectMeta) {
	if (meta == ObjectMeta{}) {
		return
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return
	}
	_ = os.WriteFile(objectPath+metaSuffix, data, 0o644)
}

func (l *LocalFS) pathFor(prefix Prefix, key string) (string, error) {
	// Defense in depth: callers at the HTTP boundary already enforce a
	// 64-hex-char shape, but the storage layer rejects anything that could
	// climb out of `root` regardless. Empty keys, separators, ".."
	// components, and non-hex bytes all land here.
	if len(key) < 2 || len(key) > 128 || !isHexKey(key) {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	return filepath.Join(l.root, string(prefix), key[:2], key), nil
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
