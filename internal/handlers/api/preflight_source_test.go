package api

import (
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
)

func preflightEntries(t *testing.T, h *Handler, u *models.User, entries []map[string]any) preflightPayload {
	t.Helper()
	w := performRequest(setupRouter(h, u), http.MethodPost, "/api/imports/preflight",
		map[string]any{"entries": entries})
	requireStatus(t, w, http.StatusOK)
	return jsonResponse[preflightPayload](t, w)
}

func seedSourcedVersion(t *testing.T, h *Handler, assetID uint, path string, size int64) {
	t.Helper()
	v := models.AssetVersion{
		AssetID:            assetID,
		SourceKind:         ptrTo(imports.SourceKindUnityCache),
		SourceRelativePath: ptrTo(path),
		SourceSizeBytes:    ptrTo(size),
	}
	if err := h.db.Create(&v).Error; err != nil {
		t.Fatalf("seed sourced version: %v", err)
	}
}

func ptrTo[T any](v T) *T { return &v }

func TestPreflight_PathAndSizeStatuses(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "path@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")

	held := seedAsset(t, db, lib.ID, "RPG Builder")
	heldPath := "Blink/Editor ExtensionsRPG Toolkits/RPG Builder.unitypackage"
	seedSourcedVersion(t, h, held.ID, heldPath, 5000)

	byName := seedAsset(t, db, lib.ID, "Fantasy Pack")

	got := preflightEntries(t, h, u, []map[string]any{
		// Same path, same size → already held.
		{"filename": "RPG Builder.unitypackage", "size_bytes": 5000,
			"source_kind": imports.SourceKindUnityCache, "relative_path": heldPath},
		// Same path, different size → a newer download of the same package.
		{"filename": "RPG Builder.unitypackage", "size_bytes": 6000,
			"source_kind": imports.SourceKindUnityCache, "relative_path": heldPath},
		// Unknown path, but the name matches an asset imported by hand.
		{"filename": "Fantasy Pack.unitypackage", "size_bytes": 100,
			"source_kind": imports.SourceKindUnityCache, "relative_path": "Someone/Audio/Fantasy Pack.unitypackage"},
		// Nothing matches at all.
		{"filename": "Brand New.unitypackage", "size_bytes": 100,
			"source_kind": imports.SourceKindUnityCache, "relative_path": "Someone/Audio/Brand New.unitypackage"},
	})

	if len(got.Results) != 4 {
		t.Fatalf("results = %d, want 4", len(got.Results))
	}
	wantStatus := []string{"already_imported", "updated", "name_match", "new"}
	for i, w := range wantStatus {
		if got.Results[i].Status != w {
			t.Errorf("results[%d].Status = %q, want %q", i, got.Results[i].Status, w)
		}
	}
	if got.Results[0].AssetID != held.ID || got.Results[1].AssetID != held.ID {
		t.Errorf("path matches should name asset %d, got %d and %d",
			held.ID, got.Results[0].AssetID, got.Results[1].AssetID)
	}
	if got.Results[1].HeldBytes != 5000 {
		t.Errorf("updated.HeldBytes = %d, want 5000", got.Results[1].HeldBytes)
	}
	if got.Results[2].AssetID != byName.ID {
		t.Errorf("name match should name asset %d, got %d", byName.ID, got.Results[2].AssetID)
	}
	if got.Results[3].AssetID != 0 {
		t.Errorf("new should name no asset, got %d", got.Results[3].AssetID)
	}
}

// A path match must beat a name match: the whole point of the source columns is
// telling a duplicate from a newer version, which the name matcher cannot do.
func TestPreflight_PathBeatsName(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "prec@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")

	held := seedAsset(t, db, lib.ID, "RPG Builder")
	path := "Blink/Tools/RPG Builder.unitypackage"
	seedSourcedVersion(t, h, held.ID, path, 5000)

	got := preflightEntries(t, h, u, []map[string]any{
		{"filename": "RPG Builder.unitypackage", "size_bytes": 5000,
			"source_kind": imports.SourceKindUnityCache, "relative_path": path},
	})
	if got.Results[0].Status != "already_imported" {
		t.Errorf("status = %q, want already_imported", got.Results[0].Status)
	}
}

