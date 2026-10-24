package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestPatchAsset_SetsMetadataFields(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/assets/1", map[string]any{
		"publisher":            "JustCreate",
		"publisher_url":        "https://justcreate3d.studio/",
		"store":                "unity",
		"store_product_id":     "162153",
		"released_at":          "2020-02-11T09:47:13Z",
		"store_updated_at":     "2023-05-06",
		"latest_store_version": "1.24",
		"engine":               "unity",
		"min_engine_version":   "2020.3",
		"price":                "0",
		"price_currency":       "USD",
		"rating":               "4.85",
		"rating_count":         "126",
	})
	requireStatus(t, w, http.StatusOK)

	var got models.Asset
	if err := db.First(&got, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Publisher == nil || *got.Publisher != "JustCreate" {
		t.Fatalf("publisher = %v", got.Publisher)
	}
	if got.PublisherURL == nil || *got.PublisherURL != "https://justcreate3d.studio/" {
		t.Fatalf("publisher_url = %v", got.PublisherURL)
	}
	if got.Store == nil || *got.Store != "unity" {
		t.Fatalf("store = %v", got.Store)
	}
	if got.StoreProductID == nil || *got.StoreProductID != "162153" {
		t.Fatalf("store_product_id = %v", got.StoreProductID)
	}
	if got.ReleasedAt == nil || !got.ReleasedAt.Equal(time.Date(2020, 2, 11, 9, 47, 13, 0, time.UTC)) {
		t.Fatalf("released_at = %v", got.ReleasedAt)
	}
	// A bare YYYY-MM-DD is accepted as well as a full timestamp.
	if got.StoreUpdatedAt == nil || !got.StoreUpdatedAt.Equal(time.Date(2023, 5, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("store_updated_at = %v", got.StoreUpdatedAt)
	}
	if got.LatestStoreVersion == nil || *got.LatestStoreVersion != "1.24" {
		t.Fatalf("latest_store_version = %v", got.LatestStoreVersion)
	}
	if got.Engine == nil || *got.Engine != "unity" {
		t.Fatalf("engine = %v", got.Engine)
	}
	if got.MinEngineVersion == nil || *got.MinEngineVersion != "2020.3" {
		t.Fatalf("min_engine_version = %v", got.MinEngineVersion)
	}
	if got.Price == nil || *got.Price != "0" {
		t.Fatalf("price = %v", got.Price)
	}
	if got.PriceCurrency == nil || *got.PriceCurrency != "USD" {
		t.Fatalf("price_currency = %v", got.PriceCurrency)
	}
	if got.Rating == nil || *got.Rating != 4.85 {
		t.Fatalf("rating = %v", got.Rating)
	}
	if got.RatingCount == nil || *got.RatingCount != 126 {
		t.Fatalf("rating_count = %v", got.RatingCount)
	}

	var auditCount int64
	if err := db.Model(&models.AuditLog{}).
		Where("action_type = ? AND target_id = ?", "asset.update", asset.ID).
		Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit row, got %d", auditCount)
	}
}

// An empty string clears a column to NULL, so a wrong hand-entered value can be
// removed rather than only overwritten.
func TestPatchAsset_BlankMetadataFieldClearsColumn(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")

	pub := "Wrong Studio"
	rating := 3.0
	released := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Model(asset).Updates(map[string]any{
		"publisher": &pub, "rating": &rating, "released_at": &released,
	}).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/assets/1", map[string]any{
		"publisher": "", "rating": "", "released_at": "",
	})
	requireStatus(t, w, http.StatusOK)

	var got models.Asset
	if err := db.First(&got, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Publisher != nil || got.Rating != nil || got.ReleasedAt != nil {
		t.Fatalf("expected cleared columns, got publisher=%v rating=%v released=%v",
			got.Publisher, got.Rating, got.ReleasedAt)
	}
}

// A field the request omits must be left alone, not blanked.
func TestPatchAsset_OmittedMetadataFieldIsUntouched(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")

	pub := "Kept Studio"
	if err := db.Model(asset).Update("publisher", &pub).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPatch, "/api/assets/1", map[string]any{"engine": "unity"})
	requireStatus(t, w, http.StatusOK)

	var got models.Asset
	if err := db.First(&got, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Publisher == nil || *got.Publisher != "Kept Studio" {
		t.Fatalf("publisher = %v, want it untouched by a patch that omitted it", got.Publisher)
	}
}

