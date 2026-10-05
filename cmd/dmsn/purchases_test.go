package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		"": "",
	} {
		if got := unityListingID(link); got != want {
			t.Errorf("unityListingID(%q) = %q, want %q", link, got, want)
		}
	}
}
