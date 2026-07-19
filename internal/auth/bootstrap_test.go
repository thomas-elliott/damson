package auth

import (
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/testdb"
)

func newBootstrapTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.NewPostgres(t)
}

func createUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	u := models.User{Email: email, DisplayName: email, Role: "user"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return &u
}

// The very first user on an empty system is promoted and handed a library.
func TestBootstrapFirstAdmin_FirstUserPromoted(t *testing.T) {
	db := newBootstrapTestDB(t)
	u := createUser(t, db, "first@example.com")

	if err := BootstrapFirstAdmin(t.Context(), db, u); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if u.Role != "sysadmin" {
		t.Fatalf("role: got %q, want sysadmin", u.Role)
	}
	if u.CurrentLibraryID == nil {
		t.Fatal("current_library_id: expected non-nil")
	}
	var mem models.LibraryMembership
	if err := db.Where("user_id = ?", u.ID).First(&mem).Error; err != nil {
		t.Fatalf("membership lookup: %v", err)
	}
	if mem.Role != "admin" {
		t.Fatalf("membership role: got %q, want admin", mem.Role)
	}
}

// Bootstrap no longer depends on the email claim: a user with no email still
// becomes the first admin (they fill in their email afterwards via PATCH /me).
func TestBootstrapFirstAdmin_NoEmailStillBootstraps(t *testing.T) {
	db := newBootstrapTestDB(t)
	u := createUser(t, db, "")

	if err := BootstrapFirstAdmin(t.Context(), db, u); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if u.Role != "sysadmin" {
		t.Fatalf("role: got %q, want sysadmin", u.Role)
	}
	if u.CurrentLibraryID == nil {
		t.Fatal("current_library_id: expected non-nil")
	}
}

// The second user to sign in is not promoted and gets no library.
func TestBootstrapFirstAdmin_SecondUserNotPromoted(t *testing.T) {
	db := newBootstrapTestDB(t)
	first := createUser(t, db, "first@example.com")
	if err := BootstrapFirstAdmin(t.Context(), db, first); err != nil {
		t.Fatalf("bootstrap first: %v", err)
	}

	second := createUser(t, db, "second@example.com")
	if err := BootstrapFirstAdmin(t.Context(), db, second); err != nil {
		t.Fatalf("bootstrap second: %v", err)
	}
	if second.Role != "user" {
		t.Fatalf("second user role: got %q, want user (unpromoted)", second.Role)
	}
	if second.CurrentLibraryID != nil {
		t.Fatal("second user current_library_id: expected nil")
	}

	var libCount int64
	db.Model(&models.Library{}).Count(&libCount)
	if libCount != 1 {
		t.Fatalf("library count: got %d, want 1", libCount)
	}
}

// Re-running on the same first user is a no-op once the library exists.
func TestBootstrapFirstAdmin_Idempotent(t *testing.T) {
	db := newBootstrapTestDB(t)
	u := createUser(t, db, "first@example.com")
	if err := BootstrapFirstAdmin(t.Context(), db, u); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := BootstrapFirstAdmin(t.Context(), db, u); err != nil {
		t.Fatalf("bootstrap retry: %v", err)
	}

	var libCount int64
	db.Model(&models.Library{}).Count(&libCount)
	if libCount != 1 {
		t.Fatalf("library count after retry: got %d, want 1", libCount)
	}
}
