package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// configDirName is the directory under the user's config root. It names the
// product, not the client — the binary is dmsn, the server is damson.
const configDirName = "damson"

const configFileName = "config.json"

// Config is the on-disk credential store. One server, one token: named
// profiles are deliberately out of scope.
type Config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

// configPath returns the config file location, honouring DAMSON_CONFIG for
// tests and for callers who keep credentials elsewhere.
func configPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("DAMSON_CONFIG")); override != "" {
		return override, nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config directory: %w", err)
	}
	return filepath.Join(root, configDirName, configFileName), nil
}

// LoadConfig reads the stored credentials. A missing file is not an error —
// it just means nobody has logged in yet.
func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}

// SaveConfig writes the credentials with owner-only permissions, replacing
// the file atomically so a crash cannot leave a half-written token behind.
func SaveConfig(c Config) (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return "", fmt.Errorf("set permissions on %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(b); err != nil {
		return "", fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("replace %s: %w", path, err)
	}
	return path, nil
}

// ClearConfig removes the stored credentials. A file that was never there is
// a success, not a failure.
func ClearConfig() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("remove %s: %w", path, err)
	}
	return path, nil
}

// displayPath abbreviates the user's home directory to ~ so output stays
// readable and doesn't broadcast a username.
func displayPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}
