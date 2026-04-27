package api

import (
	"net/http"
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
