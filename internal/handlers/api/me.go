package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
)

type currentLibrary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // 'admin' | 'member'
}

type meResponse struct {
	ID             uint            `json:"id"`
	Email          string          `json:"email"`
	DisplayName    string          `json:"display_name"`
	Role           string          `json:"role"` // 'user' | 'sysadmin'
	CurrentLibrary *currentLibrary `json:"current_library,omitempty"`
}

func (h *Handler) getMe(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	resp := meResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
	}

	if u.CurrentLibraryID != nil {
		// Resolve library + role in one query; if the user's current_library
		// points to a library they're no longer a member of, omit the block
		// (the SPA then surfaces the "no library access yet" state).
		type row struct {
			Name string
			Role string
		}
		var r row
		err := h.db.WithContext(c.Request.Context()).
			Table("library_memberships AS lm").
			Select("l.name, lm.role").
			Joins("JOIN libraries l ON l.id = lm.library_id").
			Where("lm.library_id = ? AND lm.user_id = ?", *u.CurrentLibraryID, u.ID).
			Take(&r).Error
		switch {
		case err == nil:
			resp.CurrentLibrary = &currentLibrary{
				ID:   *u.CurrentLibraryID,
				Name: r.Name,
				Role: r.Role,
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			// User's current_library points at a library they've been removed
			// from. Omit the block so the SPA surfaces the "no library access
			// yet" state.
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}
