package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

const (
	libraryContextKey    = "damson.library"
	membershipContextKey = "damson.library_membership"
)

// requireLibrary ensures the authenticated user belongs to a library and
// scopes the request to it. On success, the Library and LibraryMembership
// rows are attached to the Gin context for handlers to read via getLibrary /
// getMembership. Failures surface as 403 — never 404 — so membership
// existence isn't leaked.
//
// Library-scoped routes (assets, projects, collections, tags, etc.) mount
// this middleware in addition to auth.Middleware.
func (h *Handler) requireLibrary() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := auth.GetUser(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		lib, mem, err := h.resolveLibrary(c, u)
		if err != nil {
			respond500(c, err)
			return
		}
		if lib == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not a member of any library"})
			return
		}
		c.Set(libraryContextKey, lib)
		c.Set(membershipContextKey, mem)
		c.Next()
	}
}

// resolveLibrary picks the library a request works in: the user's current
// library while they still belong to it, otherwise the library they do belong
// to. That covers an invited member, whose current library is never set. A
// personal access token is pinned to its own library and never falls back.
// Both nil means the user belongs to no library.
//
// One JOIN fetches both rows in one round trip: every library-scoped request
// runs through here.
func (h *Handler) resolveLibrary(c *gin.Context, u *models.User) (*models.Library, *models.LibraryMembership, error) {
	var current uint
	if u.CurrentLibraryID != nil {
		current = *u.CurrentLibraryID
	}
	q := h.db.WithContext(c.Request.Context()).
		Table("library_memberships AS m").
		Select(`l.id AS library_id, l.name AS library_name, l.description AS library_description,
			l.created_at AS library_created_at, l.updated_at AS library_updated_at,
			m.id AS membership_id, m.role AS membership_role, m.created_at AS membership_created`).
		Joins("JOIN libraries l ON l.id = m.library_id").
		Where("m.user_id = ?", u.ID)
	if p := auth.GetPrincipal(c); p != nil && p.Token != nil {
		q = q.Where("m.library_id = ?", current)
	}

	var row struct {
		LibraryID          uint
		LibraryName        string
		LibraryDescription string
		LibraryCreatedAt   time.Time
		LibraryUpdatedAt   time.Time
		MembershipID       uint
		MembershipRole     string
		MembershipCreated  time.Time
	}
	err := q.Order(clause.OrderBy{Expression: clause.Expr{SQL: "m.library_id = ? DESC, m.library_id", Vars: []any{current}}}).
		Limit(1).
		Scan(&row).Error
	if err != nil || row.LibraryID == 0 {
		return nil, nil, err
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
	return lib, mem, nil
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
