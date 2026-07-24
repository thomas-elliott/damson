package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
)

type subsetRequest struct {
	FileIDs      []uint `json:"file_ids" binding:"required"`
	VersionLabel string `json:"version_label,omitempty"`
}

// postAssetVersionSubset creates a new version whose asset_files are a
// subset of the source version's files. Object keys are reused (no
// re-upload); the new version simply references the same blobs. Any
// `file_ids` that don't belong to the source version are rejected with 400
// — this catches both stale UI state and cross-asset spoofing attempts.
func (h *Handler) postAssetVersionSubset(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	assetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	versionID, ok := parseIDParam(c, "vid")
	if !ok {
		return
	}

	var asset models.Asset
	err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", assetID, lib.ID).
		First(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "asset not found")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}

	var source models.AssetVersion
	err = h.db.WithContext(c.Request.Context()).
		Where("id = ? AND asset_id = ?", versionID, asset.ID).
		First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "version not found")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}

	var req subsetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	if len(req.FileIDs) == 0 {
		respond400(c, asError("file_ids must not be empty"))
		return
	}

	var sourceFiles []models.AssetFile
	if err := h.db.WithContext(c.Request.Context()).
		Where("asset_version_id = ? AND id IN ?", source.ID, req.FileIDs).
		Find(&sourceFiles).Error; err != nil {
		respond500(c, err)
		return
	}
	if len(sourceFiles) != len(req.FileIDs) {
		respond400(c, asError("one or more file_ids do not belong to the source version"))
		return
	}

	label := req.VersionLabel

	var newVersion models.AssetVersion
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		newVersion = models.AssetVersion{
			AssetID:         asset.ID,
			VersionLabel:    label,
			ImportDate:      time.Now().UTC(),
			PackageKind:     source.PackageKind,
			Notes:           "subset of " + versionDisplayName(source),
			CreatedByUserID: &u.ID,
		}
		// Subsets don't retain the package archive — they're virtual versions
		// pointing at the same content-addressed objects as the source.
		if err := tx.Create(&newVersion).Error; err != nil {
			return err
		}
		rows := make([]models.AssetFile, 0, len(sourceFiles))
		for _, f := range sourceFiles {
			rows = append(rows, models.AssetFile{
				AssetVersionID: newVersion.ID,
				RelativePath:   f.RelativePath,
				SizeBytes:      f.SizeBytes,
				ContentHash:    f.ContentHash,
				ObjectKey:      f.ObjectKey,
				DetectedType:   f.DetectedType,
				UnityGUID:      f.UnityGUID,
			})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
		asset.CurrentVersionID = &newVersion.ID
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.version.subset",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff: audit.Created(map[string]any{
				"asset_id":          asset.ID,
				"source_version":    source.ID,
				"new_version":       newVersion.ID,
				"file_count":        len(rows),
				"new_version_label": label,
			}),
		})
	})
	if err != nil {
		respond500(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"asset_id":         asset.ID,
		"asset_version_id": newVersion.ID,
		"version_label":    newVersion.VersionLabel,
		"file_count":       len(sourceFiles),
	})
}
