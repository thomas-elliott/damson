package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
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
}

// getAuthMode is public — the SPA calls it before auth to decide whether to
// render a login form (local) or start an OIDC redirect.
func (h *handler) getAuthMode(c *gin.Context) {
	c.JSON(http.StatusOK, authModeResponse{Mode: string(h.authMode)})
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
