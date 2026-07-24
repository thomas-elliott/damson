// Package metadata fetches and normalises asset listing metadata from online
// storefronts. A generic Open Graph / JSON-LD scraper is the base; per-store
// adapters (Unity Asset Store, Sketchfab, itch.io) layer on top keyed by URL
// host and fall back to the generic scraper on failure.
//
// This package holds the first outbound HTTP in the codebase, so the client
// (client.go) is SSRF-guarded: it resolves and screens destination IPs before
// dialling, caps redirects/body-size/time, and rate-limits per host.
package metadata

import "encoding/json"

// Metadata is the normalised shape every fetcher returns, regardless of the
// source store. Empty fields mean "not found"; callers decide how to apply
// them (fill-blanks vs confirm-overwrite).
type Metadata struct {
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Publisher   string          `json:"publisher,omitempty"`
	Price       string          `json:"price,omitempty"`
	Currency    string          `json:"currency,omitempty"`
	LicenceHint string          `json:"licence_hint,omitempty"`
	LicenceText string          `json:"licence_text,omitempty"`
	CoverURL    string          `json:"cover_url,omitempty"`
	ImageURLs   []string        `json:"image_urls,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Store       string          `json:"store,omitempty"`
	StoreID     string          `json:"store_id,omitempty"`
	ListingURL  string          `json:"listing_url,omitempty"`
	Raw         json.RawMessage `json:"raw,omitempty"`
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
