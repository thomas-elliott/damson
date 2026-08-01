package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/importsource"
	"github.com/thomas-elliott/damson/internal/toon"
	"github.com/thomas-elliott/damson/internal/unitycache"
)

// preflightChunk mirrors maxPreflightEntries server-side. A manifest larger
// than one request is chunked rather than truncated or rejected.
const preflightChunk = 2000

type syncFlags struct {
	cacheRoot     string
	publisher     string
	dryRun        bool
	adoptVersions bool
	concurrency   int
	wait          bool
}

func (f *syncFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.cacheRoot, "cache-root", "", "Unity Asset Store cache directory")
	fs.StringVar(&f.publisher, "publisher", "", "only packages by this publisher")
	fs.BoolVar(&f.dryRun, "dry-run", false, "report what would happen, upload nothing")
	fs.BoolVar(&f.adoptVersions, "adopt-versions", false,
		"upload name-matched packages as new versions of the matching asset")
	fs.IntVar(&f.concurrency, "concurrency", defaultConcurrency, "parallel uploads")
	fs.BoolVar(&f.wait, "wait", false, "poll each import to a terminal stage")
}

func init() {
	f := &syncFlags{}
	register(&command{
		name:    "sync",
		summary: "import from the local Unity Asset Store cache",
		usage: "usage: dmsn sync scan|push [--cache-root PATH]\n\n" +
			"  scan   list the cache and what the library already holds\n" +
			"  push   upload only the packages the library needs\n\n" +
			"The cache root defaults to Unity's per-OS location, or\n" +
			"$ASSETSTORE_CACHE_PATH when that is set.\n\n" +
			"flags: --cache-root --publisher --dry-run --adopt-versions\n" +
			"       --concurrency --wait",
		flags: f.register,
		run: func(ctx context.Context, e *Env) error {
			return runSync(ctx, e, f)
		},
	})
}

func runSync(ctx context.Context, e *Env, f *syncFlags) error {
	if len(e.Args) == 0 {
		return axi.Usagef(
			[]string{"dmsn sync scan", "dmsn sync push"},
			"sync needs a subcommand: scan or push")
	}
	verb := e.Args[0]
	if verb != "scan" && verb != "push" {
		return axi.Usagef(
			[]string{"dmsn sync scan", "dmsn sync push"},
			"unknown sync subcommand %q", verb)
	}
	if len(e.Args) > 1 {
		return axi.Usagef(
			[]string{"dmsn sync " + verb + " --cache-root <path>"},
			"sync %s takes no positional arguments; got %s", verb, strings.Join(e.Args[1:], " "))
	}
	if f.concurrency < 1 || f.concurrency > maxConcurrency {
		return axi.Usagef(nil, "--concurrency must be between 1 and %d", maxConcurrency)
	}

	root, err := resolveCacheRoot(f.cacheRoot)
	if err != nil {
		return err
	}
	entries, err := unitycache.Scan(root)
	if err != nil {
		return axi.Wrap(axi.CodeIO, err, "cannot scan %s: %v", root, err)
	}
	entries = filterByPublisher(entries, f.publisher)

	items := cacheItems(root, entries)
	if len(items) == 0 {
		return e.reportEmptyCache(root, f)
	}

	client, err := e.Client()
	if err != nil {
		return err
	}
	statuses, err := e.preflightCache(ctx, client, items)
	if err != nil {
		return err
	}

	if verb == "scan" {
		return e.reportSyncScan(root, entries, statuses, f)
	}

	plan, skipped := syncPlan(items, statuses, f)
	if f.dryRun {
		return e.reportSyncPlan(root, plan, skipped, f)
	}
	if len(plan) == 0 {
		return e.reportSyncNothingToDo(root, skipped, f)
	}

	maxBytes, _ := e.maxUploadBytes(ctx, client)
	uploadOpts := &uploadFlags{concurrency: f.concurrency, wait: f.wait}
	return e.performUpload(ctx, client, plan, nil, maxBytes, uploadOpts)
}

// resolveCacheRoot picks the cache directory, preferring an explicit --cache-root
// over the per-OS default, and fails with the paths it tried rather than an
// empty result.
func resolveCacheRoot(explicit string) (string, error) {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		if info, err := os.Stat(explicit); err != nil || !info.IsDir() {
			return "", axi.Errorf(axi.CodeIO, "--cache-root %s is not a readable directory", explicit)
		}
		return explicit, nil
	}
	roots := unitycache.DefaultRoots()
	for _, r := range roots {
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			return r, nil
		}
	}
	err := axi.Errorf(axi.CodeIO, "no Unity Asset Store cache found")
	err.Help = []string{
		"dmsn sync scan --cache-root <path>",
		"or set " + unitycache.CacheRootEnv,
	}
	if len(roots) > 0 {
		err.Help = append(err.Help, "tried: "+strings.Join(roots, ", "))
	}
	return "", err
}

