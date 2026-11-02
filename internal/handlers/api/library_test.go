package api

import (
	"fmt"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

func seedLibrary(t *testing.T, db *gorm.DB, name string) *models.Library {
	t.Helper()
	lib := models.Library{Name: name}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	return &lib
}

func seedMembership(t *testing.T, db *gorm.DB, libID, userID uint, role string) {
	t.Helper()
	m := models.LibraryMembership{LibraryID: libID, UserID: userID, Role: role}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func TestGetMe_IncludesCurrentLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibrary(t, db, "Personal")
	seedMembership(t, db, lib.ID, u.ID, "admin")
	u.CurrentLibraryID = &lib.ID
	if err := db.Save(u).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/me")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[meResponse](t, w)
	if body.CurrentLibrary == nil {
		t.Fatalf("expected current_library in response: %s", w.Body.String())
	}
	if body.CurrentLibrary.Name != "Personal" || body.CurrentLibrary.Role != "admin" {
		t.Fatalf("current library wrong: %+v", body.CurrentLibrary)
	}
}

// A team has one library: nothing creates, lists, switches to or deletes
// another, even for a sysadmin who is an admin of it.
func TestLibraryLifecycleRoutesAreGone(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "root@example.com")
	u.Role = "sysadmin"
	lib := seedLibraryForUser(t, db, u, "Team")
	r := setupRouter(h, u)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/libraries"},
		{http.MethodPost, "/api/libraries"},
		{http.MethodDelete, fmt.Sprintf("/api/libraries/%d", lib.ID)},
		{http.MethodPost, "/api/me/current-library"},
	} {
		w := performRequest(r, tc.method, tc.path, map[string]any{"name": "Another", "library_id": lib.ID})
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: got %d, want 404", tc.method, tc.path, w.Code)
		}
	}
	var n int64
	if err := db.Model(&models.Library{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("libraries: got %d, want the one library untouched", n)
	}
}
