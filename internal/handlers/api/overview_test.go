package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

type overviewPayload struct {
	Library struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"library"`
	Counts struct {
		Assets           int `json:"assets"`
		NeedsReview      int `json:"needs_review"`
		UpdatesAvailable int `json:"updates_available"`
		Collections      int `json:"collections"`
		Tags             int `json:"tags"`
		Projects         int `json:"projects"`
		ImportsInFlight  int `json:"imports_in_flight"`
		ImportsFailed    int `json:"imports_failed"`
	} `json:"counts"`
	RecentImports []struct {
		ID        uint      `json:"id"`
		AssetID   uint      `json:"asset_id"`
		AssetName string    `json:"asset_name"`
		Stage     string    `json:"stage"`
		Error     string    `json:"error"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"recent_imports"`
}

func seedImportRow(t *testing.T, db *gorm.DB, assetID uint, stage string, at time.Time) {
	t.Helper()
	imp := models.AssetImport{AssetID: assetID, Stage: stage, UpdatedAt: at}
	if err := db.Create(&imp).Error; err != nil {
		t.Fatalf("seed import: %v", err)
	}
}

func TestGetOverview_Counts(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "over@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")

	a1 := seedAsset(t, db, lib.ID, "Reviewed")
	if err := db.Model(&models.Asset{}).Where("id = ?", a1.ID).
		Update("needs_review", false).Error; err != nil {
		t.Fatalf("clear review: %v", err)
	}
	a2 := seedAsset(t, db, lib.ID, "Unreviewed")
	a3 := seedAsset(t, db, lib.ID, "Also unreviewed")

	if err := db.Create(&models.Collection{LibraryID: lib.ID, Name: "Characters"}).Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}
	for _, p := range []string{"Characters", "Characters/Humanoid"} {
		if err := db.Create(&models.Tag{LibraryID: lib.ID, Path: p}).Error; err != nil {
			t.Fatalf("seed tag: %v", err)
		}
	}
	if err := db.Create(&models.Project{LibraryID: lib.ID, Name: "Game"}).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}

	now := time.Now().UTC()
	seedImportRow(t, db, a1.ID, "done", now.Add(-3*time.Hour))
	seedImportRow(t, db, a2.ID, "extracting", now.Add(-2*time.Hour))
	seedImportRow(t, db, a3.ID, "failed", now.Add(-time.Hour))

	w := performGET(setupRouter(h, u), "/api/overview")
	requireStatus(t, w, http.StatusOK)
	got := jsonResponse[overviewPayload](t, w)

	if got.Library.ID != lib.ID || got.Library.Name != "Main" || got.Library.Role != "admin" {
		t.Fatalf("library = %+v", got.Library)
	}
	for _, tc := range []struct {
		name string
		got  int
		want int
	}{
		{"assets", got.Counts.Assets, 3},
		{"needs_review", got.Counts.NeedsReview, 2},
		{"collections", got.Counts.Collections, 1},
		{"tags", got.Counts.Tags, 2},
		{"projects", got.Counts.Projects, 1},
		{"imports_in_flight", got.Counts.ImportsInFlight, 1},
		{"imports_failed", got.Counts.ImportsFailed, 1},
	} {
		if tc.got != tc.want {
			t.Errorf("counts.%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}

	if len(got.RecentImports) != 3 {
		t.Fatalf("recent imports = %d, want 3", len(got.RecentImports))
	}
	// Newest first.
	if got.RecentImports[0].Stage != "failed" || got.RecentImports[2].Stage != "done" {
		t.Fatalf("recent imports not ordered newest-first: %+v", got.RecentImports)
	}
	if got.RecentImports[0].AssetName != "Also unreviewed" {
		t.Fatalf("asset name not joined: %+v", got.RecentImports[0])
	}
}

// The tenancy boundary: a member of one library must see nothing of another's,
// even when both exist on the same server.
func TestGetOverview_IsolatedBetweenLibraries(t *testing.T) {
	h, db := newTestHandler(t)
	alice := seedUser(t, db, "alice@example.com")
	aliceLib := seedLibraryForUser(t, db, alice, "Alice")
	bob := seedUser(t, db, "bob@example.com")
	bobLib := seedLibraryForUser(t, db, bob, "Bob")

	seedAsset(t, db, aliceLib.ID, "Alice asset one")
	seedAsset(t, db, aliceLib.ID, "Alice asset two")
	bobAsset := seedAsset(t, db, bobLib.ID, "Bob asset")
	seedImportRow(t, db, bobAsset.ID, "failed", time.Now().UTC())
	if err := db.Create(&models.Collection{LibraryID: bobLib.ID, Name: "Bob's"}).Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}

	got := jsonResponse[overviewPayload](t, performGET(setupRouter(h, alice), "/api/overview"))
	if got.Library.Name != "Alice" {
		t.Fatalf("library = %q", got.Library.Name)
	}
	if got.Counts.Assets != 2 {
		t.Fatalf("assets = %d, want 2 — Bob's asset leaked", got.Counts.Assets)
	}
	if got.Counts.Collections != 0 {
		t.Fatalf("collections = %d, want 0 — Bob's collection leaked", got.Counts.Collections)
	}
	if got.Counts.ImportsFailed != 0 {
		t.Fatalf("imports_failed = %d, want 0 — Bob's import leaked", got.Counts.ImportsFailed)
	}
	for _, imp := range got.RecentImports {
		if imp.AssetName == "Bob asset" {
			t.Fatal("another library's import appeared in the tail")
		}
	}
}

// An empty library reports zeroes and an empty list, not a null the client
// would have to guard.
func TestGetOverview_EmptyLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "empty@example.com")
	seedLibraryForUser(t, db, u, "Fresh")

	w := performGET(setupRouter(h, u), "/api/overview")
	requireStatus(t, w, http.StatusOK)
	if body := w.Body.String(); !strings.Contains(body, `"recent_imports":[]`) {
		t.Fatalf("recent_imports must serialise as [] not null: %s", body)
	}
	got := jsonResponse[overviewPayload](t, w)
	if got.Counts.Assets != 0 || len(got.RecentImports) != 0 {
		t.Fatalf("got %+v", got.Counts)
	}
}

func TestGetOverview_CapsTheImportTail(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "many@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")

	now := time.Now().UTC()
	for i := 0; i < overviewRecentImports+4; i++ {
		a := seedAsset(t, db, lib.ID, "Asset "+string(rune('a'+i)))
		seedImportRow(t, db, a.ID, "done", now.Add(-time.Duration(i)*time.Minute))
	}

	got := jsonResponse[overviewPayload](t, performGET(setupRouter(h, u), "/api/overview"))
	if len(got.RecentImports) != overviewRecentImports {
		t.Fatalf("tail = %d, want %d", len(got.RecentImports), overviewRecentImports)
	}
	// The cap must not distort the counts, which describe the whole library.
	if got.Counts.Assets != overviewRecentImports+4 {
		t.Fatalf("assets = %d, want %d", got.Counts.Assets, overviewRecentImports+4)
	}
}

func TestGetOverview_RequiresALibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "nolib@example.com")

	w := performGET(setupRouter(h, u), "/api/overview")
	requireStatus(t, w, http.StatusForbidden)
}
