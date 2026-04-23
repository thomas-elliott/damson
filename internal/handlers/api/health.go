package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// getHealth is an unauthenticated liveness probe.
func (h *handler) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
