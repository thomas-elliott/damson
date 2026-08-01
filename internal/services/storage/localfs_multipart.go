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
	"sort"
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
	ordered := make([]Part, len(parts))
	copy(ordered, parts)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Number < ordered[j].Number })

	final, err := m.l.pathFor(m.prefix, m.key)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(final); err == nil {
		return m.key, m.discard()
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return "", err
	}

	assembled := filepath.Join(m.dir(), "assembled")
	out, err := os.Create(assembled)
	if err != nil {
		return "", err
	}
	for _, p := range ordered {
		in, err := os.Open(m.partPath(p.Number))
		if err != nil {
			_ = out.Close()
			return "", fmt.Errorf("localfs multipart: part %d: %w", p.Number, err)
		}
		_, err = io.Copy(out, in)
		_ = in.Close()
		if err != nil {
			_ = out.Close()
			return "", err
		}
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(assembled, final); err != nil {
		return "", err
	}
	writeSidecar(final, m.meta)
	return m.key, m.discard()
}

func (m *localMultipart) Abort(_ context.Context) error { return m.discard() }

func (m *localMultipart) discard() error {
	if err := os.RemoveAll(m.dir()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
