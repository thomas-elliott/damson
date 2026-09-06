// Package web mounts the embedded SPA on the Gin router. In production the
// binary serves the built Vite bundle; in dev the SPA runs under Vite on a
// different port and proxies /api to this server, so the embed only needs to
// contain a placeholder (handled by the top-level go:embed directive).
package web

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	damson "github.com/thomas-elliott/damson"
)

// Mount attaches a static file handler for the embedded SPA. Anything not
// under /api and not matching a real static file falls back to index.html so
// client-side routing works for deep links.
func Mount(r *gin.Engine) error {
	sub, err := fs.Sub(damson.WebStaticFS, "web/static")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			// SPA not built yet — serve a helpful placeholder.
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte(`<!doctype html><meta charset="utf-8"><title>Damson</title><body style="font-family:system-ui;padding:2rem"><p>SPA not built. Run <code>just build frontend</code> or <code>just dev</code>.</p></body>`))
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
	return nil
}
