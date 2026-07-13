package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
)

// UnityAdapter fetches Unity Asset Store listing metadata. It prefers the
// unofficial GraphQL batch endpoint (richest data) and degrades to scraping
// the server-rendered page's OG/JSON-LD tags on any failure.
type UnityAdapter struct{}

const (
	unityGraphQLURL = "https://assetstore.unity.com/api/graphql/batch"
	unityHost       = "assetstore.unity.com"
)

// unityProductIDRE captures the trailing numeric id Unity appends to package
// slugs: /packages/3d/props/foo-bar-12345 -> 12345.
var unityProductIDRE = regexp.MustCompile(`-(\d+)/?$`)

// unityProductQuery requests only the fields we consume; a narrow query
// reduces the surface that can break when Unity changes its schema.
const unityProductQuery = `query Product($id: ID!) {
  product(id: $id) {
    id
    name
    description
    price { finalPrice originalPrice currency }
    productPublisher { name }
    mainImage { big }
    images { imageUrl type }
    customLicense
    licenseText
  }
}`

func (UnityAdapter) Store() string { return StoreUnity }

// Fetch tries GraphQL first, then falls back to scraping the listing page.
func (UnityAdapter) Fetch(ctx context.Context, client *Client, rawURL string) (*Metadata, error) {
	if m, err := fetchUnityGraphQL(ctx, client, rawURL); err == nil {
		return m, nil
	} else {
		slog.Warn("unity: graphql fetch failed, scraping page", "url", rawURL, "err", err)
	}
	return fetchUnityPage(ctx, client, rawURL)
}

// fetchUnityGraphQL bootstraps a CSRF token from the listing page, then POSTs
// the batch query with the token + cookies Unity's endpoint requires.
func fetchUnityGraphQL(ctx context.Context, client *Client, rawURL string) (*Metadata, error) {
	productID := unityProductID(rawURL)
	if productID == "" {
		return nil, fmt.Errorf("unity: no product id in %q", rawURL)
	}

	boot, err := client.Get(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	csrf := boot.Cookie("_csrf")
	if csrf == "" {
		return nil, fmt.Errorf("unity: no _csrf cookie from bootstrap")
	}

	batch := []map[string]any{{
		"operationName": "Product",
		"query":         unityProductQuery,
		"variables":     map[string]any{"id": productID},
	}}
	payload, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"Content-Type":     "application/json",
		"X-Csrf-Token":     csrf,
		"X-Requested-With": "XMLHttpRequest",
		"Cookie":           boot.CookieHeader(),
		"Origin":           "https://" + unityHost,
		"Referer":          rawURL,
	}
	resp, err := client.Do(ctx, "POST", unityGraphQLURL, bytes.NewReader(payload), headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unity: graphql status %d", resp.StatusCode)
	}
	return parseUnityGraphQL(resp.Body, rawURL)
}

func fetchUnityPage(ctx context.Context, client *Client, rawURL string) (*Metadata, error) {
	resp, err := client.Get(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unity: page status %d", resp.StatusCode)
	}
	m := ParseHTML(resp.Body, resp.FinalURL)
	m.Store = StoreUnity
	if m.ListingURL == "" {
		m.ListingURL = resp.FinalURL
	}
	if m.LicenceHint == "" && m.LicenceText == "" {
		m.LicenceHint = "unity-eula"
	}
	if id := unityProductID(rawURL); id != "" && m.StoreID == "" {
		m.StoreID = id
	}
	return m, nil
}

func unityProductID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if match := unityProductIDRE.FindStringSubmatch(u.Path); len(match) == 2 {
		return match[1]
	}
	return ""
}

// parseUnityGraphQL defensively extracts the product node from a batch
// response (an array of {data:{product}} objects) and maps its fields.
func parseUnityGraphQL(body []byte, listingURL string) (*Metadata, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unity: decode graphql: %w", err)
	}
	product := findMapWithKeys(raw, "name", "description")
	if product == nil {
		return nil, fmt.Errorf("unity: no product node in graphql response")
	}
	m := &Metadata{
		Store:      StoreUnity,
		ListingURL: listingURL,
		Raw:        json.RawMessage(body),
	}
	m.Title = stringOf(product["name"])
	m.Description = stripTags(stringOf(product["description"]))
	m.StoreID = firstNonEmpty(stringOf(product["id"]), unityProductID(listingURL))
	if pub, ok := product["productPublisher"].(map[string]any); ok {
		m.Publisher = stringOf(pub["name"])
	}
	if price, ok := product["price"].(map[string]any); ok {
		m.Price = numOrString(firstPresent(price, "finalPrice", "originalPrice"))
		m.Currency = stringOf(price["currency"])
	}
	m.CoverURL = unityCover(product)
	m.ImageURLs = unityImages(product)
	custom, _ := product["customLicense"].(bool)
	licenceText := stringOf(product["licenseText"])
	if custom && licenceText != "" {
		m.LicenceText = licenceText
	} else {
		m.LicenceHint = "unity-eula"
	}
	return m, nil
}

func unityCover(product map[string]any) string {
	if main, ok := product["mainImage"].(map[string]any); ok {
		if big := stringOf(main["big"]); big != "" {
			return big
		}
	}
	imgs := unityImages(product)
	if len(imgs) > 0 {
		return imgs[0]
	}
	return ""
}

func unityImages(product map[string]any) []string {
	arr, ok := product["images"].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, it := range arr {
		if img, ok := it.(map[string]any); ok {
			if u := stringOf(img["imageUrl"]); u != "" {
				out = append(out, u)
			}
		}
	}
	return out
}

// findMapWithKeys walks arbitrary decoded JSON and returns the first object
// that contains all of the required keys — used to locate the product node
// without hard-coding the batch envelope shape.
func findMapWithKeys(v any, keys ...string) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if hasAllKeys(t, keys) {
			return t
		}
		for _, child := range t {
			if found := findMapWithKeys(child, keys...); found != nil {
				return found
			}
		}
	case []any:
		for _, item := range t {
			if found := findMapWithKeys(item, keys...); found != nil {
				return found
			}
		}
	}
	return nil
}

func hasAllKeys(m map[string]any, keys []string) bool {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func numOrString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		s := formatFloat(t)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(s, "0")
			s = strings.TrimSuffix(s, ".")
		}
		return s
	}
	return ""
}

// stripTags removes HTML tags from a string (Unity descriptions are HTML).
func stripTags(s string) string {
	var sb strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				sb.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}
