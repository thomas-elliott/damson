// Package metadata fetches and normalises asset listing metadata from online
// storefronts. A generic Open Graph / JSON-LD scraper is the base; per-store
// adapters (Unity Asset Store, Sketchfab, itch.io) layer on top keyed by URL
// host and fall back to the generic scraper on failure.
//
// This package holds the first outbound HTTP in the codebase, so the client
// (client.go) is SSRF-guarded: it resolves and screens destination IPs before
// dialling, caps redirects/body-size/time, and rate-limits per host.
package metadata

import (
	"encoding/json"
	"strings"
	"time"
)

// Metadata is the normalised shape every fetcher returns, regardless of the
// source store. Empty fields mean "not found"; callers decide how to apply
// them (fill-blanks vs confirm-overwrite).
type Metadata struct {
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	Publisher    string   `json:"publisher,omitempty"`
	PublisherURL string   `json:"publisher_url,omitempty"`
	Price        string   `json:"price,omitempty"`
	Currency     string   `json:"currency,omitempty"`
	LicenceHint  string   `json:"licence_hint,omitempty"`
	LicenceText  string   `json:"licence_text,omitempty"`
	CoverURL     string   `json:"cover_url,omitempty"`
	ImageURLs    []string `json:"image_urls,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	// CategoryPath is the store's category as a single hierarchical tag path
	// ("3d/environments/landscapes"), separate from the flat Tags list.
	CategoryPath string `json:"category_path,omitempty"`
	Store        string `json:"store,omitempty"`
	StoreID      string `json:"store_id,omitempty"`
	ListingURL   string `json:"listing_url,omitempty"`
	// ReleasedAt / StoreUpdatedAt are nil when the store exposes no such date.
	ReleasedAt         *time.Time `json:"released_at,omitempty"`
	StoreUpdatedAt     *time.Time `json:"store_updated_at,omitempty"`
	LatestStoreVersion string     `json:"latest_store_version,omitempty"`
	Engine             string     `json:"engine,omitempty"`
	MinEngineVersion   string     `json:"min_engine_version,omitempty"`
	Rating             float64    `json:"rating,omitempty"`
	RatingCount        int        `json:"rating_count,omitempty"`
	// Attributes is the store-specific long tail, in store-authored order.
	Attributes []Attribute     `json:"attributes,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// Attribute is one read-only spec-sheet row: a stable key for the client, the
// store's own label, and the already-formatted display value.
type Attribute struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// AddAttribute appends a spec-sheet row, skipping blank values and keys already
// present so adapters can map optimistically without checking first.
func (m *Metadata) AddAttribute(key, label, value string) {
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	for _, a := range m.Attributes {
		if a.Key == key {
			return
		}
	}
	if label == "" {
		label = key
	}
	m.Attributes = append(m.Attributes, Attribute{Key: key, Label: label, Value: value})
}

// Candidate is one search hit — the subset of Metadata a search result carries
// before the user picks it and the full listing is fetched.
type Candidate struct {
	Title      string `json:"title"`
	Publisher  string `json:"publisher,omitempty"`
	Price      string `json:"price,omitempty"`
	Currency   string `json:"currency,omitempty"`
	CoverURL   string `json:"cover_url,omitempty"`
	ListingURL string `json:"listing_url"`
	Store      string `json:"store"`
	StoreID    string `json:"store_id,omitempty"`
}

// Store identifiers used across adapters and persisted as provenance.
const (
	StoreGeneric   = "generic"
	StoreUnity     = "unity"
	StoreSketchfab = "sketchfab"
	StoreItch      = "itch"
)
