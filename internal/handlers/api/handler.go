// Package api holds HTTP handlers for the /api surface. Each feature lives in
// its own file (assets.go, projects.go, …); all handlers are receiver methods
// on the shared handler struct, per docs/CONVENTIONS.md.
package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// handler carries dependencies used by every API handler. Unexported so
// callers go through New.
type handler struct {
	db       *gorm.DB
	authMode config.AuthMode
	local    *auth.LocalAuthenticator // non-nil only in local mode
	storage  storage.Service          // nil in tests that don't exercise uploads
}

type Deps struct {
	DB       *gorm.DB
	AuthMode config.AuthMode
	Local    *auth.LocalAuthenticator
	Storage  storage.Service
}

func New(d Deps) *handler {
	return &handler{
		db:       d.DB,
		authMode: d.AuthMode,
		local:    d.Local,
		storage:  d.Storage,
	}
}

// AddRoutes registers all /api routes on root. Public routes are attached
// directly; authenticated routes go through the auth middleware passed in.
// Library-scoped routes layer on the library middleware that verifies the
// user's membership in their current library.
func AddRoutes(root *gin.RouterGroup, authMiddleware gin.HandlerFunc, h *handler) {
	// Public
	root.GET("/health", h.getHealth)
	root.GET("/auth/mode", h.getAuthMode)
	if h.authMode == config.AuthModeLocal {
		root.POST("/auth/login", h.postAuthLogin)
		root.POST("/auth/bootstrap", h.postAuthBootstrap)
	}

	// Authenticated (no library context required)
	authed := root.Group("")
	authed.Use(authMiddleware)
	authed.GET("/me", h.getMe)
	authed.GET("/libraries", h.getLibraries)
	authed.POST("/me/current-library", h.postCurrentLibrary)

	// Authenticated + library-scoped
	scoped := authed.Group("")
	scoped.Use(h.requireLibrary())
	scoped.GET("/assets", h.getAssets)
	scoped.POST("/imports", h.postImport)
	scoped.POST("/assets/:id/versions/:vid/reextract", h.postReExtract)
}
