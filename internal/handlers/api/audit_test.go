package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestGetLibraryAudit_FiltersAndPagination(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	// Two action types, one in scope by user, plus one cross-library row.
	other := seedLibrary(t, db, "Other")

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		row := models.AuditLog{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.create",
			TargetType: "asset",
			CreatedAt:  now.Add(time.Duration(i) * time.Second),
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Different action type in same library.
	if err := db.Create(&models.AuditLog{
		LibraryID:  &lib.ID,
		UserID:     &u.ID,
		ActionType: "asset.delete",
		TargetType: "asset",
		CreatedAt:  now.Add(10 * time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}
	// Cross-library row that must not leak.
	if err := db.Create(&models.AuditLog{
		LibraryID:  &other.ID,
		UserID:     &u.ID,
		ActionType: "asset.create",
		TargetType: "asset",
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)

	// All entries in the current library.
	w := performGET(r, "/api/audit")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[auditListResponse](t, w)
	if body.Total != 4 {
		t.Fatalf("expected 4 entries in scope, got %d", body.Total)
	}

	// Filter by action_type.
	w = performGET(r, "/api/audit?action_type=asset.create")
	requireStatus(t, w, http.StatusOK)
	body = jsonResponse[auditListResponse](t, w)
	if body.Total != 3 {
		t.Fatalf("expected 3 asset.create rows, got %d", body.Total)
	}
}

func TestGetAdminAudit_RequiresSysadmin(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	w := performGET(r, "/api/admin/audit")
	requireStatus(t, w, http.StatusForbidden)

	// Promote and re-attempt.
	u.Role = "sysadmin"
	if err := db.Save(u).Error; err != nil {
		t.Fatal(err)
	}
	r = setupRouter(h, u)
	w = performGET(r, "/api/admin/audit")
	requireStatus(t, w, http.StatusOK)
}

func TestGetAdminAudit_ScopeAllMergesLibraryRows(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	u.Role = "sysadmin"
	if err := db.Save(u).Error; err != nil {
		t.Fatal(err)
	}
	libA := seedLibrary(t, db, "Alpha")
	libB := seedLibrary(t, db, "Beta")

	now := time.Now().UTC()
	rows := []models.AuditLog{
		// Server-level event.
		{LibraryID: nil, UserID: &u.ID, ActionType: "library.create", TargetType: "library", CreatedAt: now},
		// Library-scoped events that the default scope hides.
		{LibraryID: &libA.ID, UserID: &u.ID, ActionType: "asset.delete", TargetType: "asset", CreatedAt: now.Add(time.Second)},
		{LibraryID: &libB.ID, UserID: &u.ID, ActionType: "asset.create", TargetType: "asset", CreatedAt: now.Add(2 * time.Second)},
	}
	for _, r := range rows {
		if err := db.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := setupRouter(h, u)

	// Default scope: server-only — backwards compatible.
	w := performGET(r, "/api/admin/audit")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[auditListResponse](t, w)
	if body.Total != 1 {
		t.Fatalf("expected 1 server-scoped row, got %d", body.Total)
	}
	if body.Entries[0].LibraryName != "" {
		t.Fatalf("library_name should be empty on default scope: %+v", body.Entries[0])
	}

	// scope=all: server + library-scoped, with library_name populated.
	w = performGET(r, "/api/admin/audit?scope=all")
	requireStatus(t, w, http.StatusOK)
	body = jsonResponse[auditListResponse](t, w)
	if body.Total != 3 {
		t.Fatalf("expected 3 rows on scope=all, got %d", body.Total)
	}
	var sawAlpha, sawBeta bool
	for _, e := range body.Entries {
		if e.LibraryID != nil && *e.LibraryID == libA.ID && e.LibraryName == "Alpha" {
			sawAlpha = true
		}
		if e.LibraryID != nil && *e.LibraryID == libB.ID && e.LibraryName == "Beta" {
			sawBeta = true
		}
	}
	if !sawAlpha || !sawBeta {
		t.Fatalf("expected library_name populated on scope=all rows: %+v", body.Entries)
	}
}

func TestGetAssetAudit_ScopedToAsset(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	asset := models.Asset{LibraryID: lib.ID, Name: "a", NeedsReview: true}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	other := models.Asset{LibraryID: lib.ID, Name: "b", NeedsReview: true}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	id1 := asset.ID
	id2 := other.ID
	rows := []models.AuditLog{
		{LibraryID: &lib.ID, UserID: &u.ID, ActionType: "asset.update", TargetType: "asset", TargetID: &id1},
		{LibraryID: &lib.ID, UserID: &u.ID, ActionType: "asset.update", TargetType: "asset", TargetID: &id1},
		{LibraryID: &lib.ID, UserID: &u.ID, ActionType: "asset.update", TargetType: "asset", TargetID: &id2},
	}
	for _, r := range rows {
		r.CreatedAt = time.Now().UTC()
		if err := db.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets/1/audit")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[auditListResponse](t, w)
	if body.Total != 2 {
		t.Fatalf("expected 2 audit rows for asset 1, got %d", body.Total)
	}

	// Cross-library asset 404s.
	otherLib := seedLibrary(t, db, "Sealed")
	stranger := models.Asset{LibraryID: otherLib.ID, Name: "x", NeedsReview: true}
	if err := db.Create(&stranger).Error; err != nil {
		t.Fatal(err)
	}
	w = performGET(r, "/api/assets/9999/audit")
	requireStatus(t, w, http.StatusNotFound)
}

// An import creates rows a person asked for, so the feed names them.
func TestPostImport_AuditsTheAssetAndVersionUnderTheImporter(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	body, ctype := buildMultipart(t, "file", "tile.png", makeTestPNG(t, 8, 8))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	first := jsonResponse[importEnqueueResponse](t, w)

	body, ctype = buildMultipartWithFields(t, "file", "tile v2.png", makeTestPNG(t, 9, 9), map[string]string{
		"asset_id": fmt.Sprint(first.AssetID), "store_version": "2",
	})
	requireStatus(t, postMultipart(r, "/api/imports", body, ctype), http.StatusAccepted)

	for _, action := range []string{"asset.create", "asset.version.create"} {
		var row models.AuditLog
		if err := db.Where("action_type = ? AND target_id = ?", action, first.AssetID).First(&row).Error; err != nil {
			t.Fatalf("no %s audit row: %v", action, err)
		}
		if row.UserID == nil || *row.UserID != u.ID || row.LibraryID == nil || *row.LibraryID != lib.ID {
			t.Fatalf("%s row names user %v in library %v, want %d in %d", action, row.UserID, row.LibraryID, u.ID, lib.ID)
		}
	}
}

// action_type takes a comma-separated list so one filter chip can cover a
// family of events; an entry ending in ".*" matches every type under it.
func TestGetLibraryAudit_ActionTypeListAndPrefix(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	for _, action := range []string{
		"asset.tag", "asset.untag", "asset.licence.set", "asset.licence.update",
		"asset.licence_note", "asset.create",
	} {
		if err := db.Create(&models.AuditLog{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: action,
			TargetType: "asset",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := setupRouter(h, u)

	for _, tc := range []struct {
		query string
		want  int64
	}{
		{"asset.tag", 1},
		{"asset.tag,asset.untag", 2},
		{"asset.licence.*", 2},
		{"asset.licence.*,asset.create", 3},
	} {
		w := performGET(r, "/api/audit?action_type="+tc.query)
		requireStatus(t, w, http.StatusOK)
		if got := jsonResponse[auditListResponse](t, w).Total; got != tc.want {
			t.Errorf("action_type=%s: got %d rows, want %d", tc.query, got, tc.want)
		}
	}
}

// An unbounded action_type list would outrun Postgres's bind-parameter limit
// and fail as a 500; past the cap it is the caller's error.
func TestGetLibraryAudit_RejectsOverlongActionTypeList(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	list := strings.TrimSuffix(strings.Repeat("asset.tag,", maxAuditActionTypes+1), ",")
	requireStatus(t, performGET(r, "/api/audit?action_type="+list), http.StatusBadRequest)
}

// q searches every entry in the feed, not one page of it: user name, user
// email and action type, case-insensitively, with LIKE wildcards literal.
func TestGetLibraryAudit_SearchesEveryEntry(t *testing.T) {
	h, db := newTestHandler(t)
	alice := seedUser(t, db, "alice@example.com")
	alice.DisplayName = "Alice Liddell"
	if err := db.Save(alice).Error; err != nil {
		t.Fatal(err)
	}
	bob := seedUser(t, db, "bob@example.com")
	lib := seedLibraryAndSwitch(t, h, alice)
	other := seedLibrary(t, db, "Other")

	now := time.Now().UTC()
	// Bob's one entry is the oldest, so it is not on the first page of the
	// unfiltered feed.
	rows := []models.AuditLog{
		{LibraryID: &lib.ID, UserID: &bob.ID, ActionType: "asset.delete", TargetType: "asset", CreatedAt: now.Add(-time.Hour)},
		// Another library's row matches q but must not leak.
		{LibraryID: &other.ID, UserID: &bob.ID, ActionType: "asset.create", TargetType: "asset", CreatedAt: now},
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, models.AuditLog{LibraryID: &lib.ID, UserID: &alice.ID, ActionType: "asset.create", TargetType: "asset", CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := setupRouter(h, alice)

	for _, tc := range []struct {
		q    string
		want int64
	}{
		{"BOB@", 1},      // email
		{"liddell", 5},   // display name
		{"asset.del", 1}, // action type
		{"%", 0},         // a wildcard matches only itself
		{"alice bob", 0}, // one substring, not words
		{"", 6},          // no search
	} {
		w := performGET(r, "/api/audit?limit=2&q="+url.QueryEscape(tc.q))
		requireStatus(t, w, http.StatusOK)
		body := jsonResponse[auditListResponse](t, w)
		if body.Total != tc.want {
			t.Errorf("q=%q: got %d rows, want %d", tc.q, body.Total, tc.want)
		}
		if tc.q == "BOB@" && (len(body.Entries) != 1 || body.Entries[0].UserEmail != "bob@example.com") {
			t.Errorf("q=%q: want bob's entry on the page, got %+v", tc.q, body.Entries)
		}
	}
}

func TestGetAdminAudit_Searches(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "root@example.com")
	u.Role = "sysadmin"
	if err := db.Save(u).Error; err != nil {
		t.Fatal(err)
	}
	lib := seedLibrary(t, db, "Alpha")
	for _, row := range []models.AuditLog{
		{LibraryID: nil, UserID: &u.ID, ActionType: "library.delete", TargetType: "library"},
		{LibraryID: nil, UserID: &u.ID, ActionType: "admin.user.update", TargetType: "user"},
		{LibraryID: &lib.ID, UserID: &u.ID, ActionType: "asset.delete", TargetType: "asset"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := setupRouter(h, u)

	for _, tc := range []struct {
		query string
		want  int64
	}{
		{"q=delete", 1},
		{"q=delete&scope=all", 2},
	} {
		w := performGET(r, "/api/admin/audit?"+tc.query)
		requireStatus(t, w, http.StatusOK)
		if got := jsonResponse[auditListResponse](t, w).Total; got != tc.want {
			t.Errorf("%s: got %d rows, want %d", tc.query, got, tc.want)
		}
	}
}
