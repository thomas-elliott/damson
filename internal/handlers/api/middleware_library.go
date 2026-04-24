package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

const libraryContextKey = "damson.library"

// requireLibrary ensures the authenticated user has a current library AND
// is a member of it. On success, the Library row is attached to the Gin
// context for handlers to read via getLibrary. Failures surface as 403 —
// never 404 — so membership existence isn't leaked.
//
// Library-scoped routes (assets, projects, collections, tags, etc.) mount
// this middleware in addition to auth.Middleware.
func (h *handler) requireLibrary() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := auth.GetUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if u.CurrentLibraryID == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no current library"})
			return
		}

		var mem models.LibraryMembership
		err := h.db.WithContext(c.Request.Context()).
			Where("library_id = ? AND user_id = ?", *u.CurrentLibraryID, u.ID).
			First(&mem).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not a member of the current library"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		var lib models.Library
		if err := h.db.WithContext(c.Request.Context()).First(&lib, *u.CurrentLibraryID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.Set(libraryContextKey, &lib)
		c.Next()
	}
}

// getLibrary returns the library resolved by requireLibrary. Handlers
// registered under that middleware may call it without a nil check.
func getLibrary(c *gin.Context) *models.Library {
	v, ok := c.Get(libraryContextKey)
	if !ok {
		return nil
	}
	lib, _ := v.(*models.Library)
	return lib
}
