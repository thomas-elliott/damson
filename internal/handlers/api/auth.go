package api

import (
	"errors"
	"net/http"
	"strings"
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
	// OIDC carries the non-secret issuer + client_id the SPA needs to
	// configure oidc-client-ts at runtime (the browser derives its redirect
	// URIs from window.location.origin). Present only in OIDC mode; absent in
	// local mode. This is what lets one prebuilt image target any IdP without
	// baking VITE_OIDC_* at build time.
	OIDC *oidcPublic `json:"oidc,omitempty"`
}

// oidcPublic is the non-secret OIDC client config handed to the browser.
type oidcPublic struct {
	Authority string `json:"authority"`
	ClientID  string `json:"client_id"`
}

// getAuthMode is public — the SPA calls it before auth to decide whether to
// render a login form (local), a bootstrap form (local, no users yet), or
// start an OIDC redirect.
func (h *Handler) getAuthMode(c *gin.Context) {
	resp := authModeResponse{Mode: string(h.authMode)}
	switch h.authMode {
	case config.AuthModeLocal:
		var n int64
		if err := h.db.WithContext(c.Request.Context()).Model(&models.User{}).Count(&n).Error; err != nil {
			respond500(c, err)
			return
		}
		resp.NeedsBootstrap = n == 0
	case config.AuthModeOIDC:
		resp.OIDC = &oidcPublic{Authority: h.oidcAuthority, ClientID: h.oidcClientID}
	}
	c.JSON(http.StatusOK, resp)
}

// postAuthLogin is only registered in local mode. OIDC tokens come from the
// identity provider; this endpoint does not exist in OIDC deployments.
func (h *Handler) postAuthLogin(c *gin.Context) {
	if h.local == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Per-IP throttle gate first (cheap), then per-email so one IP can't
	// exhaust a colleague's bucket but the same client can't iterate
	// passwords for one account either. bcrypt at default cost handles the
	// rest.
	if !h.loginRate.Allow("ip:"+c.ClientIP()) ||
		!h.loginRate.Allow("email:"+strings.ToLower(strings.TrimSpace(req.Email))) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts; try again shortly"})
		return
	}

	token, exp, err := h.local.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token, ExpiresAt: exp})
}

// postAuthRefresh issues a fresh JWT for the authenticated user. Local mode
// only — OIDC clients use the IdP's silent-renew flow. The auth middleware
// has already validated the bearer (including the tokens_valid_after
// watermark), so this handler just mints a new token off the same user.
//
// The SPA calls this proactively when its stored token is within a few
// minutes of expiry, so users don't get bounced to /login mid-session.
func (h *Handler) postAuthRefresh(c *gin.Context) {
	if h.local == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	token, exp, err := h.local.Refresh(u)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token, ExpiresAt: exp})
}

// postAuthBootstrap creates the very first local-auth admin when no users
// exist yet. Only registered in local mode. Returns 409 once a user has
// been created, so it's safe to leave wired permanently — the endpoint
// self-closes the moment bootstrap succeeds.
func (h *Handler) postAuthBootstrap(c *gin.Context) {
	if h.local == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Throttle the bootstrap path on the caller's IP — same bucket family as
	// /api/auth/login. Bootstrap closes itself after the first success (409
	// thereafter), so this is mostly anti-scan protection.
	if !h.loginRate.Allow("ip:" + c.ClientIP()) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many bootstrap attempts; try again shortly"})
		return
	}

	if _, err := auth.CreateFirstLocalAdmin(c.Request.Context(), h.db, req.Email, req.Password); err != nil {
		if errors.Is(err, auth.ErrAlreadyBootstrapped) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "already bootstrapped"})
			return
		}
		if errors.Is(err, auth.ErrPasswordTooShort) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		respond500(c, err)
		return
	}

	// Log the new user in immediately so the SPA can drop them straight
	// into the library view. Reuses Login's bcrypt comparison — we just
	// hashed this password, so it will succeed.
	token, exp, err := h.local.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusCreated, loginResponse{Token: token, ExpiresAt: exp})
}
