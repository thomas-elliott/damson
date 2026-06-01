package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestGetMe_Unauthenticated(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performGET(r, "/api/me")
	requireStatus(t, w, http.StatusUnauthorized)
}

func TestGetMe_ReturnsUser(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
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

// A user the IdP authenticated by `sub` with no email claim is flagged
// needs_profile so the SPA shows the first-login profile step.
func TestGetMe_NeedsProfile_WhenNoEmail(t *testing.T) {
	h, db := newTestHandler(t)
	u := &models.User{Subject: "sub-noemail", Role: "user"}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	r := setupRouter(h, u)

	w := performGET(r, "/api/me")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[meResponse](t, w)
	if !body.NeedsProfile {
		t.Fatal("needs_profile: expected true for a user with no email")
	}
}

func TestPatchMe_SetsEmailAndDefaultsDisplayName(t *testing.T) {
	h, db := newTestHandler(t)
	u := &models.User{Subject: "sub-noemail", Role: "user"}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPatch, "/api/me", patchMeRequest{Email: "new@example.com"})
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[meResponse](t, w)
	if body.Email != "new@example.com" {
		t.Fatalf("email: got %q, want new@example.com", body.Email)
	}
	if body.DisplayName != "new@example.com" {
		t.Fatalf("display_name: got %q, want it defaulted to the email", body.DisplayName)
	}
	if body.NeedsProfile {
		t.Fatal("needs_profile: expected false once an email is set")
	}

	var reloaded models.User
	if err := db.First(&reloaded, u.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.Email != "new@example.com" {
		t.Fatalf("persisted email: got %q", reloaded.Email)
	}
}

func TestPatchMe_KeepsProvidedDisplayName(t *testing.T) {
	h, db := newTestHandler(t)
	u := &models.User{Subject: "sub-noemail", Role: "user"}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPatch, "/api/me", patchMeRequest{
		Email:       "jane@example.com",
		DisplayName: "Jane Doe",
	})
	requireStatus(t, w, http.StatusOK)

	if body := jsonResponse[meResponse](t, w); body.DisplayName != "Jane Doe" {
		t.Fatalf("display_name: got %q, want 'Jane Doe'", body.DisplayName)
	}
}

func TestPatchMe_RejectsBlankEmail(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	r := setupRouter(h, u)

	// Whitespace passes the `required` binding but trims to empty → 400.
	w := performRequest(r, http.MethodPatch, "/api/me", map[string]string{"email": "   "})
	requireStatus(t, w, http.StatusBadRequest)
}

func TestPatchMe_Unauthenticated(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performRequest(r, http.MethodPatch, "/api/me", patchMeRequest{Email: "x@y.z"})
	requireStatus(t, w, http.StatusUnauthorized)
}

func TestHealth(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h, nil)

	w := performGET(r, "/api/health")
	requireStatus(t, w, http.StatusOK)
}
