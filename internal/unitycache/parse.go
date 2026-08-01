// Package unitycache parses and scans Unity's local Asset Store download
// cache, laid out as <Publisher>/<CategoryPath>/<Name>.unitypackage.
package unitycache

import (
	"errors"
	"path"
	"strings"
	"unicode"
)

// PackageExt is the file extension of a cached Asset Store download.
const PackageExt = ".unitypackage"

// MaxRelativePathLength caps the byte length of a cache-relative path.
const MaxRelativePathLength = 1024

// ErrInvalidRelativePath is returned for a path that is empty, absolute,
// over-long, or contains a traversal segment.
var ErrInvalidRelativePath = errors.New("invalid cache relative path")

// Parsed is the metadata carried by a cache path.
type Parsed struct {
	Publisher string
	Category  []string
	Name      string
}

// CategoryPath renders the category segments as a tag path.
func (p Parsed) CategoryPath() string {
	return strings.Join(p.Category, "/")
}

// NormaliseRelativePath converts a cache-relative path to slash form and
// validates it, returning ErrInvalidRelativePath if it is not usable.
func NormaliseRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" || len(rel) > MaxRelativePathLength {
		return "", ErrInvalidRelativePath
	}
	if strings.HasPrefix(rel, "/") {
		return "", ErrInvalidRelativePath
	}
	if len(rel) > 1 && rel[1] == ':' {
		return "", ErrInvalidRelativePath
	}
	for _, r := range rel {
		if r < 0x20 || r == 0x7f {
			return "", ErrInvalidRelativePath
		}
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", ErrInvalidRelativePath
		}
	}
	return rel, nil
}

// ParseCachePath splits a cache-relative path into publisher, category
// segments and package name. Category is empty for a bare
// <Publisher>/<Name>.unitypackage.
func ParseCachePath(rel string) (Parsed, error) {
	clean, err := NormaliseRelativePath(rel)
	if err != nil {
		return Parsed{}, err
	}
	segs := strings.Split(clean, "/")
	if len(segs) < 2 {
		return Parsed{}, ErrInvalidRelativePath
	}

	base := segs[len(segs)-1]
	if !strings.EqualFold(path.Ext(base), PackageExt) {
		return Parsed{}, ErrInvalidRelativePath
	}
	name := strings.TrimSpace(base[:len(base)-len(PackageExt)])
	if name == "" {
		return Parsed{}, ErrInvalidRelativePath
	}

	publisher := strings.TrimSpace(segs[0])
	if publisher == "" {
		return Parsed{}, ErrInvalidRelativePath
	}

	var category []string
	for _, seg := range segs[1 : len(segs)-1] {
		category = append(category, SplitCategory(seg)...)
	}
	return Parsed{Publisher: publisher, Category: category, Name: name}, nil
}

// SplitCategory splits Unity's separator-free category segment on each
// lowercase-to-uppercase boundary.
func SplitCategory(seg string) []string {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return nil
	}
	runes := []rune(seg)
	var out []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			if part := strings.TrimSpace(string(runes[start:i])); part != "" {
				out = append(out, part)
			}
			start = i
		}
	}
	if part := strings.TrimSpace(string(runes[start:])); part != "" {
		out = append(out, part)
	}
	return out
}
