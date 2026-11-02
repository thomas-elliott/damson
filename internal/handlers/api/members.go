package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/models"
)

type memberView struct {
	UserID      uint   `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"` // 'admin' | 'member'
	GlobalRole  string `json:"global_role"`
}

type memberListResponse struct {
	Members []memberView `json:"members"`
}

// getLibraryMembers lists every membership in the path's library. Library
// members can read this; admin gating only applies to writes.
func (h *Handler) getLibraryMembers(c *gin.Context) {
	libID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if !h.userIsLibraryMember(c, libID) {
		respondForbidden(c, "not a member of that library")
		return
	}
	type row struct {
		UserID      uint
		Email       string
		DisplayName string
		Role        string
		GlobalRole  string
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("library_memberships AS m").
		Select("m.user_id, u.email, u.display_name, m.role, u.role AS global_role").
		Joins("JOIN users u ON u.id = m.user_id").
		Where("m.library_id = ?", libID).
		Order("u.display_name").
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
		return
	}
	out := make([]memberView, len(rows))
	for i, r := range rows {
		out[i] = memberView{
			UserID:      r.UserID,
			Email:       r.Email,
			DisplayName: r.DisplayName,
			Role:        r.Role,
			GlobalRole:  r.GlobalRole,
		}
	}
	c.JSON(http.StatusOK, memberListResponse{Members: out})
}

type addMemberRequest struct {
	Email       string `json:"email" binding:"required"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role" binding:"required"` // 'admin' | 'member'
	Password    string `json:"password,omitempty"`      // local mode only
}

// postLibraryMember adds a user to a library. Two paths:
//
//   - Existing user (any auth mode): add a membership row, no auth changes.
//   - New user (local auth mode only): create the user with the supplied
//     password, then add the membership. OIDC mode requires the user to
//     have signed in at least once first — there's no admin-driven account
//     creation path because identity lives in the IdP.
//
// Library admins can invite. Sysadmins can invite anywhere. Duplicate
// memberships are 409.
func (h *Handler) postLibraryMember(c *gin.Context) {
	libID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if !h.userCanAdminLibrary(c, libID) {
		respondForbidden(c, "library admin required")
		return
	}
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != "admin" && role != "member" {
		respond400(c, asError("role must be 'admin' or 'member'"))
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		respond400(c, asError("email is required"))
		return
	}

	// Look up an existing user. In OIDC mode we can only attach memberships
	// to users who have signed in at least once; in local mode we create
	// the user when missing if the request supplies a password.
	var u models.User
	err := h.db.WithContext(c.Request.Context()).Where("email = ?", email).First(&u).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if h.authMode != config.AuthModeLocal {
			respond404(c, "user not found — they must sign in once before being invited")
			return
		}
		if req.Password == "" {
			respond400(c, asError("password is required when creating a new local user"))
			return
		}
		created, cErr := auth.CreateLocalUser(c.Request.Context(), h.db.WithContext(c.Request.Context()), email, req.DisplayName, req.Password)
		if cErr != nil {
			if errors.Is(cErr, auth.ErrUserExists) {
				respond409(c, "user already exists")
				return
			}
			if errors.Is(cErr, auth.ErrPasswordTooShort) {
				respond400(c, cErr)
				return
			}
			respond500(c, cErr)
			return
		}
		u = *created
	case err != nil:
		respond500(c, err)
		return
	}

	// Reject duplicate memberships.
	var existing models.LibraryMembership
	err = h.db.WithContext(c.Request.Context()).
		Where("library_id = ? AND user_id = ?", libID, u.ID).
		First(&existing).Error
	if err == nil {
		respond409(c, "user is already a member of this library")
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		respond500(c, err)
		return
	}

	actor := auth.GetUser(c)
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		mem := models.LibraryMembership{
			LibraryID: libID,
			UserID:    u.ID,
			Role:      role,
		}
		if err := tx.Create(&mem).Error; err != nil {
			return err
		}
		var actorID *uint
		if actor != nil {
			actorID = &actor.ID
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &libID,
			UserID:     actorID,
			ActionType: "library.member.add",
			TargetType: "user",
			TargetID:   &u.ID,
			Diff: audit.Created(map[string]any{
				"library_id": libID,
				"user_id":    u.ID,
				"email":      email,
				"role":       role,
			}),
		})
	})
	if err != nil {
		respond500(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id":      u.ID,
		"email":        u.Email,
		"display_name": u.DisplayName,
		"role":         role,
	})
}

type patchMemberRequest struct {
	Role string `json:"role" binding:"required"`
}

