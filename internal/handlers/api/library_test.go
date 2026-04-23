package api

import (
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

type libraryListResponse struct {
	Libraries []libraryMembershipView `json:"libraries"`
}

func TestGetLibraries_ListsMemberships(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	personal := seedLibrary(t, db, "Personal")
	shared := seedLibrary(t, db, "Shared")
	// User is a member of both libraries, with different roles.
	seedMembership(t, db, personal.ID, u.ID, "admin")
	seedMembership(t, db, shared.ID, u.ID, "member")
	// Another user's membership in a separate library is not leaked.
	other := seedUser(t, db, "alice@example.com")
	otherLib := seedLibrary(t, db, "Alice's library")
	seedMembership(t, db, otherLib.ID, other.ID, "admin")

	u.CurrentLibraryID = &personal.ID
	if err := db.Save(u).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/libraries")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[libraryListResponse](t, w)
	if len(body.Libraries) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(body.Libraries))
	}
	byName := map[string]libraryMembershipView{}
	for _, l := range body.Libraries {
		byName[l.Name] = l
	}
	if byName["Personal"].Role != "admin" || !byName["Personal"].IsCurrent {
		t.Fatalf("personal view wrong: %+v", byName["Personal"])
	}
	if byName["Shared"].Role != "member" || byName["Shared"].IsCurrent {
		t.Fatalf("shared view wrong: %+v", byName["Shared"])
	}
}

func TestPostCurrentLibrary_RejectsNonMember(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibrary(t, db, "Not mine")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/me/current-library", setCurrentLibraryRequest{
		LibraryID: lib.ID,
	})
	requireStatus(t, w, http.StatusForbidden)
}

func TestPostCurrentLibrary_UpdatesUser(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
	lib := seedLibrary(t, db, "Personal")
	seedMembership(t, db, lib.ID, u.ID, "admin")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/me/current-library", setCurrentLibraryRequest{
		LibraryID: lib.ID,
	})
	requireStatus(t, w, http.StatusOK)

	var refreshed models.User
	if err := db.First(&refreshed, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.CurrentLibraryID == nil || *refreshed.CurrentLibraryID != lib.ID {
		t.Fatalf("current_library_id not updated: %+v", refreshed.CurrentLibraryID)
	}
}

func TestGetMe_IncludesCurrentLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "thomas@datch.io")
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
