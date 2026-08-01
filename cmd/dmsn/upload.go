package main

import (
	"archive/zip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/importstage"
	"github.com/thomas-elliott/damson/internal/toon"
)

const (
	// asAsset zips a directory into one archive, so its contents become the
	// files of a single asset. asAssets treats each direct child as its own.
	asAsset  = "asset"
	asAssets = "assets"

	defaultConcurrency = 2
	maxConcurrency     = 8

	// waitPoll is how often --wait re-reads an import's stage, and waitLimit
	// bounds the whole wait so a stuck import cannot hang the caller forever.
	waitPoll  = 2 * time.Second
	waitLimit = 15 * time.Minute
)

type uploadFlags struct {
	as           string
	assetID      string
	storeVersion string
	skipExisting bool
	dryRun       bool
	wait         bool
	resume       bool
	concurrency  int
}

func (f *uploadFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.as, "as", "", "for a directory: `asset` (one asset) or `assets` (one per child)")
	fs.StringVar(&f.assetID, "asset-id", "", "add as a new version of this asset")
	fs.StringVar(&f.storeVersion, "store-version", "", "store version to record")
	fs.BoolVar(&f.skipExisting, "skip-existing", false, "skip anything the library already holds by name")
	fs.BoolVar(&f.dryRun, "dry-run", false, "report what would happen, upload nothing")
	fs.BoolVar(&f.wait, "wait", false, "poll each import to a terminal stage")
	fs.BoolVar(&f.resume, "resume", false, "send every package in resumable chunks, whatever its size")
	fs.IntVar(&f.concurrency, "concurrency", defaultConcurrency, "parallel uploads")
}

func init() {
	f := &uploadFlags{}
	register(&command{
		name:    "upload",
		summary: "upload files or folders into the library",
		usage: "usage: dmsn upload <path>... [--as asset|assets]\n\n" +
			"A file is unambiguous: one asset. A directory is not, so --as is\n" +
			"required for one:\n\n" +
			"  --as asset    zip the directory and import it as a single asset\n" +
			"  --as assets   import each direct child as its own asset\n\n" +
			"flags: --skip-existing --asset-id --store-version\n" +
			"       --concurrency --dry-run --wait --resume",
		flags: f.register,
		run: func(ctx context.Context, e *Env) error {
			return runUpload(ctx, e, f)
		},
	})
}

// item is one unit of work: a file to send as-is, or a directory to zip first.
// Its importOptions carry what varies per package: a cache sync targets a
// different asset and source path for each, where upload applies one set of
// flags to all of them.
type item struct {
	path     string
	filename string
	zipDir   bool
	size     int64
	opts     importOptions
	// note is a per-item explanation for the plan/report tables, set where the
	// caller knows something the item itself does not.
	note string
	// resumeKey identifies this payload across runs. Empty for a zipped
	// directory.
	resumeKey string
}

// importOptions are the non-file form fields of POST /api/imports.
type importOptions struct {
	assetID            string
	storeVersion       string
	sourceKind         string
	sourceRelativePath string
}

func (o importOptions) fields() map[string]string {
	out := map[string]string{}
	if o.assetID != "" {
		out["asset_id"] = o.assetID
	}
	if o.storeVersion != "" {
		out["store_version"] = o.storeVersion
	}
	if o.sourceKind != "" {
		out["source_kind"] = o.sourceKind
	}
	if o.sourceRelativePath != "" {
		out["source_relative_path"] = o.sourceRelativePath
	}
	return out
}

