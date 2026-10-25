// Package assets holds the rules for an asset's life: deleting it or one of
// its versions, merging one asset into another, and cutting a subset version.
// Each runs inside the caller's transaction and leaves the asset's current
// version as imports.ChooseCurrent picks it.
package assets

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/preview"
)

// ErrConflict and ErrBadRequest wrap a change the asset refuses. The text
// after the sentinel's own is safe to show.
var (
	ErrConflict   = errors.New("conflict")
	ErrBadRequest = errors.New("bad request")
)

func refuse(sentinel error, format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{sentinel}, args...)...)
}

// load returns the library's asset, or gorm.ErrRecordNotFound.
func load(tx *gorm.DB, libID, assetID uint) (models.Asset, error) {
	var asset models.Asset
	err := tx.Where("id = ? AND library_id = ?", assetID, libID).First(&asset).Error
	return asset, err
}

// Delete removes an asset. Its versions, files, tags, pins, licence, images,
// imports and lookups go with it through their ON DELETE CASCADE foreign
// keys. Stored objects stay; the orphan sweep reclaims them.
func Delete(tx *gorm.DB, libID, userID, assetID uint) error {
	asset, err := load(tx, libID, assetID)
	if err != nil {
		return err
	}
	if err := tx.Delete(&asset).Error; err != nil {
		return err
	}
	return audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "asset.delete",
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff:       audit.Deleted(map[string]any{"id": asset.ID, "name": asset.Name}),
	})
}

// DeleteVersion removes one of an asset's versions. A version a project pins,
// or an asset's only version, is refused.
func DeleteVersion(tx *gorm.DB, libID, userID, assetID, versionID uint) error {
	asset, err := load(tx, libID, assetID)
	if err != nil {
		return err
	}
	var version models.AssetVersion
	if err := tx.Where("id = ? AND asset_id = ?", versionID, asset.ID).First(&version).Error; err != nil {
		return err
	}
	var pins int64
	if err := tx.Model(&models.ProjectAssetPin{}).
		Where("pinned_version_id = ?", version.ID).Count(&pins).Error; err != nil {
		return err
	}
	if pins > 0 {
		return refuse(ErrConflict, "version is pinned by %d project(s); unpin before deleting", pins)
	}
	var total int64
	if err := tx.Model(&models.AssetVersion{}).Where("asset_id = ?", asset.ID).Count(&total).Error; err != nil {
		return err
	}
	if total <= 1 {
		return refuse(ErrConflict, "cannot delete the only version; delete the asset instead")
	}
	if err := tx.Delete(&models.AssetVersion{}, version.ID).Error; err != nil {
		return err
	}
	if _, err := imports.ChooseCurrent(tx, asset.ID, 0); err != nil {
		return err
	}
	return audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "asset.version.delete",
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff: audit.Deleted(map[string]any{
			"version_id":    version.ID,
			"store_version": version.StoreVersion,
			"version_label": version.VersionLabel,
		}),
	})
}

// Merged is what a merge did: how many versions moved, and the source asset
// it folded away, whose name, description and cover the caller may adopt.
type Merged struct {
	Versions int
	Source   models.Asset
}

