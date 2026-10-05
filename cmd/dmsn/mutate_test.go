package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/thomas-elliott/damson/internal/axi"
)

// capture records the method, path and decoded body of each request.
type capture struct {
	mu    sync.Mutex
	calls []call
}

type call struct {
	Method string
	Path   string
	Body   map[string]any
}

func (c *capture) record(response string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &body)
		}
		c.mu.Lock()
		c.calls = append(c.calls, call{Method: r.Method, Path: r.URL.Path, Body: body})
		c.mu.Unlock()
		if response == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(response))
	}
}

func (c *capture) matching(method, path string) []call {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []call
	for _, k := range c.calls {
		if k.Method == method && k.Path == path {
			out = append(out, k)
		}
	}
	return out
}

const assetDetail = `{"id":511,"name":"RPG Builder","needs_review":true,
  "tags":[{"id":7,"path":"Characters"}]}`

// Destructive verbs never prompt: they exit 2 with the exact command that
// would have worked (AXI principle 6).
func TestDestructiveVerbsRequireYes(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/511":    cap.record(assetDetail),
		"/api/collections/3": cap.record(`{"id":3,"name":"Favourites"}`),
		"/api/tags":          cap.record(`{"tags":[{"id":7,"path":"Characters"}]}`),
		"/api/tags/7":        cap.record(""),
		"/api/assets/bulk":   cap.record(`{"affected":2}`),
	})

	for _, tc := range []struct {
		args    []string
		confirm string
	}{
		{[]string{"asset", "rm", "511"}, "dmsn asset rm 511 --yes"},
		{[]string{"tag", "rm", "Characters"}, "dmsn tag rm Characters --yes"},
		{[]string{"collection", "rm", "3"}, "dmsn collection rm 3 --yes"},
		{[]string{"bulk", "rm", "--assets", "1,2"}, "dmsn bulk rm --assets 1,2 --yes"},
	} {
		out, code := exec(t, tc.args...)
		if code != axi.ExitUsage {
			t.Errorf("%v: exit = %d, want %d\n%s", tc.args, code, axi.ExitUsage, out)
		}
		if !strings.Contains(out, tc.confirm) {
			t.Errorf("%v: help should carry %q:\n%s", tc.args, tc.confirm, out)
		}
	}
	// Nothing was deleted while refusing.
	for _, k := range cap.calls {
		if k.Method == http.MethodDelete {
			t.Fatalf("a refused command still issued DELETE %s", k.Path)
		}
	}
}

func TestAssetRm_WithYes(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{"/api/assets/511": cap.record("")})

	out, code := exec(t, "asset", "rm", "511", "--yes")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if len(cap.matching(http.MethodDelete, "/api/assets/511")) != 1 {
		t.Fatalf("expected one DELETE, got %+v", cap.calls)
	}
}

// asset edit sends only the flags that were given: an absent key means
// "leave alone", so sending everything would blank the rest.
func TestAssetEdit_SendsOnlyWhatWasSet(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{"/api/assets/511": cap.record(assetDetail)})

	out, code := exec(t, "asset", "edit", "511", "--name", "New Name", "--publisher", "Blink")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	calls := cap.matching(http.MethodPatch, "/api/assets/511")
	if len(calls) != 1 {
		t.Fatalf("expected one PATCH, got %+v", cap.calls)
	}
	body := calls[0].Body
	if body["name"] != "New Name" || body["publisher"] != "Blink" {
		t.Fatalf("body = %v", body)
	}
	if len(body) != 2 {
		t.Fatalf("body carries fields that were not set: %v", body)
	}
}

// --clear sends an explicit empty string, which is how the API expresses
// "null this column" as distinct from "leave it alone".
func TestAssetEdit_Clear(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{"/api/assets/511": cap.record(assetDetail)})

	if _, code := exec(t, "asset", "edit", "511", "--clear", "description,price"); code != axi.ExitOK {
		t.Fatal("edit failed")
	}
	body := cap.matching(http.MethodPatch, "/api/assets/511")[0].Body
	if v, ok := body["description"]; !ok || v != "" {
		t.Fatalf("description should be an explicit empty string: %v", body)
	}
	if v, ok := body["price"]; !ok || v != "" {
		t.Fatalf("price should be an explicit empty string: %v", body)
	}

	t.Run("setting and clearing the same field is a usage error", func(t *testing.T) {
		out, code := exec(t, "asset", "edit", "511", "--name", "X", "--clear", "name")
		if code != axi.ExitUsage {
			t.Fatalf("exit = %d\n%s", code, out)
		}
		if !strings.Contains(out, "contradict") {
			t.Fatalf("got:\n%s", out)
		}
	})
}

