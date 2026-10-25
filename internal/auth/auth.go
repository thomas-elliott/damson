// Package auth exposes the Authenticator interface used by the HTTP
// middleware and two concrete implementations:
//
//   - OIDCAuthenticator — validates IdP-issued bearer tokens, upserts the
//     user from claims.
//   - LocalAuthenticator — issues and verifies server-signed JWTs against
//     users whose credentials live in the database.
//
// Which one is active is a deployment choice (DAMSON_AUTH_MODE=oidc|local).
// Mixed mode is explicitly out of scope.
package auth

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/models"
)

// ErrUnauthenticated signals that a token is missing, malformed, expired, or
// its subject no longer maps to a valid user. It is mapped to 401 by the
// middleware; callers should not rely on wrapping for differentiation.
var ErrUnauthenticated = errors.New("unauthenticated")

// Authenticator validates a raw bearer token and returns the user it
// represents. Implementations may upsert the user record (OIDC) or simply
// look it up (local).
type Authenticator interface {
	Authenticate(ctx context.Context, rawToken string) (*models.User, error)
}

// Mode is the set of supported auth modes. It is persisted nowhere — it's a
// startup-time choice — but is exposed to the SPA via GET /api/auth/mode so
// the frontend knows whether to render a login form or trigger an OIDC
// redirect.
type Mode string

const (
	ModeOIDC  Mode = "oidc"
	ModeLocal Mode = "local"
)

const (
	userContextKey      = "damson.user"
	principalContextKey = "damson.principal"
)

func setUser(c *gin.Context, u *models.User) {
	c.Set(userContextKey, u)
}

func setPrincipal(c *gin.Context, p *Principal) {
	c.Set(principalContextKey, p)
}

// GetPrincipal returns the personal access token principal when the request
// authenticated with one, and nil otherwise (an interactive session).
func GetPrincipal(c *gin.Context) *Principal {
	v, ok := c.Get(principalContextKey)
	if !ok {
		return nil
	}
	p, _ := v.(*Principal)
	return p
}

// GetUser returns the authenticated user attached by Middleware. Returns nil
// for handlers that run outside the authenticated chain (e.g. /api/health).
func GetUser(c *gin.Context) *models.User {
	v, ok := c.Get(userContextKey)
	if !ok {
		return nil
	}
	u, _ := v.(*models.User)
	return u
}