// Merge folds source into target: its versions, tags, collection entries and
// project pins move to target, and source is deleted. A source version whose
// store version target already holds stays behind to be deleted, and its pins
// move to target's equivalent.
func Merge(tx *gorm.DB, libID, userID, targetID, sourceID uint) (Merged, error) {
	if sourceID == targetID {
		return Merged{}, refuse(ErrBadRequest, "cannot merge an asset into itself")
	}
	target, err := load(tx, libID, targetID)
	if err != nil {
		return Merged{}, err
	}
	source, err := load(tx, libID, sourceID)
	if err != nil {
		return Merged{}, err
	}
	// Merging moves versions, which a reference asset neither has nor takes;
	// folding one into the other is the deferred conversion step.
	if target.IsReference || source.IsReference {
		return Merged{}, refuse(ErrConflict, "reference assets can't be merged")
	}

	targetByStore := map[string]uint{}
	var targetVersions []models.AssetVersion
	if err := tx.Where("asset_id = ?", target.ID).Find(&targetVersions).Error; err != nil {
		return Merged{}, err
	}
	for _, v := range targetVersions {
		if v.StoreVersion != "" {
			targetByStore[v.StoreVersion] = v.ID
		}
	}
	var sourceVersions []models.AssetVersion
	if err := tx.Where("asset_id = ?", source.ID).Find(&sourceVersions).Error; err != nil {
		return Merged{}, err
	}
	// A source version whose store version already exists on the target can't
	// move (the partial unique index forbids it); it stays on the source to
	// cascade-delete, and its pins are remapped to the target's equivalent.
	dropRemap := map[uint]uint{}
	moved := 0
	for _, v := range sourceVersions {
		if tid, dup := targetByStore[v.StoreVersion]; dup {
			dropRemap[v.ID] = tid
			continue
		}
		if err := tx.Model(&models.AssetVersion{}).Where("id = ?", v.ID).
			Update("asset_id", target.ID).Error; err != nil {
			return Merged{}, err
		}
		moved++
	}
	if err := moveAssetTags(tx, source.ID, target.ID); err != nil {
		return Merged{}, err
	}
	if err := moveCollectionRefs(tx, source.ID, target.ID); err != nil {
		return Merged{}, err
	}
	if err := movePins(tx, source.ID, target.ID, dropRemap); err != nil {
		return Merged{}, err
	}
	if err := tx.Delete(&models.Asset{}, source.ID).Error; err != nil {
		return Merged{}, err
	}
	if _, err := imports.ChooseCurrent(tx, target.ID, 0); err != nil {
		return Merged{}, err
	}
	return Merged{Versions: moved, Source: source}, audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "asset.merge",
		TargetType: "asset",
		TargetID:   &target.ID,
		Diff: audit.Created(map[string]any{
			"source_asset_id": source.ID,
			"merged_versions": moved,
		}),
	})
}

func moveAssetTags(tx *gorm.DB, sourceID, targetID uint) error {
	var targetTagIDs []uint
	if err := tx.Model(&models.AssetTag{}).Where("asset_id = ?", targetID).
		Pluck("tag_id", &targetTagIDs).Error; err != nil {
		return err
	}
	q := tx.Model(&models.AssetTag{}).Where("asset_id = ?", sourceID)
	if len(targetTagIDs) > 0 {
		q = q.Where("tag_id NOT IN ?", targetTagIDs)
	}
	return q.Update("asset_id", targetID).Error
}

func moveCollectionRefs(tx *gorm.DB, sourceID, targetID uint) error {
	var targetCollIDs []uint
	if err := tx.Model(&models.CollectionAsset{}).Where("asset_id = ?", targetID).
		Pluck("collection_id", &targetCollIDs).Error; err != nil {
		return err
	}
	q := tx.Model(&models.CollectionAsset{}).Where("asset_id = ?", sourceID)
	if len(targetCollIDs) > 0 {
		q = q.Where("collection_id NOT IN ?", targetCollIDs)
	}
	return q.Update("asset_id", targetID).Error
}

