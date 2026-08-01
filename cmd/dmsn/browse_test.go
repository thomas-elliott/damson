package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/thomas-elliott/damson/internal/axi"
)

const assetsBody = `{"assets":[
  {"id":511,"name":"RPG Builder","publisher":"Blink","needs_review":false,
   "file_count":1204,"size_bytes":184320112,"package_kind":"unitypackage",
   "licence_spdx":"MIT","licence_status":"ok","update_available":true,
   "created_at":"2026-07-01T00:00:00Z"},
  {"id":512,"name":"DOTween Pro","publisher":"Demigiant","needs_review":true,
   "file_count":88,"size_bytes":2048,"package_kind":"zip",
   "created_at":"2026-07-02T00:00:00Z"}
],"total":412}`

// serveJSON records the query string of each request so tests can assert what
// the CLI actually asked the server for.
func serveJSON(seen *url.Values, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = r.URL.Query()
		}
		_, _ = w.Write([]byte(body))
	}
}

func withServer(t *testing.T, handlers map[string]http.HandlerFunc) {
	t.Helper()
	srv := fakeServer(t, handlers)
	if _, err := SaveConfig(Config{Server: srv.URL, Token: testToken}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func TestAssets_DefaultProjectionAndAggregates(t *testing.T) {
	isolate(t)
	var seen url.Values
	withServer(t, map[string]http.HandlerFunc{"/api/assets": serveJSON(&seen, assetsBody)})

	out, code := exec(t, "assets")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.HasPrefix(out, "assets[2]{id,name,publisher,needs_review}:\n") {
		t.Fatalf("default projection wrong:\n%s", out)
	}
	if !strings.Contains(out, "total: 412") || !strings.Contains(out, "shown: 2") {
		t.Fatalf("aggregates missing:\n%s", out)
	}
	// Wide fields must stay out of the default view.
	for _, unwanted := range []string{"184320112", "unitypackage", "MIT"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("non-default field %q leaked:\n%s", unwanted, out)
		}
	}
	if seen.Get("limit") != "50" {
		t.Fatalf("limit not sent: %v", seen)
	}
}

func TestAssets_FiltersMapToQueryParams(t *testing.T) {
	isolate(t)
	var seen url.Values
	withServer(t, map[string]http.HandlerFunc{"/api/assets": serveJSON(&seen, assetsBody)})

	_, code := exec(t, "assets",
		"--q", "rpg", "--publisher", "Blink", "--store", "unity",
		"--package-kind", "zip", "--licence", "MIT", "--licence-status", "ok",
		"--source-host", "assetstore.unity.com", "--needs-review",
		"--updates-available", "--released-after", "2026-01-01",
		"--sort", "name_asc", "--collection", "3", "--project", "4",
		"--limit", "10", "--offset", "20")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	for key, want := range map[string]string{
		"q": "rpg", "publisher": "Blink", "store": "unity", "package_kind": "zip",
		"licence_spdx": "MIT", "licence_status": "ok",
		"source_host": "assetstore.unity.com", "needs_review": "true",
		"updates_available": "true", "released_after": "2026-01-01",
		"sort": "name_asc", "collection_id": "3", "project_id": "4",
		"limit": "10", "offset": "20",
	} {
		if got := seen.Get(key); got != want {
			t.Errorf("query %s = %q, want %q", key, got, want)
		}
	}
}

// A tag is a path by default, but a bare number is an id — the server takes
// them on different parameters.
func TestAssets_TagIsPathOrID(t *testing.T) {
	isolate(t)
	var seen url.Values
	withServer(t, map[string]http.HandlerFunc{"/api/assets": serveJSON(&seen, assetsBody)})

	exec(t, "assets", "--tag", "Characters/Humanoid")
	if seen.Get("tag_path") != "Characters/Humanoid" || seen.Get("tag_id") != "" {
		t.Fatalf("path form sent %v", seen)
	}
	exec(t, "assets", "--tag", "12")
	if seen.Get("tag_id") != "12" || seen.Get("tag_path") != "" {
		t.Fatalf("id form sent %v", seen)
	}
}

// Unset flags must be absent, not empty: an empty filter is a different
// request from no filter.
func TestAssets_UnsetFiltersAreAbsent(t *testing.T) {
	isolate(t)
	var seen url.Values
	withServer(t, map[string]http.HandlerFunc{"/api/assets": serveJSON(&seen, assetsBody)})

	exec(t, "assets")
	for _, key := range []string{"q", "publisher", "store", "tag_path", "tag_id",
		"needs_review", "updates_available", "sort", "offset"} {
		if _, present := seen[key]; present {
			t.Errorf("unset filter %q was sent as %q", key, seen.Get(key))
		}
	}
}

func TestAssets_ValidationExitsTwo(t *testing.T) {
	isolate(t)
	withServer(t, map[string]http.HandlerFunc{"/api/assets": serveJSON(nil, assetsBody)})

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"assets", "--sort", "bogus"}, "unknown sort"},
		{[]string{"assets", "--limit", "9999"}, "out of range"},
		{[]string{"assets", "--limit", "0"}, "out of range"},
		{[]string{"assets", "--collection", "abc"}, "numeric id"},
		{[]string{"assets", "--project", "x"}, "numeric id"},
		{[]string{"asset", "abc"}, "positive number"},
		{[]string{"asset", "0"}, "positive number"},
		{[]string{"asset"}, "exactly one id"},
		{[]string{"imports", "--stage", "nope"}, "unknown stage"},
		{[]string{"collection", "abc"}, "positive number"},
	} {
		out, code := exec(t, tc.args...)
		if code != axi.ExitUsage {
			t.Errorf("%v: exit = %d, want %d\n%s", tc.args, code, axi.ExitUsage, out)
		}
		if !strings.Contains(out, tc.want) {
			t.Errorf("%v: output missing %q:\n%s", tc.args, tc.want, out)
		}
	}
}

