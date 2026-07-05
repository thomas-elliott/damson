package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

const (
	libraryContextKey    = "damson.library"
	membershipContextKey = "damson.library_membership"
)

// requireLibrary ensures the authenticated user has a current library AND
// is a member of it. On success, the Library and LibraryMembership rows are
// attached to the Gin context for handlers to read via getLibrary /
// getMembership. Failures surface as 403 — never 404 — so membership
// existence isn't leaked.
//
// Library-scoped routes (assets, projects, collections, tags, etc.) mount
// this middleware in addition to auth.Middleware.
//
// Implementation note: a single JOIN fetches both rows in one round-trip.
// This is a hot path — every library-scoped endpoint runs through it.
func (h *Handler) requireLibrary() gin.HandlerFunc {
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

		type joinedRow struct {
			LibraryID          uint
			LibraryName        string
			LibraryDescription string
			LibraryCreatedAt   time.Time
			LibraryUpdatedAt   time.Time
			MembershipID       uint
			MembershipRole     string
			MembershipCreated  time.Time
		}
		var row joinedRow
		err := h.db.WithContext(c.Request.Context()).
			Table("library_memberships AS m").
			Select(`l.id AS library_id, l.name AS library_name, l.description AS library_description,
				l.created_at AS library_created_at, l.updated_at AS library_updated_at,
				m.id AS membership_id, m.role AS membership_role, m.created_at AS membership_created`).
			Joins("JOIN libraries l ON l.id = m.library_id").
			Where("m.library_id = ? AND m.user_id = ?", *u.CurrentLibraryID, u.ID).
			Limit(1).
			Scan(&row).Error
		if err != nil {
			respond500(c, err)
			return
		}
		if row.LibraryID == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not a member of the current library"})
			return
		}

		lib := &models.Library{
			ID:          row.LibraryID,
			Name:        row.LibraryName,
			Description: row.LibraryDescription,
			CreatedAt:   row.LibraryCreatedAt,
			UpdatedAt:   row.LibraryUpdatedAt,
		}
		mem := &models.LibraryMembership{
			ID:        row.MembershipID,
			LibraryID: row.LibraryID,
			UserID:    u.ID,
			Role:      row.MembershipRole,
			CreatedAt: row.MembershipCreated,
		}
		c.Set(libraryContextKey, lib)
		c.Set(membershipContextKey, mem)
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

// getMembership returns the membership row resolved by requireLibrary — used
// by handlers that need the user's role in the current library (e.g. gating
// library-admin actions).
func getMembership(c *gin.Context) *models.LibraryMembership {
	v, ok := c.Get(membershipContextKey)
	if !ok {
		return nil
	}
	m, _ := v.(*models.LibraryMembership)
	return m
}
