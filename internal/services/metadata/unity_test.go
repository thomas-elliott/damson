package metadata

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCoveoImagesDedupesAndExcludesCover(t *testing.T) {
	raw := map[string]any{
		"ec_images": []any{"a.jpg", "cover.jpg", "b.jpg", "a.jpg"},
	}
	got := coveoImages(raw, "cover.jpg")
	want := []string{"a.jpg", "b.jpg"}
	if len(got) != len(want) {
		t.Fatalf("coveoImages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("coveoImages = %v, want %v", got, want)
		}
	}
}

func TestCoveoImagesJSONStringArray(t *testing.T) {
	raw := map[string]any{"ec_images": `["x.png","y.png"]`}
	got := coveoImages(raw, "")
	if len(got) != 2 || got[0] != "x.png" || got[1] != "y.png" {
		t.Fatalf("coveoImages(json string) = %v", got)
	}
}

func TestCoveoCoverPrefersStillImageOverVideoThumbnail(t *testing.T) {
	raw := map[string]any{
		"ec_thumbnails": "https://cdn.example.com/preview/clip.mp4",
		"ec_images":     []any{"https://cdn.example.com/img/shot1.png", "https://cdn.example.com/img/shot2.png"},
	}
	got := coveoCover(raw)
	if got != "https://cdn.example.com/img/shot1.png" {
		t.Fatalf("coveoCover = %q, want the first still image (not the video thumbnail)", got)
	}
}

func TestCoveoCoverSkipsVideoEntriesInImages(t *testing.T) {
	raw := map[string]any{
		"ec_images": []any{"https://cdn.example.com/v/clip.mp4", "https://www.youtube.com/watch?v=abc", "https://cdn.example.com/img/still.jpg"},
	}
	got := coveoCover(raw)
	if got != "https://cdn.example.com/img/still.jpg" {
		t.Fatalf("coveoCover = %q, want the still image past the video entries", got)
	}
}

func TestCoveoCoverFallsBackToThumbnail(t *testing.T) {
	raw := map[string]any{"ec_thumbnails": "https://cdn.example.com/thumb.png"}
	if got := coveoCover(raw); got != "https://cdn.example.com/thumb.png" {
		t.Fatalf("coveoCover = %q, want the thumbnail fallback", got)
	}
}

func TestCoveoCoverFallsBackToFirstImageWhenAllVideo(t *testing.T) {
	raw := map[string]any{"ec_images": []any{"https://cdn.example.com/v/only.mp4"}}
	if got := coveoCover(raw); got != "https://cdn.example.com/v/only.mp4" {
		t.Fatalf("coveoCover = %q, want first image when nothing better exists", got)
	}
}

func TestHtmlToMarkdownPreservesStructure(t *testing.T) {
	md := htmlToMarkdown("<p>First para.</p><ul><li>one</li><li>two</li></ul>")
	if !strings.Contains(md, "First para.") {
		t.Fatalf("paragraph text lost: %q", md)
	}
	if !strings.Contains(md, "one") || !strings.Contains(md, "two") {
		t.Fatalf("list items lost: %q", md)
	}
	if !strings.Contains(md, "-") && !strings.Contains(md, "*") {
		t.Fatalf("list markers lost, formatting not preserved: %q", md)
	}
}

func TestHtmlToMarkdownEmpty(t *testing.T) {
	if got := htmlToMarkdown("   "); got != "" {
		t.Fatalf("empty input should yield empty, got %q", got)
	}
}

// A deprecated listing is missing from the search index, but its page is
// still up: the lookup must read the store's details from the page rather than
// fail over to the generic scraper.
func TestUnityFetch_DeprecatedListingReadFromPage(t *testing.T) {
	r := fakeRegistry(t, ClientOptions{HostMinGap: time.Millisecond},
		unityStore(t, []byte(`{"results":[]}`), readFixture(t, "unity_listing_deprecated.html")))

	md, err := r.Fetch(context.Background(), "https://assetstore.unity.com/packages/slug/32351")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if md.Store != StoreUnity || md.StoreID != "32351" {
		t.Fatalf("store = %q id = %q, want unity 32351", md.Store, md.StoreID)
	}
	if md.StoreDeprecated == nil || !*md.StoreDeprecated {
		t.Fatalf("StoreDeprecated = %v, want true", md.StoreDeprecated)
	}
	wantReleased := time.Date(2015, 3, 16, 7, 40, 35, 0, time.UTC)
	if md.ReleasedAt == nil || !md.ReleasedAt.Equal(wantReleased) {
		t.Fatalf("ReleasedAt = %v, want %v", md.ReleasedAt, wantReleased)
	}
	if md.LatestStoreVersion != "1.1.6" {
		t.Fatalf("LatestStoreVersion = %q, want 1.1.6", md.LatestStoreVersion)
	}
	if md.Publisher != "Unity Technologies" || md.Rating != 4 || md.RatingCount != 5323 {
		t.Fatalf("publisher %q rating %v (%d), want the page's", md.Publisher, md.Rating, md.RatingCount)
	}
	if md.Description == "" || len(md.ImageURLs) == 0 {
		t.Fatalf("description %q images %v, want both from the page", md.Description, md.ImageURLs)
	}
	if md.Price != "" || md.Currency != "" {
		t.Fatalf("price = %q %q, want blank", md.Price, md.Currency)
	}
}

// A live listing's page clears the flag, so a listing the store restores stops
// reading as deprecated on the next lookup.
func TestUnityFetch_LiveListingClearsDeprecated(t *testing.T) {
	r := fakeRegistry(t, ClientOptions{HostMinGap: time.Millisecond},
		unityStore(t, readFixture(t, "unity_coveo.json"), readFixture(t, "unity_listing_page.html")))

	md, err := r.Fetch(context.Background(), "https://assetstore.unity.com/packages/package/162153")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if md.StoreDeprecated == nil || *md.StoreDeprecated {
		t.Fatalf("StoreDeprecated = %v, want false", md.StoreDeprecated)
	}
	if md.Price == "" {
		t.Fatal("a live listing keeps its price")
	}
}