// review is idempotent: the endpoint toggles, so an asset already in the
// requested state must not be flipped out of it.
func TestAssetReview_IsIdempotent(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/511":        cap.record(assetDetail), // needs_review: true
		"/api/assets/511/review": cap.record(`{}`),
	})

	// needs_review is true, so `review` has work to do.
	out, code := exec(t, "asset", "review", "511")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "changed: true") {
		t.Fatalf("got:\n%s", out)
	}
	if n := len(cap.matching(http.MethodPost, "/api/assets/511/review")); n != 1 {
		t.Fatalf("expected one POST, got %d", n)
	}

	// `unreview` on an asset that already needs review is a no-op.
	cap2 := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/511":        cap2.record(assetDetail),
		"/api/assets/511/review": cap2.record(`{}`),
	})
	out, code = exec(t, "asset", "unreview", "511")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "changed: false") {
		t.Fatalf("a no-op must report changed: false:\n%s", out)
	}
	if n := len(cap2.matching(http.MethodPost, "/api/assets/511/review")); n != 0 {
		t.Fatalf("a no-op must not toggle, got %d POSTs", n)
	}
}

// Tagging reports per-path whether anything actually changed, rather than
// pretending every call did something.
func TestAssetTag_ReportsWhatChanged(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/511":      cap.record(assetDetail), // already holds "Characters"
		"/api/assets/511/tags": cap.record(`{}`),
	})

	out, code := exec(t, "asset", "tag", "511", "Characters", "Audio")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "Characters,unchanged") {
		t.Fatalf("an already-held tag must report unchanged:\n%s", out)
	}
	if !strings.Contains(out, "Audio,attached") {
		t.Fatalf("got:\n%s", out)
	}
	if !strings.Contains(out, "changed: 1") {
		t.Fatalf("got:\n%s", out)
	}
	posts := cap.matching(http.MethodPost, "/api/assets/511/tags")
	if len(posts) != 1 || posts[0].Body["path"] != "Audio" {
		t.Fatalf("only the missing tag should be posted: %+v", posts)
	}
}

func TestAssetUntag(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/511":        cap.record(assetDetail),
		"/api/assets/511/tags/7": cap.record(""),
	})

	out, code := exec(t, "asset", "untag", "511", "Characters", "NotHeld")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "Characters,detached") || !strings.Contains(out, "NotHeld,unchanged") {
		t.Fatalf("got:\n%s", out)
	}
	if n := len(cap.matching(http.MethodDelete, "/api/assets/511/tags/7")); n != 1 {
		t.Fatalf("expected one DELETE, got %d", n)
	}
}

func TestBulk(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{"/api/assets/bulk": cap.record(`{"affected":3}`)})

	out, code := exec(t, "bulk", "tag", "--assets", "1,2,3", "--tag", "Characters")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	body := cap.matching(http.MethodPost, "/api/assets/bulk")[0].Body
	if body["action"] != "add_tag" || body["path"] != "Characters" {
		t.Fatalf("body = %v", body)
	}
	if !strings.Contains(out, "affected: 3") || !strings.Contains(out, "requested: 3") {
		t.Fatalf("got:\n%s", out)
	}

	t.Run("tag verbs require --tag", func(t *testing.T) {
		out, code := exec(t, "bulk", "tag", "--assets", "1,2")
		if code != axi.ExitUsage || !strings.Contains(out, "--tag is required") {
			t.Fatalf("exit=%d out=%s", code, out)
		}
	})

	// A gap between requested and affected is meaningful, so it is explained
	// rather than left as a silent discrepancy.
	t.Run("explains an under-count", func(t *testing.T) {
		cap := &capture{}
		withServer(t, map[string]http.HandlerFunc{"/api/assets/bulk": cap.record(`{"affected":1}`)})
		out, _ := exec(t, "bulk", "review", "--assets", "1,2,3")
		if !strings.Contains(out, "2 of 3 were already in that state") {
			t.Fatalf("got:\n%s", out)
		}
	})

	t.Run("bad ids exit 2", func(t *testing.T) {
		for _, ids := range []string{"", "abc", "1,0", "1,-2"} {
			out, code := exec(t, "bulk", "review", "--assets", ids)
			if code != axi.ExitUsage {
				t.Errorf("--assets %q: exit = %d\n%s", ids, code, out)
			}
		}
	})
}

