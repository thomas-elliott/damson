package api

import (
	"net/http"
	"testing"
)

func TestGetMe_Unauthenticated(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performGET(r, "/api/me")
	requireStatus(t, w, http.StatusUnauthorized)
}

func TestGetMe_ReturnsUser(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	r := setupRouter(h, u)

	w := performGET(r, "/api/me")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[meResponse](t, w)
	if body.Email != u.Email {
		t.Fatalf("email: got %q want %q", body.Email, u.Email)
	}
	if body.Role != "user" {
		t.Fatalf("role: got %q want 'user'", body.Role)
	}
}

func TestHealth(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performGET(r, "/api/health")
	requireStatus(t, w, http.StatusOK)
}
