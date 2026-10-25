package imports

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/extract"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// ErrNoRetainedPackage is Reextract's answer for a version with no Unity
// package left to read.
var ErrNoRetainedPackage = errors.New("source version has no retained package")

// ErrBadPackage wraps an extraction that failed on the package itself.
var ErrBadPackage = errors.New("extract")

// Reextracted is what a re-extract wrote.
type Reextracted struct {
	FileCount int
	SizeBytes int64
}

// Reextract re-reads the Unity package retained for version and rewrites the
// version's files in place; it creates no version. The asset's current
// version is then chosen again, so re-extracting an older version does not
// make it current, and only a current version's harvested cover replaces the
// asset's. The rewritten files are handed to the preview queue.
func Reextract(ctx context.Context, deps Deps, asset models.Asset, version models.AssetVersion, userID uint) (Reextracted, error) {
	if version.OriginalPackageObjectKey == "" || version.PackageKind != "unitypackage" || version.OriginalPackagePurged {
		return Reextracted{}, ErrNoRetainedPackage
	}
	blob, err := OpenOriginal(ctx, deps.Storage, version)
	if err != nil {
		return Reextracted{}, err
	}
	defer blob.Close()
	ex := &extract.UnityExtractor{
		Storage: deps.Storage,
		EntryMeta: storage.ObjectMeta{
			LibraryID: asset.LibraryID,
			AssetID:   asset.ID,
			Kind:      "unitypackage-asset",
		},
	}
	result, err := ex.Extract(ctx, blob)
	if err != nil {
		return Reextracted{}, fmt.Errorf("%w: %w", ErrBadPackage, err)
	}

	var out Reextracted
	err = deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("asset_version_id = ?", version.ID).Delete(&models.AssetFile{}).Error; err != nil {
			return err
		}
		var err error
		if out.FileCount, out.SizeBytes, err = WriteAssetFiles(tx, version.ID, "", 0, "", "", result); err != nil {
			return err
		}
		if err := commitFiles(tx, asset.ID, version.ID, result.ThumbObjectKey, result.ThumbContentType); err != nil {
			return err
		}
		if err := audit.Write(tx, audit.Entry{
			LibraryID:  &asset.LibraryID,
			UserID:     &userID,
			ActionType: "asset.version.reextract",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Created(map[string]any{"version_id": version.ID, "file_count": out.FileCount}),
		}); err != nil {
			return err
		}
		// Rewritten files start with no preview, so queue them as an import
		// does.
		_, err = preview.Queue(ctx, tx, deps.Previews, preview.Job{LibraryID: asset.LibraryID, VersionID: version.ID})
		if errors.Is(err, preview.ErrDisabled) {
			return nil
		}
		return err
	})
	return out, err
}
