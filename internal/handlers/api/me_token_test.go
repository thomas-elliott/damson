package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

// setupTokenRouter mounts /api behind the real auth middleware, so PAT
// resolution, library pinning and the /me response are exercised end to end
// rather than through setupRouter's stub.
func setupTokenRouter(h *Handler, db *gorm.DB) *gin.Engine {
	r := gin.New()
	AddRoutes(r.Group("/api"), auth.Middleware(nil, auth.NewTokenAuthenticator(db)), h)
	return r
}

func seedTokenFor(t *testing.T, db *gorm.DB, userID, libraryID uint, scope string, expires *time.Time) string {
	t.Helper()
	secret, hash, prefix, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	tok := models.APIToken{
		UserID: userID, LibraryID: libraryID, Label: "cli",
		TokenHash: hash, Prefix: prefix, Scope: scope, ExpiresAt: expires,
	}
	if err := db.Create(&tok).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	return secret
}

func getWithBearer(r http.Handler, path, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// GET /api/me describes the token the request arrived on, so a CLI can report
// its own scope and expiry without reaching /api/me/tokens — which token auth
// is refused.
func TestGetMe_DescribesTheAuthenticatingToken(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "cli@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	expiry := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	secret := seedTokenFor(t, db, u.ID, lib.ID, auth.ScopeRead, &expiry)

	w := getWithBearer(setupTokenRouter(h, db), "/api/me", secret)
	requireStatus(t, w, http.StatusOK)

	var got struct {
		DisplayName    string `json:"display_name"`
		CurrentLibrary *struct {
			Name string `json:"name"`
		} `json:"current_library"`
		Token *struct {
			Prefix    string     `json:"prefix"`
			Label     string     `json:"label"`
			Scope     string     `json:"scope"`
			ExpiresAt *time.Time `json:"expires_at"`
		} `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; body=%s", err, w.Body.String())
	}
	if got.Token == nil {
		t.Fatal("token block missing — whoami cannot report scope or expiry without it")
	}
	if got.Token.Scope != auth.ScopeRead || got.Token.Label != "cli" {
		t.Fatalf("token block = %+v", got.Token)
	}
	if got.Token.ExpiresAt == nil || !got.Token.ExpiresAt.Equal(expiry) {
		t.Fatalf("expiry = %v, want %v", got.Token.ExpiresAt, expiry)
	}
	// The block must never carry anything that could reconstruct the secret.
	body := w.Body.String()
	if len(got.Token.Prefix) >= len(secret) {
		t.Fatal("prefix is not shorter than the secret")
	}
	for _, forbidden := range []string{secret, auth.HashToken(secret)} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("/api/me leaked a credential value")
		}
	}
	// The token pins the library even though the user row has none set for it.
	if got.CurrentLibrary == nil || got.CurrentLibrary.Name != "Main" {
		t.Fatalf("current library = %+v", got.CurrentLibrary)
	}
}

// An interactive session has no token block at all.
func TestGetMe_OmitsTheTokenBlockForASession(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "browser@example.com")
	seedLibraryForUser(t, db, u, "Main")

	w := performGET(setupRouter(h, u), "/api/me")
	requireStatus(t, w, http.StatusOK)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, present := got["token"]; present {
		t.Fatalf("a session response must not carry a token block: %v", got)
	}
}

func TestGetHealth_ReportsAVersion(t *testing.T) {
	h, _ := newTestHandler(t)
	w := performGET(setupRouter(h, nil), "/api/health")
	requireStatus(t, w, http.StatusOK)

	var got struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != "ok" {
		t.Fatalf("status = %q", got.Status)
	}
	if got.Version == "" {
		t.Fatal("version must never be blank — dmsn version renders it verbatim")
	}
}