func TestPatchAsset_RejectsInvalidMetadataValues(t *testing.T) {
	cases := map[string]map[string]any{
		"bad date":         {"released_at": "11/02/2020"},
		"bad rating":       {"rating": "excellent"},
		"rating too high":  {"rating": "9"},
		"negative rating":  {"rating": "-1"},
		"bad rating count": {"rating_count": "1.5"},
		"negative count":   {"rating_count": "-3"},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h, db := newTestHandler(t)
			u := seedUser(t, db, "t@example.com")
			lib := seedLibraryForUser(t, db, u, "Personal")
			asset := seedAsset(t, db, lib.ID, "asset")

			r := setupRouter(h, u)
			w := performRequest(r, http.MethodPatch, "/api/assets/1", body)
			requireStatus(t, w, http.StatusBadRequest)

			// The rejection must roll back: nothing partially written.
			var got models.Asset
			if err := db.First(&got, asset.ID).Error; err != nil {
				t.Fatal(err)
			}
			if got.ReleasedAt != nil || got.Rating != nil || got.RatingCount != nil {
				t.Fatalf("invalid patch wrote something: %+v", got)
			}
		})
	}
}

func TestGetAsset_ReturnsMetadataFields(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")

	pub := "JustCreate"
	rating := 4.85
	count := 126
	released := time.Date(2020, 2, 11, 9, 47, 13, 0, time.UTC)
	if err := db.Model(asset).Updates(map[string]any{
		"publisher": &pub, "rating": &rating, "rating_count": &count, "released_at": &released,
	}).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodGet, "/api/assets/1", nil)
	requireStatus(t, w, http.StatusOK)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["publisher"] != "JustCreate" {
		t.Fatalf("publisher = %v", body["publisher"])
	}
	if body["rating"] != 4.85 {
		t.Fatalf("rating = %v", body["rating"])
	}
	if body["rating_count"] != float64(126) {
		t.Fatalf("rating_count = %v", body["rating_count"])
	}
	if body["released_at"] == nil {
		t.Fatal("released_at missing")
	}
	// Unset columns must be omitted rather than sent as null.
	for _, key := range []string{"store", "engine", "price", "store_updated_at"} {
		if _, present := body[key]; present {
			t.Fatalf("unset field %q present in the response", key)
		}
	}
}

