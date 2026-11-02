package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/auth"
)

// An invited member has a membership but no current library. They open the
// library they belong to, in /api/me and on library-scoped routes alike.
func TestRequireLibrary_InvitedMemberOpensTheirLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	admin := seedUser(t, db, "admin@example.com")
	lib := seedLibraryForUser(t, db, admin, "Team")
	seedAsset(t, db, lib.ID, "Shared rocks")
	member := seedUser(t, db, "member@example.com")
	seedMembership(t, db, lib.ID, member.ID, "member")
	r := setupRouter(h, member)

	me := jsonResponse[meResponse](t, performGET(r, "/api/me"))
	if me.CurrentLibrary == nil || me.CurrentLibrary.ID != lib.ID || me.CurrentLibrary.Role != "member" {
		t.Fatalf("current_library: got %+v, want library %d as member", me.CurrentLibrary, lib.ID)
	}

	w := performGET(r, "/api/assets")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[assetListResponse](t, w)
	if body.Total != 1 {
		t.Fatalf("assets: got %d, want the library's 1", body.Total)
	}
}

// A current library the user no longer belongs to gives way to the one they do.
func TestRequireLibrary_StaleCurrentLibraryFallsBackToMembership(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "member@example.com")
	gone := seedLibrary(t, db, "Gone")
	kept := seedLibrary(t, db, "Kept")
	seedMembership(t, db, kept.ID, u.ID, "member")
	u.CurrentLibraryID = &gone.ID
	if err := db.Save(u).Error; err != nil {
		t.Fatal(err)
	}
	r := setupRouter(h, u)

	me := jsonResponse[meResponse](t, performGET(r, "/api/me"))
	if me.CurrentLibrary == nil || me.CurrentLibrary.ID != kept.ID {
		t.Fatalf("current_library: got %+v, want %d", me.CurrentLibrary, kept.ID)
	}
	requireStatus(t, performGET(r, "/api/assets"), http.StatusOK)
}

// A user in two libraries (only reachable on installs that predate one library
// per team) keeps the one they were in, not whichever sorts first.
func TestRequireLibrary_KeepsCurrentLibraryWhileAMember(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "member@example.com")
	first := seedLibrary(t, db, "First")
	seedMembership(t, db, first.ID, u.ID, "member")
	second := seedLibraryForUser(t, db, u, "Second")
	r := setupRouter(h, u)

	me := jsonResponse[meResponse](t, performGET(r, "/api/me"))
	if me.CurrentLibrary == nil || me.CurrentLibrary.ID != second.ID {
		t.Fatalf("current_library: got %+v, want %d", me.CurrentLibrary, second.ID)
	}
}

// A signed-in user who belongs to no library gets no library block and a 403
// on library-scoped routes: the SPA shows "No library access yet".
func TestRequireLibrary_NoMembershipIsForbidden(t *testing.T) {
	h, db := newTestHandler(t)
	admin := seedUser(t, db, "admin@example.com")
	seedLibraryForUser(t, db, admin, "Team")
	stranger := seedUser(t, db, "stranger@example.com")
	r := setupRouter(h, stranger)

	me := jsonResponse[meResponse](t, performGET(r, "/api/me"))
	if me.CurrentLibrary != nil {
		t.Fatalf("current_library: got %+v, want none", me.CurrentLibrary)
	}
	requireStatus(t, performGET(r, "/api/assets"), http.StatusForbidden)
}

// A personal access token stays pinned to its library even when its owner
// also belongs to another one.
func TestRequireLibrary_TokenStaysPinned(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "cli@example.com")
	other := seedLibrary(t, db, "Other")
	seedMembership(t, db, other.ID, u.ID, "member")
	pinned := seedLibrary(t, db, "Pinned")
	seedMembership(t, db, pinned.ID, u.ID, "member")
	seedAsset(t, db, pinned.ID, "Pinned asset")
	secret := seedTokenFor(t, db, u.ID, pinned.ID, auth.ScopeRead, nil)
	r := setupTokenRouter(h, db)

	me := jsonResponse[meResponse](t, getWithBearer(r, "/api/me", secret))
	if me.CurrentLibrary == nil || me.CurrentLibrary.ID != pinned.ID {
		t.Fatalf("current_library: got %+v, want pinned %d", me.CurrentLibrary, pinned.ID)
	}
	w := getWithBearer(r, "/api/assets", secret)
	requireStatus(t, w, http.StatusOK)
	if got := jsonResponse[assetListResponse](t, w).Total; got != 1 {
		t.Fatalf("assets: got %d, want the pinned library's 1", got)
	}
}
