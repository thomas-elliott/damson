// Package importsource holds the vocabulary shared by the import API and its
// clients: where a version's bytes came from, and what preflight can say about
// one. It is a leaf package — no gin, no gorm, no River — so the dmsn CLI can
// speak the same words the server does without linking the server.
package importsource

import "slices"

// Kind values match asset_versions.source_kind. A blank kind is the ordinary
// hand-dragged upload, which has no source.
const (
	KindUnityCache = "unity_cache"
)

// Preflight statuses, most precise first. A path match answers exactly; a name
// match only answers "something by this name exists".
const (
	StatusAlreadyImported = "already_imported"
	StatusUpdated         = "updated"
	StatusNameMatch       = "name_match"
	StatusNew             = "new"
)

var kinds = []string{KindUnityCache}

// ValidKind reports whether kind is a known source.
func ValidKind(kind string) bool { return slices.Contains(kinds, kind) }

// Kinds returns every known source kind.
func Kinds() []string { return slices.Clone(kinds) }

// SuppressesFilenameVersion reports whether a source's filenames carry no
// version information, so a version token parsed out of one is a fiction.
func SuppressesFilenameVersion(kind string) bool { return kind == KindUnityCache }