func runUpload(ctx context.Context, e *Env, f *uploadFlags) error {
	if len(e.Args) == 0 {
		return axi.Usagef(
			[]string{"dmsn upload ./Pack.unitypackage", "dmsn upload ./Packs --as assets"},
			"upload needs at least one path")
	}
	if f.concurrency < 1 || f.concurrency > maxConcurrency {
		return axi.Usagef(nil, "--concurrency must be between 1 and %d", maxConcurrency)
	}
	if f.as != "" && f.as != asAsset && f.as != asAssets {
		return axi.Usagef(
			[]string{"--as asset   (one asset)", "--as assets  (one asset per child)"},
			"--as must be %q or %q, got %q", asAsset, asAssets, f.as)
	}

	items, err := planUpload(e.Args, f.as)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return axi.Errorf(axi.CodeUsage, "nothing to upload in %s", strings.Join(e.Args, ", "))
	}
	if f.assetID != "" {
		if _, err := parsePositiveID(f.assetID, "--asset-id"); err != nil {
			return err
		}
		if len(items) != 1 {
			return axi.Usagef(nil,
				"--asset-id applies to a single upload, but %d were resolved", len(items))
		}
	}
	for i := range items {
		items[i].opts.assetID = f.assetID
		items[i].opts.storeVersion = f.storeVersion
	}

	client, err := e.Client()
	if err != nil {
		return err
	}

	maxBytes, _ := e.maxUploadBytes(ctx, client)
	skip := map[int]preflightMatch{}
	if f.skipExisting || f.dryRun {
		skip, err = e.preflight(ctx, client, items)
		if err != nil {
			return err
		}
	}

	if f.dryRun {
		return e.reportPlan(items, skip, maxBytes, f)
	}
	return e.performUpload(ctx, client, items, skip, maxBytes, f)
}

// planUpload turns the given paths into concrete work, refusing a directory
// that has no --as: "one asset" and "a folder of assets" are equally plausible
// readings and guessing at multi-gigabyte scale is worse than one extra turn.
func planUpload(paths []string, as string) ([]item, error) {
	var items []item
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, axi.Wrap(axi.CodeIO, err, "cannot read %s: %v", p, err)
		}
		if !info.IsDir() {
			items = append(items, item{
				path:      p,
				filename:  filepath.Base(p),
				size:      info.Size(),
				resumeKey: uploadResumeKey(p, info.Size(), info.ModTime()),
			})
			continue
		}
		if as == "" {
			return nil, axi.Usagef(
				[]string{
					fmt.Sprintf(`dmsn upload %q --as asset`, p),
					fmt.Sprintf(`dmsn upload %q --as assets`, p),
				},
				"%s is a directory; --as is required", p)
		}
		if as == asAsset {
			items = append(items, item{path: p, filename: zipName(p), zipDir: true})
			continue
		}
		children, err := os.ReadDir(p)
		if err != nil {
			return nil, axi.Wrap(axi.CodeIO, err, "cannot list %s: %v", p, err)
		}
		for _, ch := range children {
			if strings.HasPrefix(ch.Name(), ".") {
				continue
			}
			full := filepath.Join(p, ch.Name())
			if ch.IsDir() {
				items = append(items, item{path: full, filename: zipName(full), zipDir: true})
				continue
			}
			info, err := ch.Info()
			if err != nil {
				return nil, axi.Wrap(axi.CodeIO, err, "cannot read %s: %v", full, err)
			}
			items = append(items, item{
				path:      full,
				filename:  ch.Name(),
				size:      info.Size(),
				resumeKey: uploadResumeKey(full, info.Size(), info.ModTime()),
			})
		}
	}
	return items, nil
}

// zipName is the archive name a directory is sent under. `.zip` is a
// first-class package kind, so the server's existing extractor unpacks it with
// relative paths preserved — no new upload endpoint needed.
func zipName(dir string) string {
	return filepath.Base(strings.TrimRight(dir, string(filepath.Separator))) + ".zip"
}

type preflightMatch struct {
	AssetID      uint
	AssetName    string
	VersionCount int
}

