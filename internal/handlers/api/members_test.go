package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestGetLibraryMembers_ReturnsMembership(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "admin@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	other := seedUser(t, db, "member@example.com")
	seedMembership(t, db, lib.ID, other.ID, "member")

	r := setupRouter(h, u)
	w := performGET(r, "/api/libraries/1/members")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[memberListResponse](t, w)
	if len(body.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(body.Members))
	}
}

func TestPostLibraryMember_AddsExistingUser(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "admin@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	target := seedUser(t, db, "member@example.com")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/libraries/1/members", addMemberRequest{
		Email: "member@example.com",
		Role:  "member",
	})
	requireStatus(t, w, http.StatusCreated)

	var mem models.LibraryMembership
	if err := db.Where("library_id = ? AND user_id = ?", lib.ID, target.ID).First(&mem).Error; err != nil {
		t.Fatal(err)
	}
	if mem.Role != "member" {
		t.Fatalf("role: %q", mem.Role)
	}

	// Duplicate add → 409.
	body, _ := buildMultipart(t, "f", "f", []byte{}) // unused; kept for shape
	_ = body
	w2 := performRequest(r, http.MethodPost, "/api/libraries/1/members", addMemberRequest{
		Email: "member@example.com",
		Role:  "member",
	})
	requireStatus(t, w2, http.StatusConflict)
}

func TestPostLibraryMember_RequiresAdmin(t *testing.T) {
	h, db := newTestHandler(t)
	admin := seedUser(t, db, "admin@example.com")
	member := seedUser(t, db, "member@example.com")
	lib := seedLibrary(t, db, "Personal")
	seedMembership(t, db, lib.ID, admin.ID, "admin")
	seedMembership(t, db, lib.ID, member.ID, "member")
	member.CurrentLibraryID = &lib.ID
	if err := db.Save(member).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, member)
	w := performRequest(r, http.MethodPost, "/api/libraries/1/members", addMemberRequest{
		Email: "newcomer@example.com",
		Role:  "member",
	})
	requireStatus(t, w, http.StatusForbidden)
}

func TestPatchLibraryMember_RejectsLastAdminDemotion(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "admin@example.com")
	seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/libraries/1/members/1", patchMemberRequest{Role: "member"})
	requireStatus(t, w, http.StatusConflict)
}

func TestDeleteLibraryMember_RejectsLastAdmin(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "admin@example.com")
	seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodDelete, "/api/libraries/1/members/1", nil)
	requireStatus(t, w, http.StatusConflict)
}

func TestDeleteLibraryMember_RemovesMember(t *testing.T) {
	h, db := newTestHandler(t)
	admin := seedUser(t, db, "admin@example.com")
	lib := seedLibraryAndSwitch(t, h, admin)
	other := seedUser(t, db, "leaving@example.com")
	seedMembership(t, db, lib.ID, other.ID, "member")
	other.CurrentLibraryID = &lib.ID
	if err := db.Save(other).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, admin)
	w := performRequest(r, http.MethodDelete, "/api/libraries/1/members/2", nil)
	requireStatus(t, w, http.StatusNoContent)

	var count int64
	if err := db.Model(&models.LibraryMembership{}).
		Where("library_id = ? AND user_id = ?", lib.ID, other.ID).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected membership deleted, found %d", count)
	}
	// Removed user's current_library_id is cleared.
	var refreshed models.User
	if err := db.First(&refreshed, other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.CurrentLibraryID != nil {
		t.Fatalf("expected current_library_id cleared, got %+v", refreshed.CurrentLibraryID)
	}
}
