package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/assets"
)

type subsetRequest struct {
	FileIDs      []uint `json:"file_ids" binding:"required"`
	VersionLabel string `json:"version_label,omitempty"`
}

// postAssetVersionSubset creates a new version whose asset_files are a
// subset of the source version's files. Any `file_ids` that don't belong to
// the source version are rejected with 400 — this catches both stale UI
// state and cross-asset spoofing attempts. See assets.Subset.
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
	var req subsetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}

	var version models.AssetVersion
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		version, err = assets.Subset(tx, h.previewPool, lib.ID, u.ID, assetID, versionID, req.FileIDs, req.VersionLabel)
		return err
	})
	if err != nil {
		respondGORM(c, err, "asset or version not found")
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"asset_id":         assetID,
		"asset_version_id": version.ID,
		"version_label":    version.VersionLabel,
		"file_count":       len(req.FileIDs),
	})
}
