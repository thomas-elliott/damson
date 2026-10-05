package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
)

// writeTSV writes header and rows (each already tab-separated) to a file.
func writeTSV(t *testing.T, name string, header []string, rows ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	body := strings.Join(header, "\t") + "\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// planRows reads a plan file back as action and asset_id per package_id.
func planRows(t *testing.T, path string) map[string][2]string {
	t.Helper()
	rows, err := readTSV(path, "plan", purchasePlanColumns)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][2]string{}
	for _, r := range rows {
		out[r.Cols[0]] = [2]string{r.Cols[3], r.Cols[4]}
	}
	return out
}

// The draft plan matches a purchase by listing ID, whether the asset carries
// it as a store product ID or only in its link; a name-only match is unsure,
// never matched; and an asset linked to another listing isn't matched by name.
func TestPurchasesPlan_MatchesByListingIDThenName(t *testing.T) {
	isolate(t)
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/links": func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"assets":[
			  {"asset_id":1,"name":"Toony Skies","store":"unity","store_product_id":"11020",
			   "source_url":"https://assetstore.unity.com/packages/2d/textures-materials/sky/toony-skies-11020"},
			  {"asset_id":2,"name":"Linked by hand","source_url":"https://assetstore.unity.com/packages/3d/foo-200"},
			  {"asset_id":3,"name":"POLYGON_Fantasy_Kingdom","source_url":""},
			  {"asset_id":4,"name":"Other Thing","source_url":"https://assetstore.unity.com/packages/3d/other-thing-999"}]}`)
		},
	})
	purchases := writeTSV(t, "purchases.tsv", purchaseColumns,
		"11020\tToony Skies\t"+unityListingLink("11020"),
		"200\tFoo\t"+unityListingLink("200"),
		"300\tPOLYGON - Fantasy Kingdom\t"+unityListingLink("300"),
		"400\tBrand New\t"+unityListingLink("400"),
		"500\tOther Thing\t"+unityListingLink("500"))
	plan := filepath.Join(t.TempDir(), "plan.tsv")

	out, code := exec(t, "purchases", "plan", purchases, "--out", plan)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	want := map[string][2]string{
		"11020": {actionMatched, "1"},
		"200":   {actionMatched, "2"},
		"300":   {actionUnsure, "3"},
		"400":   {actionNew, ""},
		"500":   {actionNew, ""},
	}
	got := planRows(t, plan)
	for id, w := range want {
		if got[id] != w {
			t.Errorf("purchase %s: action, asset = %v, want %v", id, got[id], w)
		}
	}
	if !strings.Contains(out, "matched: 2") || !strings.Contains(out, "unsure: 1") || !strings.Contains(out, "new: 2") {
		t.Errorf("counts missing:\n%s", out)
	}
}

func TestUnityListingID(t *testing.T) {
	for link, want := range map[string]string{
		"https://assetstore.unity.com/packages/2d/textures-materials/sky/toony-skies-11020": "11020",
		"https://assetstore.unity.com/packages/slug/11020/":                                 "11020",
		"https://assetstore.unity.com/packages/tools/my-tool-v2":                            "",
		"https://example.itch.io/pack-11020":                                                "",
		"":                                                                                  "",
	} {
		if got := unityListingID(link); got != want {
			t.Errorf("unityListingID(%q) = %q, want %q", link, got, want)
		}
	}
}

// fakeLibrary serves the routes `purchases record` calls, logging each call as
// "METHOD path" with its JSON body. Every queued lookup joins batch 40.
type fakeLibrary struct {
	exists map[string]bool // links the library already holds, answered "exists"
	slow   map[string]bool // links whose record answers after the client gives up
	mu     sync.Mutex      // a slow answer can overlap the next request
	calls  []string
	bodies []map[string]any
}

func (l *fakeLibrary) serve(t *testing.T, links string) {
	t.Helper()
	log := func(r *http.Request) map[string]any {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		l.mu.Lock()
		defer l.mu.Unlock()
		l.calls = append(l.calls, r.Method+" "+r.URL.Path)
		l.bodies = append(l.bodies, body)
		return body
	}
	withServer(t, map[string]http.HandlerFunc{
		"GET /api/assets/links": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, links)
		},
		"POST /api/assets/references": func(w http.ResponseWriter, r *http.Request) {
			link, _ := log(r)["source_url"].(string)
			if l.slow[link] {
				time.Sleep(300 * time.Millisecond)
			}
			if l.exists[link] {
				fmt.Fprint(w, `{"outcome":"exists","asset_id":91}`)
				return
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"outcome":"created","asset_id":90,"lookup_batch_id":40}`)
		},
		"PATCH /api/assets/{id}": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, `{}`)
		},
		"POST /api/assets/bulk": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, `{"affected":1}`)
		},
		"POST /api/store-lookups": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"batch_id":40,"total":1,"queued":1}`)
		},
		"GET /api/store-lookups/batches/40": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, `{"batch_id":40,"total":4,"queued":2,"done":2,"no_store_data":1}`)
		},
	})
}

const recordLinks = `{"assets":[
  {"asset_id":1,"name":"No link","source_url":""},
  {"asset_id":2,"name":"Same listing","source_url":"https://assetstore.unity.com/packages/3d/same-200"},
  {"asset_id":3,"name":"Other link","source_url":"https://someone.itch.io/other"},
  {"asset_id":4,"name":"Swap me","source_url":"https://someone.itch.io/swap"}]}`

// An approved plan records exactly what each row says: a new purchase as a
// Needs-review reference asset; a matched asset with no link gains it and is
// marked Needs review; one already on the listing, or matched with another
// link, is not touched; swap replaces the link; a dropped row does nothing.
func TestPurchasesRecord_RecordsWhatEachRowSays(t *testing.T) {
	isolate(t)
	lib := &fakeLibrary{}
	lib.serve(t, recordLinks)
	plan := writeTSV(t, "plan.tsv", purchasePlanColumns,
		"100\tBrand New\t"+unityListingLink("100")+"\tnew\t\t\t\t",
		"101\tLookup Fails\t"+unityListingLink("101")+"\tnew\t\t\t\t",
		"102\tNo link\t"+unityListingLink("102")+"\tmatched\t1\t\t\t",
		"200\tSame listing\t"+unityListingLink("200")+"\tmatched\t2\t\t\t",
		"103\tOther link\t"+unityListingLink("103")+"\tmatched\t3\t\t\t",
		"104\tSwap me\t"+unityListingLink("104")+"\tswap\t4\t\t\t",
		"105\tNot wanted\t"+unityListingLink("105")+"\tdropped\t\t\t\t")

	captureProgress(t)
	out, code := exec(t, "purchases", "record", plan)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	want := []string{
		"GET /api/assets/links",
		"POST /api/assets/references", "POST /api/assets/references",
		"POST /api/assets/bulk", "PATCH /api/assets/1", "POST /api/store-lookups",
		"POST /api/assets/bulk", "PATCH /api/assets/4", "POST /api/store-lookups",
		"GET /api/store-lookups/batches/40",
	}
	if !slices.Equal(lib.calls, want) {
		t.Fatalf("calls =\n%s\nwant\n%s", strings.Join(lib.calls, "\n"), strings.Join(want, "\n"))
	}
	if b := lib.bodies[1]; b["needs_review"] != true || b["name"] != "Brand New" || b["source_url"] != unityListingLink("100") ||
		b["queue_lookup"] != true || b["lookup_batch_id"] != float64(0) {
		t.Errorf("new purchase body = %v, want its lookup queued in a new batch", b)
	}
	// Every later lookup joins the run's batch, whether it records or links.
	if b := lib.bodies[2]; b["lookup_batch_id"] != float64(40) {
		t.Errorf("second record body = %v, want batch 40", b)
	}
	if b := lib.bodies[5]; b["batch_id"] != float64(40) || b["refresh"] != nil || fmt.Sprint(b["asset_ids"]) != "[1]" {
		t.Errorf("link lookup body = %v, want a fill-blanks lookup of asset 1 in batch 40", b)
	}
	if b := lib.bodies[4]; b["source_url"] != unityListingLink("102") || b["store"] != nil {
		t.Errorf("link body = %v", b)
	}
	// A swap blanks the old store's identity so the Unity lookup can fill it.
	if b := lib.bodies[7]; b["source_url"] != unityListingLink("104") || b["store"] != "" ||
		b["store_product_id"] != "" || b["latest_store_version"] != "" {
		t.Errorf("swap body = %v", b)
	}
	if b := lib.bodies[3]; b["action"] != "unreview" || fmt.Sprint(b["asset_ids"]) != "[1]" {
		t.Errorf("Needs review body = %v", b)
	}
	for _, s := range []string{"added: 2", "linked: 1", "swapped: 1", "unchanged: 1", "kept_own_link: 1",
		"dropped: 1", "store_lookups_queued: 4", "lookup_batch: 40", "no_store_data_so_far: 1",
		"dmsn bulk refresh --batch 40"} {
		if !strings.Contains(out, s) {
			t.Errorf("summary lacks %q:\n%s", s, out)
		}
	}
}

// A name-only match is never applied without the operator's say-so: a plan
// still holding an unsure row records nothing.
func TestPurchasesRecord_RefusesAnUnsureRow(t *testing.T) {
	isolate(t)
	lib := &fakeLibrary{}
	lib.serve(t, recordLinks)
	plan := writeTSV(t, "plan.tsv", purchasePlanColumns,
		"100\tBrand New\t"+unityListingLink("100")+"\tnew\t\t\t\t",
		"102\tNo link\t"+unityListingLink("102")+"\tunsure\t1\t\t\t")

	out, code := exec(t, "purchases", "record", plan)
	if code != axi.ExitUsage || !strings.Contains(out, "still unsure: 1") {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if len(lib.calls) != 0 {
		t.Fatalf("the server was called: %v", lib.calls)
	}
}

// A row is one quick request whatever the store is doing, and a progress line
// goes to stderr for each, so a long plan shows where it is.
func TestPurchasesRecord_PrintsEachRowAsItGoes(t *testing.T) {
	isolate(t)
	progress := captureProgress(t)
	lib := &fakeLibrary{exists: map[string]bool{unityListingLink("100"): true}}
	lib.serve(t, recordLinks)
	plan := writeTSV(t, "plan.tsv", purchasePlanColumns,
		"100\tA\t"+unityListingLink("100")+"\tnew\t\t\t\t",
		"101\tB\t"+unityListingLink("101")+"\tnew\t\t\t\t")

	out, code := exec(t, "purchases", "record", plan)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	want := "record 1 of 2: 100 A: already_in_library\nrecord 2 of 2: 101 B: added\n"
	if progress.String() != want {
		t.Fatalf("progress =\n%s\nwant\n%s", progress, want)
	}
}

// A row that times out may have been recorded, so it says so and points at a
// rerun, which answers exists for anything already recorded.
func TestPurchasesRecord_TimedOutRowSaysToRerun(t *testing.T) {
	isolate(t)
	captureProgress(t)
	lib := &fakeLibrary{slow: map[string]bool{unityListingLink("100"): true}}
	lib.serve(t, recordLinks)
	plan := writeTSV(t, "plan.tsv", purchasePlanColumns,
		"100\tSlow\t"+unityListingLink("100")+"\tnew\t\t\t\t",
		"101\tFast\t"+unityListingLink("101")+"\tnew\t\t\t\t")

	out, code := exec(t, "purchases", "record", plan, "--timeout", "100ms")
	if code == axi.ExitOK {
		t.Fatalf("a failed row must set the exit code:\n%s", out)
	}
	if !strings.Contains(out, "may have been recorded; rerun the plan") || !strings.Contains(out, "added: 1") {
		t.Fatalf("output:\n%s", out)
	}
}