// The empty state has to say which filters produced nothing, and how much the
// library holds, so "none" is never ambiguous (AXI principle 5).
func TestAssets_EmptyStateNamesTheFilters(t *testing.T) {
	isolate(t)
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets":   serveJSON(nil, `{"assets":[],"total":0}`),
		"/api/overview": serveJSON(nil, overviewBody),
	})

	out, code := exec(t, "assets", "--tag", "Characters")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "assets[]: no assets match tag_path=Characters") {
		t.Fatalf("got:\n%s", out)
	}
	if !strings.Contains(out, "412 assets in the library") {
		t.Fatalf("the empty note should scope against the library total:\n%s", out)
	}
}

// The library-total lookup is only worth a round trip when the result is
// empty — it exists solely to disambiguate the empty state.
func TestAssets_NoExtraRequestWhenRowsExist(t *testing.T) {
	isolate(t)
	var overviewCalls atomic.Int32
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets": serveJSON(nil, assetsBody),
		"/api/overview": func(w http.ResponseWriter, _ *http.Request) {
			overviewCalls.Add(1)
			_, _ = w.Write([]byte(overviewBody))
		},
	})

	if _, code := exec(t, "assets"); code != axi.ExitOK {
		t.Fatal("assets failed")
	}
	if n := overviewCalls.Load(); n != 0 {
		t.Fatalf("a non-empty list made %d extra overview requests, want 0", n)
	}
}

// A truncated list must hand back the exact command for the next page.
func TestAssets_OffersTheNextPage(t *testing.T) {
	isolate(t)
	withServer(t, map[string]http.HandlerFunc{"/api/assets": serveJSON(nil, assetsBody)})

	out, _ := exec(t, "assets", "--limit", "2")
	if !strings.Contains(out, "dmsn assets --offset 2 --limit 2") {
		t.Fatalf("no next-page command for 2 of 412:\n%s", out)
	}

	// A complete list must not offer one.
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets": serveJSON(nil, strings.Replace(assetsBody, `"total":412`, `"total":2`, 1)),
	})
	out, _ = exec(t, "assets")
	if strings.Contains(out, "--offset") {
		t.Fatalf("offered a next page for a complete list:\n%s", out)
	}
}

