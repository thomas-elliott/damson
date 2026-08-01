package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

type preflightPayload struct {
	Results []struct {
		Index        int    `json:"index"`
		Filename     string `json:"filename"`
		Status       string `json:"status"`
		AssetID      uint   `json:"asset_id"`
		AssetName    string `json:"asset_name"`
		VersionCount int    `json:"version_count"`
	} `json:"results"`
	LibraryTotal int `json:"library_total"`
}

func preflight(t *testing.T, h *Handler, u *models.User, filenames ...string) preflightPayload {
	t.Helper()
	entries := make([]map[string]any, len(filenames))
	for i, f := range filenames {
		entries[i] = map[string]any{"filename": f, "size_bytes": 1}
	}
	w := performRequest(setupRouter(h, u), http.MethodPost, "/api/imports/preflight",
		map[string]any{"entries": entries})
	requireStatus(t, w, http.StatusOK)
	return jsonResponse[preflightPayload](t, w)
}

func TestPreflight_NameMatching(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "pre@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	held := seedAsset(t, db, lib.ID, "RPG Builder")

	got := preflight(t, h,
		u,
		"RPG Builder.unitypackage",
		"Brand New Thing.unitypackage",
	)

	if len(got.Results) != 2 {
		t.Fatalf("results = %d", len(got.Results))
	}
	if got.Results[0].Status != preflightNameMatch || got.Results[0].AssetID != held.ID {
		t.Fatalf("expected a name match: %+v", got.Results[0])
	}
	if got.Results[0].AssetName != "RPG Builder" {
		t.Fatalf("asset name = %q", got.Results[0].AssetName)
	}
	if got.Results[1].Status != preflightNew {
		t.Fatalf("expected new: %+v", got.Results[1])
	}
	if got.LibraryTotal != 1 {
		t.Fatalf("library_total = %d, want 1", got.LibraryTotal)
	}
}

// The same matcher as suggested_match: version suffixes and punctuation are
// normalised away, so `RPG Builder v1.2.unitypackage` still matches.
func TestPreflight_UsesTheSameNormalisationAsSuggestedMatch(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "norm@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	held := seedAsset(t, db, lib.ID, "RPG Builder")

	for _, filename := range []string{
		"RPG Builder.unitypackage",
		"RPG_Builder.unitypackage",
		"rpg builder.zip",
		"RPG Builder v1.2.unitypackage",
	} {
		got := preflight(t, h, u, filename)
		if got.Results[0].Status != preflightNameMatch || got.Results[0].AssetID != held.ID {
			t.Errorf("%q: expected a match, got %+v", filename, got.Results[0])
		}
	}

	for _, filename := range []string{"Totally Different.unitypackage", "RPG Builder Pro.zip"} {
		got := preflight(t, h, u, filename)
		if got.Results[0].Status != preflightNew {
			t.Errorf("%q: expected new, got %+v", filename, got.Results[0])
		}
	}
}

// Results correlate by index, not filename: a manifest may legitimately carry
// the same basename twice from different directories.
func TestPreflight_ResultsCorrelateByIndex(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "idx@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	seedAsset(t, db, lib.ID, "Shared")

	got := preflight(t, h, u, "Shared.unitypackage", "Other.zip", "Shared.unitypackage")
	if len(got.Results) != 3 {
		t.Fatalf("results = %d", len(got.Results))
	}
	for i, r := range got.Results {
		if r.Index != i {
			t.Fatalf("result %d carries index %d", i, r.Index)
		}
	}
	if got.Results[0].Status != preflightNameMatch || got.Results[2].Status != preflightNameMatch {
		t.Fatalf("both duplicates should match: %+v", got.Results)
	}
	if got.Results[1].Status != preflightNew {
		t.Fatalf("the middle entry should be new: %+v", got.Results[1])
	}
}

func TestPreflight_ReportsVersionCount(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "vc@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	asset, _ := seedAssetWithVersion(t, db, lib.ID, "Multi", "v1")
	if err := db.Create(&models.AssetVersion{AssetID: asset.ID, StoreVersion: "2.0"}).Error; err != nil {
		t.Fatalf("second version: %v", err)
	}

	got := preflight(t, h, u, "Multi.unitypackage")
	if got.Results[0].VersionCount != 2 {
		t.Fatalf("version_count = %d, want 2", got.Results[0].VersionCount)
	}
}

