package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/buildinfo"
)

// getHealth is an unauthenticated liveness probe. It also reports the build
// identifier so a client can tell which server it is talking to.
func (h *Handler) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": buildVersion()})
}

func buildVersion() string {
	if v := buildinfo.Version(); v != "" {
		return v
	}
	if commit := buildinfo.Commit(); commit != "" {
		return commit
	}
	return "dev"
}