func TestAsset_Detail(t *testing.T) {
	isolate(t)
	body := `{"id":511,"name":"RPG Builder","description":"A builder.",
	  "publisher":"Blink","store":"unity","needs_review":false,
	  "created_at":"2026-07-01T00:00:00Z","updated_at":"2026-07-02T00:00:00Z",
	  "licence":{"detected_type":"MIT","status":"ok"},
	  "tags":[{"id":1,"path":"Characters"},{"id":2,"path":"Characters/Humanoid"}],
	  "versions":[{"id":9,"store_version":"1.2","is_current":true},
	              {"id":8,"store_version":"1.1","is_current":false}],
	  "collections":[{"id":3,"name":"Favourites"}]}`
	withServer(t, map[string]http.HandlerFunc{"/api/assets/511": serveJSON(nil, body)})

	out, code := exec(t, "asset", "511")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	for _, want := range []string{
		"asset:", "id: 511", "name: RPG Builder", "publisher: Blink", "licence: MIT",
		`versions[2]: 1.2*,"1.1"`,
		"tags[2]: Characters,Characters/Humanoid",
		"collections[1]: Favourites",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("detail missing %q:\n%s", want, out)
		}
	}
	// Absent optional fields render null rather than being silently dropped.
	if !strings.Contains(out, "engine: null") {
		t.Fatalf("an unset field should be explicit:\n%s", out)
	}
}

func TestTags_PrefixFilterAndPagination(t *testing.T) {
	isolate(t)
	body := `{"tags":[
	  {"id":1,"path":"Characters","asset_count":40},
	  {"id":2,"path":"Characters/Humanoid","asset_count":12},
	  {"id":3,"path":"Characters/Creature","asset_count":8},
	  {"id":4,"path":"Audio","asset_count":90}
	]}`
	withServer(t, map[string]http.HandlerFunc{"/api/tags": serveJSON(nil, body)})

	out, code := exec(t, "tags")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "total: 4") {
		t.Fatalf("got:\n%s", out)
	}

	out, _ = exec(t, "tags", "--prefix", "Characters")
	if !strings.Contains(out, "total: 3") {
		t.Fatalf("prefix should match the node and its descendants:\n%s", out)
	}
	if strings.Contains(out, "Audio") {
		t.Fatalf("prefix leaked an unrelated branch:\n%s", out)
	}

	// A prefix that matches nothing must say what it filtered on.
	out, _ = exec(t, "tags", "--prefix", "Nope")
	if !strings.Contains(out, "tags[]: no tags match prefix=Nope") {
		t.Fatalf("got:\n%s", out)
	}

	// Client-side pagination is bounded and offers the next page.
	out, _ = exec(t, "tags", "--limit", "2")
	if !strings.Contains(out, "shown: 2") || !strings.Contains(out, "dmsn tags --offset 2 --limit 2") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestCollections_AndDetail(t *testing.T) {
	isolate(t)
	list := `{"collections":[{"id":3,"name":"Favourites","description":"Picks","asset_count":12,
	  "created_at":"2026-01-01T00:00:00Z","updated_at":"2026-02-01T00:00:00Z"}]}`
	detail := `{"id":3,"name":"Favourites","description":"Picks","asset_count":12,
	  "created_at":"2026-01-01T00:00:00Z","updated_at":"2026-02-01T00:00:00Z"}`
	var assetQuery url.Values
	withServer(t, map[string]http.HandlerFunc{
		"/api/collections":   serveJSON(nil, list),
		"/api/collections/3": serveJSON(nil, detail),
		"/api/assets":        serveJSON(&assetQuery, assetsBody),
	})

	out, code := exec(t, "collections")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "3,Favourites,12") {
		t.Fatalf("got:\n%s", out)
	}

	out, code = exec(t, "collection", "3")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "collection:") || !strings.Contains(out, "assets[2]{") {
		t.Fatalf("detail should carry the collection and its members:\n%s", out)
	}
	if assetQuery.Get("collection_id") != "3" {
		t.Fatalf("members were not scoped to the collection: %v", assetQuery)
	}

	// Composite output must be one JSON document.
	out, _ = exec(t, "collection", "3", "--json")
	dec := json.NewDecoder(strings.NewReader(out))
	var v map[string]any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if dec.More() {
		t.Fatalf("more than one JSON document:\n%s", out)
	}
}

