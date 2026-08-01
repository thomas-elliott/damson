package api

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// importSpec is everything the import transaction needs that does not depend
// on how the bytes arrived.
type importSpec struct {
	libraryID          uint
	userID             uint
	filename           string
	contentType        string
	packageKind        string
	isArchive          bool
	targetAssetID      uint
	reqStoreVersion    string
	sourceKind         string
	sourceRelativePath string
}

type importRows struct {
	assetID   uint
	versionID uint
	importID  uint
}

func newImportSpec(libraryID, userID uint, filename, contentType string) importSpec {
	name := filepath.Base(filename)
	if name == "" || name == "." || name == "/" {
		name = "upload"
	}
	s := importSpec{
		libraryID:   libraryID,
		userID:      userID,
		filename:    name,
		contentType: contentType,
		packageKind: packageKindLoose,
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".unitypackage":
		s.packageKind = packageKindUnityPackage
		s.isArchive = true
	case ".zip":
		s.packageKind = packageKindZip
		s.isArchive = true
	}
	return s
}

// storeVersion resolves the version label: parsed from the filename, dropped
// for sources whose filenames don't carry one, overridden by the client.
func (s importSpec) storeVersion() string {
	_, v := imports.ParseImportFilename(s.filename)
	if imports.SuppressesFilenameVersion(s.sourceKind) {
		v = ""
	}
	if s.reqStoreVersion != "" {
		v = s.reqStoreVersion
	}
	return v
}

func (s importSpec) objectMeta(assetID, versionID uint) storage.ObjectMeta {
	return storage.ObjectMeta{
		Filename:  s.filename,
		LibraryID: s.libraryID,
		AssetID:   assetID,
		VersionID: versionID,
		Kind:      s.packageKind,
	}
}

func (s importSpec) storagePrefix() string {
	if s.isArchive {
		return "originals"
	}
	return "blobs"
}

// checkImportConflict answers "would this upload be rejected?" without
// creating anything, so a chunked session can fail before the bytes move.
// createImportRows re-checks inside its transaction, since a multi-hour
// upload gives the answer plenty of time to change.
func (h *Handler) checkImportConflict(ctx context.Context, s importSpec) error {
	if s.targetAssetID == 0 {
		return nil
	}
	db := h.db.WithContext(ctx)
	var asset models.Asset
	if err := db.Where("id = ? AND library_id = ?", s.targetAssetID, s.libraryID).First(&asset).Error; err != nil {
		return err
	}
	return conflictingStoreVersion(db, asset.ID, s.storeVersion())
}

func conflictingStoreVersion(db *gorm.DB, assetID uint, storeVersion string) error {
	if storeVersion == "" {
		return nil
	}
	var dupes int64
	if err := db.Model(&models.AssetVersion{}).
		Where("asset_id = ? AND store_version = ?", assetID, storeVersion).
		Count(&dupes).Error; err != nil {
		return err
	}
	if dupes > 0 {
		return errConflictf("this asset already has store version %s; delete or replace that version first", storeVersion)
	}
	return nil
}

// createImportRows creates the asset (or reuses the target), the version, and
// the import row in one transaction. The version has no object key until
// finishImport runs.
func (h *Handler) createImportRows(ctx context.Context, s importSpec) (importRows, error) {
	var rows importRows
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		storeVersion := s.storeVersion()
		var asset models.Asset
		if s.targetAssetID != 0 {
			if err := tx.Where("id = ? AND library_id = ?", s.targetAssetID, s.libraryID).First(&asset).Error; err != nil {
				return err
			}
			if err := conflictingStoreVersion(tx, asset.ID, storeVersion); err != nil {
				return err
			}
		} else {
			assetName, _ := imports.ParseImportFilename(s.filename)
			asset = models.Asset{
				LibraryID:       s.libraryID,
				Name:            assetName,
				NeedsReview:     true,
				CreatedByUserID: &s.userID,
			}
			if err := tx.Create(&asset).Error; err != nil {
				return err
			}
		}
		version := models.AssetVersion{
			AssetID:         asset.ID,
			StoreVersion:    storeVersion,
			ImportDate:      time.Now().UTC(),
			CreatedByUserID: &s.userID,
			PackageKind:     s.packageKind,
		}
		if s.sourceRelativePath != "" {
			kind := s.sourceKind
			path := s.sourceRelativePath
			version.SourceKind = &kind
			version.SourceRelativePath = &path
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		imp := models.AssetImport{
			AssetID:   asset.ID,
			Stage:     imports.StageUploaded,
			UpdatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&imp).Error; err != nil {
			return err
		}
		rows = importRows{assetID: asset.ID, versionID: version.ID, importID: imp.ID}
		return nil
	})
	return rows, err
}

// finishImport records the stored object against the version, creates the
// AssetFile row for a loose (non-archive) upload, and enqueues the pipeline.
func (h *Handler) finishImport(ctx context.Context, s importSpec, rows importRows, objectKey string, size int64) error {
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"original_package_object_key": objectKey}
		if s.sourceRelativePath != "" {
			updates["source_size_bytes"] = size
		}
		if err := tx.Model(&models.AssetVersion{}).
			Where("id = ?", rows.versionID).
			Updates(updates).Error; err != nil {
			return err
		}
		if s.isArchive {
			return nil
		}
		f := models.AssetFile{
			AssetVersionID: rows.versionID,
			RelativePath:   s.filename,
			SizeBytes:      size,
			ContentHash:    objectKey,
			ObjectKey:      objectKey,
			DetectedType:   s.contentType,
		}
		return tx.Create(&f).Error
	})
	if err != nil {
		return err
	}
	h.importPool.Enqueue(imports.Job{ImportID: rows.importID, LibraryID: s.libraryID})
	return nil
}

// suggestMatch offers an existing asset with a similar name. Only meaningful
// when the upload created a new asset.
func (h *Handler) suggestMatch(ctx context.Context, s importSpec, assetID uint) *matchSuggestion {
	if s.targetAssetID != 0 {
		return nil
	}
	name, _ := imports.ParseImportFilename(s.filename)
	m, err := imports.MatchAssetByName(h.db.WithContext(ctx), s.libraryID, assetID, name)
	if err != nil || m == nil {
		return nil
	}
	return &matchSuggestion{AssetID: m.ID, Name: m.Name}
}

// abandonImport removes the rows created for an upload that then failed.
func (h *Handler) abandonImport(ctx context.Context, s importSpec, rows importRows) {
	if s.targetAssetID != 0 {
		h.cleanupFailedVersionUpload(ctx, rows.versionID, rows.importID)
		return
	}
	h.cleanupFailedUpload(ctx, rows.assetID)
}
