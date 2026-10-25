package metadata

import (
	"strings"
	"testing"
	"time"
)

// unityListingProductID is the product whose real page is captured in
// testdata/unity_listing_page.html.
const unityListingProductID = "162153"

func parseUnityFixture(t *testing.T) *unityListing {
	t.Helper()
	l, err := unityParseListing(readFixture(t, "unity_listing_page.html"), unityListingProductID)
	if err != nil {
		t.Fatalf("unityParseListing: %v", err)
	}
	return l
}

func TestUnityParseListingResolvesReferencedEntities(t *testing.T) {
	l := parseUnityFixture(t)

	if l.product == nil {
		t.Fatal("product not resolved")
	}
	if got := l.product.Name; got != "Low-Poly Simple Nature Pack" {
		t.Fatalf("product name = %q", got)
	}
	if l.version == nil || l.version.Name != "1.24" {
		t.Fatalf("version = %+v, want the currentVersion reference resolved to 1.24", l.version)
	}
	if l.publisher == nil || l.publisher.Name != "JustCreate" {
		t.Fatalf("publisher = %+v, want the publisher reference resolved", l.publisher)
	}
	if l.category == nil || l.category.Slug != "3d/environments/landscapes" {
		t.Fatalf("category = %+v, want the category reference resolved to its full slug path", l.category)
	}
	// The Category map holds every category on the store, so picking the wrong
	// key silently yields a plausible-looking but unrelated category.
	if l.category.Name != "Landscapes" {
		t.Fatalf("category name = %q, want the referenced category, not an arbitrary one", l.category.Name)
	}
	if len(l.tags) == 0 {
		t.Fatal("no tags resolved from popularTags")
	}
	joined := strings.Join(l.tags, ",")
	for _, want := range []string{"Landscape", "Terrain", "lowpoly"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("tags = %v, want %q present", l.tags, want)
		}
	}
}

func TestUnityApplyListingMapsTypedFields(t *testing.T) {
	l := parseUnityFixture(t)
	md := &Metadata{Store: StoreUnity}
	unityApplyListing(md, l, "https://assetstore.unity.com/packages/package/162153")

	if md.Engine != "unity" {
		t.Fatalf("Engine = %q", md.Engine)
	}
	wantReleased := time.Date(2020, 2, 11, 9, 47, 13, 0, time.UTC)
	if md.ReleasedAt == nil || !md.ReleasedAt.Equal(wantReleased) {
		t.Fatalf("ReleasedAt = %v, want %v", md.ReleasedAt, wantReleased)
	}
	wantUpdated := time.Date(2023, 5, 6, 19, 35, 15, 0, time.UTC)
	if md.StoreUpdatedAt == nil || !md.StoreUpdatedAt.Equal(wantUpdated) {
		t.Fatalf("StoreUpdatedAt = %v, want %v", md.StoreUpdatedAt, wantUpdated)
	}
	if md.LatestStoreVersion != "1.24" {
		t.Fatalf("LatestStoreVersion = %q, want 1.24", md.LatestStoreVersion)
	}
	if md.MinEngineVersion != "2020.3" {
		t.Fatalf("MinEngineVersion = %q, want the 2020.3.0 build trimmed to 2020.3", md.MinEngineVersion)
	}
	if md.Publisher != "JustCreate" {
		t.Fatalf("Publisher = %q", md.Publisher)
	}
	if md.PublisherURL != "https://justcreate3d.studio/" {
		t.Fatalf("PublisherURL = %q", md.PublisherURL)
	}
	if md.CategoryPath != "3d/environments/landscapes" {
		t.Fatalf("CategoryPath = %q, want one hierarchical path", md.CategoryPath)
	}
	if md.Rating != 5 || md.RatingCount != 126 {
		t.Fatalf("Rating = %v (%d), want 5 (126)", md.Rating, md.RatingCount)
	}
	if md.Currency != "USD" {
		t.Fatalf("Currency = %q", md.Currency)
	}
	if md.StoreID != "162153" {
		t.Fatalf("StoreID = %q", md.StoreID)
	}
	if len(md.ImageURLs) == 0 {
		t.Fatal("gallery not mapped from the payload's product images")
	}
	for _, u := range md.ImageURLs {
		if !strings.HasPrefix(u, "https://") {
			t.Fatalf("gallery URL %q not resolved against the page URL", u)
		}
	}
}

