package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/thomas-elliott/damson/internal/axi"
)

// received is one upload the fake server saw.
type received struct {
	Filename     string
	Body         []byte
	AssetID      string
	StoreVersion string
}

type uploadRecorder struct {
	mu       sync.Mutex
	items    []received
	inFlight atomic.Int32
	peak     atomic.Int32
	release  chan struct{}
}

func (r *uploadRecorder) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		n := r.inFlight.Add(1)
		for {
			p := r.peak.Load()
			if n <= p || r.peak.CompareAndSwap(p, n) {
				break
			}
		}
		if r.release != nil {
			<-r.release
		}
		defer r.inFlight.Add(-1)

		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("content type: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mr := multipart.NewReader(req.Body, params["boundary"])
		var got received
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("multipart: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			b, _ := io.ReadAll(part)
			switch part.FormName() {
			case "file":
				got.Filename = part.FileName()
				got.Body = b
			case "asset_id":
				got.AssetID = string(b)
			case "store_version":
				got.StoreVersion = string(b)
			}
		}

		r.mu.Lock()
		r.items = append(r.items, got)
		id := uint(100 + len(r.items))
		r.mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"asset_id":` + strconv.Itoa(int(id)) + `,"import_id":` + strconv.Itoa(int(id)) + `}`))
	}
}

func (r *uploadRecorder) sorted() []received {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]received(nil), r.items...)
	sort.Slice(out, func(i, j int) bool { return out[i].Filename < out[j].Filename })
	return out
}

// tree builds a directory layout from path→contents and returns its root.
func tree(t *testing.T, layout map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range layout {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

func uploadServer(t *testing.T, rec *uploadRecorder, extra map[string]http.HandlerFunc) {
	t.Helper()
	handlers := map[string]http.HandlerFunc{
		"/api/imports":  rec.handler(t),
		"/api/settings": serveJSON(nil, `{"max_upload_bytes":1048576}`),
		"/api/imports/preflight": serveJSON(nil,
			`{"results":[],"library_total":0}`),
	}
	for k, v := range extra {
		handlers[k] = v
	}
	withServer(t, handlers)
}

// A directory is ambiguous, so --as is required and the error hands back both
// concrete commands (AXI principle 6: no prompts, no guessing).
func TestUpload_DirectoryRequiresAs(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack/a.txt": "a"})
	dir := filepath.Join(root, "Pack")

	out, code := exec(t, "upload", dir)
	if code != axi.ExitUsage {
		t.Fatalf("exit = %d, want %d\n%s", code, axi.ExitUsage, out)
	}
	if !strings.Contains(out, "is a directory; --as is required") {
		t.Fatalf("got:\n%s", out)
	}
	if !strings.Contains(out, "--as asset") || !strings.Contains(out, "--as assets") {
		t.Fatalf("both templates must be offered:\n%s", out)
	}
}

func TestUpload_File(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack.unitypackage": "payload"})
	rec := &uploadRecorder{}
	uploadServer(t, rec, nil)

	out, code := exec(t, "upload", filepath.Join(root, "Pack.unitypackage"))
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	got := rec.sorted()
	if len(got) != 1 || got[0].Filename != "Pack.unitypackage" {
		t.Fatalf("uploads = %+v", got)
	}
	if string(got[0].Body) != "payload" {
		t.Fatalf("body = %q", got[0].Body)
	}
	if !strings.Contains(out, "imported: 1") {
		t.Fatalf("summary missing:\n%s", out)
	}
}

// --as asset zips the folder, so the server's existing zip extractor writes
// the files with their relative paths preserved.
func TestUpload_AsAssetZipsTheDirectory(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{
		"Pack/a.txt":       "alpha",
		"Pack/Sub/b.txt":   "beta",
		"Pack/Sub/c/d.txt": "delta",
	})
	rec := &uploadRecorder{}
	uploadServer(t, rec, nil)

	out, code := exec(t, "upload", filepath.Join(root, "Pack"), "--as", "asset")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	got := rec.sorted()
	if len(got) != 1 {
		t.Fatalf("expected one archive, got %d", len(got))
	}
	if got[0].Filename != "Pack.zip" {
		t.Fatalf("filename = %q — the package kind is inferred from the extension", got[0].Filename)
	}

	zr, err := zip.NewReader(bytes.NewReader(got[0].Body), int64(len(got[0].Body)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	want := []string{"Sub/b.txt", "Sub/c/d.txt", "a.txt"}
	sort.Strings(want)
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("archive entries = %v, want %v", names, want)
	}
	// Paths are relative to the folder, with forward slashes.
	for _, n := range names {
		if strings.HasPrefix(n, "/") || strings.Contains(n, "\\") || strings.Contains(n, "..") {
			t.Fatalf("unsafe archive path %q", n)
		}
	}
}

// --as assets treats each direct child as its own asset: a file goes as-is, a
// subdirectory is zipped.
func TestUpload_AsAssetsSplitsChildren(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{
		"Packs/One.unitypackage": "one",
		"Packs/Two/inner.txt":    "two",
		"Packs/.hidden":          "skip me",
	})
	rec := &uploadRecorder{}
	uploadServer(t, rec, nil)

	out, code := exec(t, "upload", filepath.Join(root, "Packs"), "--as", "assets")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	got := rec.sorted()
	if len(got) != 2 {
		t.Fatalf("expected 2 uploads, got %d: %+v", len(got), got)
	}
	if got[0].Filename != "One.unitypackage" || string(got[0].Body) != "one" {
		t.Fatalf("file child wrong: %+v", got[0])
	}
	if got[1].Filename != "Two.zip" {
		t.Fatalf("directory child should be zipped: %q", got[1].Filename)
	}
	for _, g := range got {
		if strings.HasPrefix(g.Filename, ".") {
			t.Fatalf("a dotfile was uploaded: %q", g.Filename)
		}
	}
}

func TestUpload_PassesAssetIDAndStoreVersion(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack.unitypackage": "payload"})
	rec := &uploadRecorder{}
	uploadServer(t, rec, nil)

	_, code := exec(t, "upload", filepath.Join(root, "Pack.unitypackage"),
		"--asset-id", "511", "--store-version", "1.4")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	got := rec.sorted()
	if got[0].AssetID != "511" || got[0].StoreVersion != "1.4" {
		t.Fatalf("form fields = %+v", got[0])
	}
}

// --asset-id names one asset, so it cannot apply to a plan that resolved to
// several uploads.
func TestUpload_AssetIDRejectsAMultiItemPlan(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Packs/a.unitypackage": "a", "Packs/b.unitypackage": "b"})
	uploadServer(t, &uploadRecorder{}, nil)

	out, code := exec(t, "upload", filepath.Join(root, "Packs"), "--as", "assets", "--asset-id", "511")
	if code != axi.ExitUsage {
		t.Fatalf("exit = %d, want %d\n%s", code, axi.ExitUsage, out)
	}
}

func TestUpload_SkipExisting(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Packs/Held.unitypackage": "a", "Packs/Fresh.unitypackage": "b"})
	rec := &uploadRecorder{}
	uploadServer(t, rec, map[string]http.HandlerFunc{
		"/api/imports/preflight": func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Entries []struct {
					Filename string `json:"filename"`
				} `json:"entries"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			var results []string
			for i, e := range req.Entries {
				status := "new"
				extra := ""
				if strings.HasPrefix(e.Filename, "Held") {
					status = "name_match"
					extra = `,"asset_id":511,"asset_name":"Held","version_count":2`
				}
				results = append(results, `{"index":`+strconv.Itoa(i)+`,"status":"`+status+`"`+extra+`}`)
			}
			_, _ = w.Write([]byte(`{"results":[` + strings.Join(results, ",") + `],"library_total":9}`))
		},
	})

	out, code := exec(t, "upload", filepath.Join(root, "Packs"), "--as", "assets", "--skip-existing")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	got := rec.sorted()
	if len(got) != 1 || got[0].Filename != "Fresh.unitypackage" {
		t.Fatalf("only the unheld file should upload, got %+v", got)
	}
	if !strings.Contains(out, "skipped: 1") || !strings.Contains(out, "imported: 1") {
		t.Fatalf("summary:\n%s", out)
	}

	// Without the flag, a name match is uploaded anyway.
	rec2 := &uploadRecorder{}
	uploadServer(t, rec2, nil)
	if _, code := exec(t, "upload", filepath.Join(root, "Packs"), "--as", "assets"); code != axi.ExitOK {
		t.Fatal("upload failed")
	}
	if len(rec2.sorted()) != 2 {
		t.Fatalf("without --skip-existing everything uploads, got %d", len(rec2.sorted()))
	}
}