// preflight asks the server which of these names it already holds. Name-only
// matching cannot tell a duplicate from a genuine new version, so
// --skip-existing is deliberately conservative and says so.
func (e *Env) preflight(ctx context.Context, client *Client, items []item) (map[int]preflightMatch, error) {
	type entry struct {
		Filename  string `json:"filename"`
		SizeBytes int64  `json:"size_bytes"`
	}
	req := struct {
		Entries []entry `json:"entries"`
	}{Entries: make([]entry, len(items))}
	for i, it := range items {
		req.Entries[i] = entry{Filename: it.filename, SizeBytes: it.size}
	}

	var resp struct {
		Results []struct {
			Index        int    `json:"index"`
			Status       string `json:"status"`
			AssetID      uint   `json:"asset_id"`
			AssetName    string `json:"asset_name"`
			VersionCount int    `json:"version_count"`
		} `json:"results"`
	}
	if err := client.PostJSON(ctx, "/api/imports/preflight", req, &resp); err != nil {
		return nil, err
	}

	out := map[int]preflightMatch{}
	for _, r := range resp.Results {
		if r.Status == "name_match" && r.Index >= 0 && r.Index < len(items) {
			out[r.Index] = preflightMatch{
				AssetID: r.AssetID, AssetName: r.AssetName, VersionCount: r.VersionCount,
			}
		}
	}
	return out, nil
}

// maxUploadBytes reads the server's cap so an oversized file is reported
// before it is transferred rather than after. Best-effort: 0 disables the
// pre-check and the server's 413 remains the backstop.
func (e *Env) maxUploadBytes(ctx context.Context, client *Client) (int64, error) {
	var s struct {
		MaxUploadBytes int64 `json:"max_upload_bytes"`
	}
	if err := client.GetJSON(ctx, "/api/settings", nil, &s); err != nil {
		return 0, err
	}
	return s.MaxUploadBytes, nil
}

func (e *Env) reportPlan(items []item, skip map[int]preflightMatch, maxBytes int64, f *uploadFlags) error {
	rows := make([]map[string]any, 0, len(items))
	counts := map[string]int{}
	for i, it := range items {
		action, note := plannedAction(i, it, skip, maxBytes, f)
		counts[action]++
		rows = append(rows, map[string]any{
			"filename": it.filename,
			"action":   action,
			"kind":     kindOf(it),
			"size":     sizeValue(it),
			"note":     blankToNil(note),
		})
	}

	table := axi.Table{
		Key:         "plan",
		Columns:     []string{"filename", "action", "kind"},
		Truncatable: []string{"note"},
		Rows:        rows,
		Total:       -1,
		EmptyNote:   "nothing to upload",
	}
	summary := Block{Key: "summary", Fields: []toon.KV{
		{Key: "upload", Value: counts["upload"]},
		{Key: "skip", Value: counts["skip"]},
		{Key: "too_large", Value: counts["too_large"]},
		{Key: "dry_run", Value: true},
	}}

	help := []string{"dmsn upload " + strings.Join(quoteAll(e.Args), " ") + dryRunSuffix(f)}
	if counts["skip"] > 0 {
		help = append(help,
			"a skip is a name match only — it cannot tell a duplicate from a newer version",
			"drop --skip-existing to upload them anyway")
	}
	return axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table}, help, e.Globals.options())
}

func plannedAction(i int, it item, skip map[int]preflightMatch, maxBytes int64, f *uploadFlags) (string, string) {
	if m, ok := skip[i]; ok && f.skipExisting {
		return "skip", fmt.Sprintf("matches asset %d %q (%s held)",
			m.AssetID, m.AssetName, axi.Count(m.VersionCount, "version"))
	}
	if maxBytes > 0 && !it.zipDir && it.size > maxBytes {
		return "too_large", fmt.Sprintf("%d bytes exceeds the server limit of %d", it.size, maxBytes)
	}
	if m, ok := skip[i]; ok {
		return "upload", fmt.Sprintf("name matches asset %d %q", m.AssetID, m.AssetName)
	}
	if it.zipDir {
		return "upload", "directory will be zipped before sending"
	}
	return "upload", ""
}

