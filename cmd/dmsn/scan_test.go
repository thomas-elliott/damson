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
