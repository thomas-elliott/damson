package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/axi"
)

// writeUnityCache builds a cache file with the Unity Editor's schema, holding
// one cached response per url → body. URLs are stored as the Editor stores
// them, with & written as &amp;.
func writeUnityCache(t *testing.T, pages map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "CurlRequestCache.db")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE curl_cache_response ( id integer primary key autoincrement not null, url text unique not null,
			content_size integer not null default(0), time_stamp not null default current_timestamp,
			expires not null default current_timestamp )`,
		`CREATE TABLE curl_cache_response_data ( response_id integer primary key not null, response_data blob not null,
			foreign key(response_id) references curl_cache_response(id) on delete cascade)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	for u, body := range pages {
		res, err := db.Exec(`INSERT INTO curl_cache_response (url) VALUES (?)`, strings.ReplaceAll(u, "&", "&amp;"))
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		if _, err := db.Exec(`INSERT INTO curl_cache_response_data VALUES (?, ?)`, id, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

const purchasesURL = "https://packages-v2.unity.com/-/api/purchases?"

var (
	firstPage = `{"total":3,"results":[
	  {"id":"9001","packageId":11020,"displayName":"Toony Skies","grantTime":"2020-01-01T00:00:00Z"},
	  {"id":"9002","packageId":200,"displayName":"Free Pack"}]}`
	restPage = `{"total":3,"results":[{"id":"9003","packageId":300,"displayName":"After Load All"}]}`
	// A search in My Assets caches a page whose total is only what it matched.
	searchPage = `{"total":1,"results":[{"id":"9001","packageId":11020,"displayName":"Toony Skies"}]}`
)

// After Load all, every purchase is found once, however many cached pages
// repeat it, and each is written with its listing ID and a store link.
func TestPurchasesRead_FindsTheWholeListAfterLoadAll(t *testing.T) {
	isolate(t)
	cache := writeUnityCache(t, map[string]string{
		purchasesURL + "offset=0&limit=20&orderBy=name&order=asc":             firstPage,
		purchasesURL + "offset=20&limit=409&orderBy=name&order=asc":           restPage,
		purchasesURL + "offset=0&limit=20&query=toony&orderBy=name&order=asc": searchPage,
		"https://packages-v2.unity.com/-/api/product/11020":                   `{"productPublisher":"x"}`,
	})
	out := filepath.Join(t.TempDir(), "purchases.tsv")

	got, code := exec(t, "purchases", "read", "--cache", cache, "--out", out)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, got)
	}
	for _, s := range []string{"found: 3", "total: 3", "shortfall: 0"} {
		if !strings.Contains(got, s) {
			t.Errorf("output lacks %q:\n%s", s, got)
		}
	}
	purchases, err := readPurchases(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []purchase{
		{"300", "After Load All", unityListingLink("300")},
		{"200", "Free Pack", unityListingLink("200")},
		{"11020", "Toony Skies", unityListingLink("11020")},
	}
	if len(purchases) != len(want) {
		t.Fatalf("purchases = %+v, want %+v", purchases, want)
	}
	for i := range want {
		if purchases[i] != want[i] {
			t.Errorf("purchase %d = %+v, want %+v", i, purchases[i], want[i])
		}
	}
}

// Before Load all, only the first page is cached: the shortfall is reported.
func TestPurchasesRead_ReportsTheShortfall(t *testing.T) {
	isolate(t)
	cache := writeUnityCache(t, map[string]string{
		purchasesURL + "offset=0&limit=20&orderBy=name&order=asc": firstPage,
	})
	got, code := exec(t, "purchases", "read", "--cache", cache, "--out", filepath.Join(t.TempDir(), "p.tsv"))
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, got)
	}
	for _, s := range []string{"found: 2", "total: 3", "shortfall: 1", "Load all"} {
		if !strings.Contains(got, s) {
			t.Errorf("output lacks %q:\n%s", s, got)
		}
	}
}

// A search's total counts only its matches, so a cache holding nothing but a
// search has no list total to measure against, and says so rather than
// reporting a complete list of one.
func TestPurchasesRead_IgnoresASearchsTotal(t *testing.T) {
	isolate(t)
	cache := writeUnityCache(t, map[string]string{
		purchasesURL + "offset=0&limit=20&query=toony&orderBy=name&order=asc": searchPage,
	})
	got, code := exec(t, "purchases", "read", "--cache", cache, "--out", filepath.Join(t.TempDir(), "p.tsv"))
	if code == axi.ExitOK || !strings.Contains(got, "holds no purchase list") {
		t.Fatalf("exit = %d\n%s", code, got)
	}
}
