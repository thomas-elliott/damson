package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// SketchfabAdapter fetches model metadata from Sketchfab's official,
// unauthenticated Data API v3.
type SketchfabAdapter struct{}

const sketchfabAPIBase = "https://api.sketchfab.com/v3/models/"

// sketchfabUIDRE matches the 32-hex model uid Sketchfab appends to slugs and
// uses as the bare /models/<uid> form.
var sketchfabUIDRE = regexp.MustCompile(`[0-9a-fA-F]{32}`)

func (SketchfabAdapter) Store() string { return StoreSketchfab }

// Fetch resolves the model uid from the listing URL and reads the Data API.
func (SketchfabAdapter) Fetch(ctx context.Context, client *Client, rawURL string) (*Metadata, error) {
	uid := sketchfabUID(rawURL)
	if uid == "" {
		return nil, fmt.Errorf("sketchfab: no model uid in %q", rawURL)
	}
	resp, err := client.Get(ctx, sketchfabAPIBase+uid, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("sketchfab: api status %d for %s", resp.StatusCode, uid)
	}
	return parseSketchfabModel(resp.Body, rawURL)
}

// sketchfabUID pulls the model uid — the last 32-hex token in the URL.
func sketchfabUID(rawURL string) string {
	matches := sketchfabUIDRE.FindAllString(rawURL, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.ToLower(matches[len(matches)-1])
}

func parseSketchfabModel(body []byte, listingURL string) (*Metadata, error) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("sketchfab: decode model: %w", err)
	}
	m := &Metadata{
		Store:      StoreSketchfab,
		ListingURL: listingURL,
		Raw:        json.RawMessage(body),
	}
	m.Title, _ = doc["name"].(string)
	m.Description, _ = doc["description"].(string)
	if uid, ok := doc["uid"].(string); ok {
		m.StoreID = uid
	}
	if user, ok := doc["user"].(map[string]any); ok {
		m.Publisher = firstNonEmpty(stringOf(user["displayName"]), stringOf(user["username"]))
	}
	if lic, ok := doc["license"].(map[string]any); ok {
		label := firstNonEmpty(stringOf(lic["fullName"]), stringOf(lic["label"]))
		slug := stringOf(lic["slug"])
		if spdx := firstNonEmpty(spdxFromSlug(slug), spdxFromLicenceURL(stringOf(lic["url"]))); spdx != "" {
			m.LicenceHint = spdx
		} else {
			m.LicenceHint = label
		}
	}
	m.CoverURL = sketchfabCover(doc)
	m.Tags = dedupeTags(append(sketchfabNames(doc["tags"]), sketchfabNames(doc["categories"])...))
	return m, nil
}

// sketchfabNames extracts the "name" of each object in a Sketchfab array field
// (tags and categories are both arrays of {name, slug, ...}).
func sketchfabNames(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		if obj, ok := it.(map[string]any); ok {
			if name := stringOf(obj["name"]); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// sketchfabCover picks the largest thumbnail image the API offers.
func sketchfabCover(doc map[string]any) string {
	thumbs, ok := doc["thumbnails"].(map[string]any)
	if !ok {
		return ""
	}
	images, ok := thumbs["images"].([]any)
	if !ok {
		return ""
	}
	best, bestW := "", -1.0
	for _, it := range images {
		img, ok := it.(map[string]any)
		if !ok {
			continue
		}
		w, _ := img["width"].(float64)
		if url := stringOf(img["url"]); url != "" && w > bestW {
			best, bestW = url, w
		}
	}
	return best
}

func stringOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
