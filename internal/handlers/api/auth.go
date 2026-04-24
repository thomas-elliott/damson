package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/models"
)

// loginRequest is the POST /api/auth/login payload for local auth mode.
type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type authModeResponse struct {
	Mode string `json:"mode"` // "oidc" | "local"
	// NeedsBootstrap is true in local mode iff no users exist yet, i.e. the
	// SPA should render the "create first admin" page rather than the login
	// form. Always absent/false in OIDC mode.
	NeedsBootstrap bool `json:"needs_bootstrap,omitempty"`
}

// getAuthMode is public — the SPA calls it before auth to decide whether to
// render a login form (local), a bootstrap form (local, no users yet), or
// start an OIDC redirect.
func (h *handler) getAuthMode(c *gin.Context) {
	resp := authModeResponse{Mode: string(h.authMode)}
	if h.authMode == config.AuthModeLocal {
		var n int64
		if err := h.db.WithContext(c.Request.Context()).Model(&models.User{}).Count(&n).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		resp.NeedsBootstrap = n == 0
	}
	c.JSON(http.StatusOK, resp)
}

// postAuthLogin is only registered in local mode. OIDC tokens come from the
// identity provider; this endpoint does not exist in OIDC deployments.
func (h *handler) postAuthLogin(c *gin.Context) {
	if h.local == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, exp, err := h.local.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token, ExpiresAt: exp})
}

// postAuthBootstrap creates the very first local-auth admin when no users
// exist yet. Only registered in local mode. Returns 409 once a user has
// been created, so it's safe to leave wired permanently — the endpoint
// self-closes the moment bootstrap succeeds.
func (h *handler) postAuthBootstrap(c *gin.Context) {
	if h.local == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := auth.CreateFirstLocalAdmin(c.Request.Context(), h.db, req.Email, req.Password); err != nil {
		if errors.Is(err, auth.ErrAlreadyBootstrapped) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "already bootstrapped"})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Log the new user in immediately so the SPA can drop them straight
	// into the library view. Reuses Login's bcrypt comparison — we just
	// hashed this password, so it will succeed.
	token, exp, err := h.local.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, loginResponse{Token: token, ExpiresAt: exp})
}
