package ingest

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	importVersionRE = regexp.MustCompile(`(?i)[ _.\-](v\d+(?:\.\d+)*|\d+\.\d+(?:\.\d+)*)$`)
	importSepRE     = regexp.MustCompile(`[ _.\-]+`)
)

// ParseImportFilename splits an upload filename into a human-readable asset
// name and an optional store version. The extension is stripped, a trailing
// version token is peeled off, and separators become spaces. A token counts as
// a version only when it is v-prefixed or contains a dot, so a bare trailing
// integer stays part of the name.
func ParseImportFilename(filename string) (name string, storeVersion string) {
	base := filepath.Base(filename)
	// A digit-only suffix (".1" from a "v1.1" with no real extension) is not an
	// extension; leave it on so the version parser sees the full token.
	if ext := filepath.Ext(base); ext != "" && !isAllDigits(ext[1:]) {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" || base == "." || base == "/" {
		base = filename
	}

	if m := importVersionRE.FindStringSubmatch(base); m != nil {
		storeVersion = strings.TrimLeft(m[1], "vV")
		base = base[:len(base)-len(m[0])]
	}

	name = cleanImportName(base)
	if name == "" {
		name = cleanImportName(filepath.Base(filename))
	}
	if name == "" {
		name = filename
	}
	return name, storeVersion
}

func cleanImportName(s string) string {
	return strings.TrimSpace(importSepRE.ReplaceAllString(s, " "))
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
