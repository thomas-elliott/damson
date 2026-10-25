package metadata

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAddAttributeAppendsInOrder(t *testing.T) {
	var m Metadata
	m.AddAttribute("asset_count", "Assets", "412")
	m.AddAttribute("download_size", "Size", "1.2 GB")
	m.AddAttribute("package_type", "Package type", "unitypackage")

	want := []Attribute{
		{Key: "asset_count", Label: "Assets", Value: "412"},
		{Key: "download_size", Label: "Size", Value: "1.2 GB"},
		{Key: "package_type", Label: "Package type", Value: "unitypackage"},
	}
	if len(m.Attributes) != len(want) {
		t.Fatalf("Attributes = %+v, want %d entries", m.Attributes, len(want))
	}
	for i := range want {
		if m.Attributes[i] != want[i] {
			t.Fatalf("Attributes[%d] = %+v, want %+v", i, m.Attributes[i], want[i])
		}
	}
}

func TestAddAttributeSkipsBlankAndDuplicate(t *testing.T) {
	var m Metadata
	m.AddAttribute("", "No key", "value")
	m.AddAttribute("blank", "Blank", "   ")
	m.AddAttribute("size", "Size", "1 GB")
	m.AddAttribute("size", "Size (other)", "999 GB")

	if len(m.Attributes) != 1 {
		t.Fatalf("Attributes = %+v, want only the one usable entry", m.Attributes)
	}
	if m.Attributes[0].Value != "1 GB" {
		t.Fatalf("Attributes[0].Value = %q, want the first write to win", m.Attributes[0].Value)
	}
}

func TestAddAttributeTrimsAndDefaultsLabel(t *testing.T) {
	var m Metadata
	m.AddAttribute("status", "", "  Released  ")
	if len(m.Attributes) != 1 {
		t.Fatalf("Attributes = %+v, want one entry", m.Attributes)
	}
	if got := m.Attributes[0].Value; got != "Released" {
		t.Fatalf("Value = %q, want trimmed", got)
	}
	if got := m.Attributes[0].Label; got != "status" {
		t.Fatalf("Label = %q, want the key as fallback label", got)
	}
}

// The struct is persisted to assets.source_metadata_json, so an absent date must
// stay absent rather than round-tripping as the zero time.
func TestMetadataJSONOmitsAbsentFields(t *testing.T) {
	blob, err := json.Marshal(&Metadata{Title: "Thing"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"released_at", "store_updated_at", "rating", "rating_count", "attributes", "category_path", "publisher_url"} {
		if _, present := got[k]; present {
			t.Fatalf("key %q present in %s, want omitted when unset", k, blob)
		}
	}
	if _, present := got["title"]; !present {
		t.Fatalf("title missing from %s", blob)
	}
}

func TestMetadataJSONRoundTripsNewFields(t *testing.T) {
	released := time.Date(2020, 3, 14, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2024, 11, 2, 12, 30, 0, 0, time.UTC)
	in := &Metadata{
		Title:              "Thing",
		PublisherURL:       "https://example.com/pub",
		CategoryPath:       "3d/environments/landscapes",
		ImageURLs:          []string{"https://cdn.example.com/a.png"},
		ReleasedAt:         &released,
		StoreUpdatedAt:     &updated,
		LatestStoreVersion: "1.24",
		Engine:             "unity",
		MinEngineVersion:   "2020.3",
		Rating:             4.5,
		RatingCount:        128,
		Attributes:         []Attribute{{Key: "asset_count", Label: "Assets", Value: "412"}},
	}
	blob, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Metadata
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ReleasedAt == nil || !out.ReleasedAt.Equal(released) {
		t.Fatalf("ReleasedAt = %v, want %v", out.ReleasedAt, released)
	}
	if out.StoreUpdatedAt == nil || !out.StoreUpdatedAt.Equal(updated) {
		t.Fatalf("StoreUpdatedAt = %v, want %v", out.StoreUpdatedAt, updated)
	}
	if out.PublisherURL != in.PublisherURL || out.CategoryPath != in.CategoryPath ||
		out.LatestStoreVersion != in.LatestStoreVersion || out.Engine != in.Engine ||
		out.MinEngineVersion != in.MinEngineVersion || out.Rating != in.Rating ||
		out.RatingCount != in.RatingCount {
		t.Fatalf("round trip lost fields: %+v", out)
	}
	if len(out.ImageURLs) != 1 || out.ImageURLs[0] != in.ImageURLs[0] {
		t.Fatalf("ImageURLs = %v, want the gallery preserved under its image_urls key", out.ImageURLs)
	}
	if len(out.Attributes) != 1 || out.Attributes[0] != in.Attributes[0] {
		t.Fatalf("Attributes = %+v, want round trip", out.Attributes)
	}
}
