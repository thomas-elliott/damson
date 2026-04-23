package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
func (h *handler) getLibraries(c *gin.Context) {
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

// postCurrentLibrary switches the authenticated user's active library. The
// request fails with 403 if the user isn't a member of the target library.
func (h *handler) postCurrentLibrary(c *gin.Context) {
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
