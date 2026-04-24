package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// jwtKeyBytes is the size of the HS256 signing key persisted to disk when
// DAMSON_JWT_SIGNING_KEY is unset. 32 bytes matches NewLocalAuthenticator's
// minimum and is the standard recommendation for HS256.
const jwtKeyBytes = 32

// LoadOrCreateJWTKey reads a signing key from path. If the file is absent,
// it generates 32 random bytes, writes them atomically with 0600 perms,
// and returns the new key. Used by local-auth mode to give the zero-config
// path a stable key without forcing operators to hand-craft one.
//
// Production deployments should still set DAMSON_JWT_SIGNING_KEY directly
// so the key rotates with the config, not a bind-mounted file — but either
// shape is safe.
func LoadOrCreateJWTKey(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("jwt key path is empty")
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) < jwtKeyBytes {
			return nil, fmt.Errorf("jwt key file %q is too short (%d bytes; need ≥ %d)", path, len(data), jwtKeyBytes)
		}
		return data, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read jwt key %q: %w", path, err)
	}

	// Generate a new key and persist. Writes are atomic (temp + rename) so a
	// concurrent second starter doesn't read half-written bytes.
	key := make([]byte, jwtKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate jwt key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir jwt key dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return nil, fmt.Errorf("create jwt key tmp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return nil, fmt.Errorf("chmod jwt key tmp: %w", err)
	}
	if _, err := tmp.Write(key); err != nil {
		cleanup()
		return nil, fmt.Errorf("write jwt key tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("close jwt key tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("rename jwt key: %w", err)
	}

	slog.Info("generated new JWT signing key", "path", path)
	return key, nil
}
