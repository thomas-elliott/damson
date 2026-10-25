package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/assets"
	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/licence"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/tags"
)

const maxBulkAssets = 1000

type bulkAssetsRequest struct {
	AssetIDs     []uint             `json:"asset_ids"`
	Action       string             `json:"action"`
	Path         string             `json:"path,omitempty"`
	CollectionID uint               `json:"collection_id,omitempty"`
	OriginalText string             `json:"original_text,omitempty"`
	DetectedType string             `json:"detected_type,omitempty"`
	Attributes   licence.Attributes `json:"attributes,omitempty"`
	UseDetected  bool               `json:"use_detected,omitempty"`
}

func (h *Handler) postBulkAssets(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	var req bulkAssetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	ids := dedupeIDs(req.AssetIDs)
	if len(ids) == 0 {
		respond400(c, asError("asset_ids must not be empty"))
		return
	}
	if len(ids) > maxBulkAssets {
		respond400(c, asError("too many assets in one request"))
		return
	}

	req.Path = strings.TrimSpace(req.Path)
	switch req.Action {
	case "delete", "review", "unreview":
	case "add_tag", "remove_tag":
		if req.Path == "" {
			respond400(c, asError("path is required for tag actions"))
			return
		}
	case "add_to_collection":
		if req.CollectionID == 0 {
			respond400(c, asError("collection_id is required"))
			return
		}
	case "set_licence":
		if strings.TrimSpace(req.OriginalText) == "" {
			respond400(c, asError("original_text must not be blank"))
			return
		}
	default:
		respond400(c, asError("unknown bulk action"))
		return
	}

	var lic models.Licence
	if req.Action == "set_licence" {
		var err error
		if lic, err = requestedLicence(req.OriginalText, req.DetectedType, req.Attributes, req.UseDetected); err != nil {
			respond500(c, err)
			return
		}
	}

	affected := 0
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Every id must resolve within the current library; a stray one fails
		// the whole batch (also the only scope check tags.Attach gets).
		var found int64
		if err := tx.Model(&models.Asset{}).
			Where("id IN ? AND library_id = ?", ids, lib.ID).
			Count(&found).Error; err != nil {
			return err
		}
		if int(found) != len(ids) {
			return gorm.ErrRecordNotFound
		}

		var coll models.Collection
		if req.Action == "add_to_collection" {
			if err := tx.Where("id = ? AND library_id = ?", req.CollectionID, lib.ID).
				First(&coll).Error; err != nil {
				return err
			}
		}
		var removeTagID uint
		if req.Action == "remove_tag" {
			var t models.Tag
			if err := tx.Where("library_id = ? AND path = ?", lib.ID, req.Path).
				First(&t).Error; err != nil {
				return err
			}
			removeTagID = t.ID
		}

		for _, id := range ids {
			var (
				changed bool
				err     error
			)
			switch req.Action {
			case "delete":
				err = assets.Delete(tx, lib.ID, u.ID, id)
				changed = err == nil
			case "review":
				changed, err = bulkSetReview(tx, lib.ID, u.ID, id, false)
			case "unreview":
				changed, err = bulkSetReview(tx, lib.ID, u.ID, id, true)
			case "add_tag":
				_, changed, err = tags.Attach(tx, lib.ID, &u.ID, id, req.Path)
			case "remove_tag":
				changed, err = bulkRemoveTag(tx, lib.ID, u.ID, id, removeTagID)
			case "add_to_collection":
				changed, err = bulkAddToCollection(tx, lib.ID, u.ID, coll.ID, id)
			case "set_licence":
				l := lic
				err = setLicenceTx(tx, lib.ID, u.ID, id, &l)
				changed = err == nil
			}
			if err != nil {
				return err
			}
			if changed {
				affected++
			}
		}
		return nil
	})
	if err != nil {
		respondGORM(c, err, "one or more assets not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"affected": affected})
}

func dedupeIDs(in []uint) []uint {
	seen := make(map[uint]bool, len(in))
	out := make([]uint, 0, len(in))
	for _, id := range in {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func bulkSetReview(tx *gorm.DB, libID, userID, assetID uint, needsReview bool) (bool, error) {
	var asset models.Asset
	if err := tx.Where("id = ? AND library_id = ?", assetID, libID).First(&asset).Error; err != nil {
		return false, err
	}
	if asset.NeedsReview == needsReview {
		return false, nil
	}
	before := map[string]any{"needs_review": asset.NeedsReview}
	asset.NeedsReview = needsReview
	if err := tx.Save(&asset).Error; err != nil {
		return false, err
	}
	action := "asset.review"
	if needsReview {
		action = "asset.unreview"
	}
	return true, audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: action,
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff:       audit.Updated(before, map[string]any{"needs_review": needsReview}),
	})
}

func bulkRemoveTag(tx *gorm.DB, libID, userID, assetID, tagID uint) (bool, error) {
	res := tx.Where("asset_id = ? AND tag_id = ?", assetID, tagID).Delete(&models.AssetTag{})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	return true, audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "asset.untag",
		TargetType: "asset",
		TargetID:   &assetID,
		Diff:       audit.Deleted(map[string]any{"tag_id": tagID}),
	})
}

func bulkAddToCollection(tx *gorm.DB, libID, userID, collID, assetID uint) (bool, error) {
	link := models.CollectionAsset{CollectionID: collID, AssetID: assetID, AddedByUserID: &userID}
	res := tx.Where("collection_id = ? AND asset_id = ?", collID, assetID).
		Attrs(&link).FirstOrCreate(&link)
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	return true, audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "collection.add",
		TargetType: "collection",
		TargetID:   &collID,
		Diff:       audit.Created(map[string]any{"asset_id": assetID}),
	})
}
