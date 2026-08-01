// Package buildinfo reads the build identity stamped into the binary. Both the
// server's /api/health and the CLI's `version` report it, so the rule for
// deriving it lives here rather than in either one.
package buildinfo

import (
	"runtime/debug"
	"sync"
)

// commitLength bounds the reported VCS revision to a readable prefix.
const commitLength = 12

var read = sync.OnceValues(debug.ReadBuildInfo)

// Version reports the module version stamped into the binary, or "" when the
// binary was not built from a tagged module.
func Version() string {
	info, ok := read()
	if !ok {
		return ""
	}
	if info.Main.Version == "(devel)" {
		return ""
	}
	return info.Main.Version
}

// Commit reports the VCS revision truncated to commitLength, or "" when the
// binary carries no revision stamp.
func Commit() string {
	info, ok := read()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			if len(s.Value) > commitLength {
				return s.Value[:commitLength]
			}
			return s.Value
		}
	}
	return ""
}

// GoVersion reports the toolchain version, or "" when unavailable.
func GoVersion() string {
	info, ok := read()
	if !ok {
		return ""
	}
	return info.GoVersion
}