func movePins(tx *gorm.DB, sourceID, targetID uint, dropRemap map[uint]uint) error {
	var targetProjectIDs []uint
	if err := tx.Model(&models.ProjectAssetPin{}).Where("asset_id = ?", targetID).
		Pluck("project_id", &targetProjectIDs).Error; err != nil {
		return err
	}
	targetProjects := make(map[uint]bool, len(targetProjectIDs))
	for _, id := range targetProjectIDs {
		targetProjects[id] = true
	}
	var sourcePins []models.ProjectAssetPin
	if err := tx.Where("asset_id = ?", sourceID).Find(&sourcePins).Error; err != nil {
		return err
	}
	for _, p := range sourcePins {
		if targetProjects[p.ProjectID] {
			continue
		}
		updates := map[string]any{"asset_id": targetID}
		// Merges refuse reference assets, so every pin here names a version.
		if p.PinnedVersionID != nil {
			if remapped, ok := dropRemap[*p.PinnedVersionID]; ok {
				updates["pinned_version_id"] = remapped
			}
		}
		if err := tx.Model(&models.ProjectAssetPin{}).Where("id = ?", p.ID).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

// Subset creates a version holding some of a version's files. Its files
// reuse the source's objects and previews; a preview still rendering for the
// source is queued again for the subset. fileIDs must all belong to the
// source version.
func Subset(tx *gorm.DB, previews preview.Enqueuer, libID, userID, assetID, versionID uint, fileIDs []uint, label string) (models.AssetVersion, error) {
	asset, err := load(tx, libID, assetID)
	if err != nil {
		return models.AssetVersion{}, err
	}
	var source models.AssetVersion
	if err := tx.Where("id = ? AND asset_id = ?", versionID, asset.ID).First(&source).Error; err != nil {
		return models.AssetVersion{}, err
	}
	if len(fileIDs) == 0 {
		return models.AssetVersion{}, refuse(ErrBadRequest, "file_ids must not be empty")
	}
	var sourceFiles []models.AssetFile
	if err := tx.Where("asset_version_id = ? AND id IN ?", source.ID, fileIDs).Find(&sourceFiles).Error; err != nil {
		return models.AssetVersion{}, err
	}
	if len(sourceFiles) != len(fileIDs) {
		return models.AssetVersion{}, refuse(ErrBadRequest, "one or more file_ids do not belong to the source version")
	}

	version := models.AssetVersion{
		AssetID:         asset.ID,
		VersionLabel:    label,
		ImportDate:      time.Now().UTC(),
		PackageKind:     source.PackageKind,
		Notes:           "subset of " + versionDisplayName(source),
		CreatedByUserID: &userID,
	}
	// Subsets don't retain the package archive — they're virtual versions
	// pointing at the same content-addressed objects as the source.
	if err := tx.Create(&version).Error; err != nil {
		return models.AssetVersion{}, err
	}
	queuePreviews := false
	rows := make([]models.AssetFile, 0, len(sourceFiles))
	for _, f := range sourceFiles {
		status := f.PreviewStatus
		if status == preview.StatusProcessing {
			status = preview.StatusQueued
		}
		queuePreviews = queuePreviews || status == preview.StatusQueued
		rows = append(rows, models.AssetFile{
			AssetVersionID:     version.ID,
			RelativePath:       f.RelativePath,
			SizeBytes:          f.SizeBytes,
			ContentHash:        f.ContentHash,
			ObjectKey:          f.ObjectKey,
			DetectedType:       f.DetectedType,
			UnityGUID:          f.UnityGUID,
			PreviewObjectKey:   f.PreviewObjectKey,
			PreviewContentType: f.PreviewContentType,
			PreviewStatus:      status,
			PreviewUpdatedAt:   f.PreviewUpdatedAt,
		})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return models.AssetVersion{}, err
	}
	if _, err := imports.ChooseCurrent(tx, asset.ID, 0); err != nil {
		return models.AssetVersion{}, err
	}
	if queuePreviews && previews != nil {
		if err := previews.Enqueue(tx, preview.Job{LibraryID: libID, VersionID: version.ID}); err != nil {
			return models.AssetVersion{}, err
		}
	}
	return version, audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "asset.version.subset",
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff: audit.Created(map[string]any{
			"asset_id":          asset.ID,
			"source_version":    source.ID,
			"new_version":       version.ID,
			"file_count":        len(rows),
			"new_version_label": label,
		}),
	})
}

// versionDisplayName is how a version is named to a person: its label, else
// its store version, else "unversioned".
func versionDisplayName(v models.AssetVersion) string {
	if v.VersionLabel != "" {
		return v.VersionLabel
	}
	if v.StoreVersion != "" {
		return v.StoreVersion
	}
	return "unversioned"
}
