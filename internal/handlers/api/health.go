package api

import (
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/gin-gonic/gin"
)

// getHealth is an unauthenticated liveness probe. It also reports the build
// identifier so a client can tell which server it is talking to.
func (h *Handler) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": buildVersion()})
}

var buildVersion = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			if len(s.Value) > 12 {
				return s.Value[:12]
			}
			return s.Value
		}
	}
	return "dev"
})