// The tenancy boundary: another library's assets must not produce a match.
func TestPreflight_IsolatedBetweenLibraries(t *testing.T) {
	h, db := newTestHandler(t)
	alice := seedUser(t, db, "alice@example.com")
	aliceLib := seedLibraryForUser(t, db, alice, "Alice")
	bob := seedUser(t, db, "bob@example.com")
	bobLib := seedLibraryForUser(t, db, bob, "Bob")
	seedAsset(t, db, bobLib.ID, "Bob Only")
	seedAsset(t, db, aliceLib.ID, "Alice Only")

	got := preflight(t, h, alice, "Bob Only.unitypackage")
	if got.Results[0].Status != preflightNew {
		t.Fatalf("another library's asset produced a match: %+v", got.Results[0])
	}
	if got.LibraryTotal != 1 {
		t.Fatalf("library_total = %d, want 1 — it must count only this library", got.LibraryTotal)
	}
}

func TestPreflight_Validation(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "val@example.com")
	seedLibraryForUser(t, db, u, "Main")
	r := setupRouter(h, u)

	t.Run("no entries", func(t *testing.T) {
		w := performRequest(r, http.MethodPost, "/api/imports/preflight", map[string]any{"entries": []any{}})
		requireStatus(t, w, http.StatusBadRequest)
	})

	// Exceeding the cap is an error, never a silent truncation: a client that
	// got back fewer results than it sent would skip the difference.
	t.Run("over the cap", func(t *testing.T) {
		entries := make([]map[string]any, maxPreflightEntries+1)
		for i := range entries {
			entries[i] = map[string]any{"filename": "x.unitypackage", "size_bytes": 1}
		}
		w := performRequest(r, http.MethodPost, "/api/imports/preflight", map[string]any{"entries": entries})
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("at the cap", func(t *testing.T) {
		entries := make([]map[string]any, maxPreflightEntries)
		for i := range entries {
			entries[i] = map[string]any{"filename": "x.unitypackage", "size_bytes": 1}
		}
		w := performRequest(r, http.MethodPost, "/api/imports/preflight", map[string]any{"entries": entries})
		requireStatus(t, w, http.StatusOK)
		if got := jsonResponse[preflightPayload](t, w); len(got.Results) != maxPreflightEntries {
			t.Fatalf("results = %d, want %d — every entry must get an answer",
				len(got.Results), maxPreflightEntries)
		}
	})

	t.Run("blank filename is new, not an error", func(t *testing.T) {
		got := preflight(t, h, u, "")
		if got.Results[0].Status != preflightNew {
			t.Fatalf("got %+v", got.Results[0])
		}
	})
}

func TestGetLibrarySettings_ExposesMaxUploadBytes(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "settings@example.com")
	seedLibraryForUser(t, db, u, "Main")

	w := performGET(setupRouter(h, u), "/api/settings")
	requireStatus(t, w, http.StatusOK)

	var got struct {
		MaxUploadBytes int64 `json:"max_upload_bytes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.MaxUploadBytes <= 0 {
		t.Fatal("max_upload_bytes must be readable by a member — otherwise a client only learns the cap from a 413 after transferring")
	}
}

// The upload cap is server-level, so it must not appear in the library
// settings audit diff, which records only what that endpoint can change.
func TestPatchLibrarySettings_AuditDiffExcludesMaxUploadBytes(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "audit-settings@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")

	w := performRequest(setupRouter(h, u), http.MethodPatch, "/api/settings",
		map[string]any{"dedupe_enabled": false})
	requireStatus(t, w, http.StatusOK)

	var entry models.AuditLog
	err := db.Where("library_id = ? AND action_type = ?", lib.ID, "library.settings.update").
		Take(&entry).Error
	if err != nil {
		t.Fatalf("audit row: %v", err)
	}
	if strings.Contains(string(entry.DiffJSON), "max_upload_bytes") {
		t.Fatalf("audit diff records a field this endpoint cannot change: %s", entry.DiffJSON)
	}
}
