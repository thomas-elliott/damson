package imports

import (
	"context"
	"encoding/json"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

type fakeEnricher struct {
	cands      []metadata.Candidate
	meta       *metadata.Metadata
	searchErr  error
	fetchErr   error
	searchCall int
	fetchCall  int
}

func (f *fakeEnricher) Search(_ context.Context, _ string) ([]metadata.Candidate, error) {
	f.searchCall++
	return f.cands, f.searchErr
}

func (f *fakeEnricher) Fetch(_ context.Context, _ string) (*metadata.Metadata, error) {
	f.fetchCall++
	return f.meta, f.fetchErr
}

func (f *fakeEnricher) FetchImage(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", nil
}

func enrichTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Asset{}, &models.Licence{}, &models.LibrarySettings{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedEnrichAsset(t *testing.T, db *gorm.DB, name string, enrich bool) *models.Asset {
	t.Helper()
	if err := db.Create(&models.LibrarySettings{LibraryID: 1, AutoEnrichEnabled: enrich}).Error; err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	// GORM omits a zero-value bool on Create, letting the column default:true
	// apply — force the intended value so a false toggle really is off.
	if err := db.Model(&models.LibrarySettings{}).Where("library_id = ?", 1).
		Update("auto_enrich_enabled", enrich).Error; err != nil {
		t.Fatalf("force enrich toggle: %v", err)
	}
	a := models.Asset{LibraryID: 1, Name: name}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return &a
}

func TestEnrichAsset_HighConfidenceFillsBlanks(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedEnrichAsset(t, db, "Fantasy Pack", true)
	fake := &fakeEnricher{
		cands: []metadata.Candidate{{Title: "Fantasy Pack", ListingURL: "https://x/1", Store: "unity"}},
		meta: &metadata.Metadata{
			Store:       "unity",
			Description: "A fetched description.",
			LicenceHint: "CC-BY-4.0",
			ListingURL:  "https://x/1",
		},
	}
	jc := jobContext{asset: *asset, libraryID: 1}

	enrichAsset(context.Background(), Deps{DB: db, Metadata: fake}, jc)

	if fake.fetchCall != 1 {
		t.Errorf("expected the matched top hit to be fetched, fetchCall=%d", fake.fetchCall)
	}
	var got models.Asset
	db.First(&got, asset.ID)
	// A no-op / abstaining enricher would leave these blank — this fails against it.
	if got.Description != "A fetched description." {
		t.Errorf("description not filled: %q", got.Description)
	}
	if got.LicenceID == nil {
		t.Fatal("licence not created from hint")
	}
	if len(got.SourceMetadataJSON) == 0 {
		t.Error("source metadata not persisted")
	}
	var rec metadata.SourceRecord
	if err := json.Unmarshal(got.SourceMetadataJSON, &rec); err != nil {
		t.Fatal(err)
	}
	if !rec.Matched || rec.Store != "unity" {
		t.Errorf("expected matched unity record, got %+v", rec)
	}
	if !got.NeedsReview {
		t.Error("needs_review should be set")
	}
}

func TestEnrichAsset_LowConfidenceAbstains(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedEnrichAsset(t, db, "Fantasy Pack", true)
	fake := &fakeEnricher{
		cands: []metadata.Candidate{{Title: "Completely Different Thing", ListingURL: "https://x/2", Store: "unity"}},
		meta:  &metadata.Metadata{Description: "should not be applied"},
	}
	jc := jobContext{asset: *asset, libraryID: 1}

	enrichAsset(context.Background(), Deps{DB: db, Metadata: fake}, jc)

	if fake.fetchCall != 0 {
		t.Errorf("abstain must not fetch the listing, fetchCall=%d", fake.fetchCall)
	}
	var got models.Asset
	db.First(&got, asset.ID)
	if got.Description != "" {
		t.Errorf("abstain must not fill description, got %q", got.Description)
	}
	var rec metadata.SourceRecord
	if err := json.Unmarshal(got.SourceMetadataJSON, &rec); err != nil {
		t.Fatal(err)
	}
	// Candidates must be stored for review even when the gate abstains.
	if rec.Matched || len(rec.Candidates) != 1 {
		t.Errorf("expected unmatched record with stored candidates, got %+v", rec)
	}
}

func TestEnrichAsset_ToggleOffSkips(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedEnrichAsset(t, db, "Fantasy Pack", false)
	fake := &fakeEnricher{cands: []metadata.Candidate{{Title: "Fantasy Pack", ListingURL: "u"}}}
	jc := jobContext{asset: *asset, libraryID: 1}

	enrichAsset(context.Background(), Deps{DB: db, Metadata: fake}, jc)

	if fake.searchCall != 0 {
		t.Errorf("toggle off must skip search entirely, searchCall=%d", fake.searchCall)
	}
}

func TestTitlesMatch(t *testing.T) {
	if !titlesMatch("Fantasy Pack (Deluxe)", "fantasy-pack-deluxe") {
		t.Error("normalised equal titles must match")
	}
	if titlesMatch("Fantasy Pack", "Sci-Fi Pack") {
		t.Error("different titles must not match")
	}
	if titlesMatch("", "") {
		t.Error("empty titles must not match")
	}
}
