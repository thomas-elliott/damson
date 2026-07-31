package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/testdb"
)

func init() { gin.SetMode(gin.TestMode) }

func newTokenDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.NewPostgres(t)
}

// seedTokenUser creates a user, a library and the membership between them,
// returning the user and library ids.
func seedTokenUser(t *testing.T, db *gorm.DB, email string) (uint, uint) {
	t.Helper()
	u := models.User{Email: email, DisplayName: email, Role: "user"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	lib := models.Library{Name: "lib-" + email}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	if err := db.Create(&models.LibraryMembership{LibraryID: lib.ID, UserID: u.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return u.ID, lib.ID
}

// mintToken persists a token for (userID, libraryID) and returns the raw
// secret. Fields left zero take the table defaults.
func mintToken(t *testing.T, db *gorm.DB, userID, libraryID uint, scope string, mutate func(*models.APIToken)) string {
	t.Helper()
	secret, hash, prefix, err := GenerateToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	tok := models.APIToken{
		UserID:    userID,
		LibraryID: libraryID,
		Label:     "test",
		TokenHash: hash,
		Prefix:    prefix,
		Scope:     scope,
	}
	if mutate != nil {
		mutate(&tok)
	}
	if err := db.Create(&tok).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	return secret
}

func TestGenerateToken_ShapeAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		secret, hash, prefix, err := GenerateToken()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !strings.HasPrefix(secret, TokenPrefix) {
			t.Fatalf("secret %q lacks %q", secret, TokenPrefix)
		}
		if !IsTokenBearer(secret) {
			t.Fatal("generated secret should be recognised as a token bearer")
		}
		// The stored digest must not be the secret, and must be reproducible.
		if hash == secret {
			t.Fatal("hash must differ from the secret")
		}
		if strings.Contains(hash, strings.TrimPrefix(secret, TokenPrefix)) {
			t.Fatal("hash must not embed the secret")
		}
		if HashToken(secret) != hash {
			t.Fatal("HashToken must reproduce the stored digest")
		}
		if !strings.HasPrefix(secret, prefix) {
			t.Fatalf("prefix %q is not a prefix of the secret", prefix)
		}
		if len(prefix) >= len(secret) {
			t.Fatalf("display prefix %q must be shorter than the secret", prefix)
		}
		if seen[secret] {
			t.Fatalf("duplicate secret at iteration %d", i)
		}
		seen[secret] = true
	}
}

func TestIsTokenBearer(t *testing.T) {
	if IsTokenBearer("eyJhbGciOiJIUzI1NiJ9.abc.def") {
		t.Fatal("a JWT must not be treated as a token bearer")
	}
	if IsTokenBearer("") {
		t.Fatal("empty bearer must not be treated as a token bearer")
	}
	if !IsTokenBearer(TokenPrefix + "anything") {
		t.Fatal("prefixed bearer should be recognised")
	}
}

func TestNormaliseScope(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", ScopeWrite, false},
		{"write", ScopeWrite, false},
		{"WRITE", ScopeWrite, false},
		{" read ", ScopeRead, false},
		{"admin", "", true},
		{"readwrite", "", true},
	} {
		got, err := NormaliseScope(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCheckScope(t *testing.T) {
	for _, m := range []string{"GET", "HEAD"} {
		if err := CheckScope(ScopeRead, m); err != nil {
			t.Fatalf("read scope should allow %s: %v", m, err)
		}
	}
	for _, m := range []string{"POST", "PATCH", "PUT", "DELETE"} {
		if err := CheckScope(ScopeRead, m); err == nil {
			t.Fatalf("read scope must reject %s", m)
		}
		if err := CheckScope(ScopeWrite, m); err != nil {
			t.Fatalf("write scope should allow %s: %v", m, err)
		}
	}
}

func TestTokenPathDenied(t *testing.T) {
	denied := []string{"/api/me/tokens", "/api/me/tokens/:id", "/api/admin/users", "/api/admin/storage/cleanup/scan"}
	for _, p := range denied {
		if !TokenPathDenied(p) {
			t.Fatalf("%q must be denied to token auth", p)
		}
	}
	allowed := []string{"/api/me", "/api/assets", "/api/assets/:id", "/api/imports", "/api/libraries/:id/members"}
	for _, p := range allowed {
		if TokenPathDenied(p) {
			t.Fatalf("%q must remain available to token auth", p)
		}
	}
}

func TestTokenAuthenticator_PinsLibraryWithoutTouchingTheUserRow(t *testing.T) {
	db := newTokenDB(t)
	userID, libID := seedTokenUser(t, db, "pin@example.com")
	secret := mintToken(t, db, userID, libID, ScopeWrite, nil)

	ta := NewTokenAuthenticator(db)
	p, err := ta.Authenticate(context.Background(), secret)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if p.User.ID != userID {
		t.Fatalf("user id: got %d want %d", p.User.ID, userID)
	}
	if p.User.CurrentLibraryID == nil || *p.User.CurrentLibraryID != libID {
		t.Fatalf("current library not pinned to the token's library: %v", p.User.CurrentLibraryID)
	}
	if p.Token.Scope != ScopeWrite {
		t.Fatalf("scope: got %q", p.Token.Scope)
	}

	// The pin is request-scoped: the persisted user row is untouched, so a
	// concurrent SPA session keeps whatever library it had (here, none).
	var persisted models.User
	if err := db.Where("id = ?", userID).Take(&persisted).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if persisted.CurrentLibraryID != nil {
		t.Fatalf("token auth must not persist current_library_id, got %v", *persisted.CurrentLibraryID)
	}
}

func TestTokenAuthenticator_Rejects(t *testing.T) {
	db := newTokenDB(t)
	userID, libID := seedTokenUser(t, db, "reject@example.com")
	ta := NewTokenAuthenticator(db)
	ctx := context.Background()

	t.Run("unknown secret", func(t *testing.T) {
		if _, err := ta.Authenticate(ctx, TokenPrefix+"never-issued"); err != ErrUnauthenticated {
			t.Fatalf("got %v want ErrUnauthenticated", err)
		}
	})

	t.Run("not a token bearer", func(t *testing.T) {
		if _, err := ta.Authenticate(ctx, "some.jwt.value"); err != ErrUnauthenticated {
			t.Fatalf("got %v want ErrUnauthenticated", err)
		}
	})

	t.Run("revoked", func(t *testing.T) {
		now := time.Now().UTC()
		secret := mintToken(t, db, userID, libID, ScopeWrite, func(tok *models.APIToken) {
			tok.RevokedAt = &now
		})
		if _, err := ta.Authenticate(ctx, secret); err != ErrUnauthenticated {
			t.Fatalf("got %v want ErrUnauthenticated", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		past := time.Now().UTC().Add(-time.Hour)
		secret := mintToken(t, db, userID, libID, ScopeWrite, func(tok *models.APIToken) {
			tok.ExpiresAt = &past
		})
		if _, err := ta.Authenticate(ctx, secret); err != ErrUnauthenticated {
			t.Fatalf("got %v want ErrUnauthenticated", err)
		}
	})

	t.Run("membership revoked after issue", func(t *testing.T) {
		secret := mintToken(t, db, userID, libID, ScopeWrite, nil)
		if _, err := ta.Authenticate(ctx, secret); err != nil {
			t.Fatalf("precondition: %v", err)
		}
		if err := db.Where("library_id = ? AND user_id = ?", libID, userID).
			Delete(&models.LibraryMembership{}).Error; err != nil {
			t.Fatalf("drop membership: %v", err)
		}
		if _, err := ta.Authenticate(ctx, secret); err != ErrUnauthenticated {
			t.Fatalf("got %v want ErrUnauthenticated — a token must die with its membership", err)
		}
	})
}

func TestTokenAuthenticator_TouchIsThrottled(t *testing.T) {
	db := newTokenDB(t)
	userID, libID := seedTokenUser(t, db, "touch@example.com")
	secret := mintToken(t, db, userID, libID, ScopeWrite, nil)

	now := time.Now().UTC()
	ta := NewTokenAuthenticator(db)
	ta.now = func() time.Time { return now }

	if _, err := ta.Authenticate(context.Background(), secret); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	var tok models.APIToken
	if err := db.Where("token_hash = ?", HashToken(secret)).Take(&tok).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if tok.LastUsedAt == nil {
		t.Fatal("first use must record last_used_at")
	}
	first := *tok.LastUsedAt

	// Same second: throttled, so the stored value must not move.
	if _, err := ta.Authenticate(context.Background(), secret); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if err := db.Where("token_hash = ?", HashToken(secret)).Take(&tok).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !tok.LastUsedAt.Equal(first) {
		t.Fatalf("last_used_at moved within the throttle window: %v -> %v", first, *tok.LastUsedAt)
	}

	// Past the window: it advances again.
	now = now.Add(2 * lastUsedResolution)
	if _, err := ta.Authenticate(context.Background(), secret); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if err := db.Where("token_hash = ?", HashToken(secret)).Take(&tok).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !tok.LastUsedAt.After(first) {
		t.Fatalf("last_used_at should advance past the throttle window: %v", *tok.LastUsedAt)
	}
}

// stubAuthenticator stands in for the mode-specific authenticator so the
// middleware tests can prove the PAT branch is taken without it.
type stubAuthenticator struct {
	called bool
	user   *models.User
}

func (s *stubAuthenticator) Authenticate(context.Context, string) (*models.User, error) {
	s.called = true
	if s.user == nil {
		return nil, ErrUnauthenticated
	}
	return s.user, nil
}

// tokenRouter mounts a handler at each of the routes the middleware branches
// on, recording whether the request reached one.
func tokenRouter(t *testing.T, db *gorm.DB, stub Authenticator) (*gin.Engine, *bool) {
	t.Helper()
	reached := false
	r := gin.New()
	g := r.Group("/api")
	g.Use(Middleware(stub, NewTokenAuthenticator(db)))
	hit := func(c *gin.Context) {
		reached = true
		u := GetUser(c)
		if u == nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
	g.GET("/assets", hit)
	g.POST("/assets/bulk", hit)
	g.GET("/me/tokens", hit)
	g.POST("/me/tokens", hit)
	g.GET("/admin/users", hit)
	return r, &reached
}

func doAuthed(r http.Handler, method, path, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMiddleware_TokenBranch(t *testing.T) {
	db := newTokenDB(t)
	userID, libID := seedTokenUser(t, db, "mw@example.com")
	write := mintToken(t, db, userID, libID, ScopeWrite, nil)
	read := mintToken(t, db, userID, libID, ScopeRead, nil)

	t.Run("write token reaches a mutating route", func(t *testing.T) {
		stub := &stubAuthenticator{}
		r, reached := tokenRouter(t, db, stub)
		w := doAuthed(r, http.MethodPost, "/api/assets/bulk", write)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if !*reached {
			t.Fatal("handler not reached")
		}
		if stub.called {
			t.Fatal("PAT bearer must not fall through to the mode authenticator")
		}
	})

	t.Run("read token is confined to GET", func(t *testing.T) {
		stub := &stubAuthenticator{}
		r, reached := tokenRouter(t, db, stub)
		if w := doAuthed(r, http.MethodGet, "/api/assets", read); w.Code != http.StatusOK {
			t.Fatalf("read token should pass GET, got %d", w.Code)
		}
		*reached = false
		w := doAuthed(r, http.MethodPost, "/api/assets/bulk", read)
		if w.Code != http.StatusForbidden {
			t.Fatalf("read token on POST: got %d want 403", w.Code)
		}
		if *reached {
			t.Fatal("handler must not run for a scope-denied request")
		}
	})

	t.Run("token management is off-limits", func(t *testing.T) {
		stub := &stubAuthenticator{}
		r, reached := tokenRouter(t, db, stub)
		for _, tc := range []struct{ method, path string }{
			{http.MethodGet, "/api/me/tokens"},
			{http.MethodPost, "/api/me/tokens"},
			{http.MethodGet, "/api/admin/users"},
		} {
			*reached = false
			w := doAuthed(r, tc.method, tc.path, write)
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s %s: got %d want 403", tc.method, tc.path, w.Code)
			}
			if *reached {
				t.Fatalf("%s %s: handler must not run", tc.method, tc.path)
			}
		}
	})

	t.Run("interactive session still reaches token management", func(t *testing.T) {
		stub := &stubAuthenticator{user: &models.User{ID: userID, Email: "mw@example.com", Role: "user"}}
		r, reached := tokenRouter(t, db, stub)
		w := doAuthed(r, http.MethodGet, "/api/me/tokens", "a-jwt-not-a-pat")
		if w.Code != http.StatusOK {
			t.Fatalf("got %d want 200: %s", w.Code, w.Body.String())
		}
		if !*reached || !stub.called {
			t.Fatal("non-PAT bearer must go through the mode authenticator")
		}
	})

	t.Run("unknown token is 401, not 403", func(t *testing.T) {
		stub := &stubAuthenticator{}
		r, _ := tokenRouter(t, db, stub)
		// Deliberately an admin path: an invalid token must not learn that the
		// route is admin-gated.
		if w := doAuthed(r, http.MethodGet, "/api/admin/users", TokenPrefix+"bogus"); w.Code != http.StatusUnauthorized {
			t.Fatalf("got %d want 401", w.Code)
		}
	})
}

func TestMiddleware_NilTokenAuthenticatorRejectsPATs(t *testing.T) {
	stub := &stubAuthenticator{user: &models.User{ID: 1}}
	r := gin.New()
	g := r.Group("/api")
	g.Use(Middleware(stub, nil))
	g.GET("/assets", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := doAuthed(r, http.MethodGet, "/api/assets", TokenPrefix+"anything")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401 when PAT auth is disabled", w.Code)
	}
	if stub.called {
		t.Fatal("a PAT bearer must never fall through to the mode authenticator")
	}
}
