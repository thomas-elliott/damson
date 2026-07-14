package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/storage"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestHandler returns an api handler backed by in-memory SQLite with all
// models migrated via AutoMigrate. Postgres-specific features (tsvector,
// advisory locks) are absent — SQL-migration-based tests need a real
// Postgres; see docs/archive/technical-design.md §18.
//
// Auth defaults to oidc mode (no local authenticator); tests that need
// /api/auth/login should call newTestHandlerLocal. A local-FS storage
// backend rooted in t.TempDir is attached so import tests work without a
// mock.
func newTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	store := newTestStorage(t)
	bc := imports.NewBroadcaster()
	pbc := preview.NewBroadcaster()
	previewPool := preview.NewSync(preview.Deps{DB: db, Storage: store, Broadcaster: pbc})
	pool := imports.NewSync(imports.Deps{DB: db, Storage: store, Broadcaster: bc, Previews: previewPool})
	return New(Deps{
		DB:                 db,
		AuthMode:           config.AuthModeOIDC,
		Storage:            store,
		ImportPool:         pool,
		Broadcaster:        bc,
		PreviewPool:        previewPool,
		PreviewBroadcaster: pbc,
		// Non-secret OIDC config the SPA reads off /api/auth/mode.
		OIDCAuthority: "https://auth.example.com",
		OIDCClientID:  "damson",
	}), db
}

func newTestHandlerLocal(t *testing.T) (*Handler, *gorm.DB, *auth.LocalAuthenticator) {
	t.Helper()
	db := newTestDB(t)
	la, err := auth.NewLocalAuthenticator(db, []byte("test-signing-key-at-least-thirty-two-bytes"))
	if err != nil {
		t.Fatalf("local auth: %v", err)
	}
	store := newTestStorage(t)
	bc := imports.NewBroadcaster()
	pbc := preview.NewBroadcaster()
	previewPool := preview.NewSync(preview.Deps{DB: db, Storage: store, Broadcaster: pbc})
	pool := imports.NewSync(imports.Deps{DB: db, Storage: store, Broadcaster: bc, Previews: previewPool})
	return New(Deps{
		DB:                 db,
		AuthMode:           config.AuthModeLocal,
		Local:              la,
		Storage:            store,
		ImportPool:         pool,
		Broadcaster:        bc,
		PreviewPool:        previewPool,
		PreviewBroadcaster: pbc,
	}), db, la
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.UserIdentity{},
		&models.Library{},
		&models.LibraryMembership{},
		&models.AuditLog{},
		&models.Asset{},
		&models.AssetVersion{},
		&models.AssetFile{},
		&models.AssetImport{},
		&models.AssetImage{},
		&models.Tag{},
		&models.AssetTag{},
		&models.Licence{},
		&models.Project{},
		&models.ProjectAssetPin{},
		&models.Collection{},
		&models.CollectionAsset{},
		&models.LibrarySettings{},
		&models.ServerSettings{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestStorage(t *testing.T) storage.Service {
	t.Helper()
	s, err := storage.NewLocalFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatalf("new local storage: %v", err)
	}
	return s
}

// setupRouter mounts /api under a fresh Gin engine, wrapping authed routes
// with a middleware that attaches asUser (or nil) to the context.
func setupRouter(h *Handler, asUser *models.User) *gin.Engine {
	r := gin.New()
	apiGroup := r.Group("/api")
	AddRoutes(apiGroup, func(c *gin.Context) {
		if asUser != nil {
			auth.SetUserForTest(c, asUser)
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}, h)
	return r
}

func performGET(r http.Handler, path string) *httptest.ResponseRecorder {
	return performRequest(r, http.MethodGet, path, nil)
}

func performRequest(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		buf := new(bytes.Buffer)
		_ = json.NewEncoder(buf).Encode(body)
		reader = buf
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func requireStatus(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("expected status %d, got %d: %s", code, w.Code, w.Body.String())
	}
}

func jsonResponse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode json: %v; body=%s", err, w.Body.String())
	}
	return v
}

func seedUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	u := models.User{
		Email:       email,
		DisplayName: email,
		Role:        "user",
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return &u
}

func seedLocalUser(t *testing.T, db *gorm.DB, email, password string) *models.User {
	t.Helper()
	hash, err := bcryptHash(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.User{
		Email:        email,
		DisplayName:  email,
		Role:         "user",
		PasswordHash: &hash,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed local user: %v", err)
	}
	return &u
}
