// Damson e2e-server — a thin test-only HTTP binary that mounts the same
// internal/handlers/api Handler used by cmd/server. Default backend is
// SQLite via AutoMigrate (same shape as internal/handlers/api/api_test.go)
// so the Playwright suite doesn't drag in a Postgres dependency for every
// test run. Set DAMSON_TEST_POSTGRES_DSN to switch to a real Postgres,
// applying the embedded SQL migrations through internal/db.Migrate — this
// path is what the PER-81 search spec uses to exercise tsvector + GIN.
//
// Trade-offs vs cmd/server:
//   - SQLite + AutoMigrate (default) skips Postgres-only features
//     (tsvector search, advisory locks, schema_migrations bookkeeping).
//     Specs that exercise those need DAMSON_TEST_POSTGRES_DSN.
//   - Adds /api/test/reset (dialect-aware: drop/recreate on SQLite,
//     truncate on Postgres) and /api/test/users (multi-user seeding).
//     NEVER ship this binary; it is not safe outside tests.
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
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/handlers/api"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

// PostgresDSNEnv is the env var that, when set, switches the e2e binary
// from its default SQLite/AutoMigrate path to Postgres + the SQL
// migration runner. Mirrors the constant in internal/testdb so the two
// paths agree on the env-var name.
const PostgresDSNEnv = "DAMSON_TEST_POSTGRES_DSN"

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

	gormDB, err := openDB(dbPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	if err := setupSchema(gormDB); err != nil {
		slog.Error("setup schema", "err", err)
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

	broadcaster := imports.NewBroadcaster()
	importPool := imports.New(imports.Deps{
		DB:          gormDB,
		Storage:     store,
		Broadcaster: broadcaster,
	}, 0)

	r := gin.New()
	r.Use(gin.Recovery())

	apiGroup := r.Group("/api")
	h := api.New(api.Deps{
		DB:          gormDB,
		AuthMode:    config.AuthModeLocal,
		Local:       la,
		Storage:     store,
		ImportPool:  importPool,
		Broadcaster: broadcaster,
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

// openDB picks SQLite or Postgres based on DAMSON_TEST_POSTGRES_DSN. The
// SQLite path is the default — fast, no external dependency, used by every
// tier-1/2/3 spec except the Postgres-gated ones (PER-81 search). The
// Postgres path applies the embedded SQL migrations via internal/db.Migrate
// so triggers, tsvector, and GIN indexes match production.
func openDB(sqlitePath string) (*gorm.DB, error) {
	if dsn := os.Getenv(PostgresDSNEnv); dsn != "" {
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{})
	}
	// SQLite WAL + busy_timeout: the import flow fans out concurrent
	// read/write across goroutines and the default SQLite locking spuriously
	// fails under that load.
	dsn := sqlitePath + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
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

// setupSchema brings the database up to a queryable state. SQLite uses
// AutoMigrate (cheap on an empty :memory:-backed file). Postgres runs the
// SQL migration files so the search_vector trigger + GIN index match prod.
func setupSchema(gdb *gorm.DB) error {
	if gdb.Dialector.Name() == "postgres" {
		return db.Migrate(context.Background(), gdb)
	}
	return gdb.AutoMigrate(allModels()...)
}

// resetSchema clears all data between Playwright tests. SQLite drops every
// table and re-AutoMigrates (cheap, schema is in-memory). Postgres
// truncates the data tables but keeps the migration-installed schema —
// re-running migrations on every reset would dominate the test budget,
// and the schema doesn't change across tests.
func resetSchema(gdb *gorm.DB) error {
	if gdb.Dialector.Name() == "postgres" {
		return resetPostgres(gdb)
	}
	mig := gdb.Migrator()
	for _, m := range allModels() {
		if mig.HasTable(m) {
			if err := mig.DropTable(m); err != nil {
				return err
			}
		}
	}
	return gdb.AutoMigrate(allModels()...)
}

// dataTables is the set of application-data tables. schema_migrations is
// deliberately omitted — the migrations stay applied across resets.
var dataTables = []string{
	"audit_log",
	"asset_tags",
	"tags",
	"asset_files",
	"asset_versions",
	"asset_imports",
	"project_asset_pins",
	"projects",
	"collection_assets",
	"collections",
	"library_settings",
	"library_memberships",
	"libraries",
	"licences",
	"assets",
	"users",
	"server_settings",
}

func resetPostgres(gdb *gorm.DB) error {
	stmt := "TRUNCATE TABLE " + strings.Join(dataTables, ", ") + " RESTART IDENTITY CASCADE"
	return gdb.Exec(stmt).Error
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
