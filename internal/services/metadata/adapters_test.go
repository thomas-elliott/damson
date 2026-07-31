package metadata

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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
	// tags then categories, by name.
	if got := strings.Join(m.Tags, ","); got != "tree,low-poly,Nature" {
		t.Errorf("tags: got %q want %q", got, "tree,low-poly,Nature")
	}
}

func TestParseSketchfabModelNewFields(t *testing.T) {
	m, err := parseSketchfabModel(readFixture(t, "sketchfab_model_full.json"),
		"https://sketchfab.com/3d-models/low-poly-tree-scene-free-89daa5e21f0d4f08a59dba0d566e88bd")
	if err != nil {
		t.Fatal(err)
	}
	if m.PublisherURL != "https://sketchfab.com/Nicholas01" {
		t.Errorf("publisher url: got %q", m.PublisherURL)
	}
	// Sketchfab returns naive local timestamps with fractional seconds.
	wantReleased := time.Date(2023, 4, 21, 19, 3, 26, 0, time.UTC)
	if m.ReleasedAt == nil || m.ReleasedAt.Truncate(time.Second) != wantReleased {
		t.Errorf("released at: got %v, want %v", m.ReleasedAt, wantReleased)
	}
	if m.StoreUpdatedAt == nil || m.StoreUpdatedAt.Year() != 2023 {
		t.Errorf("store updated at: got %v", m.StoreUpdatedAt)
	}
	if m.CategoryPath != "Art & Abstract" {
		t.Errorf("category path: got %q", m.CategoryPath)
	}

	got := map[string]string{}
	var order []string
	for _, a := range m.Attributes {
		got[a.Key] = a.Value
		order = append(order, a.Key)
	}
	for key, want := range map[string]string{
		"vertex_count":   "377,757",
		"face_count":     "256,652",
		"material_count": "5",
		"texture_count":  "13",
		"downloadable":   "Yes",
		"view_count":     "322,877",
		"like_count":     "3,815",
		"download_count": "112,335",
	} {
		if got[key] != want {
			t.Errorf("attribute %q = %q, want %q", key, got[key], want)
		}
	}
	// animationCount and soundCount are 0 on this model and must be omitted
	// rather than rendered as "0".
	for _, key := range []string{"animation_count", "sound_count"} {
		if _, present := got[key]; present {
			t.Errorf("attribute %q present for a zero count", key)
		}
	}
	if len(order) == 0 || order[0] != "vertex_count" {
		t.Errorf("attribute order = %v, want geometry counts first", order)
	}
}

func TestFormatCount(t *testing.T) {
	for in, want := range map[float64]string{
		0: "0", 5: "5", 999: "999", 1000: "1,000",
		12345: "12,345", 377757: "377,757", 1234567: "1,234,567",
	} {
		if got := formatCount(in); got != want {
			t.Errorf("formatCount(%v) = %q, want %q", in, got, want)
		}
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
	// ec_keywords, deduped case-insensitively. The live index field is
	// ec_keywords, not keywords — asking for the wrong name silently yielded no
	// keyword tags at all.
	if got := strings.Join(m.Tags, ","); got != "Landscape,Terrain,lowpoly" {
		t.Errorf("tags: got %q want %q", got, "Landscape,Terrain,lowpoly")
	}
	// The category must arrive as one hierarchical path, not as ec_category's
	// three arrow-separated, leading-space entries.
	if m.CategoryPath != "3d/environments/landscapes" {
		t.Errorf("category path: got %q", m.CategoryPath)
	}
	for _, tag := range m.Tags {
		if strings.Contains(tag, ">") || tag != strings.TrimSpace(tag) {
			t.Errorf("tag %q carries category separators or padding", tag)
		}
	}
	if m.Engine != "unity" {
		t.Errorf("engine: got %q", m.Engine)
	}
	if m.MinEngineVersion != "2020.3" {
		t.Errorf("min engine version: got %q, want the packed 202003 unpacked", m.MinEngineVersion)
	}
	if m.ReleasedAt == nil || !m.ReleasedAt.Equal(time.Date(2020, 2, 11, 9, 47, 13, 0, time.UTC)) {
		t.Errorf("released at: got %v", m.ReleasedAt)
	}
	if m.StoreUpdatedAt == nil || !m.StoreUpdatedAt.Equal(time.Date(2023, 5, 6, 19, 35, 15, 0, time.UTC)) {
		t.Errorf("store updated at: got %v", m.StoreUpdatedAt)
	}
	if m.Rating != 4.85 || m.RatingCount != 126 {
		t.Errorf("rating: got %v (%d)", m.Rating, m.RatingCount)
	}
	if m.PublisherURL != "https://assetstore.unity.com/publishers/44390" {
		t.Errorf("publisher url: got %q", m.PublisherURL)
	}
}

func TestCoveoCategoryPathFallsBackThroughLevels(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{"level3", map[string]any{"ec_category_level3": "3d > environments > landscapes"}, "3d/environments/landscapes"},
		{"level2 only", map[string]any{"ec_category_level2": "3d > environments"}, "3d/environments"},
		{"level1 only", map[string]any{"ec_category_level1": "3d"}, "3d"},
		{"array shape", map[string]any{"ec_category_level3": []any{"2d > gui > icons"}}, "2d/gui/icons"},
		{"none", map[string]any{}, ""},
		{"blank", map[string]any{"ec_category_level3": "  >  > "}, ""},
	}
	for _, c := range cases {
		if got := coveoCategoryPath(c.raw); got != c.want {
			t.Errorf("%s: coveoCategoryPath = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestUnityMinVersionLabel(t *testing.T) {
	cases := map[any]string{
		202003.0: "2020.3",
		201904.0: "2019.4",
		"202003": "2020.3",
		202010.0: "2020.10",
		600000.0: "6000.0",
		"":       "",
		"6":      "6",
	}
	for in, want := range cases {
		if got := unityMinVersionLabel(in); got != want {
			t.Errorf("unityMinVersionLabel(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestCoveoEpochTime(t *testing.T) {
	if got := coveoEpochTime(1581414433000.0); got == nil || !got.Equal(time.Date(2020, 2, 11, 9, 47, 13, 0, time.UTC)) {
		t.Errorf("coveoEpochTime = %v", got)
	}
	for _, in := range []any{nil, 0.0, -1.0, "", "not a number", map[string]any{}} {
		if got := coveoEpochTime(in); got != nil {
			t.Errorf("coveoEpochTime(%v) = %v, want nil", in, got)
		}
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

	tags := itchTags(body)
	if got := strings.Join(tags, ","); got != "Role Playing,pixel-art,tileset,dungeon" {
		t.Errorf("itch tags: got %q", got)
	}
}

func TestParseHTMLKeywords(t *testing.T) {
	page := []byte(`<html><head>
		<meta name="keywords" content="platformer, 2d, meta-only">
		<script type="application/ld+json">{"@type":"Product","name":"X","keywords":["Platformer","Pixel Art","platformer"]}</script>
		</head><body></body></html>`)
	m := ParseHTML(page, "https://example.com/x")
	// JSON-LD keywords first (deduped), then any new meta keyword.
	got := strings.Join(m.Tags, ",")
	if got != "Platformer,Pixel Art,2d,meta-only" {
		t.Fatalf("keywords: got %q", got)
	}
	// A no-op (no keyword capture) would leave Tags empty.
	if len(m.Tags) == 0 {
		t.Fatal("expected keywords to be captured")
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