// --dry-run must transfer nothing, and must say that a skip is name-only —
// it cannot tell a duplicate from a newer version.
func TestUpload_DryRun(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack.unitypackage": "payload"})
	rec := &uploadRecorder{}
	uploadServer(t, rec, map[string]http.HandlerFunc{
		"/api/imports/preflight": serveJSON(nil,
			`{"results":[{"index":0,"status":"name_match","asset_id":511,"asset_name":"Pack","version_count":2}],"library_total":9}`),
	})

	out, code := exec(t, "upload", filepath.Join(root, "Pack.unitypackage"), "--dry-run", "--skip-existing")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if len(rec.sorted()) != 0 {
		t.Fatal("--dry-run uploaded something")
	}
	if !strings.Contains(out, "dry_run: true") || !strings.Contains(out, "skip: 1") {
		t.Fatalf("got:\n%s", out)
	}
	if !strings.Contains(out, "cannot tell a duplicate from a newer version") {
		t.Fatalf("the conservatism of a name-only skip must be stated:\n%s", out)
	}
}

// A file over the server's cap is reported before it is transferred.
func TestUpload_RefusesOversizeBeforeTransferring(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Big.unitypackage": strings.Repeat("x", 4096)})
	rec := &uploadRecorder{}
	uploadServer(t, rec, map[string]http.HandlerFunc{
		"/api/settings": serveJSON(nil, `{"max_upload_bytes":1024}`),
	})

	out, code := exec(t, "upload", filepath.Join(root, "Big.unitypackage"))
	if code != axi.ExitError {
		t.Fatalf("exit = %d, want %d\n%s", code, axi.ExitError, out)
	}
	if len(rec.sorted()) != 0 {
		t.Fatal("an oversize file was transferred anyway")
	}
	if !strings.Contains(out, "exceeds the server limit") {
		t.Fatalf("got:\n%s", out)
	}
}

