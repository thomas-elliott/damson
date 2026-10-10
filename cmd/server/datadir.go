package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thomas-elliott/damson/internal/config"
)

// checkDataDirs fails startup with a message naming the directory and its
// override when a path the server writes to on local disk is not writable.
// In the image the defaults resolve to /data, so a missing or read-only
// volume there is reported up front rather than as a bare mkdir error.
func checkDataDirs(cfg config.Config) error {
	// An existing key file is only read, so its directory need not be writable.
	if cfg.AuthMode == config.AuthModeLocal && len(cfg.JWTSigningKey) == 0 && !fileExists(cfg.JWTKeyFile) {
		if err := checkWritable(filepath.Dir(cfg.JWTKeyFile)); err != nil {
			return fmt.Errorf("%w; mount a writable volume there, or set DAMSON_JWT_KEY_FILE (or DAMSON_JWT_SIGNING_KEY) to avoid it", err)
		}
	}
	if cfg.StorageMode == config.StorageModeLocal {
		if err := checkWritable(cfg.StorageLocalPath); err != nil {
			return fmt.Errorf("%w; mount a writable volume there, or set DAMSON_STORAGE_LOCAL_PATH to a writable directory", err)
		}
	}
	return nil
}

// checkWritable creates dir if needed and proves a file can be written in it.
func checkWritable(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("data directory %q: %w", dir, err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return fmt.Errorf("data directory %s is not writable: %w", abs, err)
	}
	f, err := os.CreateTemp(abs, ".write-check-*")
	if err != nil {
		return fmt.Errorf("data directory %s is not writable: %w", abs, err)
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
