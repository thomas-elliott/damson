// Package tagpath holds the tag-path normalisation rule. It is a leaf package
// with no dependencies so the dmsn CLI can apply the same rule the server does
// without importing internal/tags, which pulls in GORM.
//
// Both sides must agree: the CLI compares a user-typed path against the paths
// an asset already carries, and the server stores the normalised form.
package tagpath

import "strings"

// MaxLength caps the byte length of a normalised tag path. Generous enough to
// fit any legitimate hierarchy; tight enough that a malicious caller can't
// paste megabytes into the column.
const MaxLength = 1024

// Normalise trims whitespace, lowercases, collapses repeated slashes, and
// strips leading/trailing slashes. Returns "" when the result is empty or
// exceeds MaxLength so callers surface a 400.
func Normalise(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "\\", "/")
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	s = strings.Trim(s, "/")
	parts := strings.Split(s, "/")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	joined := strings.Join(out, "/")
	if len(joined) > MaxLength {
		return ""
	}
	return joined
}
