package models

import "fmt"

// Package kinds: how a version's original arrived. A version with no
// original has a blank kind.
const (
	PackageKindUnityPackage = "unitypackage"
	PackageKindZip          = "zip"
	PackageKindLoose        = "loose"
)

// IsArchiveKind reports whether kind's original is an archive kept under
// originals/. A loose upload's original key is the blobs/ key of its one file.
func IsArchiveKind(kind string) bool {
	return kind == PackageKindUnityPackage || kind == PackageKindZip
}

// HasDownloadableOriginal reports whether the version's archive can still be
// read: the key is set, the bytes were not purged and the version is an
// archive. DownloadableOriginalSQL is the same rule for a query.
func (v AssetVersion) HasDownloadableOriginal() bool {
	return v.OriginalPackageObjectKey != "" && !v.OriginalPackagePurged && IsArchiveKind(v.PackageKind)
}

// DownloadableOriginalSQL is HasDownloadableOriginal as a SQL condition over
// the asset_versions table or alias named by table.
func DownloadableOriginalSQL(table string) string {
	return fmt.Sprintf("(COALESCE(%[1]s.original_package_object_key, '') <> '' AND NOT %[1]s.original_package_purged AND %[1]s.package_kind IN ('%[2]s', '%[3]s'))",
		table, PackageKindUnityPackage, PackageKindZip)
}

// ObjectKeyColumn is a column that holds object keys, and the storage prefix
// those keys live under.
type ObjectKeyColumn struct {
	Table  string
	Column string
	Prefix string
	// Live narrows the rows whose key keeps an object alive. Blank means
	// every row.
	Live string
}

// ObjectKeyColumns lists every column that holds an object key. The orphan
// sweep deletes any object that no live row here references, so a new key
// column must be added to this list.
var ObjectKeyColumns = []ObjectKeyColumn{
	{Table: "asset_files", Column: "object_key", Prefix: "blobs"},
	{Table: "asset_files", Column: "preview_object_key", Prefix: "thumbs"},
	{Table: "assets", Column: "thumbnail_object_key", Prefix: "thumbs"},
	{Table: "asset_images", Column: "object_key", Prefix: "thumbs"},
	// A purged original's key stays on its row, so a failed purge leaves an
	// object the sweep may reclaim.
	{Table: "asset_versions", Column: "original_package_object_key", Prefix: "originals", Live: DownloadableOriginalSQL("asset_versions")},
}
