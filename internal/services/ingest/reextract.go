package ingest

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/preview"
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
// version's files in place through the pipeline's own extract, cover and write
// stages; it creates no version. The asset's current version is then chosen
// again, so re-extracting an older version does not make it current, and only
// a current version's harvested cover replaces the asset's. The rewritten
// files are handed to the preview queue.
func Reextract(ctx context.Context, deps Deps, asset models.Asset, version models.AssetVersion, userID uint) (Reextracted, error) {
	if !version.HasDownloadableOriginal() || version.PackageKind != models.PackageKindUnityPackage {
		return Reextracted{}, ErrNoRetainedPackage
	}
	jc := newJobContext(asset, version)
	jc.settings = loadSettings(ctx, deps, asset.LibraryID)
	jc.inCommit = func(tx *gorm.DB) error {
		if err := audit.Write(tx, audit.Entry{
			LibraryID:  &asset.LibraryID,
			UserID:     &userID,
			ActionType: "asset.version.reextract",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Created(map[string]any{"version_id": version.ID, "file_count": jc.fileCount}),
		}); err != nil {
			return err
		}
		// Rewritten files start with no preview, so queue them as an import
		// does.
		_, err := preview.Queue(ctx, tx, deps.Previews, preview.Job{LibraryID: asset.LibraryID, VersionID: version.ID})
		if errors.Is(err, preview.ErrDisabled) {
			return nil
		}
		return err
	}
	for _, s := range stages {
		if !s.reextract {
			continue
		}
		if err := s.run(ctx, deps, &jc); err != nil {
			return Reextracted{}, err
		}
	}
	return Reextracted{FileCount: jc.fileCount, SizeBytes: jc.sizeBytes}, nil
}
