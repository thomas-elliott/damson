package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// The library default-theme setting was never read by the SPA, so it was
// dropped: the database has no column for it, the settings API never
// returns it, and a client that still sends it is ignored, not refused.
func TestLibrarySettings_HaveNoDefaultTheme(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	var columns int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'library_settings' AND column_name = 'theme_default'`).
		Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("library_settings still has a theme_default column")
	}

	w := performRequest(r, http.MethodPatch, "/api/settings", map[string]any{"theme_default": "light", "grid_density_default": "compact"})
	requireStatus(t, w, http.StatusOK)

	w = performGET(r, "/api/settings")
	requireStatus(t, w, http.StatusOK)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, present := got["theme_default"]; present {
		t.Fatalf("settings still report theme_default: %s", w.Body.String())
	}
	if got["grid_density_default"] != "compact" {
		t.Fatalf("the rest of the patch was not applied: %s", w.Body.String())
	}
}

// Audit retention is a number of days, or null to keep every entry. An
// explicit null clears it; leaving the field out keeps it.
func TestPatchLibrarySettings_AuditRetention(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "retention@example.com")
	seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	retention := func() any {
		t.Helper()
		w := performGET(r, "/api/settings")
		requireStatus(t, w, http.StatusOK)
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got["audit_retention_days"]
	}

	requireStatus(t, performRequest(r, http.MethodPatch, "/api/settings", map[string]any{"audit_retention_days": 30}), http.StatusOK)
	if got := retention(); got != float64(30) {
		t.Fatalf("retention = %v, want 30", got)
	}

	requireStatus(t, performRequest(r, http.MethodPatch, "/api/settings", map[string]any{"retain_original_packages": false}), http.StatusOK)
	if got := retention(); got != float64(30) {
		t.Fatalf("a patch without the field changed retention to %v", got)
	}

	requireStatus(t, performRequest(r, http.MethodPatch, "/api/settings", map[string]any{"audit_retention_days": nil}), http.StatusOK)
	if got := retention(); got != nil {
		t.Fatalf("retention = %v, want null after clearing it", got)
	}

	for _, bad := range []any{0, -5, 1.5, "30"} {
		w := performRequest(r, http.MethodPatch, "/api/settings", map[string]any{"audit_retention_days": bad})
		requireStatus(t, w, http.StatusBadRequest)
	}
	if got := retention(); got != nil {
		t.Fatalf("a refused patch changed retention to %v", got)
	}
}
