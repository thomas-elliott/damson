package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
// "METHOD path" with its JSON body and when it arrived.
type fakeLibrary struct {
	exists map[string]bool // links the library already holds, answered "exists"
	calls  []string
	bodies []map[string]any
	at     []time.Time
}

func (l *fakeLibrary) serve(t *testing.T, links string, failLookupFor string) {
	t.Helper()
	log := func(r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		l.calls = append(l.calls, r.Method+" "+r.URL.Path)
		l.bodies = append(l.bodies, body)
		l.at = append(l.at, time.Now())
	}
	withServer(t, map[string]http.HandlerFunc{
		"GET /api/assets/links": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, links)
		},
		"POST /api/assets/references": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			link, _ := l.bodies[len(l.bodies)-1]["source_url"].(string)
			if l.exists[link] {
				fmt.Fprint(w, `{"outcome":"exists","asset_id":91}`)
				return
			}
			failed := link == failLookupFor
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"outcome":"created","asset_id":90,"metadata_failed":%t}`, failed)
		},
		"PATCH /api/assets/{id}": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, `{}`)
		},
		"POST /api/assets/bulk": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, `{"affected":1}`)
		},
		"POST /api/assets/{id}/metadata/fetch": func(w http.ResponseWriter, r *http.Request) {
			log(r)
			fmt.Fprint(w, `{}`)
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
	lib.serve(t, recordLinks, unityListingLink("101"))
	plan := writeTSV(t, "plan.tsv", purchasePlanColumns,
		"100\tBrand New\t"+unityListingLink("100")+"\tnew\t\t\t\t",
		"101\tLookup Fails\t"+unityListingLink("101")+"\tnew\t\t\t\t",
		"102\tNo link\t"+unityListingLink("102")+"\tmatched\t1\t\t\t",
		"200\tSame listing\t"+unityListingLink("200")+"\tmatched\t2\t\t\t",
		"103\tOther link\t"+unityListingLink("103")+"\tmatched\t3\t\t\t",
		"104\tSwap me\t"+unityListingLink("104")+"\tswap\t4\t\t\t",
		"105\tNot wanted\t"+unityListingLink("105")+"\tdropped\t\t\t\t")

	out, code := exec(t, "purchases", "record", plan, "--interval", "0s")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	want := []string{
		"GET /api/assets/links",
		"POST /api/assets/references", "POST /api/assets/references",
		"POST /api/assets/bulk", "PATCH /api/assets/1", "POST /api/assets/1/metadata/fetch",
		"POST /api/assets/bulk", "PATCH /api/assets/4", "POST /api/assets/4/metadata/fetch",
	}
	if !slices.Equal(lib.calls, want) {
		t.Fatalf("calls =\n%s\nwant\n%s", strings.Join(lib.calls, "\n"), strings.Join(want, "\n"))
	}
	if b := lib.bodies[1]; b["needs_review"] != true || b["name"] != "Brand New" || b["source_url"] != unityListingLink("100") {
		t.Errorf("new purchase body = %v", b)
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
		"dropped: 1", "no_store_data: 1"} {
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
	lib.serve(t, recordLinks, "")
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

// Every call that makes the server look a listing up waits the interval after
// the one before, whether it records a new purchase or links a matched asset.
func TestPurchasesRecord_SpacesStoreLookups(t *testing.T) {
	isolate(t)
	lib := &fakeLibrary{}
	lib.serve(t, recordLinks, "")
	plan := writeTSV(t, "plan.tsv", purchasePlanColumns,
		"100\tA\t"+unityListingLink("100")+"\tnew\t\t\t\t",
		"102\tNo link\t"+unityListingLink("102")+"\tmatched\t1\t\t\t",
		"101\tB\t"+unityListingLink("101")+"\tnew\t\t\t\t")

	interval := 60 * time.Millisecond
	if out, code := exec(t, "purchases", "record", plan, "--interval", interval.String()); code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	var lookups []time.Time
	for i, c := range lib.calls {
		if c == "POST /api/assets/references" || strings.HasSuffix(c, "/metadata/fetch") {
			lookups = append(lookups, lib.at[i])
		}
	}
	if len(lookups) != 3 {
		t.Fatalf("%d lookups, want 3: %v", len(lookups), lib.calls)
	}
	for i := 1; i < len(lookups); i++ {
		if gap := lookups[i].Sub(lookups[i-1]); gap < interval {
			t.Errorf("lookup %d came %v after the one before, want at least %v", i+1, gap, interval)
		}
	}
}

// Recording a plan again, after a run that stopped part-way, does not wait on
// the purchases already recorded: the server answers those without a lookup.
func TestPurchasesRecord_DoesNotWaitOnRecordedPurchases(t *testing.T) {
	isolate(t)
	lib := &fakeLibrary{exists: map[string]bool{
		unityListingLink("100"): true, unityListingLink("101"): true, unityListingLink("102"): true}}
	lib.serve(t, recordLinks, "")
	plan := writeTSV(t, "plan.tsv", purchasePlanColumns,
		"100	A	"+unityListingLink("100")+"	new				",
		"101	B	"+unityListingLink("101")+"	new				",
		"102	C	"+unityListingLink("102")+"	new				")

	interval := time.Second
	start := time.Now()
	out, code := exec(t, "purchases", "record", plan, "--interval", interval.String())
	if code != axi.ExitOK || !strings.Contains(out, "already_in_library: 3") {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if took := time.Since(start); took >= interval {
		t.Errorf("took %v, want under %v: it waited on rows that made no lookup", took, interval)
	}
}
