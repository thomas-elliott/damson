package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// UnityAdapter fetches Unity Asset Store metadata via the anonymous Coveo
// search API — the same source that powers the store's own search. It is used
// for both name search and fetch-by-URL (looking a product up by its id).
//
// The store's package pages are client-rendered (no server-side OG/JSON-LD to
// scrape) and the GraphQL endpoint requires a CSRF handshake the public site
// doesn't expose, so Coveo is the one reliable unauthenticated source.
type UnityAdapter struct{}

const (
	unityHost           = "assetstore.unity.com"
	unityCoveoTokenURL  = "https://assetstore.unity.com/api/coveo/search-token?searchHub=Assetstore_Search"
	unityCoveoSearchURL = "https://unitytechnologiesproductionmkahteav.org.coveo.com/rest/search/v2?organizationId=unitytechnologiesproductionmkahteav"
)

// coveoFields are the raw fields we ask Coveo to return; a narrow set keeps
// the payload small and the mapping stable.
var coveoFields = []string{
	"ec_name", "ec_price", "publisher_name", "publisher_id", "ec_product_id",
	"ec_images", "ec_thumbnails", "ec_shortdesc", "ec_description",
	"ec_category_level3", "ec_keywords",
	"first_published_at", "last_published_at", "min_unity_version",
	"ec_rating", "ec_rating_count",
}

// unityProductIDRE captures the trailing numeric id Unity uses, in both the
// /packages/package/<id> and /packages/<cat>/slug-<id> URL forms.
var unityProductIDRE = regexp.MustCompile(`(\d+)/?$`)

func (UnityAdapter) Store() string { return StoreUnity }

