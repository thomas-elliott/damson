package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
)

// mkPacks creates each relative path under root as a file, so a test can lay
// out a store the way a scan would find it.
func mkPacks(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A recorded path that is no longer on disk is reported gone, scoped to the
// folder that was scanned.
func TestScanKnown_ReportsRecordedPathsThatAreGone(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	mkPacks(t, root, "3D/Synty/Kept.unitypackage")
	var under string
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/locations": func(w http.ResponseWriter, r *http.Request) {
			under = r.URL.Query().Get("under")
			fmt.Fprintf(w, `{"locations":[
			  {"asset_id":1,"location_path":%q},
			  {"asset_id":2,"location_path":%q}]}`,
				filepath.Join(root, "3D/Synty/Kept.unitypackage"), filepath.Join(root, "3D/Synty/Moved.unitypackage"))
		},
	})

	out, code := exec(t, "scan", "known", "--under", root+"/", "--gone")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if under != root {
		t.Errorf("under sent as %q, want %q", under, root)
	}
	if !strings.Contains(out, "recorded: 2") || !strings.Contains(out, "gone: 1") {
		t.Fatalf("counts missing:\n%s", out)
	}
	if !strings.Contains(out, "Moved.unitypackage") || strings.Contains(out, "Kept.unitypackage") {
		t.Fatalf("--gone should list only the missing path:\n%s", out)
	}
}

// If the store isn't mounted, every path would look gone; refuse instead.
func TestScanKnown_RefusesARootThatIsNotThere(t *testing.T) {
	isolate(t)
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/locations": func(http.ResponseWriter, *http.Request) {
			t.Error("the server was asked although the root is missing")
		},
	})
	out, code := exec(t, "scan", "known", "--under", filepath.Join(t.TempDir(), "unmounted"))
	if code == axi.ExitOK || !strings.Contains(out, "not a readable directory") {
		t.Fatalf("exit = %d\n%s", code, out)
	}
}

// writePlan writes a plan file with the plan header and the given rows, each
// already tab-separated.
func writePlan(t *testing.T, rows ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan.tsv")
	body := strings.Join(planColumns, "\t") + "\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The whole scan's searches run as one step that writes a file: one search per
// undecided pack, none for a pack whose link is already chosen, and a failed
// or throttled search is written down rather than dropped.
func TestScanMatch_WritesEveryPacksPossibleMatchesToAFile(t *testing.T) {
	isolate(t)
	var queries []string
	withServer(t, map[string]http.HandlerFunc{
		"/api/metadata/search": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query().Get("q")
			queries = append(queries, q)
			switch q {
			case "Fantasy Kingdom":
				fmt.Fprint(w, `{"candidates":[
				  {"title":"POLYGON - Fantasy Kingdom","publisher":"Synty Studios","listing_url":"https://assetstore.unity.com/packages/k-1","store":"unity"},
				  {"title":"u2","listing_url":"https://assetstore.unity.com/packages/2","store":"unity"},
				  {"title":"u3","listing_url":"https://assetstore.unity.com/packages/3","store":"unity"},
				  {"title":"u4","listing_url":"https://assetstore.unity.com/packages/4","store":"unity"},
				  {"title":"Kingdom\ttiles","listing_url":"https://x.itch.io/k","store":"itch"}]}`)
			case "Throttled":
				fmt.Fprint(w, `{"candidates":[],"failed_stores":["unity"]}`)
			case "Nothing":
				fmt.Fprint(w, `{"candidates":[]}`)
			default:
				w.WriteHeader(http.StatusBadGateway)
				fmt.Fprint(w, `{"error":"upstream"}`)
			}
		},
	})
	plan := writePlan(t,
		"/nfs/3D/Synty/Kingdom.unitypackage\tFantasy Kingdom\tSynty\t3D\t",
		"/nfs/3D/Decided\tDecided\t\t3D\tno match",
		"/nfs/3D/Throttled\tThrottled\t\t3D\t",
		"/nfs/3D/Nothing\tNothing\t\t3D\t",
		"/nfs/3D/Broken\tBroken\t\t3D\t",
	)
	out := filepath.Join(t.TempDir(), "matches.tsv")

	stdout, code := exec(t, "scan", "match", plan, "--out", out, "--interval", "0s")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, stdout)
	}
	if strings.Join(queries, "|") != "Fantasy Kingdom|Throttled|Nothing|Broken" {
		t.Fatalf("searches: %q", queries)
	}
	got := readFile(t, out)
	for _, want := range []string{
		strings.Join(matchColumns, "\t") + "\n",
		"/nfs/3D/Synty/Kingdom.unitypackage\tcandidate\tunity\tPOLYGON - Fantasy Kingdom\tSynty Studios\thttps://assetstore.unity.com/packages/k-1\t\n",
		"/nfs/3D/Synty/Kingdom.unitypackage\tcandidate\titch\tKingdom tiles\t\thttps://x.itch.io/k\t\n",
		"/nfs/3D/Throttled\tstore_failed\tunity\t\t\t\t\n",
		"/nfs/3D/Nothing\tnone\t\t\t\t\t\n",
		"/nfs/3D/Broken\tsearch_failed\t",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("matches file lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "packages/4") || strings.Contains(got, "/nfs/3D/Decided") {
		t.Errorf("a fourth listing from one store, or a decided pack, was written:\n%s", got)
	}
	for _, want := range []string{"searched: 4", "store_failed: 1", "search_failed: 1", "already_decided: 1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("summary lacks %q:\n%s", want, stdout)
		}
	}
}

// Searches are spaced out, one pack at a time, so the stores never see a burst.
func TestScanMatch_PausesBetweenPacks(t *testing.T) {
	isolate(t)
	var at []time.Time
	withServer(t, map[string]http.HandlerFunc{
		"/api/metadata/search": func(w http.ResponseWriter, _ *http.Request) {
			at = append(at, time.Now())
			fmt.Fprint(w, `{"candidates":[]}`)
		},
	})
	plan := writePlan(t, "/nfs/a\tA\t\t\t", "/nfs/b\tB\t\t\t", "/nfs/c\tC\t\t\t")

	interval := 60 * time.Millisecond
	if out, code := exec(t, "scan", "match", plan, "--out", filepath.Join(t.TempDir(), "m.tsv"),
		"--interval", interval.String()); code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if len(at) != 3 {
		t.Fatalf("%d searches, want 3", len(at))
	}
	for i := 1; i < len(at); i++ {
		if gap := at[i].Sub(at[i-1]); gap < interval {
			t.Errorf("search %d came %v after the one before, want at least %v", i+1, gap, interval)
		}
	}
}

func TestReadPlan_RefusesAMalformedPlan(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"no header": {"/nfs/a\tA\t\t\t\n", "plan header"},
		"columns":   {strings.Join(planColumns, "\t") + "\n/nfs/a\tA\n", "has 2 columns"},
		"no name":   {strings.Join(planColumns, "\t") + "\n/nfs/a\t\t\t\t\n", "needs a path and a name"},
		"duplicate": {strings.Join(planColumns, "\t") + "\n/nfs/a\tA\t\t\t\n/nfs/a/\tB\t\t\t\n", "repeats the path on line 2"},
		"bad link":  {strings.Join(planColumns, "\t") + "\n/nfs/a\tA\t\t\tmaybe\n", "link must be"},
	} {
		p := filepath.Join(t.TempDir(), "plan.tsv")
		if err := os.WriteFile(p, []byte(tc.body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readPlan(p); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error containing %q", name, err, tc.want)
		}
	}
}