// patchLibraryMember changes a member's role. Refuses to demote the last
// admin so a library can't end up adminless.
func (h *Handler) patchLibraryMember(c *gin.Context) {
	libID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if !h.userCanAdminLibrary(c, libID) {
		respondForbidden(c, "library admin required")
		return
	}
	userID, ok := parseIDParam(c, "userId")
	if !ok {
		return
	}
	var req patchMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != "admin" && role != "member" {
		respond400(c, asError("role must be 'admin' or 'member'"))
		return
	}

	actor := auth.GetUser(c)
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		mem, admins, err := lockMembership(tx, libID, userID)
		if err != nil {
			return err
		}
		if mem.Role == "admin" && role != "admin" && admins <= 1 {
			return errConflictf("cannot demote the last admin")
		}
		beforeRole := mem.Role
		mem.Role = role
		if err := tx.Save(&mem).Error; err != nil {
			return err
		}
		var actorID *uint
		if actor != nil {
			actorID = &actor.ID
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &libID,
			UserID:     actorID,
			ActionType: "library.member.role.update",
			TargetType: "user",
			TargetID:   &userID,
			Diff:       audit.Updated(map[string]any{"role": beforeRole}, map[string]any{"role": role}),
		})
	})
	if err != nil {
		respondGORM(c, err, "membership not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
}

// deleteLibraryMember removes a membership. Same last-admin guard as the
// patch path. A user removing themselves is allowed if they're not the
// last admin — this matches typical "leave" behaviour.
func (h *Handler) deleteLibraryMember(c *gin.Context) {
	libID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if !h.userCanAdminLibrary(c, libID) {
		respondForbidden(c, "library admin required")
		return
	}
	userID, ok := parseIDParam(c, "userId")
	if !ok {
		return
	}

	actor := auth.GetUser(c)
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		mem, admins, err := lockMembership(tx, libID, userID)
		if err != nil {
			return err
		}
		if mem.Role == "admin" && admins <= 1 {
			return errConflictf("cannot remove the last admin")
		}
		if err := tx.Delete(&mem).Error; err != nil {
			return err
		}
		// Bump tokens_valid_after so any open session for this user is
		// invalidated immediately. Combined with current_library_id clearing
		// below, the next API call from a stale tab fails 401 → SPA bounces
		// to login. Without this the tab keeps reading "no library access"
		// indefinitely.
		now := time.Now().UTC()
		if err := tx.Model(&models.User{}).
			Where("id = ?", userID).
			Update("tokens_valid_after", &now).Error; err != nil {
			return err
		}
		// Don't leave the removed user pointing at a library they no longer
		// belong to; requireLibrary would skip it anyway.
		if err := tx.Model(&models.User{}).
			Where("id = ? AND current_library_id = ?", userID, libID).
			Update("current_library_id", nil).Error; err != nil {
			return err
		}
		var actorID *uint
		if actor != nil {
			actorID = &actor.ID
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &libID,
			UserID:     actorID,
			ActionType: "library.member.remove",
			TargetType: "user",
			TargetID:   &userID,
			Diff:       audit.Deleted(map[string]any{"role": mem.Role}),
		})
	})
	if err != nil {
		respondGORM(c, err, "membership not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// lockMembership loads a membership and counts the library's admins, holding
// a lock on the admin rows until tx ends. Two admins demoting each other at
// once then see each other's change, so one of them hits the last-admin check.
func lockMembership(tx *gorm.DB, libID, userID uint) (models.LibraryMembership, int, error) {
	var admins []models.LibraryMembership
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("library_id = ? AND role = ?", libID, "admin").
		Find(&admins).Error; err != nil {
		return models.LibraryMembership{}, 0, err
	}
	var mem models.LibraryMembership
	err := tx.Where("library_id = ? AND user_id = ?", libID, userID).First(&mem).Error
	return mem, len(admins), err
}

// userCanAdminLibrary returns true when the authenticated user is either
// a sysadmin or a library admin of libID.
func (h *Handler) userCanAdminLibrary(c *gin.Context, libID uint) bool {
	u := auth.GetUser(c)
	if u == nil {
		return false
	}
	if u.Role == "sysadmin" {
		return true
	}
	var mem models.LibraryMembership
	err := h.db.WithContext(c.Request.Context()).
		Where("library_id = ? AND user_id = ? AND role = ?", libID, u.ID, "admin").
		First(&mem).Error
	return err == nil
}

// userIsLibraryMember returns true when the user has any membership in
// libID. Sysadmins bypass the check.
func (h *Handler) userIsLibraryMember(c *gin.Context, libID uint) bool {
	u := auth.GetUser(c)
	if u == nil {
		return false
	}
	if u.Role == "sysadmin" {
		return true
	}
	var mem models.LibraryMembership
	err := h.db.WithContext(c.Request.Context()).
		Where("library_id = ? AND user_id = ?", libID, u.ID).
		First(&mem).Error
	return err == nil
}
