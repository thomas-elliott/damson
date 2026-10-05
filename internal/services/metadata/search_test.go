package metadata

import (
	"context"
	"errors"
	"testing"
)

func TestParseSketchfabSearch(t *testing.T) {
	cands := parseSketchfabSearch(readFixture(t, "sketchfab_search.json"))
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate (uid-less one skipped), got %d", len(cands))
	}
	c := cands[0]
	if c.Title != "Stylised Rock" || c.Store != StoreSketchfab {
		t.Errorf("candidate: %+v", c)
	}
	if c.Publisher != "RockStar" {
		t.Errorf("publisher: %q", c.Publisher)
	}
	if c.CoverURL != "https://media.sketchfab.com/rock-l.jpg" {
		t.Errorf("cover (largest): %q", c.CoverURL)
	}
}

func TestParseItchSearch(t *testing.T) {
	cands := parseItchSearch(readFixture(t, "itch_search.html"))
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(cands), cands)
	}
	if cands[0].Title != "Dungeon Tiles" || cands[0].ListingURL != "https://pixelsmith.itch.io/dungeon-tiles" {
		t.Errorf("first candidate: %+v", cands[0])
	}
	if cands[0].Publisher != "PixelSmith" {
		t.Errorf("publisher: %q", cands[0].Publisher)
	}
	if cands[0].CoverURL != "https://img.itch.zone/dungeon.png" {
		t.Errorf("cover: %q", cands[0].CoverURL)
	}
}

// stubSearcher lets the fan-out be tested without network.
type stubSearcher struct {
	store string
	cands []Candidate
	err   error
}

func (s stubSearcher) Store() string { return s.store }
func (s stubSearcher) Search(_ context.Context, _ *Client, _ string) ([]Candidate, error) {
	return s.cands, s.err
}

func TestRegistrySearch_MergesAndTolerates(t *testing.T) {
	r := NewRegistry(nil)
	r.RegisterSearcher(stubSearcher{store: "a", cands: []Candidate{{Title: "A1", ListingURL: "u1", Store: "a"}}})
	r.RegisterSearcher(stubSearcher{store: "b", err: errors.New("boom")}) // failing store must not sink the search
	r.RegisterSearcher(stubSearcher{store: "c", cands: []Candidate{{Title: "C1", ListingURL: "u2", Store: "c"}}})

	got, err := r.Search(context.Background(), "query")
	if err != nil {
		t.Fatal(err)
	}
	// Two healthy stores contribute; the erroring one is dropped, not fatal.
	if len(got) != 2 {
		t.Fatalf("expected 2 merged candidates, got %d: %+v", len(got), got)
	}
}

func TestRegistrySearchStores_NamesTheStoresThatFailed(t *testing.T) {
	r := NewRegistry(nil)
	r.RegisterSearcher(stubSearcher{store: "unity", err: errors.New("429 too many requests")})
	r.RegisterSearcher(stubSearcher{store: "itch", cands: []Candidate{{Title: "I1", ListingURL: "u1", Store: "itch"}}})
	r.RegisterSearcher(stubSearcher{store: "sketchfab"})

	got, failed := r.SearchStores(context.Background(), "query")
	if len(got) != 1 || len(failed) != 1 || failed[0] != "unity" {
		t.Fatalf("got %+v, failed %v; want itch's one candidate and unity named as failed", got, failed)
	}
}

func TestRegistrySearch_EmptyQuery(t *testing.T) {
	r := NewRegistry(nil)
	r.RegisterSearcher(stubSearcher{store: "a", cands: []Candidate{{Title: "x"}}})
	got, _ := r.Search(context.Background(), "   ")
	if len(got) != 0 {
		t.Errorf("empty query must short-circuit, got %d", len(got))
	}
}
