package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

type libraryMembershipView struct {
	LibraryID   uint   `json:"library_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Role        string `json:"role"` // 'admin' | 'member'
	IsCurrent   bool   `json:"is_current"`
}

// getLibraries returns the libraries the authenticated user is a member of,
// with their role per library. This is the list that populates the library
// switcher in the SPA.
func (h *Handler) getLibraries(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type row struct {
		LibraryID   uint
		Name        string
		Description string
		Role        string
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("library_memberships AS lm").
		Select("lm.library_id, l.name, l.description, lm.role").
		Joins("JOIN libraries l ON l.id = lm.library_id").
		Where("lm.user_id = ?", u.ID).
		Order("l.name").
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
		return
	}

	out := make([]libraryMembershipView, len(rows))
	for i, r := range rows {
		out[i] = libraryMembershipView{
			LibraryID:   r.LibraryID,
			Name:        r.Name,
			Description: r.Description,
			Role:        r.Role,
			IsCurrent:   u.CurrentLibraryID != nil && *u.CurrentLibraryID == r.LibraryID,
		}
	}
	c.JSON(http.StatusOK, gin.H{"libraries": out})
}

type setCurrentLibraryRequest struct {
	LibraryID uint `json:"library_id" binding:"required"`
}

type createLibraryRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
}

// postLibrary creates a new library with the caller as its admin. Any
// authenticated user may create a library — there's no global gate. The
// caller's current_library_id is left untouched; switching is an explicit
// follow-up via /api/me/current-library.
func (h *Handler) postLibrary(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req createLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		respond400(c, asError("name must not be blank"))
		return
	}

	lib := models.Library{Name: name, Description: req.Description}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&lib).Error; err != nil {
			return err
		}
		mem := models.LibraryMembership{LibraryID: lib.ID, UserID: u.ID, Role: "admin"}
		if err := tx.Create(&mem).Error; err != nil {
			return err
		}
		actorID := u.ID
		libID := lib.ID
		return audit.Write(tx, audit.Entry{
			LibraryID:  &libID,
			UserID:     &actorID,
			ActionType: "library.create",
			TargetType: "library",
			TargetID:   &libID,
			Diff: audit.Created(map[string]any{
				"name":        lib.Name,
				"description": lib.Description,
			}),
		})
	})
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":          lib.ID,
		"name":        lib.Name,
		"description": lib.Description,
	})
}

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

// deleteLibrary tears down a library and every row scoped to it. Library
// admins (or sysadmins) may call this — same gate as patchLibrary.
//
// Deleting the libraries row removes the rest: every library-scoped table
// references it ON DELETE CASCADE, and each asset's children cascade from
// the asset. users.current_library_id is ON DELETE SET NULL.
//
// The deletion event is recorded with library_id=NULL so the row survives
// `audit_log.library_id ON DELETE CASCADE`. The diff carries the library
// name, id, and the count of memberships removed so the sysadmin server
// feed can render useful detail. Every former member's
// tokens_valid_after is bumped — open SPA tabs are forced to re-fetch and
// land on requireLibrary's "no current library" path on the next call,
// rather than silently keeping a dead library_id in cache.
func (h *Handler) deleteLibrary(c *gin.Context) {
	libID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if !h.userCanAdminLibrary(c, libID) {
		respondForbidden(c, "library admin required")
		return
	}
	actor := auth.GetUser(c)

	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var lib models.Library
		if err := tx.First(&lib, libID).Error; err != nil {
			return err
		}
		var memberIDs []uint
		if err := tx.Model(&models.LibraryMembership{}).
			Where("library_id = ?", libID).
			Pluck("user_id", &memberIDs).Error; err != nil {
			return err
		}
		var assetCount int64
		if err := tx.Model(&models.Asset{}).Where("library_id = ?", libID).Count(&assetCount).Error; err != nil {
			return err
		}
		if err := tx.Delete(&lib).Error; err != nil {
			return err
		}
		if len(memberIDs) > 0 {
			if err := tx.Model(&models.User{}).
				Where("id IN ?", memberIDs).
				Update("tokens_valid_after", time.Now().UTC()).Error; err != nil {
				return err
			}
		}
		var actorID *uint
		if actor != nil {
			actorID = &actor.ID
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  nil,
			UserID:     actorID,
			ActionType: "library.delete",
			TargetType: "library",
			TargetID:   &libID,
			Diff: audit.Deleted(map[string]any{
				"library_id":          libID,
				"name":                lib.Name,
				"memberships_removed": len(memberIDs),
				"asset_count":         assetCount,
			}),
		})
	})
	if err != nil {
		respondGORM(c, err, "library not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// postCurrentLibrary switches the authenticated user's active library. The
// request fails with 403 if the user isn't a member of the target library.
func (h *Handler) postCurrentLibrary(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req setCurrentLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var mem models.LibraryMembership
	err := h.db.WithContext(c.Request.Context()).
		Where("library_id = ? AND user_id = ?", req.LibraryID, u.ID).
		First(&mem).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not a member of that library"})
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}

	u.CurrentLibraryID = &req.LibraryID
	if err := h.db.WithContext(c.Request.Context()).Save(u).Error; err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"library_id": req.LibraryID})
}
