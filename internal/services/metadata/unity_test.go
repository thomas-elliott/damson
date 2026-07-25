package metadata

import (
	"strings"
	"testing"
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

func TestUnityGalleryFromPage(t *testing.T) {
	page := "https://assetstore.unity.com/packages/vfx/particles/polygon-particle-fx-pack-art-by-synty-168372"
	got := unityGalleryFromPage(readFixture(t, "unity_listing.html"), page)
	want := []string{
		"https://assetstorev1-prd-cdn.unity3d.com/package-screenshot/39b9d632-47f1-422c-b36e-3de01adcde8e_scaled.jpg",
		"https://assetstorev1-prd-cdn.unity3d.com/package-screenshot/fcbfa2ba-d303-486a-b0da-5cc98b938de2_scaled.jpg",
		"https://assetstorev1-prd-cdn.unity3d.com/package-screenshot/c6f80131-9a09-4ae4-938c-4ec917ddad89_scaled.jpg",
	}
	if len(got) != len(want) {
		t.Fatalf("gallery = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gallery[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUnityGalleryFromPageNoImages(t *testing.T) {
	if got := unityGalleryFromPage([]byte(`<html><body>{"images":"none"}</body></html>`), "https://x/y"); got != nil {
		t.Fatalf("expected no gallery, got %v", got)
	}
}

func TestUnityGalleryFromPageSkipsUnrelatedImagesArray(t *testing.T) {
	body := []byte(`{"images":[],"other":1,"images":[{"type":"screenshot","imageUrl":"//cdn/x_scaled.jpg"}]}`)
	got := unityGalleryFromPage(body, "https://assetstore.unity.com/packages/package/1")
	if len(got) != 1 || got[0] != "https://cdn/x_scaled.jpg" {
		t.Fatalf("gallery = %v, want the populated array's image", got)
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
