// Package api holds HTTP handlers for the /api surface. Each feature lives in
// its own file (assets.go, projects.go, …); all handlers are receiver methods
// on the shared Handler struct, per docs/CONVENTIONS.md.
package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// Handler carries dependencies used by every API handler. Construct with
// New(Deps{...}); register routes with AddRoutes. Exported so cmd/server (and
// out-of-tree tests) can hold a typed handle without leaking the field set.
type Handler struct {
	db          *gorm.DB
	authMode    config.AuthMode
	local       *auth.LocalAuthenticator // non-nil only in local mode
	storage     storage.Service          // nil in tests that don't exercise uploads
	loginRate   *auth.LoginThrottle
	importPool  *imports.Pool
	broadcaster *imports.Broadcaster
	// oidcAuthority / oidcClientID are the non-secret OIDC client config
	// echoed to the SPA via /api/auth/mode in OIDC mode, so the browser
	// configures oidc-client-ts at runtime instead of baking VITE_OIDC_* at
	// build time. Both empty in local mode.
	oidcAuthority string
	oidcClientID  string
}

type Deps struct {
	DB       *gorm.DB
	AuthMode config.AuthMode
	Local    *auth.LocalAuthenticator
	Storage  storage.Service
	// LoginThrottle is consulted on POST /api/auth/login and
	// /api/auth/bootstrap. Optional — a nil value installs the default
	// throttle (5 burst, one token per minute).
	LoginThrottle *auth.LoginThrottle
	// ImportPool drains async import jobs. Tests that don't exercise the
	// import flow can leave it nil; tests that do should pass a synchronous
	// pool (imports.NewSync) so stage transitions stay deterministic.
	ImportPool *imports.Pool
	// Broadcaster fans import stage events to SSE subscribers. Nil-safe:
	// the pipeline tolerates a missing broadcaster.
	Broadcaster *imports.Broadcaster
	// OIDCAuthority / OIDCClientID are the issuer URL + client_id echoed to
	// the SPA on /api/auth/mode in OIDC mode (the values the browser needs to
	// start the auth-code+PKCE flow). Ignored in local mode.
	OIDCAuthority string
	OIDCClientID  string
}

func New(d Deps) *Handler {
	throttle := d.LoginThrottle
	if throttle == nil {
		throttle = auth.NewLoginThrottle(5, time.Minute)
	}
	return &Handler{
		db:          d.DB,
		authMode:    d.AuthMode,
		local:       d.Local,
		storage:     d.Storage,
		loginRate:   throttle,
		importPool:  d.ImportPool,
		broadcaster: d.Broadcaster,

		oidcAuthority: d.OIDCAuthority,
		oidcClientID:  d.OIDCClientID,
	}
}