func TestUnityApplyListingBuildsSpecSheet(t *testing.T) {
	l := parseUnityFixture(t)
	md := &Metadata{Store: StoreUnity}
	unityApplyListing(md, l, "https://assetstore.unity.com/packages/package/162153")

	got := map[string]string{}
	var order []string
	for _, a := range md.Attributes {
		got[a.Key] = a.Value
		order = append(order, a.Key)
	}
	for key, want := range map[string]string{
		"asset_count":              "57",
		"download_size":            "1.9 MB",
		"package_type":             "unitypackage",
		"supported_unity_versions": "2020.3.0",
		"entitlement":              "Free",
	} {
		if got[key] != want {
			t.Fatalf("attribute %q = %q, want %q (all: %v)", key, got[key], want, got)
		}
	}
	if !strings.Contains(got["changelog"], "Added LODs") {
		t.Fatalf("changelog = %q, want the publish notes converted to markdown", got["changelog"])
	}
	if got["render_pipelines"] == "" || !strings.Contains(got["render_pipelines"], "standard") {
		t.Fatalf("render_pipelines = %q, want the SRP types", got["render_pipelines"])
	}
	want := []string{
		"supported_unity_versions", "download_size", "asset_count",
		"package_type", "render_pipelines", "entitlement", "changelog",
	}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("attribute order = %v, want %v", order, want)
	}
}

// Coveo is fetched first and is authoritative for the fields it carries; the
// listing payload must not clobber them.
func TestUnityApplyListingDoesNotOverwriteCoveoFields(t *testing.T) {
	l := parseUnityFixture(t)
	md := &Metadata{
		Store:       StoreUnity,
		Title:       "Coveo Title",
		Description: "Coveo description",
		Publisher:   "Coveo Publisher",
		Price:       "12",
		StoreID:     "999",
		ImageURLs:   []string{"https://cdn.example.com/coveo.png"},
		CoverURL:    "https://cdn.example.com/coveo.png",
	}
	unityApplyListing(md, l, "https://assetstore.unity.com/packages/package/162153")

	if md.Title != "Coveo Title" || md.Description != "Coveo description" ||
		md.Publisher != "Coveo Publisher" || md.Price != "12" || md.StoreID != "999" {
		t.Fatalf("listing overwrote Coveo-supplied fields: %+v", md)
	}
	if len(md.ImageURLs) != 1 || md.ImageURLs[0] != "https://cdn.example.com/coveo.png" {
		t.Fatalf("ImageURLs = %v, want the Coveo gallery kept", md.ImageURLs)
	}
	if md.LatestStoreVersion != "1.24" {
		t.Fatalf("LatestStoreVersion = %q, want listing-only fields still filled", md.LatestStoreVersion)
	}
}

// The fixture's product is not a custom licence, so the blanket EULA hint stays.
func TestUnityApplyListingKeepsEULAHintForStandardEntitlement(t *testing.T) {
	l := parseUnityFixture(t)
	md := &Metadata{Store: StoreUnity, LicenceHint: "unity-eula"}
	unityApplyListing(md, l, "https://assetstore.unity.com/packages/package/162153")

	if md.LicenceHint != "unity-eula" {
		t.Fatalf("LicenceHint = %q, want the EULA hint retained", md.LicenceHint)
	}
	if md.LicenceText != "" {
		t.Fatalf("LicenceText = %q, want none for a non-custom licence", md.LicenceText)
	}
}

// A custom per-asset licence is the fidelity Coveo cannot express: its text must
// reach LicenceText so the detector runs on it instead of the blanket hint.
func TestUnityApplyListingPromotesCustomLicenceText(t *testing.T) {
	md := &Metadata{Store: StoreUnity, LicenceHint: "unity-eula"}
	l := &unityListing{product: &unityProduct{
		Name:          "Custom",
		CustomLicense: true,
		LicenseText:   "<p>You may not redistribute this asset.</p>",
	}}
	unityApplyListing(md, l, "https://assetstore.unity.com/packages/package/1")

	if md.LicenceText != "You may not redistribute this asset." {
		t.Fatalf("LicenceText = %q, want the custom licence text stripped of tags", md.LicenceText)
	}
	if md.LicenceHint != "" {
		t.Fatalf("LicenceHint = %q, want the blanket hint cleared so the text is detected", md.LicenceHint)
	}
}

