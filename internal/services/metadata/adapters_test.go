package metadata

import (
	"encoding/json"
	"testing"
)

func TestSketchfabUID(t *testing.T) {
	cases := map[string]string{
		"https://sketchfab.com/3d-models/low-poly-tree-abc123def4567890abc123def4567890": "abc123def4567890abc123def4567890",
		"https://sketchfab.com/models/ABC123DEF4567890ABC123DEF4567890":                  "abc123def4567890abc123def4567890",
		"https://sketchfab.com/3d-models/no-uid-here":                                    "",
	}
	for in, want := range cases {
		if got := sketchfabUID(in); got != want {
			t.Errorf("sketchfabUID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSketchfabModel(t *testing.T) {
	m, err := parseSketchfabModel(readFixture(t, "sketchfab_model.json"), "https://sketchfab.com/3d-models/low-poly-tree-abc123def4567890abc123def4567890")
	if err != nil {
		t.Fatal(err)
	}
	if m.Store != StoreSketchfab {
		t.Errorf("store: got %q", m.Store)
	}
	if m.Title != "Low Poly Tree" {
		t.Errorf("title: got %q", m.Title)
	}
	if m.Publisher != "TreeMaker" {
		t.Errorf("publisher: got %q", m.Publisher)
	}
	// cc-by slug must normalise to the SPDX id, not the raw label.
	if m.LicenceHint != "CC-BY-4.0" {
		t.Errorf("licence hint: got %q want CC-BY-4.0", m.LicenceHint)
	}
	// largest thumbnail wins.
	if m.CoverURL != "https://media.sketchfab.com/large.jpg" {
		t.Errorf("cover: got %q", m.CoverURL)
	}
	if len(m.Raw) == 0 {
		t.Error("raw payload must be persisted")
	}
}

func TestUnityProductID(t *testing.T) {
	cases := map[string]string{
		"https://assetstore.unity.com/packages/3d/props/fantasy-environment-pack-12345":  "12345",
		"https://assetstore.unity.com/packages/3d/props/fantasy-environment-pack-12345/": "12345",
		"https://assetstore.unity.com/packages/package/162153":                           "162153",
		"https://assetstore.unity.com/packages/no-id":                                    "",
	}
	for in, want := range cases {
		if got := unityProductID(in); got != want {
			t.Errorf("unityProductID(%q) = %q, want %q", in, got, want)
		}
	}
}

func decodeCoveo(t *testing.T, name string) coveoResponse {
	t.Helper()
	var resp coveoResponse
	if err := json.Unmarshal(readFixture(t, name), &resp); err != nil {
		t.Fatalf("decode coveo fixture: %v", err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("fixture has no results")
	}
	return resp
}

func TestCoveoMetadata(t *testing.T) {
	resp := decodeCoveo(t, "unity_coveo.json")
	m := coveoMetadata(resp.Results[0], "https://assetstore.unity.com/packages/package/162153")

	if m.Title != "Low-Poly Simple Nature Pack" {
		t.Errorf("title: got %q", m.Title)
	}
	// HTML descriptions are converted to Markdown, preserving inline emphasis.
	if m.Description != "A **tidy** low-poly nature set." {
		t.Errorf("description: got %q", m.Description)
	}
	if m.Publisher != "JustCreate" {
		t.Errorf("publisher: got %q", m.Publisher)
	}
	if m.Price != "0" || m.Currency != "USD" {
		t.Errorf("price/currency: got %q %q", m.Price, m.Currency)
	}
	// ec_product_id is a JSON array — the first element is the id.
	if m.StoreID != "162153" {
		t.Errorf("store id: got %q", m.StoreID)
	}
	// Cover comes from ec_thumbnails.
	if m.CoverURL != "https://assetstorev1-prd-cdn.unity3d.com/key-image/abc.jpg" {
		t.Errorf("cover: got %q", m.CoverURL)
	}
	if m.LicenceHint != "unity-eula" {
		t.Errorf("licence hint: got %q want unity-eula", m.LicenceHint)
	}
	if len(m.Raw) == 0 {
		t.Error("raw payload must be persisted")
	}
}

func TestCoveoCandidate(t *testing.T) {
	resp := decodeCoveo(t, "unity_coveo.json")
	c := coveoCandidate(resp.Results[0])
	if c.Title != "Low-Poly Simple Nature Pack" || c.Store != StoreUnity {
		t.Errorf("candidate: %+v", c)
	}
	if c.ListingURL != "https://assetstore.unity.com/packages/package/162153" {
		t.Errorf("listing url: got %q", c.ListingURL)
	}
	if c.StoreID != "162153" {
		t.Errorf("store id from array: got %q", c.StoreID)
	}
}

func TestItchLicenceAndParse(t *testing.T) {
	body := readFixture(t, "itch_listing.html")
	meta := ParseHTML(body, "https://pixelsmith.itch.io/pixel-dungeon-tileset")
	// itch omits og:title — title must fall back to twitter:title.
	if meta.Title != "Pixel Dungeon Tileset" {
		t.Errorf("title fallback: got %q", meta.Title)
	}

	label, href := itchLicence(body)
	if href != "https://creativecommons.org/licenses/by-sa/4.0/" {
		t.Errorf("licence href: got %q", href)
	}
	if spdxFromLicenceURL(href) != "CC-BY-SA-4.0" {
		t.Errorf("licence url mapping: got %q", spdxFromLicenceURL(href))
	}
	if label == "" {
		t.Error("licence label should not be empty")
	}
}

func TestSPDXMappers(t *testing.T) {
	if spdxFromSlug("cc0") != "CC0-1.0" {
		t.Error("cc0 slug")
	}
	if spdxFromSlug("totally-unknown") != "" {
		t.Error("unknown slug must map to empty")
	}
	if spdxFromLabel("Creative Commons Attribution NonCommercial ShareAlike") != "CC-BY-NC-SA-4.0" {
		t.Error("nc-sa label")
	}
	if spdxFromLabel("proprietary blah") != "" {
		t.Error("unknown label must map to empty")
	}
}
