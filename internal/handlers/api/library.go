package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
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
		respond500(c)
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
		respond500(c)
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
// Postgres FK cascades reach assets/projects/collections/tags/settings/
// memberships from the libraries row directly, plus their child rows.
// SQLite (test backend) does not enforce FKs by default, so we walk the
// child tables explicitly inside the same transaction; the writes are
// idempotent against Postgres.
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

	var deletedName string
	var memberCount int
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var lib models.Library
		if err := tx.First(&lib, libID).Error; err != nil {
			return err
		}
		deletedName = lib.Name

		// Capture every member so we can bump their token watermark and
		// clear current_library_id pointing at this library.
		var memberIDs []uint
		if err := tx.Model(&models.LibraryMembership{}).
			Where("library_id = ?", libID).
			Pluck("user_id", &memberIDs).Error; err != nil {
			return err
		}
		memberCount = len(memberIDs)

		// Walk the children that the SQLite test path doesn't cascade.
		// Order matters: leaf rows first, then the rows they reference.
		var assetIDs []uint
		if err := tx.Model(&models.Asset{}).
			Where("library_id = ?", libID).
			Pluck("id", &assetIDs).Error; err != nil {
			return err
		}
		if len(assetIDs) > 0 {
			var versionIDs []uint
			if err := tx.Model(&models.AssetVersion{}).
				Where("asset_id IN ?", assetIDs).
				Pluck("id", &versionIDs).Error; err != nil {
				return err
			}
			if len(versionIDs) > 0 {
				if err := tx.Where("asset_version_id IN ?", versionIDs).
					Delete(&models.AssetFile{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("asset_id IN ?", assetIDs).
				Delete(&models.AssetVersion{}).Error; err != nil {
				return err
			}
			if err := tx.Where("asset_id IN ?", assetIDs).Delete(&models.AssetTag{}).Error; err != nil {
				return err
			}
			if err := tx.Where("asset_id IN ?", assetIDs).Delete(&models.CollectionAsset{}).Error; err != nil {
				return err
			}
			if err := tx.Where("asset_id IN ?", assetIDs).Delete(&models.ProjectAssetPin{}).Error; err != nil {
				return err
			}
			if err := tx.Where("asset_id IN ?", assetIDs).Delete(&models.Licence{}).Error; err != nil {
				return err
			}
			if err := tx.Where("asset_id IN ?", assetIDs).Delete(&models.AssetImport{}).Error; err != nil {
				return err
			}
			// Break asset.current_version_id / asset.licence_id self-refs
			// before the asset rows go.
			if err := tx.Model(&models.Asset{}).
				Where("library_id = ?", libID).
				Updates(map[string]any{"current_version_id": nil, "licence_id": nil}).Error; err != nil {
				return err
			}
			if err := tx.Where("library_id = ?", libID).Delete(&models.Asset{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("library_id = ?", libID).Delete(&models.Project{}).Error; err != nil {
			return err
		}
		if err := tx.Where("library_id = ?", libID).Delete(&models.Collection{}).Error; err != nil {
			return err
		}
		if err := tx.Where("library_id = ?", libID).Delete(&models.Tag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("library_id = ?", libID).Delete(&models.LibrarySettings{}).Error; err != nil {
			return err
		}
		if err := tx.Where("library_id = ?", libID).Delete(&models.LibraryMembership{}).Error; err != nil {
			return err
		}
		// Audit_log.library_id ON DELETE CASCADE means in-library audit rows
		// disappear with the library. Drop them explicitly for the SQLite
		// test path; harmless on Postgres.
		if err := tx.Where("library_id = ?", libID).Delete(&models.AuditLog{}).Error; err != nil {
			return err
		}
		// Force every former member off the library: clear current_library_id
		// pointing here and bump tokens_valid_after so any open tab fails
		// 401 on its next API call instead of holding stale state.
		if len(memberIDs) > 0 {
			if err := tx.Model(&models.User{}).
				Where("id IN ? AND current_library_id = ?", memberIDs, libID).
				Update("current_library_id", nil).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			if err := tx.Model(&models.User{}).
				Where("id IN ?", memberIDs).
				Update("tokens_valid_after", &now).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&lib).Error; err != nil {
			return err
		}
		var actorID *uint
		if actor != nil {
			actorID = &actor.ID
		}
		// LibraryID is intentionally nil — the library row is gone, so the
		// audit row would cascade-delete with it. Recording at server scope
		// preserves the deletion event in /api/admin/audit.
		return audit.Write(tx, audit.Entry{
			LibraryID:  nil,
			UserID:     actorID,
			ActionType: "library.delete",
			TargetType: "library",
			TargetID:   &libID,
			Diff: audit.Deleted(map[string]any{
				"library_id":          libID,
				"name":                deletedName,
				"memberships_removed": memberCount,
				"asset_count":         len(assetIDs),
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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	u.CurrentLibraryID = &req.LibraryID
	if err := h.db.WithContext(c.Request.Context()).Save(u).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"library_id": req.LibraryID})
}
