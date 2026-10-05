package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// scanFlags are the controls for `dmsn scan`, the steps an agent runs to
// catalogue a folder of packs as reference assets.
type scanFlags struct {
	under    string
	gone     bool
	out      string
	interval time.Duration
}

// defaultSearchInterval spaces one pack's search from the next. Each search
// asks every store once, so this is the gap each store sees: slow enough that
// a run over thousands of packs isn't throttled, at about 30 packs a minute.
const defaultSearchInterval = 2 * time.Second

func (f *scanFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.under, "under", "", "known: the folder that was scanned")
	fs.BoolVar(&f.gone, "gone", false, "known: list only recorded paths that no longer exist")
	fs.StringVar(&f.out, "out", "", "match: the file to write possible matches to")
	fs.DurationVar(&f.interval, "interval", defaultSearchInterval, "match: pause between one pack's search and the next")
}

func init() {
	f := &scanFlags{}
	register(&command{
		name:    "scan",
		summary: "catalogue a folder of packs as reference assets",
		usage: "usage: dmsn scan known --under PATH [--gone]\n" +
			"       dmsn scan match <plan.tsv> --out <matches.tsv> [--interval 2s]\n" +
			"       dmsn scan record <plan.tsv>\n\n" +
			"  known  every path the library records under PATH, and whether it\n" +
			"         still exists here; --gone keeps only the ones that don't\n" +
			"  match  search the stores for each plan row with a blank link, one\n" +
			"         pack at a time, and write the possible matches to --out\n" +
			"  record record every plan row as a reference asset marked Needs\n" +
			"         review; a path already recorded is skipped, never changed\n\n" +
			"A plan is a tab-separated file with the header\n" +
			"  " + strings.Join(planColumns, "\t") + "\n" +
			"one row per pack. tags is comma-separated; link is a store URL,\n" +
			"\"" + noMatch + "\", or blank until matched.\n\n" +
			"Nothing here is deleted: a gone path is reported, never removed.",
		flags: f.register,
		run: func(ctx context.Context, e *Env) error {
			return runScan(ctx, e, f)
		},
	})
}

func runScan(ctx context.Context, e *Env, f *scanFlags) error {
	usage := []string{"dmsn scan known --under /mnt/nfs", "dmsn scan match plan.tsv --out matches.tsv",
		"dmsn scan record plan.tsv"}
	if len(e.Args) == 0 {
		return axi.Usagef(usage, "scan needs a subcommand: known, match or record")
	}
	switch e.Args[0] {
	case "known":
		return runScanKnown(ctx, e, f)
	case "match":
		return runScanMatch(ctx, e, f)
	case "record":
		return runScanRecord(ctx, e)
	default:
		return axi.Usagef(usage, "unknown scan subcommand %q", e.Args[0])
	}
}

// trimScanPath applies the server's sameness rule to a path given here:
// surrounding whitespace and one trailing slash don't count.
func trimScanPath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

type locationsResponse struct {
	Locations []struct {
		AssetID      uint   `json:"asset_id"`
		LocationPath string `json:"location_path"`
	} `json:"locations"`
}

// failsEveryRow reports an error that would fail every remaining row the same
// way — a bad or read-only token — so a run stops rather than report each one.
func failsEveryRow(err error) bool {
	code := axi.AsError(err).Code
	return code == axi.CodeAuth || code == axi.CodeForbidden
}

func runScanKnown(ctx context.Context, e *Env, f *scanFlags) error {
	under := trimScanPath(f.under)
	if under == "" || len(e.Args) > 1 {
		return axi.Usagef([]string{"dmsn scan known --under /mnt/nfs"},
			"scan known takes --under, the folder that was scanned, and no arguments")
	}
	// Every recorded path would read as gone if the store weren't mounted, so
	// an unreadable root is an error, not a report.
	if info, err := os.Stat(under); err != nil || !info.IsDir() {
		return axi.Errorf(axi.CodeIO, "--under %s is not a readable directory here", under)
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	var resp locationsResponse
	if err := client.GetJSON(ctx, "/api/assets/locations", url.Values{"under": {under}}, &resp); err != nil {
		return err
	}

	rows := []map[string]any{}
	gone := 0
	for _, l := range resp.Locations {
		present, err := pathExists(l.LocationPath)
		if err != nil {
			return axi.Wrap(axi.CodeIO, err, "cannot check %s: %v", l.LocationPath, err)
		}
		if !present {
			gone++
		}
		if f.gone && present {
			continue
		}
		rows = append(rows, map[string]any{"path": l.LocationPath, "asset_id": l.AssetID, "present": present})
	}

	note := "no paths recorded under " + under
	if f.gone {
		note = "every path recorded under " + under + " still exists"
	}
	table := axi.Table{
		Key: "known", Columns: []string{"path", "asset_id", "present"},
		Rows: rows, Total: -1, EmptyNote: note,
	}
	summary := Block{Key: "scan", Fields: []toon.KV{
		{Key: "under", Value: under},
		{Key: "recorded", Value: len(resp.Locations)},
		{Key: "gone", Value: gone},
	}}
	return axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table}, nil, e.Globals.options())
}

