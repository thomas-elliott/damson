package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// respond500 writes the canonical generic 500 body. Don't leak internals.
func respond500(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// respond400 logs the real error and responds with err.Error() per
// CONVENTIONS.md ("400 includes err.Error()").
func respond400(c *gin.Context, err error) {
	slog.Warn("400 bad request", "path", c.FullPath(), "err", err)
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func respond404(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": msg})
}

func respond409(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": msg})
}

func respondForbidden(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": msg})
}

// isNotFound reports whether err is the canonical GORM record-not-found.
func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// requireAdmin returns nil iff the membership attached to the context has
// role='admin'; otherwise it aborts with 403 and returns false.
func requireAdmin(c *gin.Context) bool {
	m := getMembership(c)
	if m == nil || m.Role != "admin" {
		respondForbidden(c, "library admin required")
		return false
	}
	return true
}