type uploadOutcome struct {
	filename string
	status   string
	assetID  uint
	importID uint
	stage    string
	detail   string
}

func (e *Env) performUpload(ctx context.Context, client *Client, items []item, skip map[int]preflightMatch, maxBytes int64, f *uploadFlags) error {
	outcomes := make([]uploadOutcome, len(items))

	// A fixed pool rather than a goroutine per item: a folder of two thousand
	// children would otherwise park two thousand stacks on the semaphore to do
	// --concurrency things at a time.
	queue := make(chan int)
	var wg sync.WaitGroup
	for range f.concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				outcomes[i] = e.uploadOne(ctx, client, items[i], maxBytes, f)
				if outcomes[i].detail == "" {
					outcomes[i].detail = items[i].note
				}
			}
		}()
	}
	for i, it := range items {
		if m, ok := skip[i]; ok && f.skipExisting {
			outcomes[i] = uploadOutcome{
				filename: it.filename, status: "skipped", assetID: m.AssetID,
				detail: fmt.Sprintf("already hold %q", m.AssetName),
			}
			continue
		}
		queue <- i
	}
	close(queue)
	wg.Wait()

	return e.reportOutcomes(ctx, client, outcomes, f)
}

// uploadOne sends a single item. A per-item failure is recorded and the run
// continues: one bad file in a folder of two hundred should not abort the
// other hundred and ninety-nine.
func (e *Env) uploadOne(ctx context.Context, client *Client, it item, maxBytes int64, f *uploadFlags) uploadOutcome {
	out := uploadOutcome{filename: it.filename}

	body, size, cleanup, err := openPayload(it)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		out.status, out.detail = "failed", axi.AsError(err).Message
		return out
	}
	defer body.Close()

	if maxBytes > 0 && size > maxBytes {
		out.status = "failed"
		out.detail = fmt.Sprintf("%d bytes exceeds the server limit of %d", size, maxBytes)
		return out
	}

	resp, err := e.send(ctx, client, it, body, size, f)
	if err != nil {
		out.status, out.detail = "failed", axi.AsError(err).Message
		return out
	}
	out.status = "imported"
	out.assetID = resp.AssetID
	out.importID = resp.ImportID
	if resp.SuggestedMatch != nil {
		out.detail = fmt.Sprintf("may be a version of asset %d %q",
			resp.SuggestedMatch.AssetID, resp.SuggestedMatch.Name)
	}

	if f.wait {
		stage, werr := client.WaitForImport(ctx, resp.ImportID)
		out.stage = stage
		if werr != nil {
			out.detail = axi.AsError(werr).Message
		}
		if stage == "failed" {
			out.status = "failed"
		}
	}
	return out
}

// send picks the transport: chunks above the threshold, or whenever --resume
// asks for them.
func (e *Env) send(ctx context.Context, client *Client, it item, body io.ReadCloser, size int64, f *uploadFlags) (*importEnqueueResponse, error) {
	if !f.resume && size < chunkedUploadThreshold {
		return client.PostImport(ctx, it.filename, body, it.opts)
	}
	ranged, ok := body.(seekableReaderAt)
	if !ok {
		return client.PostImport(ctx, it.filename, body, it.opts)
	}
	return client.UploadChunked(ctx, it.filename, ranged, size, it.opts, it.resumeKey, nil)
}

