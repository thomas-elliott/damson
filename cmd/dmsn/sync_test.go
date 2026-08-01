package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/importsource"
)

// cacheTree builds a Unity-shaped cache under a temp root.
func cacheTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

var sampleCache = map[string]string{
	"Blink/Editor ExtensionsRPG Toolkits/RPG Builder.unitypackage": "rpg-builder-bytes",
	"Demigiant/ScriptingAnimation/DOTween HOTween v2.unitypackage": "dotween",
	"A2 Games/3D ModelsCharacters/Satomi.unitypackage":             "satomi",
	"Demigiant/ScriptingAnimation/.DOTween-content__1.tmp":         "partial",
	"Demigiant/ScriptingAnimation/.DOTween-content__1.tmp.json":    "{}",
	"Demigiant/ScriptingAnimation/readme.txt":                      "not a package",
}

// preflightRecorder captures the manifests sync sends and replies with a
// caller-supplied status per relative path.
type preflightRecorder struct {
	mu       sync.Mutex
	requests [][]map[string]any
	byPath   map[string]map[string]any
}

func (p *preflightRecorder) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Entries []map[string]any `json:"entries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		p.mu.Lock()
		p.requests = append(p.requests, body.Entries)
		p.mu.Unlock()

		results := make([]map[string]any, 0, len(body.Entries))
		for i, e := range body.Entries {
			res := map[string]any{"index": i, "status": importsource.StatusNew}
			if canned, ok := p.byPath[fmt.Sprint(e["relative_path"])]; ok {
				for k, v := range canned {
					res[k] = v
				}
				res["index"] = i
			}
			results = append(results, res)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": results, "library_total": 3,
		})
	}
}

func (p *preflightRecorder) requestCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

func syncServer(t *testing.T, rec *uploadRecorder, pre *preflightRecorder) {
	t.Helper()
	withServer(t, map[string]http.HandlerFunc{
		"/api/imports":           rec.handler(t),
		"/api/settings":          serveJSON(nil, `{"max_upload_bytes":1048576}`),
		"/api/imports/preflight": pre.handler(),
	})
}

func TestSync_RequiresSubcommand(t *testing.T) {
	isolate(t)
	out, code := exec(t, "sync")
	if code != axi.ExitUsage {
		t.Fatalf("exit = %d, want %d\n%s", code, axi.ExitUsage, out)
	}
	if !strings.Contains(out, "scan") || !strings.Contains(out, "push") {
		t.Fatalf("both subcommands should be offered:\n%s", out)
	}

	out, code = exec(t, "sync", "pull")
	if code != axi.ExitUsage {
		t.Fatalf("unknown subcommand exit = %d, want %d\n%s", code, axi.ExitUsage, out)
	}
}

func TestSync_ScanReportsCacheAndStatuses(t *testing.T) {
	isolate(t)
	root := cacheTree(t, sampleCache)
	pre := &preflightRecorder{byPath: map[string]map[string]any{
		"Blink/Editor ExtensionsRPG Toolkits/RPG Builder.unitypackage": {
			"status": importsource.StatusAlreadyImported, "asset_id": 7, "asset_name": "RPG Builder",
		},
	}}
	syncServer(t, &uploadRecorder{}, pre)

	out, code := exec(t, "sync", "scan", "--cache-root", root)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	// Three packages: the .tmp pair and the .txt must not appear.
	if !strings.Contains(out, "packages: 3") {
		t.Fatalf("want 3 packages (dotfiles and non-packages excluded):\n%s", out)
	}
	if strings.Contains(out, "readme") || strings.Contains(out, ".tmp") {
		t.Fatalf("non-package files leaked into the scan:\n%s", out)
	}
	for _, want := range []string{"Blink", "Demigiant", "A2 Games", "already_imported"} {
		if !strings.Contains(out, want) {
			t.Fatalf("scan output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "already_imported: 1") || !strings.Contains(out, "new: 2") {
		t.Fatalf("status counts wrong:\n%s", out)
	}
}

func TestSync_PushSendsSourceHintsAndSkipsHeld(t *testing.T) {
	isolate(t)
	root := cacheTree(t, sampleCache)
	rec := &uploadRecorder{}
	pre := &preflightRecorder{byPath: map[string]map[string]any{
		"Blink/Editor ExtensionsRPG Toolkits/RPG Builder.unitypackage": {
			"status": importsource.StatusAlreadyImported, "asset_id": 7, "asset_name": "RPG Builder",
		},
		"Demigiant/ScriptingAnimation/DOTween HOTween v2.unitypackage": {
			"status": importsource.StatusUpdated, "asset_id": 9,
			"asset_name": "DOTween HOTween", "held_bytes": 11,
		},
	}}
	syncServer(t, rec, pre)

	out, code := exec(t, "sync", "push", "--cache-root", root)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}

	got := rec.sorted()
	if len(got) != 2 {
		t.Fatalf("uploaded %d packages, want 2 (already_imported must be skipped): %#v", len(got), got)
	}

	byName := map[string]received{}
	for _, r := range got {
		byName[r.Filename] = r
	}
	if _, held := byName["RPG Builder.unitypackage"]; held {
		t.Error("an already_imported package was uploaded")
	}

	dot := byName["DOTween HOTween v2.unitypackage"]
	if dot.SourceKind != importsource.KindUnityCache {
		t.Errorf("source_kind = %q, want %q", dot.SourceKind, importsource.KindUnityCache)
	}
	if dot.SourceRelativePath != "Demigiant/ScriptingAnimation/DOTween HOTween v2.unitypackage" {
		t.Errorf("source_relative_path = %q", dot.SourceRelativePath)
	}
	if dot.AssetID != "9" {
		t.Errorf("an updated package must target its asset: asset_id = %q, want 9", dot.AssetID)
	}
	if string(dot.Body) != "dotween" {
		t.Errorf("body = %q — the file must be read from the cache root, not the relative path", dot.Body)
	}

	satomi := byName["Satomi.unitypackage"]
	if satomi.AssetID != "" {
		t.Errorf("a new package must not target an asset: asset_id = %q", satomi.AssetID)
	}
	if satomi.SourceRelativePath != "A2 Games/3D ModelsCharacters/Satomi.unitypackage" {
		t.Errorf("source_relative_path = %q", satomi.SourceRelativePath)
	}
}

func TestSync_AdoptVersions(t *testing.T) {
	isolate(t)
	root := cacheTree(t, map[string]string{
		"Blink/Tools/RPG Builder.unitypackage": "bytes",
	})
	pre := &preflightRecorder{byPath: map[string]map[string]any{
		"Blink/Tools/RPG Builder.unitypackage": {
			"status": importsource.StatusNameMatch, "asset_id": 7, "asset_name": "RPG Builder",
		},
	}}

	rec := &uploadRecorder{}
	syncServer(t, rec, pre)
	out, code := exec(t, "sync", "push", "--cache-root", root)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if len(rec.sorted()) != 0 {
		t.Fatalf("a name_match must be skipped without --adopt-versions: %#v", rec.sorted())
	}
	if !strings.Contains(out, "--adopt-versions") {
		t.Fatalf("the skip should say how to adopt:\n%s", out)
	}

	rec2 := &uploadRecorder{}
	syncServer(t, rec2, pre)
	out, code = exec(t, "sync", "push", "--cache-root", root, "--adopt-versions")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	got := rec2.sorted()
	if len(got) != 1 {
		t.Fatalf("--adopt-versions should upload the name match: %#v", got)
	}
	if got[0].AssetID != "7" {
		t.Errorf("adopted upload asset_id = %q, want 7", got[0].AssetID)
	}
	if got[0].SourceRelativePath == "" {
		t.Error("an adopted upload must still carry its source path, or the next sync cannot answer precisely")
	}
}

func TestSync_DryRunUploadsNothing(t *testing.T) {
	isolate(t)
	root := cacheTree(t, sampleCache)
	rec := &uploadRecorder{}
	syncServer(t, rec, &preflightRecorder{})

	out, code := exec(t, "sync", "push", "--cache-root", root, "--dry-run")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if len(rec.sorted()) != 0 {
		t.Fatalf("--dry-run uploaded %d packages", len(rec.sorted()))
	}
	if !strings.Contains(out, "dry_run: true") {
		t.Fatalf("dry run not declared:\n%s", out)
	}
	if !strings.Contains(out, "upload: 3") {
		t.Fatalf("plan should count 3 uploads:\n%s", out)
	}
}

func TestSync_PublisherFilter(t *testing.T) {
	isolate(t)
	root := cacheTree(t, sampleCache)
	rec := &uploadRecorder{}
	syncServer(t, rec, &preflightRecorder{})

	out, code := exec(t, "sync", "push", "--cache-root", root, "--publisher", "demigiant")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	got := rec.sorted()
	if len(got) != 1 || got[0].Filename != "DOTween HOTween v2.unitypackage" {
		t.Fatalf("--publisher should match case-insensitively and narrow to one: %#v", got)
	}
}

func TestSync_ChunksLargeManifest(t *testing.T) {
	isolate(t)
	files := make(map[string]string, preflightChunk+5)
	for i := range preflightChunk + 5 {
		files[fmt.Sprintf("Pub/Audio/Pack %04d.unitypackage", i)] = "x"
	}
	root := cacheTree(t, files)
	pre := &preflightRecorder{}
	syncServer(t, &uploadRecorder{}, pre)

	out, code := exec(t, "sync", "scan", "--cache-root", root)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if pre.requestCount() != 2 {
		t.Fatalf("preflight requests = %d, want 2 (chunked at %d)", pre.requestCount(), preflightChunk)
	}
	pre.mu.Lock()
	first, second := len(pre.requests[0]), len(pre.requests[1])
	pre.mu.Unlock()
	if first != preflightChunk || second != 5 {
		t.Errorf("chunk sizes = %d, %d; want %d, 5", first, second, preflightChunk)
	}
	if !strings.Contains(out, fmt.Sprintf("packages: %d", preflightChunk+5)) {
		t.Errorf("every package must still be reported — no silent truncation")
	}
}

func TestSync_UnreadableCacheRoot(t *testing.T) {
	isolate(t)
	out, code := exec(t, "sync", "scan", "--cache-root", filepath.Join(t.TempDir(), "nope"))
	if code == axi.ExitOK {
		t.Fatalf("a missing cache root should fail:\n%s", out)
	}
	if !strings.Contains(out, "not a readable directory") {
		t.Fatalf("error should name the problem:\n%s", out)
	}
}

func TestSync_EmptyCacheIsDefinitive(t *testing.T) {
	isolate(t)
	root := cacheTree(t, map[string]string{"Pub/Audio/notes.txt": "x"})
	syncServer(t, &uploadRecorder{}, &preflightRecorder{})

	out, code := exec(t, "sync", "scan", "--cache-root", root)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "no .unitypackage files under") {
		t.Fatalf("empty state must say why, not just be silent:\n%s", out)
	}
}

func TestSync_NothingToDoIsDefinitive(t *testing.T) {
	isolate(t)
	root := cacheTree(t, map[string]string{"Blink/Tools/RPG Builder.unitypackage": "b"})
	pre := &preflightRecorder{byPath: map[string]map[string]any{
		"Blink/Tools/RPG Builder.unitypackage": {
			"status": importsource.StatusAlreadyImported, "asset_id": 7, "asset_name": "RPG Builder",
		},
	}}
	rec := &uploadRecorder{}
	syncServer(t, rec, pre)

	out, code := exec(t, "sync", "push", "--cache-root", root)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if len(rec.sorted()) != 0 {
		t.Fatal("nothing should have uploaded")
	}
	if !strings.Contains(out, "already holds every package") {
		t.Fatalf("empty state must be definitive:\n%s", out)
	}
}

func TestSync_JSONOutput(t *testing.T) {
	isolate(t)
	root := cacheTree(t, sampleCache)
	syncServer(t, &uploadRecorder{}, &preflightRecorder{})

	out, code := exec(t, "sync", "scan", "--cache-root", root, "--json")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("--json must emit one parseable document: %v\n%s", err, out)
	}
	if _, ok := payload["packages"]; !ok {
		t.Errorf("packages missing from JSON payload: %v", payload)
	}
}
