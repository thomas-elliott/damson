package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

type currentLibrary struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Role        string `json:"role"` // 'admin' | 'member'
}

type meResponse struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"` // 'user' | 'sysadmin'
	// NeedsProfile is true when the user has no email yet — i.e. the IdP sent
	// no email claim at first sign-in. The SPA gates on this to show the
	// first-login profile step before the rest of the app.
	NeedsProfile   bool            `json:"needs_profile"`
	CurrentLibrary *currentLibrary `json:"current_library,omitempty"`
	// Token describes the personal access token this request authenticated
	// with, and is absent for an interactive session. It lets a token holder
	// see its own scope and expiry without reaching /api/me/tokens, which
	// token auth is refused — describing one token is not enumerating them.
	Token *tokenPrincipalView `json:"token,omitempty"`
}

type tokenPrincipalView struct {
	Prefix    string     `json:"prefix"`
	Label     string     `json:"label"`
	Scope     string     `json:"scope"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (h *Handler) getMe(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	h.respondMe(c, u)
}

// patchMeRequest is the PATCH /api/me payload. Email is required (it's the
// field a first-login user fills in when the IdP sent no email claim); display
// name is optional and defaults to the email when blank.
type patchMeRequest struct {
	Email       string `json:"email" binding:"required"`
	DisplayName string `json:"display_name"`
}

// patchMe lets the authenticated user fill in / update their own profile. OIDC
// identity is the immutable `sub`; email and display name are app-owned, so
// this is the only path that writes them after account creation.
func (h *Handler) patchMe(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req patchMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = email
	}

	if err := h.db.WithContext(c.Request.Context()).
		Model(&models.User{}).
		Where("id = ?", u.ID).
		Updates(map[string]any{
			"email":        email,
			"display_name": displayName,
		}).Error; err != nil {
		respond500(c, err)
		return
	}
	u.Email = email
	u.DisplayName = displayName

	h.respondMe(c, u)
}

// respondMe builds the /api/me response for u, resolving the current library
// block when present. Shared by getMe and patchMe.
func (h *Handler) respondMe(c *gin.Context, u *models.User) {
	resp := meResponse{
		ID:           u.ID,
		Email:        u.Email,
		DisplayName:  u.DisplayName,
		Role:         u.Role,
		NeedsProfile: u.Email == "",
	}

	if p := auth.GetPrincipal(c); p != nil && p.Token != nil {
		resp.Token = &tokenPrincipalView{
			Prefix:    p.Token.Prefix,
			Label:     p.Token.Label,
			Scope:     p.Token.Scope,
			ExpiresAt: p.Token.ExpiresAt,
		}
	}

	// A user who belongs to no library gets no block, and the SPA shows the
	// "No library access yet" state.
	lib, mem, err := h.resolveLibrary(c, u)
	if err != nil {
		respond500(c, err)
		return
	}
	if lib != nil {
		resp.CurrentLibrary = &currentLibrary{
			ID:          lib.ID,
			Name:        lib.Name,
			Description: lib.Description,
			Role:        mem.Role,
		}
	}

	c.JSON(http.StatusOK, resp)
}
