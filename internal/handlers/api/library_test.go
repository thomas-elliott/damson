package api

import (
	"net/http"
	"testing"
	"time"

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
	u := seedUser(t, db, "librarian@example.com")
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
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibrary(t, db, "Not mine")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/me/current-library", setCurrentLibraryRequest{
		LibraryID: lib.ID,
	})
	requireStatus(t, w, http.StatusForbidden)
}

func TestPostCurrentLibrary_UpdatesUser(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
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

func TestDeleteLibrary_RequiresAdmin(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibrary(t, db, "Personal")
	seedMembership(t, db, lib.ID, u.ID, "member")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodDelete, "/api/libraries/1", nil)
	requireStatus(t, w, http.StatusForbidden)
}

func TestDeleteLibrary_CascadesAndAudits(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	other := seedUser(t, db, "alice@example.com")
	lib := seedLibrary(t, db, "Personal")
	keep := seedLibrary(t, db, "Keep")
	seedMembership(t, db, lib.ID, u.ID, "admin")
	seedMembership(t, db, lib.ID, other.ID, "member")
	seedMembership(t, db, keep.ID, u.ID, "admin")

	// Seed library-scoped rows that should all disappear with the library.
	asset := models.Asset{LibraryID: lib.ID, Name: "victim", NeedsReview: false}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	version := models.AssetVersion{AssetID: asset.ID, VersionLabel: "v1", ImportDate: time.Now().UTC()}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AssetFile{AssetVersionID: version.ID, RelativePath: "a", SizeBytes: 1, ContentHash: "h", ObjectKey: "o"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Project{LibraryID: lib.ID, Name: "p"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Collection{LibraryID: lib.ID, Name: "c"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Tag{LibraryID: lib.ID, Path: "tag"}).Error; err != nil {
		t.Fatal(err)
	}
	// In-library audit row that will cascade away with the library.
	libID := lib.ID
	if err := db.Create(&models.AuditLog{LibraryID: &libID, UserID: &u.ID, ActionType: "asset.create", TargetType: "asset", CreatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	// Audit row in another library — must survive.
	keepID := keep.ID
	if err := db.Create(&models.AuditLog{LibraryID: &keepID, UserID: &u.ID, ActionType: "asset.create", TargetType: "asset", CreatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}

	// Both members had this library as current.
	currentLib := lib.ID
	u.CurrentLibraryID = &currentLib
	other.CurrentLibraryID = &currentLib
	if err := db.Save(u).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Save(other).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodDelete, "/api/libraries/1", nil)
	requireStatus(t, w, http.StatusNoContent)

	// Library row gone.
	var libCount int64
	if err := db.Model(&models.Library{}).Where("id = ?", lib.ID).Count(&libCount).Error; err != nil {
		t.Fatal(err)
	}
	if libCount != 0 {
		t.Fatalf("library not deleted")
	}

	for _, table := range []string{"assets", "asset_versions", "asset_files", "projects", "collections", "tags", "library_memberships"} {
		var n int64
		if err := db.Table(table).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if table == "library_memberships" && n != 1 {
			t.Fatalf("expected 1 membership left (the keep library), got %d", n)
		}
		if table != "library_memberships" && n != 0 {
			t.Fatalf("expected %s to be empty, got %d rows", table, n)
		}
	}

	// Each former member's current_library_id cleared and tokens_valid_after bumped.
	var refreshedActor models.User
	if err := db.First(&refreshedActor, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshedActor.CurrentLibraryID != nil {
		t.Fatalf("actor current_library_id not cleared: %v", *refreshedActor.CurrentLibraryID)
	}
	if refreshedActor.TokensValidAfter == nil {
		t.Fatalf("actor tokens_valid_after not bumped")
	}
	var refreshedOther models.User
	if err := db.First(&refreshedOther, other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshedOther.TokensValidAfter == nil {
		t.Fatalf("other tokens_valid_after not bumped")
	}

	// One server-scoped audit row written; the other-library row is preserved.
	var serverRows []models.AuditLog
	if err := db.Where("library_id IS NULL AND action_type = ?", "library.delete").Find(&serverRows).Error; err != nil {
		t.Fatal(err)
	}
	if len(serverRows) != 1 {
		t.Fatalf("expected 1 library.delete row, got %d", len(serverRows))
	}
	var keepRows int64
	if err := db.Model(&models.AuditLog{}).Where("library_id = ?", keep.ID).Count(&keepRows).Error; err != nil {
		t.Fatal(err)
	}
	if keepRows != 1 {
		t.Fatalf("audit row in other library was clobbered (rows=%d)", keepRows)
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
