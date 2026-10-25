package enrich

import (
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

func strPtr(s string) *string { return &s }

func fullMeta() *metadata.Metadata {
	released := time.Date(2020, 2, 11, 9, 47, 13, 0, time.UTC)
	updated := time.Date(2023, 5, 6, 19, 35, 15, 0, time.UTC)
	return &metadata.Metadata{
		Publisher:          "JustCreate",
		PublisherURL:       "https://justcreate3d.studio/",
		Store:              metadata.StoreUnity,
		StoreID:            "162153",
		LatestStoreVersion: "1.24",
		Engine:             "unity",
		MinEngineVersion:   "2020.3",
		Price:              "0",
		Currency:           "USD",
		ReleasedAt:         &released,
		StoreUpdatedAt:     &updated,
		Rating:             4.85,
		RatingCount:        126,
	}
}

func TestFieldUpdatesFillsEveryBlankColumn(t *testing.T) {
	asset := &models.Asset{ID: 1, Name: "asset"}
	before, updates, conflicts := fieldUpdates(asset, fullMeta(), nil)

	if len(conflicts) != 0 {
		t.Fatalf("conflicts on a blank asset: %+v", conflicts)
	}
	if len(before) != 0 {
		t.Fatalf("before should be empty when nothing was overwritten: %+v", before)
	}
	for _, column := range []string{
		"publisher", "publisher_url", "store", "store_product_id",
		"latest_store_version", "engine", "min_engine_version",
		"price", "price_currency", "released_at", "store_updated_at",
		"rating", "rating_count",
	} {
		if _, ok := updates[column]; !ok {
			t.Fatalf("column %q not filled (updates: %v)", column, updates)
		}
	}
	// The asset itself must be mutated so the handler's response is current.
	if asset.Publisher == nil || *asset.Publisher != "JustCreate" {
		t.Fatalf("asset.Publisher = %v", asset.Publisher)
	}
	if asset.Rating == nil || *asset.Rating != 4.85 {
		t.Fatalf("asset.Rating = %v", asset.Rating)
	}
	if asset.ReleasedAt == nil || !asset.ReleasedAt.Equal(*fullMeta().ReleasedAt) {
		t.Fatalf("asset.ReleasedAt = %v", asset.ReleasedAt)
	}
}

func TestFieldUpdatesReportsConflictsInsteadOfOverwriting(t *testing.T) {
	existingReleased := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	rating := 3.0
	count := 10
	asset := &models.Asset{
		ID:          1,
		Publisher:   strPtr("Hand Edited Studio"),
		ReleasedAt:  &existingReleased,
		Rating:      &rating,
		RatingCount: &count,
	}
	_, updates, conflicts := fieldUpdates(asset, fullMeta(), nil)

	byField := map[string]Conflict{}
	for _, c := range conflicts {
		byField[c.Field] = c
	}
	for _, column := range []string{"publisher", "released_at", "rating", "rating_count"} {
		if _, ok := byField[column]; !ok {
			t.Fatalf("no conflict reported for %q (conflicts: %+v)", column, conflicts)
		}
		if _, wrote := updates[column]; wrote {
			t.Fatalf("column %q was overwritten without confirmation", column)
		}
	}
	if got := byField["publisher"]; got.Current != "Hand Edited Studio" || got.Proposed != "JustCreate" {
		t.Fatalf("publisher conflict = %+v", got)
	}
	if got := byField["rating"]; got.Current != "3" || got.Proposed != "4.85" {
		t.Fatalf("rating conflict = %+v", got)
	}
	// The user's value must survive in the struct too.
	if *asset.Publisher != "Hand Edited Studio" {
		t.Fatalf("asset.Publisher was mutated to %q", *asset.Publisher)
	}
	// Blank columns are still filled in the same pass.
	if _, ok := updates["engine"]; !ok {
		t.Fatalf("blank columns should still fill alongside conflicts: %v", updates)
	}
}

func TestFieldUpdatesOverwritesOnlyConfirmedFields(t *testing.T) {
	rating := 3.0
	asset := &models.Asset{
		ID:        1,
		Publisher: strPtr("Hand Edited Studio"),
		Rating:    &rating,
	}
	before, updates, conflicts := fieldUpdates(asset, fullMeta(), map[string]bool{"publisher": true})

	if updates["publisher"] != "JustCreate" {
		t.Fatalf("confirmed publisher not written: %v", updates)
	}
	if before["publisher"] != "Hand Edited Studio" {
		t.Fatalf("before[publisher] = %v, want the old value for the audit diff", before["publisher"])
	}
	if _, wrote := updates["rating"]; wrote {
		t.Fatalf("unconfirmed rating was written: %v", updates)
	}
	if len(conflicts) != 1 || conflicts[0].Field != "rating" {
		t.Fatalf("conflicts = %+v, want only the unconfirmed rating", conflicts)
	}
}

// An unchanged value is neither an update nor a conflict — a re-fetch of an
// already-enriched asset must be quiet.
func TestFieldUpdatesIgnoresUnchangedValues(t *testing.T) {
	meta := fullMeta()
	asset := &models.Asset{ID: 1}
	if _, updates, _ := fieldUpdates(asset, meta, nil); len(updates) == 0 {
		t.Fatal("first pass wrote nothing")
	}
	before, updates, conflicts := fieldUpdates(asset, meta, nil)
	if len(updates) != 0 || len(conflicts) != 0 || len(before) != 0 {
		t.Fatalf("second pass was not a no-op: updates=%v conflicts=%+v before=%v", updates, conflicts, before)
	}
}

// A store that exposes nothing must not blank the columns a previous fetch set.
func TestFieldUpdatesEmptyMetadataWritesNothing(t *testing.T) {
	asset := &models.Asset{ID: 1, Publisher: strPtr("Kept")}
	before, updates, conflicts := fieldUpdates(asset, &metadata.Metadata{}, nil)
	if len(updates) != 0 || len(conflicts) != 0 || len(before) != 0 {
		t.Fatalf("empty metadata produced writes: updates=%v conflicts=%+v", updates, conflicts)
	}
	if asset.Publisher == nil || *asset.Publisher != "Kept" {
		t.Fatalf("asset.Publisher = %v, want it kept", asset.Publisher)
	}
}

func TestTagListAppendsCategoryPath(t *testing.T) {
	got := tagList(&metadata.Metadata{
		Tags:         []string{"Landscape", "Terrain"},
		CategoryPath: "3d/environments/landscapes",
	})
	want := []string{"Landscape", "Terrain", "3d/environments/landscapes"}
	if len(got) != len(want) {
		t.Fatalf("tagList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tagList = %v, want %v", got, want)
		}
	}
	// No category means no extra entry, not a blank tag.
	if got := tagList(&metadata.Metadata{Tags: []string{"x"}}); len(got) != 1 {
		t.Fatalf("tagList without a category = %v", got)
	}
	if got := tagList(&metadata.Metadata{CategoryPath: "   "}); len(got) != 0 {
		t.Fatalf("a whitespace-only category path became a tag: %v", got)
	}
}