// Fetch resolves the product id from the listing URL and looks it up in Coveo,
// then reads the listing page for the screenshot gallery Coveo omits.
func (UnityAdapter) Fetch(ctx context.Context, client *Client, rawURL string) (*Metadata, error) {
	id := unityProductID(rawURL)
	if id == "" {
		return nil, fmt.Errorf("unity: no product id in %q", rawURL)
	}
	resp, err := unityCoveo(ctx, client, map[string]any{
		"aq":                "@ec_product_id==" + id,
		"enableQuerySyntax": true,
		"numberOfResults":   1,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("unity: no product %s in coveo", id)
	}
	md := coveoMetadata(resp.Results[0], rawURL)
	listing, pageURL := unityFetchListing(ctx, client, firstNonEmpty(md.ListingURL, rawURL), id)
	unityApplyListing(md, listing, firstNonEmpty(pageURL, md.ListingURL, rawURL))
	return md, nil
}

// unityGalleryLimit caps how many listing screenshots one fetch carries.
const unityGalleryLimit = 24

// unityProductImage is one entry of the listing page's product image array.
type unityProductImage struct {
	Type         string `json:"type"`
	ImageURL     string `json:"imageUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

// unityGalleryURLs resolves, filters and dedupes product image entries. Video
// entries (a YouTube embed is listed alongside the screenshots) are dropped —
// their imageUrl isn't an image.
func unityGalleryURLs(imgs []unityProductImage, pageURL string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(imgs))
	for _, img := range imgs {
		u := resolveURL(pageURL, strings.TrimSpace(img.ImageURL))
		if u == "" || seen[u] || !isStillImageURL(u) {
			continue
		}
		seen[u] = true
		out = append(out, u)
		if len(out) == unityGalleryLimit {
			break
		}
	}
	return out
}

// Search queries the Unity Asset Store via Coveo.
func (UnityAdapter) Search(ctx context.Context, client *Client, query string) ([]Candidate, error) {
	resp, err := unityCoveo(ctx, client, map[string]any{
		"q":                 query,
		"enableQuerySyntax": false,
		"numberOfResults":   maxCandidatesPerStore,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(resp.Results))
	for _, r := range resp.Results {
		c := coveoCandidate(r)
		if c.Title != "" && c.ListingURL != "" {
			out = append(out, c)
		}
	}
	return out, nil
}

type coveoResult struct {
	ClickURI string         `json:"clickUri"`
	Title    string         `json:"title"`
	Raw      map[string]any `json:"raw"`
}

type coveoResponse struct {
	Results []coveoResult `json:"results"`
}

// unityCoveo fetches an anonymous search token, then POSTs a Coveo query. The
// base body carries the context + field set the store's pipeline requires (a
// query without the assetstore context is filtered to zero results); callers
// supply q or aq plus paging via extra.
func unityCoveo(ctx context.Context, client *Client, extra map[string]any) (*coveoResponse, error) {
	tokResp, err := client.Get(ctx, unityCoveoTokenURL, nil)
	if err != nil {
		return nil, err
	}
	if tokResp.StatusCode >= 400 {
		return nil, fmt.Errorf("unity: coveo token status %d", tokResp.StatusCode)
	}
	// The token endpoint returns a bare JSON string, not an object.
	var token string
	if err := json.Unmarshal(tokResp.Body, &token); err != nil || token == "" {
		return nil, fmt.Errorf("unity: unreadable coveo token")
	}

	body := map[string]any{
		"searchHub":         "Assetstore_Search",
		"enableQuerySyntax": false,
		"sortCriteria":      "relevancy",
		"fieldsToInclude":   coveoFields,
		"context": map[string]any{
			"website":        "assetstore",
			"language":       "en-US",
			"engineUniqueId": "Assetstore_Search",
			"userGroups":     []string{"assetStoreUsers"},
		},
	}
	for k, v := range extra {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(ctx, "POST", unityCoveoSearchURL, bytes.NewReader(payload), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + token,
		"Origin":        "https://" + unityHost,
		"Referer":       "https://" + unityHost + "/",
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unity: coveo search status %d", resp.StatusCode)
	}
	var out coveoResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("unity: decode coveo response: %w", err)
	}
	return &out, nil
}

func coveoCandidate(r coveoResult) Candidate {
	raw := r.Raw
	return Candidate{
		Title:      stringOf(raw["ec_name"]),
		Publisher:  stringOf(raw["publisher_name"]),
		Price:      numOrString(raw["ec_price"]),
		Currency:   "USD",
		CoverURL:   coveoCover(raw),
		ListingURL: r.ClickURI,
		Store:      StoreUnity,
		StoreID:    firstArrayString(raw["ec_product_id"]),
	}
}

func coveoMetadata(r coveoResult, listingURL string) *Metadata {
	raw := r.Raw
	rawJSON, _ := json.Marshal(r)
	cover := coveoCover(raw)
	return &Metadata{
		Store:            StoreUnity,
		Title:            stringOf(raw["ec_name"]),
		Description:      htmlToMarkdown(firstNonEmpty(stringOf(raw["ec_description"]), stringOf(raw["ec_shortdesc"]))),
		Publisher:        stringOf(raw["publisher_name"]),
		PublisherURL:     coveoPublisherURL(raw),
		Price:            numOrString(raw["ec_price"]),
		Currency:         "USD",
		CoverURL:         cover,
		ImageURLs:        coveoImages(raw, cover),
		Tags:             coveoTags(raw),
		CategoryPath:     coveoCategoryPath(raw),
		StoreID:          firstNonEmpty(firstArrayString(raw["ec_product_id"]), unityProductID(listingURL)),
		ListingURL:       firstNonEmpty(r.ClickURI, listingURL),
		Engine:           "unity",
		MinEngineVersion: unityMinVersionLabel(raw["min_unity_version"]),
		ReleasedAt:       coveoEpochTime(raw["first_published_at"]),
		StoreUpdatedAt:   coveoEpochTime(raw["last_published_at"]),
		Rating:           coveoFloat(raw["ec_rating"]),
		RatingCount:      int(coveoFloat(raw["ec_rating_count"])),
		// Coveo exposes no per-asset licence field; Asset Store assets carry
		// the standard EULA.
		LicenceHint: "unity-eula",
		Raw:         rawJSON,
	}
}

// coveoPublisherURL builds the store's publisher page from publisher_id. The
// listing payload's own publisher URL is preferred where it exists; this is the
// fallback when only Coveo answered.
func coveoPublisherURL(raw map[string]any) string {
	id := strings.TrimSpace(firstArrayString(raw["publisher_id"]))
	if id == "" {
		return ""
	}
	return "https://" + unityHost + "/publishers/" + id
}

// coveoCategoryPath turns ec_category_level3 ("3d > environments > landscapes")
// into a single hierarchical tag path ("3d/environments/landscapes"). The
// ec_category field is deliberately not used: it arrives as three entries, each
// with the arrow separators and a leading space, which yields three malformed
// flat tags instead of one path. Falls back to the shallower levels.
func coveoCategoryPath(raw map[string]any) string {
	for _, key := range []string{"ec_category_level3", "ec_category_level2", "ec_category_level1"} {
		v := firstNonEmpty(coveoStringList(raw[key])...)
		if v == "" {
			continue
		}
		parts := strings.Split(v, ">")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return strings.Join(out, "/")
		}
	}
	return ""
}

// unityMinVersionLabel renders min_unity_version, a packed YYYYMM integer
// (202003), as the release label Unity itself uses (2020.3). Anything that
// isn't in that shape passes through as-is.
func unityMinVersionLabel(v any) string {
	s := strings.TrimSpace(numOrString(v))
	if len(s) < 5 {
		return s
	}
	minor := strings.TrimLeft(s[4:], "0")
	if minor == "" {
		minor = "0"
	}
	return s[:4] + "." + minor
}

// coveoEpochTime reads a Coveo millisecond timestamp. Returns nil for a missing
// or non-positive value so an absent date stays absent.
func coveoEpochTime(v any) *time.Time {
	ms := int64(coveoFloat(v))
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

// coveoFloat reads a numeric Coveo field, which may arrive as a number or as a
// string.
func coveoFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0
		}
		return f
	}
	return 0
}

// coveoCover picks the cover image, preferring the first ec_images entry that
// is a real still image over ec_thumbnails — the thumbnail field sometimes
// carries a video/stream URL, which renders broken as a cover. Falls back to
// ec_thumbnails, then to the first ec_images entry of any kind.
func coveoCover(raw map[string]any) string {
	imgs := coveoImageList(raw)
	for _, u := range imgs {
		if isStillImageURL(u) {
			return u
		}
	}
	if t := stringOf(raw["ec_thumbnails"]); t != "" {
		return t
	}
	if len(imgs) > 0 {
		return imgs[0]
	}
	return ""
}

// videoExts and videoHostMarkers identify URLs that are not still images, so
// they can be skipped when choosing a cover.
var videoExts = map[string]bool{
	".mp4": true, ".webm": true, ".mov": true, ".m4v": true,
	".avi": true, ".mkv": true, ".ogv": true,
}

var videoHostMarkers = []string{"youtube", "youtu.be", "vimeo", "/video/"}

// isStillImageURL reports whether a gallery URL looks like a still image
// rather than a video or streaming embed. Unknown shapes are treated as still
// images (the common case), so only recognised video markers are excluded.
func isStillImageURL(rawURL string) bool {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	if u == "" {
		return false
	}
	for _, m := range videoHostMarkers {
		if strings.Contains(u, m) {
			return false
		}
	}
	p := u
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	return !videoExts[path.Ext(p)]
}

// coveoImageList parses ec_images (a JSON array, a JSON-encoded string array,
// or a bare string) into an ordered list of URLs, without dedup or filtering.
func coveoImageList(raw map[string]any) []string {
	switch v := raw["ec_images"].(type) {
	case string:
		s := strings.TrimSpace(v)
		if strings.HasPrefix(s, "[") {
			var arr []string
			if json.Unmarshal([]byte(s), &arr) == nil {
				return arr
			}
		} else if s != "" {
			return []string{s}
		}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if u := stringOf(item); u != "" {
				out = append(out, u)
			}
		}
		return out
	}
	return nil
}

// coveoImages returns every gallery image URL from ec_images, deduplicated and
// with the cover excluded so the gallery doesn't repeat it.
func coveoImages(raw map[string]any, cover string) []string {
	urls := coveoImageList(raw)
	seen := map[string]bool{}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" || u == cover || seen[u] || !isStillImageURL(u) {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// coveoTags gathers the keyword tags from a Coveo result. The field may arrive
// as a bare string, a JSON-encoded string array, or a []any. Category is not
// included here — it becomes CategoryPath, a single hierarchical path.
func coveoTags(raw map[string]any) []string {
	return dedupeTags(coveoStringList(raw["ec_keywords"]))
}

// coveoStringList normalises a Coveo field into a slice of strings, handling
// the bare-string, JSON-encoded-array, and []any shapes Coveo uses.
func coveoStringList(v any) []string {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if strings.HasPrefix(s, "[") {
			var arr []string
			if json.Unmarshal([]byte(s), &arr) == nil {
				return arr
			}
		}
		if s != "" {
			return []string{s}
		}
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := stringOf(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// htmlToMarkdown converts an HTML fragment to Markdown, preserving structure
// (paragraphs, lists). It falls back to the tag-stripped plain text when the
// converter errors or yields nothing.
func htmlToMarkdown(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	md, err := htmltomarkdown.ConvertString(s)
	if err == nil {
		if md = strings.TrimSpace(md); md != "" {
			return md
		}
	}
	return stripTags(s)
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

func firstArrayString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) > 0 {
			return stringOf(t[0])
		}
	case float64:
		return numOrString(t)
	}
	return ""
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

// stripTags removes HTML tags and decodes entities (Unity descriptions are
// HTML with numeric entities).
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
	return strings.TrimSpace(html.UnescapeString(sb.String()))
}
