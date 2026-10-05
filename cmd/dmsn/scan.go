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
			"       dmsn scan match <plan.tsv> --out <matches.tsv> [--interval 2s]\n\n" +
			"  known  every path the library records under PATH, and whether it\n" +
			"         still exists here; --gone keeps only the ones that don't\n" +
			"  match  search the stores for each plan row with a blank link, one\n" +
			"         pack at a time, and write the possible matches to --out\n\n" +
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
	usage := []string{"dmsn scan known --under /mnt/nfs", "dmsn scan match plan.tsv --out matches.tsv"}
	if len(e.Args) == 0 {
		return axi.Usagef(usage, "scan needs a subcommand: known or match")
	}
	switch e.Args[0] {
	case "known":
		return runScanKnown(ctx, e, f)
	case "match":
		return runScanMatch(ctx, e, f)
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
	var resp struct {
		Locations []struct {
			AssetID      uint   `json:"asset_id"`
			LocationPath string `json:"location_path"`
		} `json:"locations"`
	}
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
			// A bad token fails every pack the same way, so stop rather than
			// write a file of failures.
			if code := axi.AsError(err).Code; code == axi.CodeAuth || code == axi.CodeForbidden {
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
