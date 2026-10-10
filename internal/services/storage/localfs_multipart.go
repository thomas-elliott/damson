package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const (
	uploadsDir     = "uploads"
	uploadMetaFile = "meta.json"
)

func (l *LocalFS) BeginMultipart(_ context.Context, prefix, objectKey string, meta ObjectMeta) (MultipartUpload, error) {
	if _, err := l.pathFor(prefix, objectKey); err != nil {
		return nil, err
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, fmt.Errorf("upload id: %w", err)
	}
	id := hex.EncodeToString(b[:])
	m := &localMultipart{l: l, prefix: prefix, key: objectKey, id: id, meta: meta}
	if err := os.MkdirAll(m.dir(), 0o755); err != nil {
		return nil, err
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(m.dir(), uploadMetaFile), data, 0o644); err != nil {
		return nil, err
	}
	return m, nil
}

func (l *LocalFS) ResumeMultipart(_ context.Context, prefix, objectKey, uploadID string) (MultipartUpload, error) {
	if _, err := l.pathFor(prefix, objectKey); err != nil {
		return nil, err
	}
	if len(uploadID) != 32 || !isHexKey(uploadID) {
		return nil, fmt.Errorf("invalid upload id %q", uploadID)
	}
	m := &localMultipart{l: l, prefix: prefix, key: objectKey, id: uploadID}
	if _, err := os.Stat(m.dir()); err != nil {
		return nil, fmt.Errorf("upload %s: %w", uploadID, err)
	}
	// S3 fixes the metadata at CreateMultipartUpload; this is the equivalent.
	// Complete almost always runs on a resumed handle — it is a separate HTTP
	// request — so without restoring it the sidecar would always be empty.
	data, err := os.ReadFile(filepath.Join(m.dir(), uploadMetaFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &m.meta); err != nil {
			return nil, err
		}
	}
	return m, nil
}

type localMultipart struct {
	l      *LocalFS
	prefix string
	key    string
	id     string
	meta   ObjectMeta
}

func (m *localMultipart) ID() string { return m.id }

func (m *localMultipart) dir() string { return filepath.Join(m.l.root, uploadsDir, m.id) }

func (m *localMultipart) partPath(n int) string {
	return filepath.Join(m.dir(), "part-"+strconv.Itoa(n))
}

func (m *localMultipart) UploadPart(_ context.Context, n int, size int64, r io.Reader) (string, error) {
	// 10,000 is an S3 limit, not a filesystem one. Enforced here too so a
	// part-numbering bug fails the same way on the dev backend.
	if n < 1 || n > s3MaxParts {
		return "", fmt.Errorf("localfs multipart: part number %d out of range", n)
	}
	h := sha256.New()
	counted := &countingReader{r: io.TeeReader(io.LimitReader(r, size), h)}
	if err := writeAtomic(m.partPath(n), counted); err != nil {
		return "", err
	}
	if got := counted.count(); got != size {
		_ = os.Remove(m.partPath(n))
		return "", fmt.Errorf("localfs multipart: part %d is %d bytes, declared %d", n, got, size)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *localMultipart) Complete(_ context.Context, parts []Part) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("localfs multipart: no parts to complete")
	}
	ordered := sortedParts(parts)

	final, err := m.l.pathFor(m.prefix, m.key)
	if err != nil {
		return "", err
	}
	if present, err := touchFile(final); err != nil {
		return "", err
	} else if present {
		return m.key, m.discard()
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return "", err
	}

	assembled := filepath.Join(m.dir(), "assembled")
	if err := m.concat(assembled, ordered); err != nil {
		return "", err
	}
	if err := os.Rename(assembled, final); err != nil {
		return "", err
	}
	writeSidecar(final, m.meta)
	return m.key, m.discard()
}

// concat writes the parts end to end into dst.
func (m *localMultipart) concat(dst string, ordered []Part) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	for _, p := range ordered {
		in, err := os.Open(m.partPath(p.Number))
		if err != nil {
			return fmt.Errorf("localfs multipart: part %d: %w", p.Number, err)
		}
		_, err = io.Copy(out, in)
		_ = in.Close()
		if err != nil {
			return err
		}
	}
	return out.Close()
}

func (m *localMultipart) Abort(_ context.Context) error { return m.discard() }

func (m *localMultipart) discard() error {
	if err := os.RemoveAll(m.dir()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
