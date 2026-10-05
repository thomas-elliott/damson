package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"html"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure Go, so the CLI still builds with CGO_ENABLED=0

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// purchasePagesSQL selects the Package Manager purchase-list pages the Unity
// Editor cached when My Assets was loaded. It reads only the URL and body
// tables: the cached response headers can hold a Unity access token.
const purchasePagesSQL = `SELECT r.url, d.response_data
	FROM curl_cache_response r JOIN curl_cache_response_data d ON d.response_id = r.id
	WHERE r.url LIKE 'https://packages-v2.unity.com/-/api/purchases?%'`

// purchasePage is one cached page of the purchase list. id is the purchase's
// own number; packageId is the Asset Store listing ID.
type purchasePage struct {
	Total   *int `json:"total"`
	Results []struct {
		ID          string      `json:"id"`
		PackageID   json.Number `json:"packageId"`
		DisplayName string      `json:"displayName"`
	} `json:"results"`
}

// defaultUnityCachePath is where the Editor keeps its HTTP cache on macOS.
// Elsewhere it hasn't been checked, so --cache is required there.
func defaultUnityCachePath() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", axi.Usagef([]string{"dmsn purchases read --cache <path to CurlRequestCache.db> --out purchases.tsv"},
			"the Unity Editor cache's location is only known on macOS; give it with --cache")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", axi.Wrap(axi.CodeIO, err, "cannot find the home folder")
	}
	return filepath.Join(home, "Library", "Caches", "com.unity3d.UnityEditor", "CurlRequestCache.db"), nil
}

// readCachedPurchases merges every cached purchase-list page by purchase. The
// list's total comes only from unfiltered pages: a page cached by a search in
// My Assets (query=) or a lookup (ids=) counts only what it matched.
func readCachedPurchases(ctx context.Context, cachePath string) (purchases []purchase, total int, err error) {
	if _, err := os.Stat(cachePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, axi.Errorf(axi.CodeNotFound,
				"no Unity Editor cache at %s; open My Assets in Package Manager, or give --cache", cachePath)
		}
		return nil, 0, axi.Wrap(axi.CodeIO, err, "cannot read %s", cachePath)
	}
	// Read-only, so the Editor's own cache is never changed.
	db, err := sql.Open("sqlite", "file:"+cachePath+"?mode=ro")
	if err != nil {
		return nil, 0, axi.Wrap(axi.CodeIO, err, "cannot open %s", cachePath)
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, purchasePagesSQL)
	if err != nil {
		return nil, 0, axi.Wrap(axi.CodeIO, err, "cannot read the purchase list from %s: %v", cachePath, err)
	}
	defer rows.Close()

	byID := map[string]purchase{}
	total = -1
	for rows.Next() {
		var (
			rawURL string
			body   []byte
		)
		if err := rows.Scan(&rawURL, &body); err != nil {
			return nil, 0, axi.Wrap(axi.CodeIO, err, "cannot read %s: %v", cachePath, err)
		}
		var page purchasePage
		if err := json.Unmarshal(body, &page); err != nil {
			// One unreadable page leaves the rest usable; the shortfall shows it.
			continue
		}
		for _, r := range page.Results {
			pkg := r.PackageID.String()
			name := strings.TrimSpace(r.DisplayName)
			if pkg == "" || name == "" {
				continue
			}
			byID[firstNonEmpty(r.ID, pkg)] = purchase{PackageID: pkg, Name: name, Link: unityListingLink(pkg)}
		}
		if page.Total != nil && !filteredPurchasePage(rawURL) && *page.Total > total {
			total = *page.Total
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, axi.Wrap(axi.CodeIO, err, "cannot read %s: %v", cachePath, err)
	}
	if total < 0 {
		return nil, 0, axi.Errorf(axi.CodeNotFound,
			"the Unity Editor cache holds no purchase list; open My Assets in Package Manager and choose Load all")
	}

	for _, p := range byID {
		purchases = append(purchases, p)
	}
	sort.Slice(purchases, func(i, j int) bool {
		return strings.ToLower(purchases[i].Name) < strings.ToLower(purchases[j].Name)
	})
	return purchases, total, nil
}

// filteredPurchasePage reports whether a cached page URL was a search or a
// lookup rather than the plain list. The Editor stores its URLs with & written
// as &amp;.
func filteredPurchasePage(rawURL string) bool {
	u, err := url.Parse(html.UnescapeString(rawURL))
	if err != nil {
		return true
	}
	q := u.Query()
	return q.Get("query") != "" || q.Get("ids") != ""
}

func runPurchasesRead(ctx context.Context, e *Env, f *purchasesFlags) error {
	if len(e.Args) != 1 || strings.TrimSpace(f.out) == "" {
		return axi.Usagef([]string{"dmsn purchases read --out purchases.tsv"}, "purchases read takes --out and no arguments")
	}
	cachePath := f.cache
	if cachePath == "" {
		var err error
		if cachePath, err = defaultUnityCachePath(); err != nil {
			return err
		}
	}
	purchases, total, err := readCachedPurchases(ctx, cachePath)
	if err != nil {
		return err
	}
	lines := []string{tsvLine(purchaseColumns...)}
	for _, p := range purchases {
		lines = append(lines, tsvLine(p.PackageID, p.Name, p.Link))
	}
	if err := os.WriteFile(f.out, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return axi.Wrap(axi.CodeIO, err, "cannot write %s", f.out)
	}

	shortfall := max(total-len(purchases), 0)
	var help []string
	if shortfall > 0 {
		help = []string{"in the Unity Editor, open Package Manager > My Assets, choose Load all, then read again"}
	}
	return e.Block("purchases", []toon.KV{
		{Key: "out", Value: f.out},
		{Key: "cache", Value: cachePath},
		{Key: "found", Value: len(purchases)},
		{Key: "total", Value: total},
		{Key: "shortfall", Value: shortfall},
	}, help)
}
