package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const perStoreSearchTimeout = 8 * time.Second

// maxCandidatesPerStore caps how many hits each store contributes to the
// merged list, keeping the picker scannable.
const maxCandidatesPerStore = 12

// Search fans out the query to every registered searcher concurrently, each
// under its own short timeout, and merges the results. A store that errors or
// times out contributes nothing rather than failing the whole search.
func (r *Registry) Search(ctx context.Context, query string) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	var wg sync.WaitGroup
	out := make(chan []Candidate, len(r.searchers))
	for _, s := range r.searchers {
		wg.Add(1)
		go func(s Searcher) {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ctx, perStoreSearchTimeout)
			defer cancel()
			cands, err := s.Search(sctx, r.client, query)
			if err != nil {
				slog.Warn("metadata: store search failed", "store", s.Store(), "err", err)
				return
			}
			if len(cands) > maxCandidatesPerStore {
				cands = cands[:maxCandidatesPerStore]
			}
			out <- cands
		}(s)
	}
	wg.Wait()
	close(out)

	var merged []Candidate
	for cs := range out {
		merged = append(merged, cs...)
	}
	return merged, nil
}

// ---------------- Sketchfab search -----------------------------------------

const sketchfabSearchURL = "https://api.sketchfab.com/v3/search?type=models&q="

// Search queries Sketchfab's official search API.
func (SketchfabAdapter) Search(ctx context.Context, client *Client, query string) ([]Candidate, error) {
	resp, err := client.Get(ctx, sketchfabSearchURL+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("sketchfab: search status %d", resp.StatusCode)
	}
	return parseSketchfabSearch(resp.Body), nil
}

func parseSketchfabSearch(body []byte) []Candidate {
	var doc struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}
	var out []Candidate
	for _, res := range doc.Results {
		uid := stringOf(res["uid"])
		listing := stringOf(res["viewerUrl"])
		if listing == "" && uid != "" {
			listing = "https://sketchfab.com/models/" + uid
		}
		c := Candidate{
			Title:      stringOf(res["name"]),
			CoverURL:   sketchfabCover(res),
			ListingURL: listing,
			Store:      StoreSketchfab,
			StoreID:    uid,
		}
		if user, ok := res["user"].(map[string]any); ok {
			c.Publisher = firstNonEmpty(stringOf(user["displayName"]), stringOf(user["username"]))
		}
		if c.Title != "" && c.ListingURL != "" {
			out = append(out, c)
		}
	}
	return out
}

// ---------------- Unity Coveo search ---------------------------------------

const (
	unityCoveoTokenURL  = "https://assetstore.unity.com/api/coveo/search-token?searchHub=Assetstore_Search"
	unityCoveoSearchURL = "https://unitytechnologiesproductionmkahteav.org.coveo.com/rest/search/v2?organizationId=unitytechnologiesproductionmkahteav"
)

// Search queries the Unity Asset Store via its anonymous Coveo endpoint: fetch
// a search token, then POST the query.
func (UnityAdapter) Search(ctx context.Context, client *Client, query string) ([]Candidate, error) {
	tokResp, err := client.Get(ctx, unityCoveoTokenURL, nil)
	if err != nil {
		return nil, err
	}
	if tokResp.StatusCode >= 400 {
		return nil, fmt.Errorf("unity: coveo token status %d", tokResp.StatusCode)
	}
	var tok struct {
		Token       string `json:"token"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(tokResp.Body, &tok); err != nil {
		return nil, fmt.Errorf("unity: decode coveo token: %w", err)
	}
	token := firstNonEmpty(tok.Token, tok.AccessToken)
	if token == "" {
		return nil, fmt.Errorf("unity: empty coveo token")
	}
	payload, _ := json.Marshal(map[string]any{
		"q":               query,
		"searchHub":       "Assetstore_Search",
		"numberOfResults": maxCandidatesPerStore,
	})
	resp, err := client.Do(ctx, "POST", unityCoveoSearchURL, strings.NewReader(string(payload)), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unity: coveo search status %d", resp.StatusCode)
	}
	return parseCoveoResults(resp.Body), nil
}

func parseCoveoResults(body []byte) []Candidate {
	var doc struct {
		Results []struct {
			ClickUri string         `json:"clickUri"`
			Title    string         `json:"title"`
			Raw      map[string]any `json:"raw"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}
	var out []Candidate
	for _, res := range doc.Results {
		c := Candidate{
			Title:      firstNonEmpty(stringOf(res.Raw["ec_name"]), res.Title),
			Publisher:  stringOf(res.Raw["publisher_name"]),
			Price:      numOrString(res.Raw["ec_price"]),
			CoverURL:   coveoImage(res.Raw["ec_images"]),
			ListingURL: res.ClickUri,
			Store:      StoreUnity,
			StoreID:    numOrString(res.Raw["ec_product_id"]),
		}
		if c.Title != "" && c.ListingURL != "" {
			out = append(out, c)
		}
	}
	return out
}

// coveoImage pulls the first image URL from Coveo's ec_images, which may be a
// JSON array or a JSON-encoded string.
func coveoImage(v any) string {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if strings.HasPrefix(s, "[") {
			var arr []string
			if json.Unmarshal([]byte(s), &arr) == nil && len(arr) > 0 {
				return arr[0]
			}
		}
		return s
	case []any:
		for _, item := range t {
			if u := stringOf(item); u != "" {
				return u
			}
		}
	}
	return ""
}

// ---------------- itch.io search -------------------------------------------

const itchSearchURL = "https://itch.io/search?q="

// Search scrapes itch.io's HTML search results.
func (ItchAdapter) Search(ctx context.Context, client *Client, query string) ([]Candidate, error) {
	resp, err := client.Get(ctx, itchSearchURL+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("itch: search status %d", resp.StatusCode)
	}
	return parseItchSearch(resp.Body), nil
}

func parseItchSearch(body []byte) []Candidate {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	var out []Candidate
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && classContains(n, "game_cell") {
			if c, ok := itchCell(n); ok && !seen[c.ListingURL] {
				seen[c.ListingURL] = true
				out = append(out, c)
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	return out
}

func itchCell(cell *html.Node) (Candidate, bool) {
	c := Candidate{Store: StoreItch}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "a" && classContains(n, "title"):
				c.Title = strings.TrimSpace(textOf(n))
				c.ListingURL = attrVal(n, "href")
			case n.Data == "div" && classContains(n, "game_author"):
				if c.Publisher == "" {
					c.Publisher = strings.TrimSpace(textOf(n))
				}
			case (n.Data == "img") && c.CoverURL == "":
				if src := attrVal(n, "src"); src != "" {
					c.CoverURL = src
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(cell)
	return c, c.Title != "" && c.ListingURL != ""
}

func classContains(n *html.Node, want string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, cls := range strings.Fields(a.Val) {
				if cls == want {
					return true
				}
			}
		}
	}
	return false
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