// Publisher joins the search vector at weight C. This only works if the column
// is in the trigger's UPDATE OF list as well as the function body — the trap
// migration 007 exists to fix.
func TestSearchMatchesPublisher(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "a nature pack")
	other := seedAsset(t, db, lib.ID, "an unrelated asset")

	pub := "JustCreate"
	if err := db.Model(asset).Update("publisher", &pub).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodGet, "/api/assets?q=JustCreate", nil)
	requireStatus(t, w, http.StatusOK)

	var body struct {
		Assets []struct {
			ID uint `json:"id"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Assets) != 1 || body.Assets[0].ID != asset.ID {
		t.Fatalf("search for a publisher name returned %+v, want only asset %d", body.Assets, asset.ID)
	}
	if body.Assets[0].ID == other.ID {
		t.Fatal("search matched the unrelated asset")
	}
}

// seedAssetWithFields creates an asset carrying storefront metadata.
func seedAssetWithFields(t *testing.T, db *gorm.DB, libID uint, name string, fields map[string]any) *models.Asset {
	t.Helper()
	a := seedAsset(t, db, libID, name)
	if len(fields) > 0 {
		if err := db.Model(a).Updates(fields).Error; err != nil {
			t.Fatalf("seed fields: %v", err)
		}
	}
	return a
}

func listAssetIDs(t *testing.T, r http.Handler, query string) []uint {
	t.Helper()
	w := performGET(r, query)
	requireStatus(t, w, http.StatusOK)
	var body struct {
		Assets []struct {
			ID uint `json:"id"`
		} `json:"assets"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	out := make([]uint, 0, len(body.Assets))
	for _, a := range body.Assets {
		out = append(out, a.ID)
	}
	if int64(len(out)) > body.Total {
		t.Fatalf("total %d is less than the %d returned rows", body.Total, len(out))
	}
	return out
}

func TestAssetFilters_PublisherStoreAndReleasedAfter(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	old := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	unity := seedAssetWithFields(t, db, lib.ID, "unity thing", map[string]any{
		"publisher": "JustCreate", "store": "unity", "released_at": &recent,
	})
	sketch := seedAssetWithFields(t, db, lib.ID, "sketch thing", map[string]any{
		"publisher": "TreeMaker", "store": "sketchfab", "released_at": &old,
	})
	bare := seedAssetWithFields(t, db, lib.ID, "bare thing", nil)

	r := setupRouter(h, u)

	if got := listAssetIDs(t, r, "/api/assets?publisher=JustCreate"); len(got) != 1 || got[0] != unity.ID {
		t.Fatalf("publisher filter = %v, want [%d]", got, unity.ID)
	}
	if got := listAssetIDs(t, r, "/api/assets?store=sketchfab"); len(got) != 1 || got[0] != sketch.ID {
		t.Fatalf("store filter = %v, want [%d]", got, sketch.ID)
	}
	// released_after excludes both the older asset and the one with no date.
	got := listAssetIDs(t, r, "/api/assets?released_after=2020-01-01")
	if len(got) != 1 || got[0] != unity.ID {
		t.Fatalf("released_after = %v, want [%d]", got, unity.ID)
	}
	// Filters compose.
	if got := listAssetIDs(t, r, "/api/assets?store=unity&publisher=TreeMaker"); len(got) != 0 {
		t.Fatalf("composed contradictory filters returned %v, want none", got)
	}
	// An unfiltered list still returns everything, including the bare asset.
	if got := listAssetIDs(t, r, "/api/assets"); len(got) != 3 {
		t.Fatalf("unfiltered list = %v, want all 3 including asset %d", got, bare.ID)
	}
	// An unparseable date is ignored rather than filtering everything out.
	if got := listAssetIDs(t, r, "/api/assets?released_after=not-a-date"); len(got) != 3 {
		t.Fatalf("bad released_after = %v, want the filter ignored", got)
	}
}

func TestAssetFilters_StoreDeprecated(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	deprecated := seedAssetWithFields(t, db, lib.ID, "deprecated", map[string]any{"store_deprecated": true})
	live := seedAssetWithFields(t, db, lib.ID, "live", nil)

	r := setupRouter(h, u)
	if got := listAssetIDs(t, r, "/api/assets?store_deprecated=true"); len(got) != 1 || got[0] != deprecated.ID {
		t.Fatalf("store_deprecated=true = %v, want [%d]", got, deprecated.ID)
	}
	if got := listAssetIDs(t, r, "/api/assets?store_deprecated=false"); len(got) != 1 || got[0] != live.ID {
		t.Fatalf("store_deprecated=false = %v, want [%d]", got, live.ID)
	}
}

func TestAssetSort_ByReleaseDate(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	oldDate := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	newDate := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	older := seedAssetWithFields(t, db, lib.ID, "older", map[string]any{"released_at": &oldDate})
	newer := seedAssetWithFields(t, db, lib.ID, "newer", map[string]any{"released_at": &newDate})
	undated := seedAssetWithFields(t, db, lib.ID, "undated", nil)

	r := setupRouter(h, u)

	// Assets with no release date sort last in both directions, so they never
	// crowd out the dated ones the sort is for.
	desc := listAssetIDs(t, r, "/api/assets?sort=released_desc")
	if len(desc) != 3 || desc[0] != newer.ID || desc[1] != older.ID || desc[2] != undated.ID {
		t.Fatalf("released_desc = %v, want [%d %d %d]", desc, newer.ID, older.ID, undated.ID)
	}
	asc := listAssetIDs(t, r, "/api/assets?sort=released_asc")
	if len(asc) != 3 || asc[0] != older.ID || asc[1] != newer.ID || asc[2] != undated.ID {
		t.Fatalf("released_asc = %v, want [%d %d %d]", asc, older.ID, newer.ID, undated.ID)
	}
	// An unknown sort falls back to the default rather than erroring or
	// interpolating the value into the ORDER BY.
	fallback := listAssetIDs(t, r, "/api/assets?sort=a.name;DROP+TABLE+assets")
	if len(fallback) != 3 {
		t.Fatalf("unknown sort = %v, want the default ordering over all 3", fallback)
	}
	if def := listAssetIDs(t, r, "/api/assets"); len(def) != len(fallback) || def[0] != fallback[0] {
		t.Fatalf("unknown sort %v did not match the default %v", fallback, def)
	}
}

func TestAssetSort_PagesTiedRowsOnce(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	// Every asset ties on every sort column: same name, same created_at and no
	// release date. Only the id tiebreaker gives the pages a stable order.
	const n = 23
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	want := map[uint]bool{}
	for i := 0; i < n; i++ {
		a := seedAssetWithFields(t, db, lib.ID, "pack", map[string]any{"created_at": created})
		want[a.ID] = true
	}

	r := setupRouter(h, u)
	for sort := range assetSortColumns {
		seen := map[uint]int{}
		for offset := 0; offset < n; offset += 5 {
			query := "/api/assets?limit=5&sort=" + sort + "&offset=" + strconv.Itoa(offset)
			for _, id := range listAssetIDs(t, r, query) {
				seen[id]++
			}
		}
		for id := range want {
			if seen[id] != 1 {
				t.Errorf("sort=%s: asset %d appeared on %d pages, want 1", sort, id, seen[id])
			}
		}
	}
}

func TestGetAssetFacets(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	other := seedLibrary(t, db, "Other")

	seedAssetWithFields(t, db, lib.ID, "a", map[string]any{"publisher": "JustCreate", "store": "unity", "engine": "unity"})
	seedAssetWithFields(t, db, lib.ID, "b", map[string]any{"publisher": "JustCreate", "store": "unity"})
	seedAssetWithFields(t, db, lib.ID, "c", map[string]any{"publisher": "TreeMaker", "store": "sketchfab"})
	seedAssetWithFields(t, db, lib.ID, "d", nil)
	// Another library's values must not leak into this library's facets.
	seedAssetWithFields(t, db, other.ID, "e", map[string]any{"publisher": "Leaked Studio", "store": "itch"})

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets/facets")
	requireStatus(t, w, http.StatusOK)
	got := jsonResponse[assetFacetsResponse](t, w)

	if len(got.Publishers) != 2 {
		t.Fatalf("publishers = %+v, want 2", got.Publishers)
	}
	// Ordered by count descending.
	if got.Publishers[0].Value != "JustCreate" || got.Publishers[0].Count != 2 {
		t.Fatalf("publishers[0] = %+v, want JustCreate x2 first", got.Publishers[0])
	}
	if got.Publishers[1].Value != "TreeMaker" || got.Publishers[1].Count != 1 {
		t.Fatalf("publishers[1] = %+v", got.Publishers[1])
	}
	for _, p := range got.Publishers {
		if p.Value == "Leaked Studio" {
			t.Fatal("another library's publisher leaked into the facets")
		}
	}
	if len(got.Stores) != 2 {
		t.Fatalf("stores = %+v, want unity and sketchfab", got.Stores)
	}
	if len(got.Engines) != 1 || got.Engines[0].Value != "unity" {
		t.Fatalf("engines = %+v", got.Engines)
	}
}

// The facets route must not be swallowed by /assets/:id.
func TestGetAssetFacets_RouteDoesNotCollideWithAssetID(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)
	w := performGET(r, "/api/assets/facets")
	requireStatus(t, w, http.StatusOK)
	if body := w.Body.String(); !strings.Contains(body, "publishers") {
		t.Fatalf("facets route returned %q — likely matched /assets/:id instead", body)
	}
}