// openPayload returns the bytes to send. A directory is zipped to a temp file
// first so the request has a known length — which gives a real size check
// before transfer and makes a retry cheap.
func openPayload(it item) (io.ReadCloser, int64, func(), error) {
	if !it.zipDir {
		fh, err := os.Open(it.path)
		if err != nil {
			return nil, 0, nil, axi.Wrap(axi.CodeIO, err, "cannot open %s: %v", it.path, err)
		}
		return fh, it.size, nil, nil
	}

	tmp, err := os.CreateTemp("", "dmsn-*.zip")
	if err != nil {
		return nil, 0, nil, axi.Wrap(axi.CodeIO, err, "cannot create a temp archive: %v", err)
	}
	cleanup := func() { os.Remove(tmp.Name()) }

	if err := zipDirectory(it.path, tmp); err != nil {
		tmp.Close()
		return nil, 0, cleanup, err
	}
	info, err := tmp.Stat()
	if err != nil {
		tmp.Close()
		return nil, 0, cleanup, axi.Wrap(axi.CodeIO, err, "cannot size the archive: %v", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		return nil, 0, cleanup, axi.Wrap(axi.CodeIO, err, "cannot rewind the archive: %v", err)
	}
	return tmp, info.Size(), cleanup, nil
}

// zipDirectory writes dir into w with paths relative to dir, entries sorted so
// the archive is reproducible. Symlinks are skipped rather than followed: a
// link out of the tree would silently pull in unrelated files.
func zipDirectory(dir string, w io.Writer) error {
	zw := zip.NewWriter(w)
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		zw.Close()
		return axi.Wrap(axi.CodeIO, err, "cannot walk %s: %v", dir, err)
	}
	sort.Strings(paths)

	for _, p := range paths {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			zw.Close()
			return axi.Wrap(axi.CodeIO, err, "cannot relativise %s: %v", p, err)
		}
		entry, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			zw.Close()
			return axi.Wrap(axi.CodeIO, err, "cannot add %s: %v", rel, err)
		}
		fh, err := os.Open(p)
		if err != nil {
			zw.Close()
			return axi.Wrap(axi.CodeIO, err, "cannot read %s: %v", p, err)
		}
		_, copyErr := io.Copy(entry, fh)
		fh.Close()
		if copyErr != nil {
			zw.Close()
			return archiveError(copyErr, "cannot pack %s", rel)
		}
	}
	if err := zw.Close(); err != nil {
		return archiveError(err, "cannot finish the archive")
	}
	return nil
}

// archiveError classifies a write failure while building the temp archive.
// A full scratch volume gets its own code: the remedy is TMPDIR, not the
// upload.
func archiveError(err error, format string, args ...any) error {
	if errors.Is(err, syscall.ENOSPC) {
		e := axi.Wrap(axi.CodeNoSpace, err, "%s: out of space in %s", fmt.Sprintf(format, args...), os.TempDir())
		e.Help = []string{
			"free space on " + os.TempDir(),
			"or set TMPDIR to a volume with room for the zipped folder",
		}
		return e
	}
	return axi.Wrap(axi.CodeIO, err, "%s: %v", fmt.Sprintf(format, args...), err)
}

func (e *Env) reportOutcomes(ctx context.Context, client *Client, outcomes []uploadOutcome, f *uploadFlags) error {
	counts := map[string]int{}
	rows := make([]map[string]any, 0, len(outcomes))
	var failed []uploadOutcome
	for _, o := range outcomes {
		counts[o.status]++
		if o.status == "failed" {
			failed = append(failed, o)
		}
		rows = append(rows, map[string]any{
			"filename": o.filename,
			"status":   o.status,
			"asset_id": zeroToNil(o.assetID),
			"import":   zeroToNil(o.importID),
			"stage":    blankToNil(o.stage),
			"detail":   blankToNil(o.detail),
		})
	}

	columns := []string{"filename", "status", "asset_id"}
	if f.wait {
		columns = append(columns, "stage")
	}
	// Why something failed is the whole point of the report when something
	// did, so widen rather than making the reader run a second command.
	if len(failed) > 0 {
		columns = append(columns, "detail")
	}
	table := axi.Table{
		Key:         "uploads",
		Columns:     columns,
		Truncatable: []string{"detail"},
		Rows:        rows,
		Total:       -1,
		EmptyNote:   "nothing was uploaded",
	}
	summary := Block{Key: "summary", Fields: []toon.KV{
		{Key: "imported", Value: counts["imported"]},
		{Key: "skipped", Value: counts["skipped"]},
		{Key: "failed", Value: counts["failed"]},
	}}

	var help []string
	if len(failed) > 0 {
		for _, o := range failed {
			if o.importID != 0 {
				help = append(help, fmt.Sprintf("dmsn import retry %d", o.importID))
			}
		}
	}
	if counts["imported"] > 0 && !f.wait {
		help = append(help, "dmsn imports", "dmsn upload <path> --wait  # block until each import settles")
	}

	if err := axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table}, help, e.Globals.options()); err != nil {
		return err
	}
	if counts["failed"] > 0 {
		// The table above is the result; this only sets the exit code.
		return axi.Errorf(axi.CodeIO, "%s failed", axi.Count(counts["failed"], "upload"))
	}
	return nil
}

