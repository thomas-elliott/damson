package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Middleware validates the bearer token on each request and attaches the
// resulting user to the Gin context.
//
// Bearers prefixed with TokenPrefix are personal access tokens and go through
// tokens (nil disables PAT auth entirely); everything else goes to the
// mode-specific Authenticator.
//
// All authentication failures surface as 401 with a generic message; we
// deliberately don't leak whether a token was missing, malformed, or expired.
func Middleware(a Authenticator, tokens *TokenAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := extractBearer(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if IsTokenBearer(raw) {
			authenticateToken(c, tokens, raw)
			return
		}

		user, err := a.Authenticate(c.Request.Context(), raw)
		if err != nil {
			if errors.Is(err, ErrUnauthenticated) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			abort500(c, err)
			return
		}
		setUser(c, user)
		c.Next()
	}
}

func authenticateToken(c *gin.Context, tokens *TokenAuthenticator, raw string) {
	if tokens == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	p, err := tokens.Authenticate(c.Request.Context(), raw)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		abort500(c, err)
		return
	}
	if err := CheckScope(p.Token.Scope, c.Request.Method); err != nil {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "token is read-only"})
		return
	}
	setUser(c, p.User)
	setPrincipal(c, p)
	c.Next()
}

// RefuseTokens closes a route to personal access tokens — token management and
// the sysadmin surface. Registered on the route rather than matched from its
// URL, so a new privileged route inherits the gate from the group it is
// declared in instead of from the shape of its path.
//
// It runs after Middleware, which is what keeps an invalid token from
// distinguishing a gated route (403) from any other (401).
func RefuseTokens() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetPrincipal(c) != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not available to personal access tokens"})
			return
		}
		c.Next()
	}
}

// abort500 attaches the cause to c.Errors so requestLogger surfaces it. Safe
// for token auth: the cause is a wrapped GORM/JWT error, and bound query
// parameters — the token digest — are not part of its message.
func abort500(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

func extractBearer(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", errors.New("missing authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", errors.New("bearer scheme required")
	}
	tok := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if tok == "" {
		return "", errors.New("empty bearer token")
	}
	return tok, nil
}