func TestCollectionVerbs(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/collections":            cap.record(`{"id":3,"name":"Favourites"}`),
		"/api/collections/3":          cap.record(`{"id":3,"name":"Favourites"}`),
		"/api/collections/3/assets/2": cap.record(""),
		"/api/assets/bulk":            cap.record(`{"affected":2}`),
		"/api/assets":                 serveJSON(nil, assetsBody),
	})

	t.Run("new", func(t *testing.T) {
		out, code := exec(t, "collection", "new", "Favourites", "--description", "Picks")
		if code != axi.ExitOK {
			t.Fatalf("exit = %d: %s", code, out)
		}
		body := cap.matching(http.MethodPost, "/api/collections")[0].Body
		if body["name"] != "Favourites" || body["description"] != "Picks" {
			t.Fatalf("body = %v", body)
		}
	})

	t.Run("edit sends only what was set", func(t *testing.T) {
		if _, code := exec(t, "collection", "edit", "3", "--name", "Picks"); code != axi.ExitOK {
			t.Fatal("edit failed")
		}
		body := cap.matching(http.MethodPatch, "/api/collections/3")[0].Body
		if body["name"] != "Picks" {
			t.Fatalf("body = %v", body)
		}
		if _, present := body["description"]; present {
			t.Fatalf("an unset flag must not be sent: %v", body)
		}
	})

	t.Run("edit with nothing set is a usage error", func(t *testing.T) {
		out, code := exec(t, "collection", "edit", "3")
		if code != axi.ExitUsage || !strings.Contains(out, "nothing to change") {
			t.Fatalf("exit=%d out=%s", code, out)
		}
	})

	t.Run("add goes through the audited bulk endpoint", func(t *testing.T) {
		if _, code := exec(t, "collection", "add", "3", "--assets", "1,2"); code != axi.ExitOK {
			t.Fatal("add failed")
		}
		body := cap.matching(http.MethodPost, "/api/assets/bulk")[0].Body
		if body["action"] != "add_to_collection" || body["collection_id"] != float64(3) {
			t.Fatalf("body = %v", body)
		}
	})

	t.Run("a bare id still reads", func(t *testing.T) {
		out, code := exec(t, "collection", "3")
		if code != axi.ExitOK {
			t.Fatalf("exit = %d: %s", code, out)
		}
		if !strings.Contains(out, "collection:") {
			t.Fatalf("got:\n%s", out)
		}
	})
}

// Removing an asset that is not a member reaches the requested end state, so
// a 404 is a success and must not abort the remaining removals.
func TestCollectionRemove_TolerantOfNonMembers(t *testing.T) {
	isolate(t)
	var deleted int
	withServer(t, map[string]http.HandlerFunc{
		"/api/collections/3/assets/1": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not a member"}`))
		},
		"/api/collections/3/assets/2": func(w http.ResponseWriter, _ *http.Request) {
			deleted++
			w.WriteHeader(http.StatusNoContent)
		},
	})

	out, code := exec(t, "collection", "remove", "3", "--assets", "1,2")
	if code != axi.ExitOK {
		t.Fatalf("a non-member must not fail the command; exit = %d\n%s", code, out)
	}
	if deleted != 1 {
		t.Fatalf("the second removal should still have run, got %d", deleted)
	}
	if !strings.Contains(out, "affected: 1") || !strings.Contains(out, "requested: 2") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestTagVerbs(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/tags":   cap.record(`{"tags":[{"id":7,"path":"Characters"}],"id":9,"path":"Audio"}`),
		"/api/tags/7": cap.record(""),
	})

	t.Run("new", func(t *testing.T) {
		if _, code := exec(t, "tag", "new", "Audio/FX"); code != axi.ExitOK {
			t.Fatal("tag new failed")
		}
		posts := cap.matching(http.MethodPost, "/api/tags")
		if len(posts) != 1 || posts[0].Body["path"] != "Audio/FX" {
			t.Fatalf("posts = %+v", posts)
		}
	})

	// DELETE takes an id, but the CLI speaks in paths — so it resolves first.
	t.Run("rm resolves the path to an id", func(t *testing.T) {
		if _, code := exec(t, "tag", "rm", "Characters", "--yes"); code != axi.ExitOK {
			t.Fatal("tag rm failed")
		}
		if n := len(cap.matching(http.MethodDelete, "/api/tags/7")); n != 1 {
			t.Fatalf("expected DELETE /api/tags/7, got %+v", cap.calls)
		}
	})

	t.Run("an unknown path is not_found, not a bad request", func(t *testing.T) {
		out, code := exec(t, "tag", "rm", "Nope", "--yes")
		if code != axi.ExitError {
			t.Fatalf("exit = %d\n%s", code, out)
		}
		if !strings.Contains(out, "code: not_found") {
			t.Fatalf("got:\n%s", out)
		}
	})
}

