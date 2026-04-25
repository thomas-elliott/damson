package api

import (
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func bcryptHash(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	return string(h), err
}

func TestAuthMode_OIDC(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performGET(r, "/api/auth/mode")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[authModeResponse](t, w)
	if body.Mode != "oidc" {
		t.Fatalf("mode: got %q, want oidc", body.Mode)
	}
}

func TestAuthMode_Local_NeedsBootstrap(t *testing.T) {
	h, _, _ := newTestHandlerLocal(t)
	r := setupRouter(h, nil)

	w := performGET(r, "/api/auth/mode")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[authModeResponse](t, w)
	if body.Mode != "local" {
		t.Fatalf("mode: expected local, got %q", body.Mode)
	}
	if !body.NeedsBootstrap {
		t.Fatalf("needs_bootstrap: expected true on an empty DB")
	}
}

func TestAuthMode_Local_AfterBootstrap(t *testing.T) {
	h, db, _ := newTestHandlerLocal(t)
	_ = seedLocalUser(t, db, "thomas@datch.io", "correct-horse-battery-staple")
	r := setupRouter(h, nil)

	w := performGET(r, "/api/auth/mode")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[authModeResponse](t, w)
	if body.NeedsBootstrap {
		t.Fatalf("needs_bootstrap: expected false once a user exists")
	}
}

func TestLogin_NotFound_InOIDCMode(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/login", loginRequest{
		Email: "a@b.c", Password: "pw",
	})
	// Route isn't registered in OIDC mode.
	requireStatus(t, w, http.StatusNotFound)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	h, _, _ := newTestHandlerLocal(t)
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/login", loginRequest{
		Email: "nobody@example.com", Password: "hunter2",
	})
	requireStatus(t, w, http.StatusUnauthorized)
}

func TestLogin_Success(t *testing.T) {
	h, db, la := newTestHandlerLocal(t)
	_ = seedLocalUser(t, db, "thomas@datch.io", "correct-horse-battery-staple")
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/login", loginRequest{
		Email: "thomas@datch.io", Password: "correct-horse-battery-staple",
	})
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[loginResponse](t, w)
	if body.Token == "" {
		t.Fatal("expected token")
	}

	// Round-trip: the issued token should authenticate against the same
	// authenticator.
	if _, err := la.Authenticate(t.Context(), body.Token); err != nil {
		t.Fatalf("authenticate issued token: %v", err)
	}
}

func TestBootstrap_CreatesFirstUser(t *testing.T) {
	h, _, la := newTestHandlerLocal(t)
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/bootstrap", loginRequest{
		Email: "thomas@datch.io", Password: "correct-horse-battery-staple",
	})
	requireStatus(t, w, http.StatusCreated)

	body := jsonResponse[loginResponse](t, w)
	if body.Token == "" {
		t.Fatal("expected token")
	}

	// User is issued a working token and exists in the DB with sysadmin
	// role + a library the user admins — the same post-bootstrap state
	// the env-seeded path produces.
	u, err := la.Authenticate(t.Context(), body.Token)
	if err != nil {
		t.Fatalf("authenticate issued token: %v", err)
	}
	if u.Role != "sysadmin" {
		t.Fatalf("first user role: got %q, want sysadmin", u.Role)
	}
	if u.CurrentLibraryID == nil {
		t.Fatal("first user current_library_id: expected non-nil")
	}

	// And /api/auth/mode now reports needs_bootstrap=false.
	mw := performGET(r, "/api/auth/mode")
	requireStatus(t, mw, http.StatusOK)
	if jsonResponse[authModeResponse](t, mw).NeedsBootstrap {
		t.Fatal("needs_bootstrap: expected false after bootstrap")
	}
}

func TestBootstrap_Conflict_WhenUsersExist(t *testing.T) {
	h, db, _ := newTestHandlerLocal(t)
	_ = seedLocalUser(t, db, "existing@example.com", "hunter2")
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/bootstrap", loginRequest{
		Email: "new@example.com", Password: "correct-horse",
	})
	requireStatus(t, w, http.StatusConflict)
}

func TestBootstrap_NotFound_InOIDCMode(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/bootstrap", loginRequest{
		Email: "new@example.com", Password: "correct-horse",
	})
	requireStatus(t, w, http.StatusNotFound)
}

func TestBootstrap_BadRequest_MissingFields(t *testing.T) {
	h, _, _ := newTestHandlerLocal(t)
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/bootstrap", map[string]string{
		"email": "new@example.com",
		// password missing
	})
	requireStatus(t, w, http.StatusBadRequest)
}

func TestBootstrap_BadRequest_PasswordTooShort(t *testing.T) {
	h, _, _ := newTestHandlerLocal(t)
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/bootstrap", loginRequest{
		Email:    "thomas@datch.io",
		Password: "short", // < 8 chars
	})
	requireStatus(t, w, http.StatusBadRequest)
}
