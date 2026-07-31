package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// GenericFetcher scrapes Open Graph tags + JSON-LD Product data from any
// listing page. It is the base fetcher and the fallback every store adapter
// degrades to.
type GenericFetcher struct{}

// Fetch retrieves rawURL and parses generic listing metadata from the HTML.
func (GenericFetcher) Fetch(ctx context.Context, client *Client, rawURL string) (*Metadata, error) {
	resp, err := client.Get(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("metadata: fetch %s: status %d", rawURL, resp.StatusCode)
	}
	meta := ParseHTML(resp.Body, resp.FinalURL)
	if meta.ListingURL == "" {
		meta.ListingURL = resp.FinalURL
	}
	if meta.Store == "" {
		meta.Store = StoreGeneric
	}
	return meta, nil
}

// ParseHTML extracts a Metadata from a listing page's HTML. pageURL is used to
// resolve relative image URLs. It never returns nil — an unparseable page
// yields a zero-ish Metadata the caller can treat as "nothing found".
func ParseHTML(data []byte, pageURL string) *Metadata {
	m := &Metadata{}
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return m
	}

	metaTags := map[string]string{}
	var titleTag string
	var jsonLD []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				key, content := "", ""
				for _, a := range n.Attr {
					switch strings.ToLower(a.Key) {
					case "property", "name":
						if key == "" {
							key = strings.ToLower(a.Val)
						}
					case "content":
						content = a.Val
					}
				}
				if key != "" && content != "" {
					if _, seen := metaTags[key]; !seen {
						metaTags[key] = content
					}
				}
			case "title":
				if titleTag == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					titleTag = strings.TrimSpace(n.FirstChild.Data)
				}
			case "script":
				if attrEquals(n, "type", "application/ld+json") && n.FirstChild != nil {
					jsonLD = append(jsonLD, n.FirstChild.Data)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	product := firstProduct(jsonLD)

	m.Title = firstNonEmpty(metaTags["og:title"], metaTags["twitter:title"], stringField(product, "name"), titleTag)
	m.Description = firstNonEmpty(metaTags["og:description"], metaTags["twitter:description"], metaTags["description"], stringField(product, "description"))
	m.Publisher = firstNonEmpty(metaTags["og:site_name"], brandName(product), sellerName(product))
	m.CoverURL = resolveURL(pageURL, firstNonEmpty(metaTags["og:image"], metaTags["twitter:image"], imageField(product)))

	if product != nil {
		price, currency := offerPrice(product)
		m.Price = price
		m.Currency = currency
		m.LicenceHint = stringField(product, "license")
		m.PublisherURL = resolveURL(pageURL, publisherURLField(product))
		m.ReleasedAt = parseTimestamp(stringField(product, "datePublished"))
		m.StoreUpdatedAt = parseTimestamp(stringField(product, "dateModified"))
		m.Rating, m.RatingCount = aggregateRating(product)
	}
	if m.ReleasedAt == nil {
		m.ReleasedAt = parseTimestamp(firstNonEmpty(metaTags["article:published_time"], metaTags["og:article:published_time"]))
	}
	m.Tags = dedupeTags(append(keywordsField(product), metaKeywords(metaTags)...))
	return m
}

// publisherURLField reads the URL of the JSON-LD brand or offer seller, so a
// generic listing still links out to whoever made the asset.
func publisherURLField(product map[string]any) string {
	for _, node := range []any{product["brand"], product["offers"], product["publisher"], product["author"]} {
		switch v := node.(type) {
		case map[string]any:
			if u := stringOf(v["url"]); u != "" {
				return u
			}
			if seller, ok := v["seller"].(map[string]any); ok {
				if u := stringOf(seller["url"]); u != "" {
					return u
				}
			}
		}
	}
	return ""
}

// aggregateRating reads a JSON-LD aggregateRating node. Both members arrive as
// either a number or a numeric string depending on the site.
func aggregateRating(product map[string]any) (float64, int) {
	node, ok := product["aggregateRating"].(map[string]any)
	if !ok {
		return 0, 0
	}
	return numberOf(node["ratingValue"]), int(numberOf(node["ratingCount"]))
}

// numberOf reads a JSON value that may be a number or a numeric string.
func numberOf(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0
		}
		return f
	}
	return 0
}

