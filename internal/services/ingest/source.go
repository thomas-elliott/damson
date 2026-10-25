package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/importsource"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/tags"
	"github.com/thomas-elliott/damson/internal/unitycache"
)

// SourceKindUnityCache marks a version ingested from the local Unity Asset
// Store cache. Aliased from importsource, the leaf package the CLI shares.
const SourceKindUnityCache = importsource.KindUnityCache

// ErrUnknownSourceKind is returned for a source_kind outside the closed set.
var ErrUnknownSourceKind = fmt.Errorf("source_kind must be one of %s",
	strings.Join(importsource.Kinds(), ", "))

// ValidateSource checks a client-supplied source_kind / source_relative_path
// pair, returning the normalised path. Both blank is the ordinary upload case
// and is not an error.
func ValidateSource(kind, relativePath string) (string, error) {
	kind = strings.TrimSpace(kind)
	relativePath = strings.TrimSpace(relativePath)
	if kind == "" && relativePath == "" {
		return "", nil
	}
	if !importsource.ValidKind(kind) {
		return "", ErrUnknownSourceKind
	}
	if relativePath == "" {
		return "", fmt.Errorf("source_relative_path is required with source_kind=%s", kind)
	}
	clean, err := unitycache.NormaliseRelativePath(relativePath)
	if err != nil {
		return "", fmt.Errorf("source_relative_path: %w", err)
	}
	return clean, nil
}

// SuppressesFilenameVersion is importsource.SuppressesFilenameVersion.
func SuppressesFilenameVersion(kind string) bool {
	return importsource.SuppressesFilenameVersion(kind)
}

// ApplySourceHints fills asset fields derivable from the version's source
// path — publisher, and the store category tree as tags. Best-effort: any
// failure is logged and swallowed so a hint never fails an import.
func ApplySourceHints(ctx context.Context, deps Deps, jc jobContext) {
	if jc.version.SourceKind == nil || *jc.version.SourceKind != SourceKindUnityCache {
		return
	}
	if jc.version.SourceRelativePath == nil {
		return
	}
	parsed, err := unitycache.ParseCachePath(*jc.version.SourceRelativePath)
	if err != nil {
		slog.Warn("source hints: unparseable cache path", "err", err, "asset_id", jc.asset.ID)
		return
	}

	db := deps.DB.WithContext(ctx)
	if parsed.Publisher != "" {
		before, err := assetSnapshot(ctx, deps.DB, jc.asset.ID)
		if err != nil {
			slog.Warn("source hints: audit snapshot", "err", err, "asset_id", jc.asset.ID)
		}
		if err := db.Model(&models.Asset{}).
			Where("id = ? AND (publisher IS NULL OR publisher = '')", jc.asset.ID).
			Update("publisher", parsed.Publisher).Error; err != nil {
			slog.Warn("source hints: set publisher", "err", err, "asset_id", jc.asset.ID)
		}
		auditSystemUpdate(ctx, deps.DB, jc.libraryID, jc.asset.ID, before)
	}

	if path := parsed.CategoryPath(); path != "" {
		if _, _, err := tags.Attach(db, jc.libraryID, nil, jc.asset.ID, path); err != nil {
			slog.Warn("source hints: attach category", "err", err,
				"asset_id", jc.asset.ID, "path", path)
		}
	}
}

// SourceMatch is one version already holding a given source path.
type SourceMatch struct {
	AssetID   uint
	AssetName string
	VersionID uint
	SizeBytes int64
}

// SourceMatches looks up, for a batch of cache-relative paths, the newest
// version in the library already carrying each one. Newest wins because an
// updated re-upload leaves an older version on the same path, and it is the
// latest bytes a client is asking about.
func SourceMatches(db *gorm.DB, libraryID uint, kind string, paths []string) (map[string]SourceMatch, error) {
	out := map[string]SourceMatch{}
	if len(paths) == 0 {
		return out, nil
	}
	type row struct {
		AssetID      uint
		AssetName    string
		VersionID    uint
		RelativePath string
		SizeBytes    *int64
	}
	var rows []row
	err := db.Table("asset_versions AS v").
		Select("a.id AS asset_id, a.name AS asset_name, v.id AS version_id, "+
			"v.source_relative_path AS relative_path, v.source_size_bytes AS size_bytes").
		Joins("JOIN assets a ON a.id = v.asset_id").
		Where("a.library_id = ?", libraryID).
		Where("v.source_kind = ?", kind).
		Where("v.source_relative_path IN ?", paths).
		Order("v.import_date ASC, v.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// Rows arrive oldest-first, so the last write per path is the newest version.
	for _, r := range rows {
		m := SourceMatch{AssetID: r.AssetID, AssetName: r.AssetName, VersionID: r.VersionID}
		if r.SizeBytes != nil {
			m.SizeBytes = *r.SizeBytes
		}
		out[r.RelativePath] = m
	}
	return out, nil
}
