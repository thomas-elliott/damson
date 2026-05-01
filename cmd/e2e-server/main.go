// Damson e2e-server — a thin test-only HTTP binary that mounts the same
// internal/handlers/api Handler used by cmd/server, but backed by SQLite
// (via AutoMigrate, the same pattern as internal/handlers/api/api_test.go)
// instead of Postgres + the embedded migrations. It exists so Playwright
// E2E tests can drive a real Go process — full HTTP layer, real auth,
// real storage, real services — without dragging in a Postgres
// dependency for every test run.
//
// Trade-offs vs cmd/server:
//   - SQLite + AutoMigrate, so Postgres-only features (tsvector search,
//     advisory locks, schema_migrations bookkeeping) are absent. Tests
//     that exercise those belong against a real Postgres harness.
//   - Adds a /api/test/reset endpoint that wipes + re-AutoMigrates the
//     schema. NEVER ship this binary; it is not safe outside tests.
//   - Skips the SPA mount and the production security middleware — the
//     SPA is served by Vite dev in the same Playwright run.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/handlers/api"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// Deterministic signing key — this binary is test-only, so a fixed key is
// fine and lets tests issue tokens without coordinating with the backend.
// Length matches HS256's >= 32-byte requirement enforced by NewLocalAuthenticator.
var testSigningKey = []byte("damson-e2e-test-signing-key-32by")

func main() {
	gin.SetMode(gin.TestMode)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	addr := envOr("E2E_LISTEN_ADDR", ":8080")
	dataDir := envOr("E2E_DATA_DIR", "")
	if dataDir == "" {
		tmp, err := os.MkdirTemp("", "damson-e2e-*")
		if err != nil {
			slog.Error("mktemp", "err", err)
			os.Exit(1)
		}
		dataDir = tmp
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		slog.Error("mkdir data", "err", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(dataDir, "e2e.db")
	storeRoot := filepath.Join(dataDir, "storage")
	baseURL := envOr("E2E_BASE_URL", "http://localhost"+addr)

	gormDB, err := openSQLite(dbPath)
	if err != nil {
		slog.Error("open sqlite", "err", err)
		os.Exit(1)
	}
	if err := autoMigrate(gormDB); err != nil {
		slog.Error("automigrate", "err", err)
		os.Exit(1)
	}

	la, err := auth.NewLocalAuthenticatorWithIssuer(gormDB, testSigningKey, baseURL)
	if err != nil {
		slog.Error("local auth", "err", err)
		os.Exit(1)
	}

	store, err := storage.NewLocalFS(storeRoot, baseURL)
	if err != nil {
		slog.Error("storage", "err", err)
		os.Exit(1)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	apiGroup := r.Group("/api")
	h := api.New(api.Deps{
		DB:       gormDB,
		AuthMode: config.AuthModeLocal,
		Local:    la,
		Storage:  store,
		// Tests fire login/bootstrap rapidly; the production 5-burst rate
		// limit will spuriously 429 inside a Playwright run. Disable it.
		LoginThrottle: auth.NewLoginThrottle(1_000_000, time.Hour),
	})
	api.AddRoutes(apiGroup, auth.Middleware(la), h)

	// Test-only: drop every table and re-AutoMigrate. Lets each Playwright
	// test start from a known-empty schema. Returns 204 on success.
	apiGroup.POST("/test/reset", func(c *gin.Context) {
		if err := resetSchema(gormDB); err != nil {
			slog.Error("reset", "err", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	})

	// Test-only: seed an additional user without going through the bootstrap
	// flow (which only accepts the FIRST user) or the admin-invite flow
	// (which doesn't exist yet). Tests for sysadmin-vs-member gating need
	// two coexisting users on a fresh schema; this is the lever for that.
	apiGroup.POST("/test/users", func(c *gin.Context) {
		seedTestUser(c, gormDB)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("e2e-server listening", "addr", addr, "data_dir", dataDir)
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-stop:
		slog.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// openSQLite opens the test DB with WAL + a generous busy_timeout. The
// import flow can fan out concurrent reads/writes across goroutines and
// SQLite's default locking will spuriously fail under that load.
func openSQLite(path string) (*gorm.DB, error) {
	dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
	return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
}

// allModels mirrors the AutoMigrate list in internal/handlers/api/api_test.go.
// Keep it in sync when models are added.
func allModels() []any {
	return []any{
		&models.User{},
		&models.Library{},
		&models.LibraryMembership{},
		&models.AuditLog{},
		&models.Asset{},
		&models.AssetVersion{},
		&models.AssetFile{},
		&models.AssetImport{},
		&models.Tag{},
		&models.AssetTag{},
		&models.Licence{},
		&models.Project{},
		&models.ProjectAssetPin{},
		&models.Collection{},
		&models.CollectionAsset{},
		&models.LibrarySettings{},
		&models.ServerSettings{},
	}
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(allModels()...)
}

func resetSchema(db *gorm.DB) error {
	mig := db.Migrator()
	for _, m := range allModels() {
		if mig.HasTable(m) {
			if err := mig.DropTable(m); err != nil {
				return err
			}
		}
	}
	return autoMigrate(db)
}

// seedTestUser creates an arbitrary user (with optional sysadmin role and
// optional library membership). Test-only — there's no admin-invite flow in
// the product yet, so multi-user E2E specs need this lever.
func seedTestUser(c *gin.Context, db *gorm.DB) {
	var req struct {
		Email          string `json:"email" binding:"required"`
		Password       string `json:"password" binding:"required"`
		Role           string `json:"role,omitempty"`            // "user" (default) | "sysadmin"
		LibraryID      uint   `json:"library_id,omitempty"`      // optional: attach a membership
		MembershipRole string `json:"membership_role,omitempty"` // "admin" (default) | "member"
		SetCurrent     bool   `json:"set_current,omitempty"`     // also set current_library_id
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := auth.CreateLocalUser(c.Request.Context(), db, req.Email, req.Email, req.Password)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{}
	if req.Role == "sysadmin" {
		updates["role"] = "sysadmin"
		user.Role = "sysadmin"
	}
	if req.LibraryID != 0 {
		role := req.MembershipRole
		if role == "" {
			role = "admin"
		}
		if err := db.Create(&models.LibraryMembership{
			LibraryID: req.LibraryID,
			UserID:    user.ID,
			Role:      role,
		}).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if req.SetCurrent {
			updates["current_library_id"] = req.LibraryID
			user.CurrentLibraryID = &req.LibraryID
		}
	}
	if len(updates) > 0 {
		if err := db.Model(user).Updates(updates).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}
