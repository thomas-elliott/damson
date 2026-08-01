package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/axi"
)

const overviewBody = `{
  "library": {"id": 7, "name": "Main", "role": "admin"},
  "counts": {
    "assets": 412, "needs_review": 3, "updates_available": 2,
    "collections": 5, "tags": 88, "projects": 1,
    "imports_in_flight": 1, "imports_failed": 2
  },
  "recent_imports": [
    {"id": 9, "asset_id": 511, "asset_name": "RPG Builder", "stage": "done",
     "updated_at": "2026-08-01T09:00:00Z"},
    {"id": 8, "asset_id": 512, "asset_name": "DOTween", "stage": "failed",
     "error": "extract: unexpected EOF", "updated_at": "2026-08-01T08:00:00Z"}
  ]
}`

func overviewServer(t *testing.T, body string) string {
	t.Helper()
	srv := fakeServer(t, map[string]http.HandlerFunc{
		"/api/overview": func(w http.ResponseWriter, r *http.Request) {
			requireBearer(t, r, testToken)
			_, _ = w.Write([]byte(body))
		},
	})
	if _, err := SaveConfig(Config{Server: srv.URL, Token: testToken}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	return srv.URL
}

func TestHome_ShowsLiveLibraryState(t *testing.T) {
	isolate(t)
	overviewServer(t, overviewBody)

	out, code := exec(t)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	for _, want := range []string{
		"name: Main", "assets: 412", "needs_review: 3", "updates_available: 2",
		"collections: 5", "tags: 88", "projects: 1",
		"imports_in_flight: 1", "imports_failed: 2",
		"recent_imports[2]{asset_id,asset_name,stage}:",
		"511,RPG Builder,done",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("home output missing %q:\n%s", want, out)
		}
	}
}

// A composite result must be ONE parseable document, not several concatenated.
func TestHome_JSONIsASingleDocument(t *testing.T) {
	isolate(t)
	overviewServer(t, overviewBody)

	out, code := exec(t, "--json")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	var got struct {
		Dmsn map[string]any `json:"dmsn"`
		Lib  struct {
			Name   string `json:"name"`
			Assets int    `json:"assets"`
		} `json:"library"`
		Recent []map[string]any `json:"recent_imports"`
		Help   []string         `json:"help"`
	}
	dec := json.NewDecoder(strings.NewReader(out))
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	// Anything after the first document means separate calls were concatenated.
	if dec.More() {
		t.Fatalf("output contains more than one JSON document:\n%s", out)
	}
	if got.Lib.Name != "Main" || got.Lib.Assets != 412 {
		t.Fatalf("library block = %+v", got.Lib)
	}
	if len(got.Recent) != 2 || len(got.Dmsn) == 0 || len(got.Help) == 0 {
		t.Fatalf("composite lost a part: %+v", got)
	}
}

// Contextual help leads with the work that is outstanding, so the next command
// follows from the state rather than a fixed menu (AXI principle 9).
func TestHome_HelpLeadsWithOutstandingWork(t *testing.T) {
	isolate(t)
	overviewServer(t, overviewBody)

	out, _ := exec(t)
	idx := strings.Index(out, "help[")
	if idx < 0 {
		t.Fatalf("no help block:\n%s", out)
	}
	help := out[idx:]
	failedAt := strings.Index(help, "dmsn imports --stage failed")
	reviewAt := strings.Index(help, "dmsn assets --needs-review")
	genericAt := strings.Index(help, "\n  dmsn assets\n")
	if failedAt < 0 || reviewAt < 0 || genericAt < 0 {
		t.Fatalf("help block:\n%s", help)
	}
	if !(failedAt < reviewAt && reviewAt < genericAt) {
		t.Fatalf("outstanding work should precede the generic entries:\n%s", help)
	}
}

func TestHome_QuietLibraryOffersNoBusywork(t *testing.T) {
	isolate(t)
	body := strings.NewReplacer(
		`"needs_review": 3`, `"needs_review": 0`,
		`"updates_available": 2`, `"updates_available": 0`,
		`"imports_failed": 2`, `"imports_failed": 0`,
	).Replace(overviewBody)
	overviewServer(t, body)

	out, _ := exec(t)
	for _, unwanted := range []string{"--needs-review", "--updates-available", "--stage failed"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("help offered %q with nothing to do:\n%s", unwanted, out)
		}
	}
}

// An empty result has to say why, and an empty library and a quiet one are
// different facts (AXI principle 5).
func TestHome_EmptyImportsAreNeverSilent(t *testing.T) {
	isolate(t)

	t.Run("empty library", func(t *testing.T) {
		body := strings.NewReplacer(
			`"assets": 412`, `"assets": 0`,
			`"recent_imports": [
    {"id": 9, "asset_id": 511, "asset_name": "RPG Builder", "stage": "done",
     "updated_at": "2026-08-01T09:00:00Z"},
    {"id": 8, "asset_id": 512, "asset_name": "DOTween", "stage": "failed",
     "error": "extract: unexpected EOF", "updated_at": "2026-08-01T08:00:00Z"}
  ]`, `"recent_imports": []`,
		).Replace(overviewBody)
		overviewServer(t, body)

		out, _ := exec(t)
		if !strings.Contains(out, "recent_imports[]: nothing imported yet") {
			t.Fatalf("got:\n%s", out)
		}
	})

	t.Run("populated but quiet library", func(t *testing.T) {
		body := strings.Replace(overviewBody, `"recent_imports": [
    {"id": 9, "asset_id": 511, "asset_name": "RPG Builder", "stage": "done",
     "updated_at": "2026-08-01T09:00:00Z"},
    {"id": 8, "asset_id": 512, "asset_name": "DOTween", "stage": "failed",
     "error": "extract: unexpected EOF", "updated_at": "2026-08-01T08:00:00Z"}
  ]`, `"recent_imports": []`, 1)
		overviewServer(t, body)

		out, _ := exec(t)
		if !strings.Contains(out, "412 assets in the library, none imported recently") {
			t.Fatalf("got:\n%s", out)
		}
	})
}

// A server that cannot be reached still produces a usable home view: the
// binary describes itself and says what is wrong.
func TestHome_UnreachableServerStillReportsState(t *testing.T) {
	isolate(t)
	if _, err := SaveConfig(Config{Server: "http://127.0.0.1:1", Token: testToken}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, code := exec(t)
	if code != axi.ExitOK {
		t.Fatalf("home should not fail on an unreachable server; exit = %d", code)
	}
	if !strings.Contains(out, "connected: false") || !strings.Contains(out, "reason:") {
		t.Fatalf("got:\n%s", out)
	}
	if !strings.Contains(out, "dmsn:") {
		t.Fatalf("the self-description should survive a connection failure:\n%s", out)
	}
}