// seedAssetWithVersions creates an asset carrying a latest-known store version
// plus the store versions it already holds.
func seedAssetWithVersions(t *testing.T, db *gorm.DB, libID uint, name, latest string, held ...string) *models.Asset {
	t.Helper()
	a := seedAsset(t, db, libID, name)
	if latest != "" {
		if err := db.Model(a).Update("latest_store_version", &latest).Error; err != nil {
			t.Fatalf("set latest_store_version: %v", err)
		}
	}
	for _, sv := range held {
		v := models.AssetVersion{AssetID: a.ID, StoreVersion: sv}
		if err := db.Create(&v).Error; err != nil {
			t.Fatalf("seed version %q: %v", sv, err)
		}
		if a.CurrentVersionID == nil {
			id := v.ID
			if err := db.Model(a).Update("current_version_id", &id).Error; err != nil {
				t.Fatalf("set current version: %v", err)
			}
			a.CurrentVersionID = &id
		}
	}
	return a
}

// The SQL filter and the Go comparison that drives the badge must agree, or the
// grid shows a badge on an asset the filter hides (and vice versa). They are
// separate implementations, so this pins them together over the same rows.
func TestUpdatesAvailableFilterAgreesWithTheBadge(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	type spec struct {
		name   string
		latest string
		held   []string
		want   bool
	}
	specs := []spec{
		{"outdated", "1.24", []string{"1.23"}, true},
		{"current", "1.24", []string{"1.24"}, false},
		{"held newer than store", "1.23", []string{"1.24"}, false},
		{"double digit minor", "1.10", []string{"1.9"}, true},
		{"double digit minor reversed", "1.9", []string{"1.10"}, false},
		{"highest held wins", "1.24", []string{"1.9", "1.23", "1.10"}, true},
		{"holds the newest already", "1.24", []string{"1.9", "1.24"}, false},
		{"trailing zero is equal", "1.0", []string{"1"}, false},
		{"no versions held", "1.24", nil, false},
		{"nothing fetched", "", []string{"1.23"}, false},
		// A hand-edited latest version need not be dotted-numeric. It must not
		// abort the query with a cast error, and must not match.
		{"latest unorderable", "1.24-beta", []string{"1.23"}, false},
		{"held unorderable", "1.24", []string{"nightly"}, false},
		{"mixed orderability", "1.24", []string{"nightly", "1.23"}, true},
		{"outdated but deprecated", "1.24", []string{"1.23"}, false},
	}
	// The store won't publish a newer version of a deprecated listing.
	deprecated := map[string]bool{"outdated but deprecated": true}

	byID := map[uint]spec{}
	wantIDs := map[uint]bool{}
	for _, sp := range specs {
		a := seedAssetWithVersions(t, db, lib.ID, sp.name, sp.latest, sp.held...)
		if deprecated[sp.name] {
			if err := db.Model(a).Update("store_deprecated", true).Error; err != nil {
				t.Fatalf("set store_deprecated: %v", err)
			}
		}
		byID[a.ID] = sp
		if sp.want {
			wantIDs[a.ID] = true
		}
	}

	r := setupRouter(h, u)

	// The badge, as the grid derives it.
	w := performGET(r, "/api/assets?limit=200")
	requireStatus(t, w, http.StatusOK)
	var list struct {
		Assets []struct {
			ID              uint `json:"id"`
			UpdateAvailable bool `json:"update_available"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Assets) != len(specs) {
		t.Fatalf("listed %d assets, want %d", len(list.Assets), len(specs))
	}
	for _, a := range list.Assets {
		if a.UpdateAvailable != byID[a.ID].want {
			t.Errorf("badge for %q = %v, want %v", byID[a.ID].name, a.UpdateAvailable, byID[a.ID].want)
		}
	}

	// The filter, which runs the comparison in SQL.
	filtered := listAssetIDs(t, r, "/api/assets?updates_available=true&limit=200")
	if len(filtered) != len(wantIDs) {
		names := make([]string, 0, len(filtered))
		for _, id := range filtered {
			names = append(names, byID[id].name)
		}
		t.Fatalf("filter matched %v, want %d assets", names, len(wantIDs))
	}
	for _, id := range filtered {
		if !wantIDs[id] {
			t.Errorf("filter matched %q, which the badge says is up to date", byID[id].name)
		}
	}

	// The overview counts with the same SQL as the filter.
	ow := performGET(r, "/api/overview")
	requireStatus(t, ow, http.StatusOK)
	var overview struct {
		Counts struct {
			UpdatesAvailable int `json:"updates_available"`
		} `json:"counts"`
	}
	if err := json.Unmarshal(ow.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Counts.UpdatesAvailable != len(wantIDs) {
		t.Errorf("overview updates_available = %d, want %d", overview.Counts.UpdatesAvailable, len(wantIDs))
	}

	// And the detail view, which derives it a third way from the version rows.
	for id, sp := range byID {
		dw := performGET(r, "/api/assets/"+strconv.FormatUint(uint64(id), 10))
		requireStatus(t, dw, http.StatusOK)
		var detail struct {
			UpdateAvailable bool `json:"update_available"`
		}
		if err := json.Unmarshal(dw.Body.Bytes(), &detail); err != nil {
			t.Fatal(err)
		}
		if detail.UpdateAvailable != sp.want {
			t.Errorf("detail view for %q = %v, want %v", sp.name, detail.UpdateAvailable, sp.want)
		}
	}
}