func filterByPublisher(entries []unitycache.Entry, publisher string) []unitycache.Entry {
	publisher = strings.TrimSpace(publisher)
	if publisher == "" {
		return entries
	}
	out := entries[:0]
	for _, e := range entries {
		if strings.EqualFold(e.Publisher, publisher) {
			out = append(out, e)
		}
	}
	return out
}

// cacheItems turns scanned entries into upload work, each carrying its own
// source hint so the server can parse the path and answer idempotently.
func cacheItems(root string, entries []unitycache.Entry) []item {
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		path := filepath.Join(root, filepath.FromSlash(e.RelativePath))
		items = append(items, item{
			path:      path,
			filename:  filepath.Base(filepath.FromSlash(e.RelativePath)),
			size:      e.SizeBytes,
			resumeKey: uploadResumeKey(path, e.SizeBytes, e.ModTime),
			opts: importOptions{
				sourceKind:         importsource.KindUnityCache,
				sourceRelativePath: e.RelativePath,
			},
		})
	}
	return items
}

// preflightCache asks the server about the whole manifest, returning one
// status per item index.
func (e *Env) preflightCache(ctx context.Context, client *Client, items []item) ([]preflightStatus, error) {
	out := make([]preflightStatus, len(items))
	for start := 0; start < len(items); start += preflightChunk {
		end := min(start+preflightChunk, len(items))
		chunk, err := e.preflightChunkRequest(ctx, client, items[start:end])
		if err != nil {
			return nil, err
		}
		copy(out[start:end], chunk)
	}
	return out, nil
}

type preflightStatus struct {
	Status    string
	AssetID   uint
	AssetName string
	HeldBytes int64
}

func (e *Env) preflightChunkRequest(ctx context.Context, client *Client, items []item) ([]preflightStatus, error) {
	type entry struct {
		Filename     string `json:"filename"`
		SizeBytes    int64  `json:"size_bytes"`
		SourceKind   string `json:"source_kind"`
		RelativePath string `json:"relative_path"`
	}
	req := struct {
		Entries []entry `json:"entries"`
	}{Entries: make([]entry, len(items))}
	for i, it := range items {
		req.Entries[i] = entry{
			Filename:     it.filename,
			SizeBytes:    it.size,
			SourceKind:   it.opts.sourceKind,
			RelativePath: it.opts.sourceRelativePath,
		}
	}

	var resp struct {
		Results []struct {
			Index     int    `json:"index"`
			Status    string `json:"status"`
			AssetID   uint   `json:"asset_id"`
			AssetName string `json:"asset_name"`
			HeldBytes int64  `json:"held_bytes"`
		} `json:"results"`
	}
	if err := client.PostJSON(ctx, "/api/imports/preflight", req, &resp); err != nil {
		return nil, err
	}

	out := make([]preflightStatus, len(items))
	for i := range out {
		out[i].Status = importsource.StatusNew
	}
	for _, r := range resp.Results {
		if r.Index < 0 || r.Index >= len(out) {
			continue
		}
		out[r.Index] = preflightStatus{
			Status: r.Status, AssetID: r.AssetID,
			AssetName: r.AssetName, HeldBytes: r.HeldBytes,
		}
	}
	return out, nil
}

// syncPlan decides what actually uploads. already_imported never does;
// updated uploads as a new version of the asset holding that path; name_match
// only uploads under --adopt-versions, because a name match cannot tell a
// duplicate from a newer version.
func syncPlan(items []item, statuses []preflightStatus, f *syncFlags) (plan []item, skipped []skippedItem) {
	for i, it := range items {
		s := statuses[i]
		switch s.Status {
		case importsource.StatusAlreadyImported:
			skipped = append(skipped, skippedItem{item: it, status: s,
				reason: fmt.Sprintf("asset %d already holds these bytes", s.AssetID)})
		case importsource.StatusUpdated:
			it.opts.assetID = strconv.FormatUint(uint64(s.AssetID), 10)
			it.note = fmt.Sprintf("asset %d holds %d bytes at this path, sending %d",
				s.AssetID, s.HeldBytes, it.size)
			plan = append(plan, it)
		case importsource.StatusNameMatch:
			if !f.adoptVersions {
				skipped = append(skipped, skippedItem{item: it, status: s,
					reason: fmt.Sprintf("name matches asset %d %q; --adopt-versions to add a version",
						s.AssetID, s.AssetName)})
				continue
			}
			it.opts.assetID = strconv.FormatUint(uint64(s.AssetID), 10)
			it.note = fmt.Sprintf("adopted as a new version of asset %d %q", s.AssetID, s.AssetName)
			plan = append(plan, it)
		default:
			plan = append(plan, it)
		}
	}
	return plan, skipped
}

type skippedItem struct {
	item   item
	status preflightStatus
	reason string
}