// One bad file must not abort the rest, and the exit code must still say the
// run did not fully succeed.
func TestUpload_PartialFailureContinuesAndExitsNonZero(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{
		"Packs/good.unitypackage": "ok",
		"Packs/bad.unitypackage":  "boom",
	})
	var uploaded atomic.Int32
	withServer(t, map[string]http.HandlerFunc{
		"/api/settings": serveJSON(nil, `{"max_upload_bytes":1048576}`),
		"/api/imports": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if bytes.Contains(body, []byte("boom")) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":"store_version already exists"}`))
				return
			}
			uploaded.Add(1)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"asset_id":1,"import_id":1}`))
		},
	})

	out, code := exec(t, "upload", filepath.Join(root, "Packs"), "--as", "assets")
	if code != axi.ExitError {
		t.Fatalf("exit = %d, want %d\n%s", code, axi.ExitError, out)
	}
	if n := uploaded.Load(); n != 1 {
		t.Fatalf("the good file should still have uploaded, got %d", n)
	}
	if !strings.Contains(out, "imported: 1") || !strings.Contains(out, "failed: 1") {
		t.Fatalf("summary should report both:\n%s", out)
	}
	// The server's reason has to reach the user.
	if !strings.Contains(out, "store_version already exists") {
		t.Fatalf("the server's message was lost:\n%s", out)
	}
}

// --concurrency bounds how many transfers are in flight at once.
func TestUpload_ConcurrencyIsBounded(t *testing.T) {
	isolate(t)
	layout := map[string]string{}
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		layout["Packs/"+n+".unitypackage"] = n
	}
	root := tree(t, layout)

	rec := &uploadRecorder{release: make(chan struct{})}
	uploadServer(t, rec, nil)
	// Let every request through only once they have all had a chance to
	// arrive, so the peak reflects the real limit.
	go func() {
		for i := 0; i < 6; i++ {
			rec.release <- struct{}{}
		}
	}()

	if _, code := exec(t, "upload", filepath.Join(root, "Packs"), "--as", "assets", "--concurrency", "2"); code != axi.ExitOK {
		t.Fatal("upload failed")
	}
	if peak := rec.peak.Load(); peak > 2 {
		t.Fatalf("peak in-flight uploads = %d, want at most 2", peak)
	}
	if len(rec.sorted()) != 6 {
		t.Fatalf("uploads = %d, want 6", len(rec.sorted()))
	}
}

func TestUpload_Wait(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack.unitypackage": "payload"})
	rec := &uploadRecorder{}
	uploadServer(t, rec, map[string]http.HandlerFunc{
		"/api/imports/101": serveJSON(nil, `{"stage":"done"}`),
	})

	out, code := exec(t, "upload", filepath.Join(root, "Pack.unitypackage"), "--wait")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "stage") || !strings.Contains(out, "done") {
		t.Fatalf("--wait should report the terminal stage:\n%s", out)
	}
}

// An import that settles as failed must set the exit code, not just be listed.
func TestUpload_WaitSurfacesAFailedImport(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack.unitypackage": "payload"})
	rec := &uploadRecorder{}
	uploadServer(t, rec, map[string]http.HandlerFunc{
		"/api/imports/101": serveJSON(nil, `{"stage":"failed","error":"extract: unexpected EOF"}`),
	})

	out, code := exec(t, "upload", filepath.Join(root, "Pack.unitypackage"), "--wait")
	if code != axi.ExitError {
		t.Fatalf("exit = %d, want %d\n%s", code, axi.ExitError, out)
	}
	if !strings.Contains(out, "unexpected EOF") {
		t.Fatalf("the import's error should be reported:\n%s", out)
	}
}

func TestUpload_TempArchivesAreCleanedUp(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack/a.txt": "alpha"})
	rec := &uploadRecorder{}
	uploadServer(t, rec, nil)

	before := countTempArchives(t)
	if _, code := exec(t, "upload", filepath.Join(root, "Pack"), "--as", "asset"); code != axi.ExitOK {
		t.Fatal("upload failed")
	}
	if after := countTempArchives(t); after != before {
		t.Fatalf("temp archives leaked: %d before, %d after", before, after)
	}
}

func countTempArchives(t *testing.T) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "dmsn-*.zip"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return len(matches)
}

func TestUpload_Validation(t *testing.T) {
	isolate(t)
	root := tree(t, map[string]string{"Pack/a.txt": "a"})
	uploadServer(t, &uploadRecorder{}, nil)

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"upload"}, "at least one path"},
		{[]string{"upload", filepath.Join(root, "Pack"), "--as", "thing"}, `--as must be`},
		{[]string{"upload", filepath.Join(root, "Pack"), "--as", "asset", "--concurrency", "0"}, "--concurrency"},
		{[]string{"upload", filepath.Join(root, "Pack"), "--as", "asset", "--concurrency", "99"}, "--concurrency"},
		{[]string{"upload", filepath.Join(root, "nope")}, "cannot read"},
	} {
		out, code := exec(t, tc.args...)
		if code == axi.ExitOK {
			t.Errorf("%v: expected a failure\n%s", tc.args, out)
		}
		if !strings.Contains(out, tc.want) {
			t.Errorf("%v: want %q:\n%s", tc.args, tc.want, out)
		}
	}
}
