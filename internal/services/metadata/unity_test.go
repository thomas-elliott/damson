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