func TestImportRetry(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/imports/9/retry": cap.record(`{"asset_id":511,"import_id":9}`),
	})

	out, code := exec(t, "import", "retry", "9")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "import_id: 9") {
		t.Fatalf("got:\n%s", out)
	}

	out, code = exec(t, "import", "9")
	if code != axi.ExitUsage {
		t.Fatalf("a missing verb should exit 2, got %d\n%s", code, out)
	}
}

func TestSetup(t *testing.T) {
	isolate(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	out, code := exec(t, "setup")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}

	skill := filepath.Join(home, ".claude", "skills", "damson", "SKILL.md")
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("skill not written: %v", err)
	}
	// The skill has to teach the output contract, not just list commands.
	for _, want := range []string{"---\nname: damson", "TOON", "exit code", "--fields", "--as asset", "--yes"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("skill omits %q", want)
		}
	}
	// And must never carry a credential.
	if strings.Contains(string(body), "damson_pat_") {
		t.Error("the skill must not contain a token")
	}

	hook := filepath.Join(home, ".claude", "hooks", "damson-session-start.sh")
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatalf("hook not written: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("hook is not executable: %04o", info.Mode().Perm())
	}

	t.Run("idempotent", func(t *testing.T) {
		if _, code := exec(t, "setup"); code != axi.ExitOK {
			t.Fatal("second run failed")
		}
		again, err := os.ReadFile(skill)
		if err != nil || string(again) != string(body) {
			t.Fatal("rerunning setup should reproduce the same skill")
		}
	})

	t.Run("--no-hook", func(t *testing.T) {
		fresh := t.TempDir()
		t.Setenv("HOME", fresh)
		if _, code := exec(t, "setup", "--no-hook"); code != axi.ExitOK {
			t.Fatal("setup failed")
		}
		if _, err := os.Stat(filepath.Join(fresh, ".claude", "hooks", "damson-session-start.sh")); !os.IsNotExist(err) {
			t.Fatal("--no-hook still wrote a hook")
		}
	})

	t.Run("--project writes beside the working directory", func(t *testing.T) {
		wd := t.TempDir()
		prev, _ := os.Getwd()
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		defer os.Chdir(prev)
		if _, code := exec(t, "setup", "--project"); code != axi.ExitOK {
			t.Fatal("setup failed")
		}
		if _, err := os.Stat(filepath.Join(wd, ".claude", "skills", "damson", "SKILL.md")); err != nil {
			t.Fatalf("project skill not written: %v", err)
		}
	})
}

// The server normalises tag paths (lowercase, collapsed slashes), so a
// user-typed path must be compared on its normalised form — otherwise every
// attach reports "attached" and a no-op looks like a change.
func TestAssetTag_MatchesTheServersNormalisation(t *testing.T) {
	isolate(t)
	cap := &capture{}
	// The asset already holds the normalised form of "Characters/Humanoid".
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/511": cap.record(
			`{"id":511,"name":"RPG Builder","tags":[{"id":7,"path":"characters/humanoid"}]}`),
		"/api/assets/511/tags": cap.record(`{}`),
	})

	for _, typed := range []string{"Characters/Humanoid", "characters//humanoid", " Characters/Humanoid "} {
		cap.calls = nil
		out, code := exec(t, "asset", "tag", "511", typed)
		if code != axi.ExitOK {
			t.Fatalf("%q: exit = %d\n%s", typed, code, out)
		}
		if !strings.Contains(out, "unchanged") || !strings.Contains(out, "changed: 0") {
			t.Fatalf("%q: an already-held tag reported as new:\n%s", typed, out)
		}
		if n := len(cap.matching(http.MethodPost, "/api/assets/511/tags")); n != 0 {
			t.Fatalf("%q: re-attached an already-held tag (%d POSTs)", typed, n)
		}
	}
}

