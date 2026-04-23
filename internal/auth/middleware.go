package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Middleware validates the bearer token on each request using the configured
// Authenticator and attaches the resulting user to the Gin context.
//
// All failures surface as 401 with a generic message; we deliberately don't
// leak whether a token was missing, malformed, or expired.
func Middleware(a Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := extractBearer(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		user, err := a.Authenticate(c.Request.Context(), raw)
		if err != nil {
			if errors.Is(err, ErrUnauthenticated) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		setUser(c, user)
		c.Next()
	}
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
