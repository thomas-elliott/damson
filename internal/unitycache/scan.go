package unitycache

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// CacheRootEnv overrides the per-OS default cache root.
const CacheRootEnv = "ASSETSTORE_CACHE_PATH"

// CacheDirName is the leaf directory Unity writes Asset Store downloads into.
const CacheDirName = "Asset Store-5.x"

// MaxScanDepth bounds how deep Scan descends below the cache root.
const MaxScanDepth = 12

const maxScanEntries = 20_000

// ErrTooManyEntries is returned when a tree holds more package files than a
// cache plausibly contains.
var ErrTooManyEntries = fmt.Errorf("cache holds more than %d packages", maxScanEntries)

// Entry is one cached package found by Scan.
type Entry struct {
	Parsed
	RelativePath string
	SizeBytes    int64
	ModTime      time.Time
}

// DefaultRoots returns the cache roots to try, most specific first.
func DefaultRoots() []string {
	if v := strings.TrimSpace(os.Getenv(CacheRootEnv)); v != "" {
		return []string{v}
	}
	switch runtime.GOOS {
	case "windows":
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			return []string{filepath.Join(appData, "Unity", CacheDirName)}
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return []string{filepath.Join(home, "Library", "Unity", CacheDirName)}
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return []string{filepath.Join(home, ".local", "share", "unity3d", CacheDirName)}
		}
	}
	return nil
}

// Scan walks root and returns every cached package it holds, sorted by
// relative path. Dotfiles, symlinks, non-.unitypackage files, packages sitting
// directly in root, and anything below MaxScanDepth are excluded; a tree
// holding more than maxScanEntries packages is an error rather than a
// truncated result.
func Scan(root string) ([]Entry, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var entries []Entry
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		depth := strings.Count(rel, "/") + 1

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || depth >= MaxScanDepth {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !d.Type().IsRegular() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), PackageExt) {
			return nil
		}

		parsed, perr := ParseCachePath(rel)
		if perr != nil {
			if errors.Is(perr, ErrInvalidRelativePath) {
				return nil
			}
			return perr
		}
		fi, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		if len(entries) >= maxScanEntries {
			return ErrTooManyEntries
		}
		entries = append(entries, Entry{
			Parsed:       parsed,
			RelativePath: rel,
			SizeBytes:    fi.Size(),
			ModTime:      fi.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelativePath < entries[j].RelativePath
	})
	return entries, nil
}
