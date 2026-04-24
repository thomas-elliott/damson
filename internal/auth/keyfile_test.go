package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateJWTKey_GeneratesThenReuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jwt.key")

	first, err := LoadOrCreateJWTKey(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(first) != jwtKeyBytes {
		t.Fatalf("first key length: got %d, want %d", len(first), jwtKeyBytes)
	}

	// File should exist with 0600 perms.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm: got %o, want 0600", info.Mode().Perm())
	}

	// Second call reads the same key rather than generating a new one.
	second, err := LoadOrCreateJWTKey(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("second call generated a new key instead of reusing the on-disk one")
	}
}

func TestLoadOrCreateJWTKey_RejectsShortFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jwt.key")
	if err := os.WriteFile(path, []byte("too-short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateJWTKey(path); err == nil {
		t.Fatal("expected error for under-length key file, got nil")
	}
}

func TestLoadOrCreateJWTKey_CreatesMissingDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "also-nested", "jwt.key")
	if _, err := LoadOrCreateJWTKey(path); err != nil {
		t.Fatalf("LoadOrCreateJWTKey: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat created key: %v", err)
	}
}
