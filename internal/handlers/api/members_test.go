package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

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

// Nobody can switch library, so a membership of a second library could never
// be reached. Only an install from before libraries stopped being created can
// have one.
func TestPostLibraryMember_RejectsUserOfAnotherLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "admin@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	target := seedUser(t, db, "member@example.com")
	seedLibraryForUser(t, db, target, "Elsewhere")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, fmt.Sprintf("/api/libraries/%d/members", lib.ID), addMemberRequest{
		Email: "member@example.com",
		Role:  "member",
	})
	requireStatus(t, w, http.StatusConflict)

	var n int64
	if err := db.Model(&models.LibraryMembership{}).
		Where("library_id = ? AND user_id = ?", lib.ID, target.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("membership created despite 409")
	}
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

// holdAdminCounts makes each caller wait, after counting a library's admins,
// until both have counted or a moment has passed. It lines two requests up on
// the window between the last-admin check and the write.
func holdAdminCounts(t *testing.T, db *gorm.DB) {
	t.Helper()
	var calls atomic.Int32
	var arrived sync.WaitGroup
	arrived.Add(2)
	err := db.Callback().Query().After("gorm:query").Register("test:hold_admin_counts", func(d *gorm.DB) {
		sql := d.Statement.SQL.String()
		if d.Statement.Table != "library_memberships" || !strings.Contains(sql, "role =") || strings.Contains(sql, "user_id") {
			return
		}
		if calls.Add(1) > 2 {
			return
		}
		arrived.Done()
		both := make(chan struct{})
		go func() { arrived.Wait(); close(both) }()
		select {
		case <-both:
		case <-time.After(500 * time.Millisecond):
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Two admins demoting or removing each other at once must leave one admin.
func TestLibraryMember_ConcurrentChangesKeepAnAdmin(t *testing.T) {
	cases := map[string]struct {
		method string
		body   any
	}{
		"demote": {http.MethodPatch, patchMemberRequest{Role: "member"}},
		"remove": {http.MethodDelete, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, db := newTestHandler(t)
			a := seedUser(t, db, "a@example.com")
			lib := seedLibraryAndSwitch(t, h, a)
			b := seedUser(t, db, "b@example.com")
			seedMembership(t, db, lib.ID, b.ID, "admin")
			holdAdminCounts(t, db)

			codes := make([]int, 2)
			var done sync.WaitGroup
			for i, pair := range [][2]*models.User{{a, b}, {b, a}} {
				done.Add(1)
				go func() {
					defer done.Done()
					path := fmt.Sprintf("/api/libraries/%d/members/%d", lib.ID, pair[1].ID)
					codes[i] = performRequest(setupRouter(h, pair[0]), tc.method, path, tc.body).Code
				}()
			}
			done.Wait()

			var admins int64
			if err := db.Model(&models.LibraryMembership{}).Where("library_id = ? AND role = ?", lib.ID, "admin").Count(&admins).Error; err != nil {
				t.Fatal(err)
			}
			if admins != 1 {
				t.Fatalf("%d admins left (statuses %v), want 1", admins, codes)
			}
			if !slices.Contains(codes, http.StatusConflict) {
				t.Errorf("statuses %v, want one 409", codes)
			}
		})
	}
}
