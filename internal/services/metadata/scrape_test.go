package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseHTML_OGAndJSONLD(t *testing.T) {
	m := parseHTML(readFixture(t, "kenney.html"), "https://kenney.nl/assets/platformer-pack-redux")

	if m.Title != "Platformer Pack Redux" {
		t.Errorf("title: got %q", m.Title)
	}
	if m.Publisher != "Kenney" {
		t.Errorf("publisher: got %q", m.Publisher)
	}
	if m.Currency != "USD" || m.Price != "0" {
		t.Errorf("price/currency: got %q %q", m.Price, m.Currency)
	}
	if m.LicenceHint != "https://creativecommons.org/publicdomain/zero/1.0/" {
		t.Errorf("licence hint: got %q", m.LicenceHint)
	}
	// og:image is relative — must be resolved against the page URL.
	want := "https://kenney.nl/media/pages/assets/platformer-pack-redux/cover.png"
	if m.CoverURL != want {
		t.Errorf("cover url: got %q want %q", m.CoverURL, want)
	}
	// A no-op parser (returning zero Metadata) would fail every assertion above.
	if m.Description == "" {
		t.Error("description should not be empty")
	}
}

func TestParseHTML_GraphArrayAndFloatPrice(t *testing.T) {
	m := parseHTML(readFixture(t, "og_array_offers.html"), "https://example.com/x")

	if m.Title != "Graph Product" {
		t.Errorf("title from JSON-LD name: got %q", m.Title)
	}
	if m.Price != "19.99" || m.Currency != "EUR" {
		t.Errorf("array-offer float price: got %q %q", m.Price, m.Currency)
	}
	if m.CoverURL != "https://cdn.example.com/a.jpg" {
		t.Errorf("first image from array: got %q", m.CoverURL)
	}
	// Description falls back to og:description when JSON-LD has none.
	if m.Description != "Described via OG only." {
		t.Errorf("og description fallback: got %q", m.Description)
	}
}

func TestParseHTML_TitleFallback(t *testing.T) {
	// No og:title, no JSON-LD name — must fall back to <title>.
	html := []byte(`<html><head><title>Bare Title</title></head><body></body></html>`)
	m := parseHTML(html, "https://example.com")
	if m.Title != "Bare Title" {
		t.Errorf("title fallback: got %q", m.Title)
	}
}

func TestParseHTML_Garbage(t *testing.T) {
	m := parseHTML([]byte("not html <<< at all"), "https://example.com")
	if m == nil {
		t.Fatal("ParseHTML must never return nil")
	}
	if m.Title != "" {
		t.Errorf("garbage input should yield no title, got %q", m.Title)
	}
}
