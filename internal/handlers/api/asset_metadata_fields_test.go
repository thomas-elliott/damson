package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

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
