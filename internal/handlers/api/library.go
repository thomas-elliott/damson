package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

type patchLibraryRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// patchLibrary updates the library's name and/or description. Library
// admins (or sysadmins) may call this — gated via userCanAdminLibrary.
// Empty `name` is rejected; description can be cleared to empty string.
func (h *Handler) patchLibrary(c *gin.Context) {
	libID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if !h.userCanAdminLibrary(c, libID) {
		respondForbidden(c, "library admin required")
		return
	}
	var req patchLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	var lib models.Library
	if err := h.db.WithContext(c.Request.Context()).First(&lib, libID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respond404(c, "library not found")
			return
		}
		respond500(c, err)
		return
	}
	before := map[string]any{}
	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			respond400(c, asError("name must not be blank"))
			return
		}
		before["name"] = lib.Name
		lib.Name = name
		updates["name"] = name
	}
	if req.Description != nil {
		before["description"] = lib.Description
		lib.Description = *req.Description
		updates["description"] = lib.Description
	}
	actor := auth.GetUser(c)
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if len(updates) == 0 {
			return nil
		}
		if err := tx.Model(&lib).Updates(updates).Error; err != nil {
			return err
		}
		var actorID *uint
		if actor != nil {
			actorID = &actor.ID
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &libID,
			UserID:     actorID,
			ActionType: "library.update",
			TargetType: "library",
			TargetID:   &libID,
			Diff:       audit.Updated(before, updates),
		})
	})
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          lib.ID,
		"name":        lib.Name,
		"description": lib.Description,
	})
}