func (e *Env) reportSyncScan(root string, entries []unitycache.Entry, statuses []preflightStatus, f *syncFlags) error {
	rows := make([]map[string]any, 0, len(entries))
	counts := map[string]int{}
	var totalBytes int64
	for i, en := range entries {
		s := statuses[i]
		counts[s.Status]++
		totalBytes += en.SizeBytes
		rows = append(rows, map[string]any{
			"publisher": en.Publisher,
			"package":   en.Name,
			"category":  blankToNil(en.CategoryPath()),
			"size":      en.SizeBytes,
			"status":    s.Status,
			"asset_id":  zeroToNil(s.AssetID),
		})
	}

	table := axi.Table{
		Key:         "packages",
		Columns:     []string{"publisher", "package", "size", "status"},
		Truncatable: []string{"category"},
		Rows:        rows,
		Total:       len(rows),
		EmptyNote:   "no packages in " + root,
	}
	summary := Block{Key: "cache", Fields: []toon.KV{
		{Key: "root", Value: root},
		{Key: "packages", Value: len(rows)},
		{Key: "bytes", Value: totalBytes},
		{Key: "new", Value: counts[importsource.StatusNew]},
		{Key: "updated", Value: counts[importsource.StatusUpdated]},
		{Key: "already_imported", Value: counts[importsource.StatusAlreadyImported]},
		{Key: "name_match", Value: counts[importsource.StatusNameMatch]},
	}}

	help := []string{"dmsn sync push" + cacheRootSuffix(f)}
	if counts[importsource.StatusNameMatch] > 0 {
		help = append(help,
			"a name_match was imported some other way — it has no cache path yet",
			"dmsn sync push --adopt-versions  # add those as versions of the matching asset")
	}
	return axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table}, help, e.Globals.options())
}

func (e *Env) reportSyncPlan(root string, plan []item, skipped []skippedItem, f *syncFlags) error {
	rows := make([]map[string]any, 0, len(plan)+len(skipped))
	var uploadBytes int64
	for _, it := range plan {
		uploadBytes += it.size
		rows = append(rows, map[string]any{
			"package":  it.filename,
			"action":   "upload",
			"size":     it.size,
			"asset_id": blankToNil(it.opts.assetID),
			"note":     blankToNil(it.note),
		})
	}
	for _, s := range skipped {
		rows = append(rows, map[string]any{
			"package":  s.item.filename,
			"action":   "skip",
			"size":     s.item.size,
			"asset_id": zeroToNil(s.status.AssetID),
			"note":     s.reason,
		})
	}

	table := axi.Table{
		Key:         "plan",
		Columns:     []string{"package", "action", "size"},
		Truncatable: []string{"note"},
		Rows:        rows,
		Total:       len(rows),
		EmptyNote:   "nothing to do",
	}
	summary := Block{Key: "summary", Fields: []toon.KV{
		{Key: "root", Value: root},
		{Key: "upload", Value: len(plan)},
		{Key: "skip", Value: len(skipped)},
		{Key: "bytes", Value: uploadBytes},
		{Key: "dry_run", Value: true},
	}}
	help := []string{"dmsn sync push" + cacheRootSuffix(f)}
	return axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table}, help, e.Globals.options())
}

func (e *Env) reportSyncNothingToDo(root string, skipped []skippedItem, f *syncFlags) error {
	summary := Block{Key: "summary", Fields: []toon.KV{
		{Key: "root", Value: root},
		{Key: "upload", Value: 0},
		{Key: "skip", Value: len(skipped)},
	}}
	help := []string{"dmsn sync scan" + cacheRootSuffix(f)}
	if slices.ContainsFunc(skipped, func(s skippedItem) bool {
		return s.status.Status == importsource.StatusNameMatch
	}) {
		help = append(help, "dmsn sync push --adopt-versions  # adopt the name-matched packages")
	}
	note := fmt.Sprintf("the library already holds every package in %s", root)
	return axi.RenderComposite(e.Out, []Block{summary},
		[]axi.List{{Key: "note", Items: []any{note}}}, nil, help, e.Globals.options())
}

func (e *Env) reportEmptyCache(root string, f *syncFlags) error {
	note := "no .unitypackage files under " + root
	if f.publisher != "" {
		note += " for publisher " + f.publisher
	}
	summary := Block{Key: "cache", Fields: []toon.KV{
		{Key: "root", Value: root},
		{Key: "packages", Value: 0},
	}}
	help := []string{"dmsn sync scan --cache-root <path>"}
	return axi.RenderComposite(e.Out, []Block{summary},
		[]axi.List{{Key: "note", Items: []any{note}}}, nil, help, e.Globals.options())
}

func cacheRootSuffix(f *syncFlags) string {
	if f.cacheRoot == "" {
		return ""
	}
	return fmt.Sprintf(" --cache-root %q", f.cacheRoot)
}
