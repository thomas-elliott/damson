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
