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
	if TokenPathDenied(routePath(c)) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not available to personal access tokens"})
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

// abort500 attaches the cause to c.Errors — requestLogger surfaces it — and
// writes the canonical generic body. The cause is a wrapped GORM/JWT error;
// bound query parameters (i.e. the token digest) are not part of it.
func abort500(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// routePath prefers the registered route pattern so a path parameter can't be
// used to dodge TokenPathDenied, falling back to the raw path when no route
// matched.
func routePath(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return c.Request.URL.Path
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