func TestUnityParseListingErrors(t *testing.T) {
	cases := map[string][]byte{
		"no render call":  []byte(`<html><body>nothing here</body></html>`),
		"no object":       []byte(`<script>__component__.ReactDOMrender(</script>`),
		"unterminated":    []byte(`<script>__component__.ReactDOMrender({"data":{"ENTITY":{</script>`),
		"not json":        []byte(`<script>__component__.ReactDOMrender({not json at all})</script>`),
		"empty entity":    []byte(`<script>__component__.ReactDOMrender({"data":{"ENTITY":{}}})</script>`),
		"no product":      []byte(`<script>__component__.ReactDOMrender({"data":{"ENTITY":{"Product":{}}}})</script>`),
		"other id, multi": []byte(`<script>__component__.ReactDOMrender({"data":{"ENTITY":{"Product":{"1":{"id":"1"},"2":{"id":"2"}}}}})</script>`),
	}
	for name, body := range cases {
		if _, err := unityParseListing(body, unityListingProductID); err == nil {
			t.Fatalf("%s: expected an error, got none", name)
		}
	}
}

// The whole listing path is best-effort: a nil listing must leave Coveo's
// metadata untouched rather than panicking or blanking fields.
func TestUnityApplyListingNilIsNoOp(t *testing.T) {
	md := &Metadata{Store: StoreUnity, Title: "Kept"}
	unityApplyListing(md, nil, "https://assetstore.unity.com/packages/package/1")
	unityApplyListing(md, &unityListing{}, "https://assetstore.unity.com/packages/package/1")
	unityApplyListing(nil, &unityListing{}, "")
	if md.Title != "Kept" || md.Engine != "" {
		t.Fatalf("nil listing changed metadata: %+v", md)
	}
}

// A brace inside a string literal must not close the argument object early.
func TestUnityRenderArgumentIgnoresBracesInStrings(t *testing.T) {
	body := []byte(`<script>x.ReactDOMrender({"a":"} not the end \" also }","b":{"c":1}});more()</script>`)
	arg, err := unityRenderArgument(body)
	if err != nil {
		t.Fatalf("unityRenderArgument: %v", err)
	}
	want := `{"a":"} not the end \" also }","b":{"c":1}}`
	if string(arg) != want {
		t.Fatalf("argument = %s, want %s", arg, want)
	}
}

func TestUnityPickProductFallsBackToSoleEntry(t *testing.T) {
	only := &unityProduct{ID: "7", Name: "Only"}
	if got := unityPickProduct(map[string]*unityProduct{"7": only}, "mismatch"); got != only {
		t.Fatalf("unityPickProduct = %v, want the sole entry when the id does not match", got)
	}
}

func TestUnityShortVersion(t *testing.T) {
	for in, want := range map[string]string{
		"2020.3.0f1": "2020.3",
		"2020.3.0":   "2020.3",
		"2021.1":     "2021.1",
		"6":          "6",
		"":           "",
	} {
		if got := unityShortVersion(in); got != want {
			t.Fatalf("unityShortVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnityHumanSize(t *testing.T) {
	for in, want := range map[string]string{
		"2011664":      "1.9 MB",
		"512":          "512 B",
		"1048576":      "1.0 MB",
		"0":            "",
		"":             "",
		"not-a-number": "",
	} {
		if got := unityHumanSize(in); got != want {
			t.Fatalf("unityHumanSize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	if got := parseTimestamp("2020-02-11T09:47:13Z"); got == nil || got.Year() != 2020 {
		t.Fatalf("unityParseTime = %v", got)
	}
	for _, in := range []string{"", "   ", "not a date", "11/02/2020"} {
		if got := parseTimestamp(in); got != nil {
			t.Fatalf("parseTimestamp(%q) = %v, want nil", in, got)
		}
	}
}