func TestImports_StageFilterAndEmptyStates(t *testing.T) {
	isolate(t)
	body := `{"imports":[
	  {"id":9,"asset_id":511,"asset_name":"RPG Builder","stage":"failed",
	   "error":"extract: unexpected EOF","needs_review":true,
	   "updated_at":"2026-08-01T09:00:00Z","created_at":"2026-08-01T08:00:00Z"}
	],"total":1}`
	var seen url.Values
	withServer(t, map[string]http.HandlerFunc{"/api/imports": serveJSON(&seen, body)})

	out, code := exec(t, "imports")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "9,RPG Builder,failed") {
		t.Fatalf("got:\n%s", out)
	}
	// A failure in the list must surface the commands that address it.
	if !strings.Contains(out, "dmsn import retry <id>") ||
		!strings.Contains(out, "--fields id,asset_name,error") {
		t.Fatalf("help should react to the failure:\n%s", out)
	}

	exec(t, "imports", "--stage", "done")
	if seen.Get("stage") != "done" {
		t.Fatalf("stage not sent: %v", seen)
	}

	// The three empty cases are three different facts.
	withServer(t, map[string]http.HandlerFunc{
		"/api/imports": serveJSON(nil, `{"imports":[],"total":0}`),
	})
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"imports"}, "the review backlog is clear"},
		{[]string{"imports", "--stage", "all"}, "nothing has been imported yet"},
		{[]string{"imports", "--stage", "failed"}, `no imports in stage "failed"`},
	} {
		out, _ := exec(t, tc.args...)
		if !strings.Contains(out, tc.want) {
			t.Errorf("%v: want %q:\n%s", tc.args, tc.want, out)
		}
	}
}

// --fields widens the projection, and an unknown name lists the real set.
func TestReadCommands_FieldsProjection(t *testing.T) {
	isolate(t)
	withServer(t, map[string]http.HandlerFunc{"/api/assets": serveJSON(nil, assetsBody)})

	out, code := exec(t, "assets", "--fields", "id,size_bytes,licence_spdx")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.HasPrefix(out, "assets[2]{id,size_bytes,licence_spdx}:\n") {
		t.Fatalf("got:\n%s", out)
	}

	out, code = exec(t, "assets", "--fields", "nope")
	if code != axi.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, axi.ExitUsage)
	}
	for _, f := range []string{"package_kind", "update_available", "size_bytes"} {
		if !strings.Contains(out, f) {
			t.Fatalf("the error should list %q as available:\n%s", f, out)
		}
	}
}

// Global flags must work after a positional argument. stdlib flag stops at
// the first non-flag argument, and `dmsn asset 511 --json` is the natural way
// to type it — so the args are permuted before parsing.
func TestFlagsMayFollowPositionalArguments(t *testing.T) {
	isolate(t)
	detail := `{"id":511,"name":"RPG Builder","publisher":"Blink"}`
	var seen url.Values
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/511":  serveJSON(nil, detail),
		"/api/collections": serveJSON(nil, `{"collections":[]}`),
		"/api/assets":      serveJSON(&seen, assetsBody),
	})

	t.Run("bool flag after a positional", func(t *testing.T) {
		out, code := exec(t, "asset", "511", "--json")
		if code != axi.ExitOK {
			t.Fatalf("exit = %d: %s", code, out)
		}
		var v map[string]any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("--json ignored after a positional: %v\n%s", err, out)
		}
	})

	t.Run("value flag after a positional keeps its value", func(t *testing.T) {
		out, code := exec(t, "asset", "511", "--fields", "id,name")
		if code != axi.ExitOK {
			t.Fatalf("exit = %d: %s", code, out)
		}
		if strings.Contains(out, "publisher") {
			t.Fatalf("--fields ignored after a positional:\n%s", out)
		}
	})

	t.Run("flags interleaved with positionals", func(t *testing.T) {
		if _, code := exec(t, "--quiet", "asset", "511", "--no-help"); code != axi.ExitOK {
			t.Fatalf("exit = %d", code)
		}
	})

	t.Run("everything after -- stays positional", func(t *testing.T) {
		out, code := exec(t, "asset", "--", "511")
		if code != axi.ExitOK {
			t.Fatalf("exit = %d: %s", code, out)
		}
		if !strings.Contains(out, "id: 511") {
			t.Fatalf("got:\n%s", out)
		}
	})

	t.Run("a value that looks like a flag is not hoisted twice", func(t *testing.T) {
		exec(t, "assets", "--q", "--needs-review")
		if seen.Get("q") != "--needs-review" {
			t.Fatalf("a flag-shaped value was mis-parsed: q=%q", seen.Get("q"))
		}
		if seen.Get("needs_review") != "" {
			t.Fatalf("the value was also treated as a flag: %v", seen)
		}
	})
}
