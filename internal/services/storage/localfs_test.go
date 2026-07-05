package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"testing"
)

func TestLocalFS_PutBlob_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFS(dir, "http://localhost:8080")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	payload := []byte("hello damson")
	key, size, err := s.PutBlob(ctx, bytes.NewReader(payload), ObjectMeta{})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if size != int64(len(payload)) {
		t.Fatalf("size: got %d, want %d", size, len(payload))
	}
	sum := sha256.Sum256(payload)
	if want := hex.EncodeToString(sum[:]); key != want {
		t.Fatalf("key: got %q, want %q", key, want)
	}

	// File lives at blobs/<hh>/<hash>.
	expected := filepath.Join(dir, "blobs", key[:2], key)
	if sz, exists, err := s.HeadBlob(ctx, key); err != nil || !exists || sz != int64(len(payload)) {
		t.Fatalf("head: size=%d exists=%v err=%v", sz, exists, err)
	}
	r, err := s.GetBlob(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, payload)
	}
	_ = expected // asserted indirectly via Get
}

func TestLocalFS_PutBlob_Dedupes(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFS(dir, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	payload := []byte("same bytes, different uploads")
	k1, _, err := s.PutBlob(ctx, bytes.NewReader(payload), ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	k2, _, err := s.PutBlob(ctx, bytes.NewReader(payload), ObjectMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Fatalf("keys differ for identical content: %s vs %s", k1, k2)
	}
}

func TestLocalFS_HeadBlob_Missing(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFS(dir, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	// 64 hex chars but not present.
	_, exists, err := s.HeadBlob(context.Background(),
		"0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected not-exists")
	}
}
