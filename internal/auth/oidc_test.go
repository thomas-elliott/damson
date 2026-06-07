package auth

import (
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// resolveUser is the IdP-identity reconciliation core, exercised here without a
// live verifier (it takes already-validated claim values). newBootstrapTestDB
// migrates users + user_identities + libraries, which is all these paths touch.

func countIdentities(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&models.UserIdentity{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		t.Fatalf("count identities: %v", err)
	}
	return n
}

// First sign-in creates the user, records the identity, and bootstraps them as
// the first admin.
func TestResolveUser_NewUserGetsIdentityAndBootstrap(t *testing.T) {
	db := newBootstrapTestDB(t)
	a := &OIDCAuthenticator{db: db}

	u, err := a.resolveUser(t.Context(), "https://idp.example", "sub-1", "alice@example.com", true, "Alice")
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	if u.Email != "alice@example.com" || u.DisplayName != "Alice" {
		t.Fatalf("seeded profile: got %q / %q", u.Email, u.DisplayName)
	}
	if u.Role != "sysadmin" {
		t.Fatalf("first user role: got %q, want sysadmin (bootstrapped)", u.Role)
	}
	if got := countIdentities(t, db, u.ID); got != 1 {
		t.Fatalf("identities: got %d, want 1", got)
	}
}

// The same (issuer, subject) always resolves to the same user without creating
// a second identity.
func TestResolveUser_KnownIdentityIsStable(t *testing.T) {
	db := newBootstrapTestDB(t)
	a := &OIDCAuthenticator{db: db}

	first, err := a.resolveUser(t.Context(), "https://idp.example", "sub-1", "alice@example.com", true, "Alice")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	again, err := a.resolveUser(t.Context(), "https://idp.example", "sub-1", "alice@example.com", true, "Alice")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("user id: got %d, want %d", again.ID, first.ID)
	}
	if got := countIdentities(t, db, first.ID); got != 1 {
		t.Fatalf("identities: got %d, want 1 (no duplicate)", got)
	}
}

// A different IdP identity with the same VERIFIED email joins the existing
// account rather than creating a new one — the Google-then-Apple case.
func TestResolveUser_VerifiedEmailLinksSecondIdP(t *testing.T) {
	db := newBootstrapTestDB(t)
	a := &OIDCAuthenticator{db: db}

	orig, err := a.resolveUser(t.Context(), "https://google", "g-1", "alice@example.com", true, "Alice")
	if err != nil {
		t.Fatalf("google login: %v", err)
	}
	apple, err := a.resolveUser(t.Context(), "https://apple", "a-1", "alice@example.com", true, "Alice")
	if err != nil {
		t.Fatalf("apple login: %v", err)
	}
	if apple.ID != orig.ID {
		t.Fatalf("expected same account: apple=%d google=%d", apple.ID, orig.ID)
	}
	if got := countIdentities(t, db, orig.ID); got != 2 {
		t.Fatalf("identities: got %d, want 2 (both IdPs linked)", got)
	}

	var users int64
	db.Model(&models.User{}).Count(&users)
	if users != 1 {
		t.Fatalf("user count: got %d, want 1", users)
	}
}

// An UNVERIFIED email must not link to an existing account — it creates a
// separate user, so a hostile IdP can't assert someone else's address to take
// over their account.
func TestResolveUser_UnverifiedEmailDoesNotLink(t *testing.T) {
	db := newBootstrapTestDB(t)
	a := &OIDCAuthenticator{db: db}

	orig, err := a.resolveUser(t.Context(), "https://google", "g-1", "alice@example.com", true, "Alice")
	if err != nil {
		t.Fatalf("google login: %v", err)
	}
	other, err := a.resolveUser(t.Context(), "https://evil", "e-1", "alice@example.com", false, "Mallory")
	if err != nil {
		t.Fatalf("evil login: %v", err)
	}
	if other.ID == orig.ID {
		t.Fatal("unverified email must not adopt the existing account")
	}
	// The seeded email is dropped for unverified claims, leaving needs_profile.
	if other.Email != "" {
		t.Fatalf("unverified email should not seed profile: got %q", other.Email)
	}
}

// No email claim → user created blank (needs_profile) with the identity still
// recorded.
func TestResolveUser_NoEmailCreatesBlankProfile(t *testing.T) {
	db := newBootstrapTestDB(t)
	a := &OIDCAuthenticator{db: db}

	u, err := a.resolveUser(t.Context(), "https://idp.example", "sub-1", "", false, "")
	if err != nil {
		t.Fatalf("resolveUser: %v", err)
	}
	if u.Email != "" {
		t.Fatalf("email: got %q, want blank", u.Email)
	}
	if got := countIdentities(t, db, u.ID); got != 1 {
		t.Fatalf("identities: got %d, want 1", got)
	}
}