// AddRoutes registers all /api routes on root. Public routes are attached
// directly; authenticated routes go through the auth middleware passed in.
// Library-scoped routes layer on the library middleware that verifies the
// user's membership in their current library.
func AddRoutes(root *gin.RouterGroup, authMiddleware gin.HandlerFunc, h *Handler) {
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
	authed.POST("/libraries", h.postLibrary)
	authed.POST("/me/current-library", h.postCurrentLibrary)
	if h.authMode == config.AuthModeLocal {
		// Local-mode session refresh. OIDC sessions silent-renew via the
		// IdP, so this is local-only.
		authed.POST("/auth/refresh", h.postAuthRefresh)
	}

	// Library member management — gated by membership/admin checks inside the
	// handlers because these routes don't share the current_library_id
	// requirement (a sysadmin can administer any library).
	authed.PATCH("/libraries/:id", h.patchLibrary)
	authed.DELETE("/libraries/:id", h.deleteLibrary)
	authed.GET("/libraries/:id/members", h.getLibraryMembers)
	authed.POST("/libraries/:id/members", h.postLibraryMember)
	authed.PATCH("/libraries/:id/members/:userId", h.patchLibraryMember)
	authed.DELETE("/libraries/:id/members/:userId", h.deleteLibraryMember)

	// Server admin (sysadmin only — gated inside each handler)
	authed.GET("/admin/users", h.getAdminUsers)
	authed.PATCH("/admin/users/:id", h.patchAdminUser)
	authed.GET("/admin/settings", h.getServerSettings)
	authed.PATCH("/admin/settings", h.patchServerSettings)
	authed.GET("/admin/audit", h.getAdminAudit)

	// Authenticated + library-scoped
	scoped := authed.Group("")
	scoped.Use(h.requireLibrary())

	// Assets
	scoped.GET("/assets", h.getAssets)
	scoped.GET("/assets/:id", h.getAssetDetail)
	scoped.PATCH("/assets/:id", h.patchAsset)
	scoped.DELETE("/assets/:id", h.deleteAsset)
	scoped.POST("/assets/:id/review", h.postAssetReview)

	// Imports
	scoped.POST("/imports", h.postImport)
	scoped.GET("/imports", h.getImports)
	scoped.GET("/imports/events", h.getImportEvents)
	scoped.GET("/imports/:id", h.getImport)
	scoped.GET("/imports/:id/events", h.getImportEventsByID)
	scoped.POST("/imports/:id/retry", h.postImportRetry)
	scoped.POST("/assets/:id/versions/:vid/reextract", h.postReExtract)
	scoped.POST("/assets/:id/versions/:vid/subset", h.postAssetVersionSubset)

	// Files / thumbnails
	scoped.GET("/files/:objectKey", h.getFile)
	scoped.GET("/thumbs/:objectKey", h.getThumb)

	// Tags (M3)
	scoped.GET("/tags", h.getTags)
	scoped.POST("/tags", h.postTag)
	scoped.DELETE("/tags/:id", h.deleteTag)
	scoped.POST("/assets/:id/tags", h.postAssetTag)
	scoped.DELETE("/assets/:id/tags/:tagId", h.deleteAssetTag)

	// Licences (M3)
	scoped.GET("/assets/:id/licence", h.getLicence)
	scoped.PUT("/assets/:id/licence", h.putLicence)
	scoped.POST("/assets/:id/licence/review", h.postLicenceReview)
	scoped.POST("/licences/detect", h.postDetectLicence)
	scoped.GET("/assets/:id/licence/detect", h.getDetectLicenceFromFiles)

	// Projects (M4)
	scoped.GET("/projects", h.getProjects)
	scoped.POST("/projects", h.postProject)
	scoped.GET("/projects/:id", h.getProject)
	scoped.PATCH("/projects/:id", h.patchProject)
	scoped.DELETE("/projects/:id", h.deleteProject)
	scoped.POST("/projects/:id/pins", h.postProjectPin)
	scoped.DELETE("/projects/:id/pins/:pinId", h.deleteProjectPin)

	// Collections (M5)
	scoped.GET("/collections", h.getCollections)
	scoped.POST("/collections", h.postCollection)
	scoped.GET("/collections/:id", h.getCollection)
	scoped.PATCH("/collections/:id", h.patchCollection)
	scoped.DELETE("/collections/:id", h.deleteCollection)
	scoped.POST("/collections/:id/assets", h.postCollectionAsset)
	scoped.DELETE("/collections/:id/assets/:assetId", h.deleteCollectionAsset)

	// Search — convenience alias over /assets that always expects `q`.
	scoped.GET("/search", h.getAssets)

	// Library settings (M5)
	scoped.GET("/settings", h.getLibrarySettings)
	scoped.PATCH("/settings", h.patchLibrarySettings)

	// Audit (M7)
	scoped.GET("/audit", h.getLibraryAudit)
	scoped.GET("/assets/:id/audit", h.getAssetAudit)
	scoped.GET("/projects/:id/audit", h.getProjectAudit)
}
