package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
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
	cands, _ := r.SearchStores(ctx, query)
	return cands, nil
}

// SearchStores is Search that also names, sorted, each store that errored or
// timed out, so a caller can tell "no listing" from "the store didn't answer".
func (r *Registry) SearchStores(ctx context.Context, query string) ([]Candidate, []string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	type result struct {
		store string
		cands []Candidate
		err   error
	}
	out := make(chan result, len(r.searchers))
	var wg sync.WaitGroup
	for _, s := range r.searchers {
		wg.Add(1)
		go func(s Searcher) {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ctx, perStoreSearchTimeout)
			defer cancel()
			cands, err := s.Search(sctx, r.client, query)
			if err != nil {
				slog.Warn("metadata: store search failed", "store", s.Store(), "err", err)
			}
			if len(cands) > maxCandidatesPerStore {
				cands = cands[:maxCandidatesPerStore]
			}
			out <- result{store: s.Store(), cands: cands, err: err}
		}(s)
	}
	wg.Wait()
	close(out)

	var merged []Candidate
	var failed []string
	for res := range out {
		if res.err != nil {
			failed = append(failed, res.store)
			continue
		}
		merged = append(merged, res.cands...)
	}
	sort.Strings(failed)
	return merged, failed
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
	doc, err := html.Parse(bytes.NewReader(body))
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
