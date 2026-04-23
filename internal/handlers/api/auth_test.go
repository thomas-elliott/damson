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

func TestAuthMode_Local(t *testing.T) {
	h, _, _ := newTestHandlerLocal(t)
	r := setupRouter(h, nil)

	w := performGET(r, "/api/auth/mode")
	requireStatus(t, w, http.StatusOK)
	if jsonResponse[authModeResponse](t, w).Mode != "local" {
		t.Fatalf("mode: expected local")
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
	_ = seedLocalUser(t, db, "librarian@example.com", "correct-horse-battery-staple")
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPost, "/api/auth/login", loginRequest{
		Email: "librarian@example.com", Password: "correct-horse-battery-staple",
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