func kindOf(it item) string {
	if it.zipDir {
		return "directory"
	}
	return "file"
}

func sizeValue(it item) any {
	if it.zipDir {
		return nil
	}
	return it.size
}

func zeroToNil(v uint) any {
	if v == 0 {
		return nil
	}
	return v
}

func dryRunSuffix(f *uploadFlags) string {
	if f.skipExisting {
		return " --skip-existing"
	}
	return ""
}

func quoteAll(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t") {
			out[i] = strconv.Quote(a)
		} else {
			out[i] = a
		}
	}
	return out
}

// PostImport sends one multipart upload. The body is streamed through a pipe
// so a multi-gigabyte package never lands in memory.
func (c *Client) PostImport(ctx context.Context, filename string, body io.Reader, opts importOptions) (*importEnqueueResponse, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	fields := opts.fields()
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)

	go func() {
		var err error
		defer func() { pw.CloseWithError(err) }()
		for _, k := range names {
			if err = mw.WriteField(k, fields[k]); err != nil {
				return
			}
		}
		var part io.Writer
		if part, err = mw.CreateFormFile("file", filename); err != nil {
			return
		}
		if _, err = io.Copy(part, body); err != nil {
			return
		}
		err = mw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/imports", pr)
	if err != nil {
		return nil, axi.Wrap(axi.CodeUsage, err, "build upload request")
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.authorise(req)

	resp, err := c.uploadHTTP().Do(req)
	if err != nil {
		return nil, networkError(err, c.BaseURL)
	}
	defer resp.Body.Close()
	var out importEnqueueResponse
	if err := finish(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// uploadHTTP is the client used for transfers. --timeout bounds a normal API
// call, but applying it to an upload would abort every package that takes
// longer than it to send; the transfer is bounded by the context instead.
func (c *Client) uploadHTTP() *http.Client {
	return &http.Client{Transport: c.HTTP.Transport}
}

type importEnqueueResponse struct {
	AssetID        uint `json:"asset_id"`
	ImportID       uint `json:"import_id"`
	SuggestedMatch *struct {
		AssetID uint   `json:"asset_id"`
		Name    string `json:"name"`
	} `json:"suggested_match"`
}

// WaitForImport polls until the import reaches a terminal stage, the context
// is cancelled, or the request timeout elapses.
func (c *Client) WaitForImport(ctx context.Context, importID uint) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, waitLimit)
	defer cancel()
	path := "/api/imports/" + strconv.FormatUint(uint64(importID), 10)
	for {
		var row struct {
			Stage string `json:"stage"`
			Error string `json:"error"`
		}
		if err := c.GetJSON(ctx, path, nil, &row); err != nil {
			return "", err
		}
		if importstage.IsTerminal(row.Stage) {
			if row.Stage == importstage.Failed && row.Error != "" {
				return row.Stage, axi.Errorf(axi.CodeServer, "%s", row.Error)
			}
			return row.Stage, nil
		}
		select {
		case <-ctx.Done():
			return row.Stage, axi.Wrap(axi.CodeIO, ctx.Err(), "gave up waiting at stage %q", row.Stage)
		case <-time.After(waitPoll):
		}
	}
}
