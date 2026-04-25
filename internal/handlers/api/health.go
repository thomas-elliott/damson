package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// getHealth is an unauthenticated liveness probe.
func (h *Handler) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