func TestAssetRecord_SendsLocationAndReportsTheOutcome(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/references": cap.record(`{"outcome":"updated","asset_id":41,
		  "name":"Kingdom","location_path":"/mnt/nfs/Kingdom"}`),
	})

	out, code := exec(t, "asset", "record", "--location-path", "/mnt/nfs/Kingdom", "--needs-review")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	calls := cap.matching(http.MethodPost, "/api/assets/references")
	if len(calls) != 1 {
		t.Fatalf("expected one POST, got %+v", cap.calls)
	}
	body := calls[0].Body
	if body["location_path"] != "/mnt/nfs/Kingdom" || body["needs_review"] != true {
		t.Fatalf("body = %v", body)
	}
	if _, sent := body["source_url"]; sent {
		t.Fatalf("an unset link was sent: %v", body)
	}
	if !strings.Contains(out, "outcome: updated") || strings.Contains(out, "added") {
		t.Fatalf("a known path must read as updated, not added:\n%s", out)
	}
}

// One call records a scanned pack with everything the scan read from its path.
func TestAssetRecord_SendsPublisherAndTags(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/references": cap.record(`{"outcome":"created","asset_id":42,"name":"Kingdom"}`),
	})

	out, code := exec(t, "asset", "record", "--location-path", "/mnt/nfs/3D/Environments/Synty/Kingdom",
		"--publisher", "Synty", "--tags", "3D, Environments", "--needs-review")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	body := cap.matching(http.MethodPost, "/api/assets/references")[0].Body
	tags, _ := body["tags"].([]any)
	if body["publisher"] != "Synty" || len(tags) != 2 || tags[0] != "3D" || tags[1] != "Environments" {
		t.Fatalf("body = %v", body)
	}

	out, code = exec(t, "asset", "edit", "42", "--name", "x", "--tags", "3D")
	if code != axi.ExitUsage || !strings.Contains(out, "--tags only applies to asset record") {
		t.Fatalf("--tags outside record: exit = %d\n%s", code, out)
	}
}

func TestAssetRecord_DefaultsNeedsReviewOffAndNamesAnExistingAsset(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/references": cap.record(`{"outcome":"exists","asset_id":9,"name":"Imported Kingdom"}`),
	})

	out, code := exec(t, "asset", "record", "--source-url", "https://assetstore.unity.com/packages/k-1")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if body := cap.matching(http.MethodPost, "/api/assets/references")[0].Body; body["needs_review"] != false {
		t.Fatalf("needs_review should default off: %v", body)
	}
	if !strings.Contains(out, "already_in_library") || !strings.Contains(out, "Imported Kingdom") {
		t.Fatalf("output should name the existing asset:\n%s", out)
	}
}

func TestAssetRecord_RequiresALocationAndAskedNamesAreUsage(t *testing.T) {
	isolate(t)
	withServer(t, map[string]http.HandlerFunc{
		"/api/assets/references": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":"name is required: the path and store link give no name"}`))
		},
	})

	out, code := exec(t, "asset", "record", "--name", "x")
	if code != axi.ExitUsage || !strings.Contains(out, "--location-path, --source-url, or both") {
		t.Fatalf("no location: exit = %d\n%s", code, out)
	}

	out, code = exec(t, "asset", "record", "--location-path", "/mnt/p", "--description", "d")
	if code != axi.ExitUsage || !strings.Contains(out, "--description does not apply to asset record") {
		t.Fatalf("an edit-only flag should be refused: exit = %d\n%s", code, out)
	}

	out, code = exec(t, "asset", "record", "--source-url", "https://example.com/broken")
	if code != axi.ExitUsage || !strings.Contains(out, "name is required") || !strings.Contains(out, "--name") {
		t.Fatalf("nameless record should be a usage error pointing at --name: exit = %d\n%s", code, out)
	}
}

func TestAssetEdit_LocationPath(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{"/api/assets/511": cap.record(assetDetail)})

	if out, code := exec(t, "asset", "edit", "511", "--clear", "location-path"); code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	body := cap.matching(http.MethodPatch, "/api/assets/511")[0].Body
	if v, ok := body["location_path"]; !ok || v != "" {
		t.Fatalf("--clear location-path should send an empty location_path: %v", body)
	}
}

func TestAsset_NeedsReviewOutsideRecordIsRefused(t *testing.T) {
	isolate(t)
	cap := &capture{}
	withServer(t, map[string]http.HandlerFunc{"/api/assets/511": cap.record(assetDetail)})

	out, code := exec(t, "asset", "edit", "511", "--name", "x", "--needs-review")
	if code != axi.ExitUsage || !strings.Contains(out, "only applies to asset record") {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if len(cap.calls) != 0 {
		t.Fatalf("a refused command still reached the server: %+v", cap.calls)
	}
}
