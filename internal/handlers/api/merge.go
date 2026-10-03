package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
)

type mergeRequest struct {
	SourceAssetID uint `json:"source_asset_id" binding:"required"`
}

type mergeAdoptable struct {
	Name                 string `json:"name,omitempty"`
	Description          string `json:"description,omitempty"`
	ThumbnailObjectKey   string `json:"thumbnail_object_key,omitempty"`
	ThumbnailContentType string `json:"thumbnail_content_type,omitempty"`
}

type mergeResponse struct {
	TargetAssetID  uint           `json:"target_asset_id"`
	MergedVersions int            `json:"merged_versions"`
	Adoptable      mergeAdoptable `json:"adoptable"`
}

func (h *Handler) postAssetMerge(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	targetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req mergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	if req.SourceAssetID == targetID {
		respond400(c, asError("cannot merge an asset into itself"))
		return
	}

	var resp mergeResponse
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var target, source models.Asset
		if err := tx.Where("id = ? AND library_id = ?", targetID, lib.ID).First(&target).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND library_id = ?", req.SourceAssetID, lib.ID).First(&source).Error; err != nil {
			return err
		}
		// Merging moves versions, which a reference asset neither has nor
		// takes; folding one into the other is the deferred conversion step.
		if target.IsReference || source.IsReference {
			return errConflictf("reference assets can't be merged")
		}

		targetByStore := map[string]uint{}
		var targetVersions []models.AssetVersion
		if err := tx.Where("asset_id = ?", target.ID).Find(&targetVersions).Error; err != nil {
			return err
		}
		for _, v := range targetVersions {
			if v.StoreVersion != "" {
				targetByStore[v.StoreVersion] = v.ID
			}
		}

		var sourceVersions []models.AssetVersion
		if err := tx.Where("asset_id = ?", source.ID).Find(&sourceVersions).Error; err != nil {
			return err
		}
		// A source version whose store version already exists on the target
		// can't move (the partial unique index forbids it); it stays on the
		// source to cascade-delete, and its pins are remapped to the target's
		// equivalent below.
		dropRemap := map[uint]uint{}
		moved := 0
		for _, v := range sourceVersions {
			if v.StoreVersion != "" {
				if tid, dup := targetByStore[v.StoreVersion]; dup {
					dropRemap[v.ID] = tid
					continue
				}
			}
			if err := tx.Model(&models.AssetVersion{}).Where("id = ?", v.ID).
				Update("asset_id", target.ID).Error; err != nil {
				return err
			}
			moved++
		}

		if err := moveAssetTags(tx, source.ID, target.ID); err != nil {
			return err
		}
		if err := moveCollectionRefs(tx, source.ID, target.ID); err != nil {
			return err
		}
		if err := movePins(tx, source.ID, target.ID, dropRemap); err != nil {
			return err
		}

		if err := tx.Delete(&models.Asset{}, source.ID).Error; err != nil {
			return err
		}

		resp = mergeResponse{
			TargetAssetID:  target.ID,
			MergedVersions: moved,
			Adoptable: mergeAdoptable{
				Name:                 source.Name,
				Description:          source.Description,
				ThumbnailObjectKey:   source.ThumbnailObjectKey,
				ThumbnailContentType: source.ThumbnailContentType,
			},
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.merge",
			TargetType: "asset",
			TargetID:   &target.ID,
			Diff: audit.Created(map[string]any{
				"source_asset_id": source.ID,
				"merged_versions": moved,
			}),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.JSON(http.StatusOK, resp)
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
		if remapped, ok := dropRemap[p.PinnedVersionID]; ok {
			updates["pinned_version_id"] = remapped
		}
		if err := tx.Model(&models.ProjectAssetPin{}).Where("id = ?", p.ID).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}
