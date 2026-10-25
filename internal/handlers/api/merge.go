package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/assets"
	"github.com/thomas-elliott/damson/internal/auth"
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

// postAssetMerge folds the source asset into this one. See assets.Merge.
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

	var merged assets.Merged
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		merged, err = assets.Merge(tx, lib.ID, u.ID, targetID, req.SourceAssetID)
		return err
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.JSON(http.StatusOK, mergeResponse{
		TargetAssetID:  targetID,
		MergedVersions: merged.Versions,
		Adoptable: mergeAdoptable{
			Name:                 merged.Source.Name,
			Description:          merged.Source.Description,
			ThumbnailObjectKey:   merged.Source.ThumbnailObjectKey,
			ThumbnailContentType: merged.Source.ThumbnailContentType,
		},
	})
}