// pathExists reports whether p is there. Only "does not exist" counts as gone;
// any other failure (permissions, a stale mount) is an error, so a path is
// never reported gone on a guess.
func pathExists(p string) (bool, error) {
	_, err := os.Lstat(p)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// planColumns is the plan file's header, in order. The agent writes the plan,
// the operator reads and corrects it, and dmsn reads it back to search and to
// record, so all three see the same file.
var planColumns = []string{"path", "name", "publisher", "tags", "link"}

// noMatch is the link a plan row carries when no store listing was chosen.
const noMatch = "no match"

type planRow struct {
	Path      string
	Name      string
	Publisher string
	Tags      []string
	Link      string // a store URL, or "" for none
	Decided   bool   // the link column holds a URL or "no match"
}

// readPlan parses a plan file. Every problem is reported with its line, so the
// file can be fixed by hand and read again.
func readPlan(path string) ([]planRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, axi.Wrap(axi.CodeIO, err, "cannot read plan %s", path)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	header := strings.Join(planColumns, "\t")
	if strings.TrimSpace(lines[0]) != header {
		return nil, axi.Usagef([]string{"first line: " + strings.ReplaceAll(header, "\t", "<tab>")},
			"plan %s does not start with the plan header", path)
	}
	var rows []planRow
	seen := map[string]int{}
	for i, line := range lines[1:] {
		n := i + 2
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != len(planColumns) {
			return nil, axi.Usagef(nil, "plan line %d has %d columns, want %d (tab-separated)", n, len(cols), len(planColumns))
		}
		row := planRow{
			Path:      trimScanPath(cols[0]),
			Name:      strings.TrimSpace(cols[1]),
			Publisher: strings.TrimSpace(cols[2]),
			Tags:      axi.ParseFields(cols[3]),
		}
		if row.Path == "" || row.Name == "" {
			return nil, axi.Usagef(nil, "plan line %d needs a path and a name", n)
		}
		if first, dup := seen[row.Path]; dup {
			return nil, axi.Usagef(nil, "plan line %d repeats the path on line %d: %s", n, first, row.Path)
		}
		seen[row.Path] = n
		switch link := strings.TrimSpace(cols[4]); {
		case link == "":
		case strings.EqualFold(link, noMatch):
			row.Decided = true
		case strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "http://"):
			row.Link, row.Decided = link, true
		default:
			return nil, axi.Usagef(nil, "plan line %d: link must be a store URL, %q, or blank; got %q", n, noMatch, link)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// matchColumns is the matches file's header. A pack gets one row per possible
// listing, or a single row saying it has none or that its search failed.
var matchColumns = []string{"path", "result", "store", "title", "publisher", "listing_url", "error"}

// Match results.
const (
	matchCandidate   = "candidate"
	matchNone        = "none"
	matchStoreFailed = "store_failed"  // one store errored or was throttled; the others still answered
	matchFailed      = "search_failed" // damson itself could not be asked
)

// maxMatchesPerStore keeps the file to what an agent can weigh for every pack
// of a large scan: the best few listings from each store, not every hit.
const maxMatchesPerStore = 3

type searchCandidate struct {
	Title      string `json:"title"`
	Publisher  string `json:"publisher"`
	ListingURL string `json:"listing_url"`
	Store      string `json:"store"`
}

func runScanMatch(ctx context.Context, e *Env, f *scanFlags) error {
	usage := []string{"dmsn scan match plan.tsv --out matches.tsv"}
	if len(e.Args) != 2 || strings.TrimSpace(f.out) == "" {
		return axi.Usagef(usage, "scan match takes one plan file and --out")
	}
	if f.interval < 0 {
		return axi.Usagef(usage, "--interval cannot be negative")
	}
	rows, err := readPlan(e.Args[1])
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	file, err := os.Create(f.out)
	if err != nil {
		return axi.Wrap(axi.CodeIO, err, "cannot write %s", f.out)
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	writeRow := func(cols ...string) {
		clean := make([]string, len(cols))
		for i, c := range cols {
			// A tab or newline in a store's title would break the row.
			clean[i] = strings.Join(strings.Fields(c), " ")
		}
		fmt.Fprintln(w, strings.Join(clean, "\t"))
	}
	writeRow(matchColumns...)

	counts := map[string]int{}
	searched := 0
	for _, row := range rows {
		if row.Decided {
			counts["skipped"]++
			continue
		}
		if searched > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(f.interval):
			}
		}
		searched++
		var resp struct {
			Candidates   []searchCandidate `json:"candidates"`
			FailedStores []string          `json:"failed_stores"`
		}
		err := client.GetJSON(ctx, "/api/metadata/search", url.Values{"q": {row.Name}}, &resp)
		if err != nil {
			if failsEveryRow(err) {
				return err
			}
			writeRow(row.Path, matchFailed, "", "", "", "", axi.AsError(err).Message)
			counts[matchFailed]++
		} else {
			perStore := map[string]int{}
			kept := 0
			for _, c := range resp.Candidates {
				if perStore[c.Store] >= maxMatchesPerStore {
					continue
				}
				perStore[c.Store]++
				kept++
				writeRow(row.Path, matchCandidate, c.Store, c.Title, c.Publisher, c.ListingURL, "")
			}
			for _, store := range resp.FailedStores {
				writeRow(row.Path, matchStoreFailed, store, "", "", "", "")
			}
			switch {
			case len(resp.FailedStores) > 0:
				counts[matchStoreFailed]++
			case kept == 0:
				writeRow(row.Path, matchNone, "", "", "", "", "")
				counts[matchNone]++
			default:
				counts[matchCandidate]++
			}
		}
		// Flush per pack, so a run stopped part-way leaves what it found.
		if err := w.Flush(); err != nil {
			return axi.Wrap(axi.CodeIO, err, "cannot write %s", f.out)
		}
	}
	if err := w.Flush(); err != nil {
		return axi.Wrap(axi.CodeIO, err, "cannot write %s", f.out)
	}

	return e.Block("match", []toon.KV{
		{Key: "out", Value: f.out},
		{Key: "searched", Value: searched},
		{Key: "with_candidates", Value: counts[matchCandidate]},
		{Key: "no_candidates", Value: counts[matchNone]},
		{Key: "store_failed", Value: counts[matchStoreFailed]},
		{Key: "search_failed", Value: counts[matchFailed]},
		{Key: "already_decided", Value: counts["skipped"]},
	}, nil)
}

// runScanRecord records an approved plan: every row as a reference asset with
// its name, publisher, tags and link, marked Needs review. A path the library
// already records is skipped rather than re-recorded, because re-recording a
// known path would replace its store link; a rerun leaves known packs alone.
func runScanRecord(ctx context.Context, e *Env) error {
	if len(e.Args) != 2 {
		return axi.Usagef([]string{"dmsn scan record plan.tsv"}, "scan record takes one plan file")
	}
	rows, err := readPlan(e.Args[1])
	if err != nil {
		return err
	}
	// A blank link means the match step never ran for that pack, so the plan
	// shown for approval didn't say what would be linked.
	undecided := 0
	for _, row := range rows {
		if !row.Decided {
			undecided++
		}
	}
	if undecided > 0 {
		return axi.Usagef([]string{"dmsn scan match " + e.Args[1] + " --out matches.tsv"},
			"plan rows with a blank link: %d; set a store URL or %q in each first", undecided, noMatch)
	}

	client, err := e.Client()
	if err != nil {
		return err
	}
	var known locationsResponse
	if err := client.GetJSON(ctx, "/api/assets/locations", nil, &known); err != nil {
		return err
	}
	recorded := map[string]bool{}
	for _, l := range known.Locations {
		recorded[l.LocationPath] = true
	}

	counts := map[string]int{}
	report := []map[string]any{}
	for _, row := range rows {
		if recorded[row.Path] {
			counts["known"]++
			report = append(report, map[string]any{"path": row.Path, "outcome": "known", "asset_id": nil, "detail": nil})
			continue
		}
		body := map[string]any{
			"location_path": row.Path,
			"name":          row.Name,
			"needs_review":  true,
		}
		if row.Publisher != "" {
			body["publisher"] = row.Publisher
		}
		if len(row.Tags) > 0 {
			body["tags"] = row.Tags
		}
		if row.Link != "" {
			body["source_url"] = row.Link
		}
		var out struct {
			Outcome string `json:"outcome"`
			AssetID uint   `json:"asset_id"`
		}
		if err := client.PostJSON(ctx, "/api/assets/references", body, &out); err != nil {
			if failsEveryRow(err) {
				return err
			}
			counts["failed"]++
			report = append(report, map[string]any{"path": row.Path, "outcome": "failed", "asset_id": nil,
				"detail": axi.AsError(err).Message})
			continue
		}
		outcome := firstNonEmpty(recordOutcomes[out.Outcome], out.Outcome)
		counts[outcome]++
		// Added rows are the bulk of a scan and need no attention; the table
		// carries only the rows that do.
		if outcome != recordOutcomes["created"] {
			report = append(report, map[string]any{"path": row.Path, "outcome": outcome, "asset_id": out.AssetID, "detail": nil})
		}
	}

	table := axi.Table{
		Key: "not_added", Columns: []string{"path", "outcome", "asset_id", "detail"},
		Truncatable: []string{"detail"}, Rows: report, Total: -1,
		EmptyNote: "every row was added",
	}
	summary := Block{Key: "record", Fields: []toon.KV{
		{Key: "rows", Value: len(rows)},
		{Key: "added", Value: counts[recordOutcomes["created"]]},
		{Key: "known", Value: counts["known"]},
		{Key: "failed", Value: counts["failed"]},
	}}
	help := []string{"dmsn assets --needs-review"}
	if err := axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table}, help, e.Globals.options()); err != nil {
		return err
	}
	if counts["failed"] > 0 {
		// The table above is the result; this only sets the exit code.
		return axi.Errorf(axi.CodeIO, "%s failed", axi.Count(counts["failed"], "row"))
	}
	return nil
}