// timestampLayouts covers the shapes the stores use: RFC 3339, the naive
// local-time form Sketchfab returns, plain dates, and itch's "@"-separated
// abbr title. A fractional second is accepted even where the layout omits it.
var timestampLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
	"2 January 2006 @ 15:04 MST",
	"2 January 2006 @ 15:04",
	"2 January 2006",
	"January 2, 2006",
}

// parseTimestamp reads a store timestamp in any of the layouts the adapters
// encounter. Returns nil for a blank or unparseable value, so an absent date
// stays absent rather than becoming the zero time.
func parseTimestamp(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			utc := t.UTC()
			return &utc
		}
	}
	return nil
}

// keywordsField reads a JSON-LD "keywords" value, which may be a
// comma-separated string or an array of strings.
func keywordsField(node map[string]any) []string {
	if node == nil {
		return nil
	}
	switch v := node["keywords"].(type) {
	case string:
		return splitKeywords(v)
	case []any:
		var out []string
		for _, item := range v {
			if s := stringOf(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// metaKeywords pulls tags from the <meta name="keywords"> content and any
// repeated article:tag values collapsed into metaTags (only the first survives
// dedup, so this is best-effort for the common single-tag case).
func metaKeywords(metaTags map[string]string) []string {
	var out []string
	out = append(out, splitKeywords(metaTags["keywords"])...)
	out = append(out, splitKeywords(metaTags["article:tag"])...)
	return out
}

func splitKeywords(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// dedupeTags trims, drops blanks, and removes case-insensitive duplicates while
// keeping the first occurrence's original casing and input order.
func dedupeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		k := strings.ToLower(t)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func attrEquals(n *html.Node, key, val string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.EqualFold(strings.TrimSpace(a.Val), val)
		}
	}
	return false
}

// firstProduct scans JSON-LD blocks for a node whose @type is (or includes)
// Product, descending into @graph arrays and top-level arrays.
func firstProduct(blocks []string) map[string]any {
	for _, b := range blocks {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(b), &v); err != nil {
			continue
		}
		if p := findProduct(v); p != nil {
			return p
		}
	}
	return nil
}

func findProduct(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if hasType(t, "Product") {
			return t
		}
		if graph, ok := t["@graph"]; ok {
			if p := findProduct(graph); p != nil {
				return p
			}
		}
	case []any:
		for _, item := range t {
			if p := findProduct(item); p != nil {
				return p
			}
		}
	}
	return nil
}

func hasType(node map[string]any, want string) bool {
	switch t := node["@type"].(type) {
	case string:
		return strings.EqualFold(t, want)
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && strings.EqualFold(s, want) {
				return true
			}
		}
	}
	return false
}

func stringField(node map[string]any, key string) string {
	if node == nil {
		return ""
	}
	switch v := node[key].(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return ""
}

func brandName(node map[string]any) string {
	if node == nil {
		return ""
	}
	return stringField(node, "brand")
}

func sellerName(node map[string]any) string {
	if node == nil {
		return ""
	}
	if offers, ok := node["offers"].(map[string]any); ok {
		return stringField(offers, "seller")
	}
	return ""
}

func imageField(node map[string]any) string {
	if node == nil {
		return ""
	}
	switch v := node["image"].(type) {
	case string:
		return v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
			if m, ok := item.(map[string]any); ok {
				if u, ok := m["url"].(string); ok {
					return u
				}
			}
		}
	case map[string]any:
		if u, ok := v["url"].(string); ok {
			return u
		}
	}
	return ""
}

// offerPrice pulls price + currency from an "offers" node that may be a single
// object or an array of offers.
func offerPrice(node map[string]any) (string, string) {
	offers := node["offers"]
	switch v := offers.(type) {
	case map[string]any:
		return priceFromOffer(v)
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if p, cur := priceFromOffer(m); p != "" {
					return p, cur
				}
			}
		}
	}
	return "", ""
}

func priceFromOffer(offer map[string]any) (string, string) {
	var price string
	switch p := offer["price"].(type) {
	case string:
		price = p
	case float64:
		price = strings.TrimSuffix(strings.TrimRight(formatFloat(p), "0"), ".")
	}
	currency, _ := offer["priceCurrency"].(string)
	return price, currency
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// resolveURL turns a possibly-relative asset URL into an absolute one against
// the page URL. Returns the raw value if either URL fails to parse.
func resolveURL(pageURL, ref string) string {
	if ref == "" {
		return ""
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(u).String()
}
