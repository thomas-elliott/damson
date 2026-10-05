package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"net/url"
	"os"
	"strings"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// scanFlags are the controls for `dmsn scan`, the steps an agent runs to
// catalogue a folder of packs as reference assets.
type scanFlags struct {
	under string
	gone  bool
}

func (f *scanFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.under, "under", "", "known: the folder that was scanned")
	fs.BoolVar(&f.gone, "gone", false, "known: list only recorded paths that no longer exist")
}

func init() {
	f := &scanFlags{}
	register(&command{
		name:    "scan",
		summary: "catalogue a folder of packs as reference assets",
		usage: "usage: dmsn scan known --under PATH [--gone]\n\n" +
			"  known  every path the library records under PATH, and whether it\n" +
			"         still exists here; --gone keeps only the ones that don't\n\n" +
			"Nothing here is deleted: a gone path is reported, never removed.",
		flags: f.register,
		run: func(ctx context.Context, e *Env) error {
			return runScan(ctx, e, f)
		},
	})
}

func runScan(ctx context.Context, e *Env, f *scanFlags) error {
	usage := []string{"dmsn scan known --under /mnt/nfs"}
	if len(e.Args) == 0 {
		return axi.Usagef(usage, "scan needs a subcommand: known")
	}
	switch e.Args[0] {
	case "known":
		return runScanKnown(ctx, e, f)
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
