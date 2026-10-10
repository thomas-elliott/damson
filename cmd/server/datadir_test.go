package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/config"
)

// With the default config the JWT key and blobs live under ./data, which is
// /data in the image. When it is not writable, startup must say which
// directory and which env vars move it, not fail later on a bare mkdir error.
func TestCheckDataDirs(t *testing.T) {
	root := t.TempDir()
	// A file where the data directory should be: nobody can create or write
	// to it, root included. CI runs as root, which ignores a 0500 directory.
	unwritable := filepath.Join(root, "data")
	if err := os.WriteFile(unwritable, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	local := config.Config{
		AuthMode:         config.AuthModeLocal,
		StorageMode:      config.StorageModeLocal,
		JWTKeyFile:       filepath.Join(unwritable, "jwt.key"),
		StorageLocalPath: filepath.Join(unwritable, "blobs"),
	}

	err := checkDataDirs(local)
	if err == nil {
		t.Fatal("an unwritable data directory passed the startup check")
	}
	for _, want := range []string{unwritable, "DAMSON_JWT_KEY_FILE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}

	// Storage alone on an unwritable path names its own override.
	storageOnly := local
	storageOnly.JWTSigningKey = []byte("set")
	if err := checkDataDirs(storageOnly); err == nil || !strings.Contains(err.Error(), "DAMSON_STORAGE_LOCAL_PATH") {
		t.Errorf("storage path check = %v", err)
	}

	// Nothing on disk is needed when both are configured elsewhere.
	elsewhere := config.Config{AuthMode: config.AuthModeOIDC, StorageMode: config.StorageModeS3}
	if err := checkDataDirs(elsewhere); err != nil {
		t.Errorf("OIDC + S3 needs no data directory, got %v", err)
	}

	writable := local
	writable.JWTKeyFile = filepath.Join(root, "ok", "jwt.key")
	writable.StorageLocalPath = filepath.Join(root, "ok", "blobs")
	if err := checkDataDirs(writable); err != nil {
		t.Errorf("writable directories failed the check: %v", err)
	}
}

// An existing key file mounted read-only is only ever read, so its directory
// need not be writable.
func TestCheckDataDirs_AcceptsAnExistingReadOnlyKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "jwt.key")
	if err := os.WriteFile(key, make([]byte, 64), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	cfg := config.Config{AuthMode: config.AuthModeLocal, StorageMode: config.StorageModeS3, JWTKeyFile: key}
	if err := checkDataDirs(cfg); err != nil {
		t.Fatalf("an existing read-only key was refused: %v", err)
	}
}