func TestPreflight_SourcePathsAreLibraryScoped(t *testing.T) {
	h, db := newTestHandler(t)
	mine := seedUser(t, db, "mine@example.com")
	myLib := seedLibraryForUser(t, db, mine, "Mine")

	theirs := seedUser(t, db, "theirs@example.com")
	theirLib := seedLibraryForUser(t, db, theirs, "Theirs")
	theirAsset := seedAsset(t, db, theirLib.ID, "RPG Builder")

	path := "Blink/Tools/RPG Builder.unitypackage"
	seedSourcedVersion(t, h, theirAsset.ID, path, 5000)

	got := preflightEntries(t, h, mine, []map[string]any{
		{"filename": "RPG Builder.unitypackage", "size_bytes": 5000,
			"source_kind": imports.SourceKindUnityCache, "relative_path": path},
	})
	if got.Results[0].Status != "new" {
		t.Errorf("status = %q, want new — another library's cache path must not leak", got.Results[0].Status)
	}
	if got.Results[0].AssetID != 0 {
		t.Errorf("asset_id = %d, want 0", got.Results[0].AssetID)
	}
	_ = myLib
}

// An entry with no source hint must answer exactly as it did before this
// slice, so the shipped `dmsn upload --skip-existing` keeps working.
func TestPreflight_NoSourceHintIsUnchanged(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "plain@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	held := seedAsset(t, db, lib.ID, "RPG Builder")
	seedSourcedVersion(t, h, held.ID, "Blink/Tools/RPG Builder.unitypackage", 5000)

	got := preflightEntries(t, h, u, []map[string]any{
		{"filename": "RPG Builder.unitypackage", "size_bytes": 5000},
		{"filename": "Brand New.unitypackage", "size_bytes": 5000},
	})
	if got.Results[0].Status != "name_match" {
		t.Errorf("results[0].Status = %q, want name_match", got.Results[0].Status)
	}
	if got.Results[1].Status != "new" {
		t.Errorf("results[1].Status = %q, want new", got.Results[1].Status)
	}
}

func TestPreflight_RejectsBadSourcePaths(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "badpath@example.com")
	_ = seedLibraryForUser(t, db, u, "Main")

	cases := []struct {
		name  string
		entry map[string]any
	}{
		{"absolute", map[string]any{"filename": "x.unitypackage", "size_bytes": 1,
			"source_kind": imports.SourceKindUnityCache, "relative_path": "/etc/x.unitypackage"}},
		{"traversal", map[string]any{"filename": "x.unitypackage", "size_bytes": 1,
			"source_kind": imports.SourceKindUnityCache, "relative_path": "a/../../x.unitypackage"}},
		{"unknown kind", map[string]any{"filename": "x.unitypackage", "size_bytes": 1,
			"source_kind": "dropbox", "relative_path": "a/x.unitypackage"}},
		{"path without kind", map[string]any{"filename": "x.unitypackage", "size_bytes": 1,
			"relative_path": "a/x.unitypackage"}},
	}
	for _, c := range cases {
		w := performRequest(setupRouter(h, u), http.MethodPost, "/api/imports/preflight",
			map[string]any{"entries": []map[string]any{c.entry}})
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", c.name, w.Code)
		}
	}
}

func TestPreflight_RepeatedPathIsOneLookup(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "dupe@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	held := seedAsset(t, db, lib.ID, "RPG Builder")
	path := "Blink/Tools/RPG Builder.unitypackage"
	seedSourcedVersion(t, h, held.ID, path, 5000)

	got := preflightEntries(t, h, u, []map[string]any{
		{"filename": "RPG Builder.unitypackage", "size_bytes": 5000,
			"source_kind": imports.SourceKindUnityCache, "relative_path": path},
		{"filename": "RPG Builder.unitypackage", "size_bytes": 9999,
			"source_kind": imports.SourceKindUnityCache, "relative_path": path},
	})
	if got.Results[0].Status != "already_imported" {
		t.Errorf("results[0].Status = %q, want already_imported", got.Results[0].Status)
	}
	if got.Results[1].Status != "updated" {
		t.Errorf("results[1].Status = %q, want updated", got.Results[1].Status)
	}
}

func TestDistinctPaths(t *testing.T) {
	got := distinctPaths([]string{"a", "", "b", "a", "", "b", "c"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("distinctPaths = %#v, want %#v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("distinctPaths[%d] = %q, want %q", i, got[i], w)
		}
	}
	if len(distinctPaths(nil)) != 0 {
		t.Error("distinctPaths(nil) should be empty")
	}
}
