package metadata

import (
	"context"
	"fmt"
	"time"
)

// SourceRecord is the shape persisted to Asset.SourceMetadataJSON: the
// normalised metadata (which carries its own raw payload), provenance, and any
// stored search candidates. Shared by the manual fetch handler and the import
// auto-enrichment stage so both write the same envelope.
type SourceRecord struct {
	Store      string      `json:"store,omitempty"`
	ListingURL string      `json:"listing_url,omitempty"`
	FetchedAt  time.Time   `json:"fetched_at"`
	Confidence float64     `json:"match_confidence,omitempty"`
	Matched    bool        `json:"matched"`
	Metadata   *Metadata   `json:"metadata,omitempty"`
	Candidates []Candidate `json:"candidates,omitempty"`
}

// FetchImage retrieves a cover/preview image with the SSRF-guarded client and
// returns its bytes and content type. Callers must validate the bytes decode
// as an image before storing (the thumbnail generator does this).
func (r *Registry) FetchImage(ctx context.Context, rawURL string) ([]byte, string, error) {
	resp, err := r.client.Get(ctx, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("metadata: image fetch %s: status %d", rawURL, resp.StatusCode)
	}
	return resp.Body, resp.ContentType, nil
}
