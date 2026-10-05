package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// purchasesFlags are the controls for `dmsn purchases`, the steps an agent runs
// to catalogue the Unity Asset Store purchase list.
type purchasesFlags struct {
	cache    string
	out      string
	interval time.Duration
}

func (f *purchasesFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.cache, "cache", "", "read: the Unity Editor's HTTP cache file, when it isn't in the usual place")
	fs.StringVar(&f.out, "out", "", "read, plan: the file to write")
	fs.DurationVar(&f.interval, "interval", defaultSearchInterval, "record: pause between one store lookup and the next")
}

func init() {
	f := &purchasesFlags{}
	register(&command{
		name:    "purchases",
		summary: "catalogue the Unity Asset Store purchase list as reference assets",
		usage: "usage: dmsn purchases read --out <purchases.tsv> [--cache PATH]\n" +
			"       dmsn purchases plan <purchases.tsv> --out <plan.tsv>\n" +
			"       dmsn purchases record <plan.tsv> [--interval 2s]\n\n" +
			"  read   read the purchase list from the Unity Editor's HTTP cache, with\n" +
			"         no network call, and report how many of its total were found\n" +
			"  plan   match each purchase against the library and draft the plan\n" +
			"  record record an approved plan, marking everything it changes\n" +
			"         Needs review, with store lookups spaced --interval apart\n\n" +
			"A plan is a tab-separated file with the header\n" +
			"  " + strings.Join(purchasePlanColumns, "\t") + "\n" +
			"one row per purchase. action is one of: " + strings.Join(purchaseActions, ", ") + ".\n" +
			"matched links the asset only if it has no link; swap replaces its link.\n" +
			"A plan with an unsure row is refused until each one is decided.",
		flags: f.register,
		run: func(ctx context.Context, e *Env) error {
			return runPurchases(ctx, e, f)
		},
	})
}

func runPurchases(ctx context.Context, e *Env, f *purchasesFlags) error {
	usage := []string{"dmsn purchases read --out purchases.tsv", "dmsn purchases plan purchases.tsv --out plan.tsv",
		"dmsn purchases record plan.tsv"}
	if len(e.Args) == 0 {
		return axi.Usagef(usage, "purchases needs a subcommand: read, plan or record")
	}
	switch e.Args[0] {
	case "read":
		return runPurchasesRead(ctx, e, f)
	case "plan":
		return runPurchasesPlan(ctx, e, f)
	case "record":
		return runPurchasesRecord(ctx, e, f)
	default:
		return axi.Usagef(usage, "unknown purchases subcommand %q", e.Args[0])
	}
}

// purchaseColumns is the purchase file's header. `purchases read` writes it
// and `purchases plan` reads it back.
var purchaseColumns = []string{"package_id", "name", "link"}

type purchase struct {
	PackageID string // the Asset Store listing ID
	Name      string
	Link      string
}

// unityListingLink is the store link for a listing known only by its ID. The
// store redirects it to the listing's full address, and the Unity lookup reads
// the ID from its end.
func unityListingLink(packageID string) string {
	return "https://assetstore.unity.com/packages/slug/" + packageID
}

// unityListingID is the Asset Store listing ID a store link ends in, or "" for
// a link that isn't to an Asset Store listing.
func unityListingID(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || !strings.EqualFold(u.Hostname(), "assetstore.unity.com") {
		return ""
	}
	last := strings.TrimSuffix(u.Path, "/")
	last = last[strings.LastIndex(last, "/")+1:]
	last = last[strings.LastIndex(last, "-")+1:]
	if _, err := strconv.ParseUint(last, 10, 64); err != nil {
		return ""
	}
	return last
}

// tsvLine joins columns into one tab-separated line, folding any tab or
// newline inside a value (a store title can hold one) into a space.
func tsvLine(cols ...string) string {
	clean := make([]string, len(cols))
	for i, c := range cols {
		clean[i] = strings.Join(strings.Fields(c), " ")
	}
	return strings.Join(clean, "\t")
}

// readTSV reads a tab-separated file that must start with header, returning
// each non-blank row with its line number. what names the file in errors.
func readTSV(path, what string, header []string) ([]tsvRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, axi.Wrap(axi.CodeIO, err, "cannot read %s %s", what, path)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	want := strings.Join(header, "\t")
	if strings.TrimSpace(lines[0]) != want {
		return nil, axi.Usagef([]string{"first line: " + strings.ReplaceAll(want, "\t", "<tab>")},
			"%s %s does not start with the %s header", what, path, what)
	}
	var rows []tsvRow
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != len(header) {
			return nil, axi.Usagef(nil, "%s line %d has %d columns, want %d (tab-separated)", what, i+2, len(cols), len(header))
		}
		for j := range cols {
			cols[j] = strings.TrimSpace(cols[j])
		}
		rows = append(rows, tsvRow{Line: i + 2, Cols: cols})
	}
	return rows, nil
}

type tsvRow struct {
	Line int
	Cols []string
}

func readPurchases(path string) ([]purchase, error) {
	rows, err := readTSV(path, "purchase file", purchaseColumns)
	if err != nil {
		return nil, err
	}
	out := make([]purchase, 0, len(rows))
	for _, r := range rows {
		p := purchase{PackageID: r.Cols[0], Name: r.Cols[1], Link: r.Cols[2]}
		if p.PackageID == "" || p.Name == "" || p.Link == "" {
			return nil, axi.Usagef(nil, "purchase file line %d needs a package_id, a name and a link", r.Line)
		}
		out = append(out, p)
	}
	return out, nil
}

// purchasePlanColumns is the plan file's header. dmsn drafts it, the agent
// and the operator decide every row, and dmsn records exactly what it says.
// asset_name, asset_link and note are there for the operator to read; record
// ignores them.
var purchasePlanColumns = []string{"package_id", "name", "link", "action", "asset_id", "asset_name", "asset_link", "note"}

const (
	actionNew     = "new"     // record as a new reference asset with the purchase's link
	actionMatched = "matched" // the purchase is asset_id: link it if it has no link, else leave it
	actionSwap    = "swap"    // the purchase is asset_id: replace its link with the purchase's
	actionDropped = "dropped" // record nothing
	actionUnsure  = "unsure"  // matched by name only; the operator must decide before recording
)

var purchaseActions = []string{actionNew, actionMatched, actionSwap, actionDropped, actionUnsure}

type assetLinksResponse struct {
	Assets []recordedAsset `json:"assets"`
}

type recordedAsset struct {
	AssetID        uint   `json:"asset_id"`
	Name           string `json:"name"`
	SourceURL      string `json:"source_url"`
	Store          string `json:"store"`
	StoreProductID string `json:"store_product_id"`
}

// listingID is the Asset Store listing an asset is linked to, from its store
// product ID where the Unity lookup filled one, else from its link.
func (a recordedAsset) listingID() string {
	if a.Store == "unity" && a.StoreProductID != "" {
		return a.StoreProductID
	}
	return unityListingID(a.SourceURL)
}

// sameListing reports whether two store links name the same listing: equal,
// or both Asset Store links ending in the same listing ID.
func sameListing(a, b string) bool {
	if a == b {
		return true
	}
	id := unityListingID(a)
	return id != "" && id == unityListingID(b)
}

// matchKey reduces a name to its letters and digits, lowercased, so
// "POLYGON - Fantasy Kingdom" and "POLYGON_Fantasy_Kingdom" compare equal.
func matchKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// draftRow is one drafted plan row: the purchase, what to do with it, and the
// asset it matched, if any.
type draftRow struct {
	Purchase purchase
	Action   string
	Asset    *recordedAsset
	Note     string
}

// line is the row as it is written to the plan file.
func (r draftRow) line() string {
	var id, name, link string
	if r.Asset != nil {
		id, name, link = strconv.FormatUint(uint64(r.Asset.AssetID), 10), r.Asset.Name, r.Asset.SourceURL
	}
	p := r.Purchase
	return tsvLine(p.PackageID, p.Name, p.Link, r.Action, id, name, link, r.Note)
}

// draftPurchaseRow decides one purchase's row: matched by listing ID, else
// unsure by name, else new. A match by exact link is a match by listing ID,
// since a purchase's link ends in its listing ID.
func draftPurchaseRow(p purchase, byListing map[string][]recordedAsset, byName map[string][]recordedAsset) draftRow {
	row := func(action string, a *recordedAsset, note string) draftRow {
		return draftRow{Purchase: p, Action: action, Asset: a, Note: note}
	}
	also := func(more []recordedAsset) string {
		if len(more) == 0 {
			return ""
		}
		ids := make([]string, len(more))
		for i, a := range more {
			ids[i] = strconv.FormatUint(uint64(a.AssetID), 10)
		}
		return "; also matches asset " + strings.Join(ids, ", ")
	}

	if found := byListing[p.PackageID]; len(found) > 0 {
		a := found[0]
		var note string
		switch {
		case a.SourceURL == "":
			note = "same listing; gains the link"
		case sameListing(a.SourceURL, p.Link):
			note = "same listing, already linked"
		default:
			note = "same listing; has another link, kept if matched, replaced if swap"
		}
		return row(actionMatched, &a, note+also(found[1:]))
	}
	if found := byName[matchKey(p.Name)]; len(found) > 0 {
		a := found[0]
		note := "name only; no link, so matched would add the link"
		if a.SourceURL != "" {
			note = "name only; has another link, kept if matched, replaced if swap"
		}
		return row(actionUnsure, &a, note+also(found[1:]))
	}
	return row(actionNew, nil, "")
}

func runPurchasesPlan(ctx context.Context, e *Env, f *purchasesFlags) error {
	if len(e.Args) != 2 || strings.TrimSpace(f.out) == "" {
		return axi.Usagef([]string{"dmsn purchases plan purchases.tsv --out plan.tsv"},
			"purchases plan takes one purchase file and --out")
	}
	purchases, err := readPurchases(e.Args[1])
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	var recorded assetLinksResponse
	if err := client.GetJSON(ctx, "/api/assets/links", nil, &recorded); err != nil {
		return err
	}
	byListing := map[string][]recordedAsset{}
	byName := map[string][]recordedAsset{}
	for _, a := range recorded.Assets {
		if id := a.listingID(); id != "" {
			byListing[id] = append(byListing[id], a)
			// An asset already linked to another listing is a different product,
			// whatever its name.
			continue
		}
		if key := matchKey(a.Name); key != "" {
			byName[key] = append(byName[key], a)
		}
	}

	lines := []string{tsvLine(purchasePlanColumns...)}
	counts := map[string]int{}
	conflicts := 0
	for _, p := range purchases {
		row := draftPurchaseRow(p, byListing, byName)
		counts[row.Action]++
		if row.Action == actionUnsure && row.Asset.SourceURL != "" {
			conflicts++
		}
		lines = append(lines, row.line())
	}
	if err := os.WriteFile(f.out, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return axi.Wrap(axi.CodeIO, err, "cannot write %s", f.out)
	}
	return e.Block("plan", []toon.KV{
		{Key: "out", Value: f.out},
		{Key: "purchases", Value: len(purchases)},
		{Key: "new", Value: counts[actionNew]},
		{Key: "matched", Value: counts[actionMatched]},
		{Key: "unsure", Value: counts[actionUnsure]},
		{Key: "unsure_with_another_link", Value: conflicts},
	}, []string{fmt.Sprintf("decide every unsure row in %s, then dmsn purchases record %s", f.out, f.out)})
}

type purchasePlanRow struct {
	Line    int
	ID      string
	Name    string
	Link    string
	Action  string
	AssetID uint
}

// readPurchasePlan parses an approved plan. It refuses an unsure row, because
// a name-only match is never applied without the operator's say-so, and a row
// that doesn't carry what its action needs.
func readPurchasePlan(path string) ([]purchasePlanRow, error) {
	rows, err := readTSV(path, "plan", purchasePlanColumns)
	if err != nil {
		return nil, err
	}
	out := make([]purchasePlanRow, 0, len(rows))
	unsure := 0
	for _, r := range rows {
		row := purchasePlanRow{Line: r.Line, ID: r.Cols[0], Name: r.Cols[1], Link: r.Cols[2], Action: strings.ToLower(r.Cols[3])}
		switch row.Action {
		case actionUnsure:
			unsure++
		case actionDropped:
		case actionNew, actionMatched, actionSwap:
			if row.Name == "" || !(strings.HasPrefix(row.Link, "https://") || strings.HasPrefix(row.Link, "http://")) {
				return nil, axi.Usagef(nil, "plan line %d needs a name and a store link", r.Line)
			}
			if row.Action == actionNew {
				break
			}
			id, err := strconv.ParseUint(r.Cols[4], 10, 32)
			if err != nil || id == 0 {
				return nil, axi.Usagef(nil, "plan line %d: %s needs the asset_id it matches, got %q", r.Line, row.Action, r.Cols[4])
			}
			row.AssetID = uint(id)
		default:
			return nil, axi.Usagef(nil, "plan line %d: action must be one of %s, got %q",
				r.Line, strings.Join(purchaseActions, ", "), r.Cols[3])
		}
		out = append(out, row)
	}
	if unsure > 0 {
		return nil, axi.Usagef(nil, "plan rows still unsure: %d; set each to %s, %s, %s or %s first",
			unsure, actionMatched, actionSwap, actionNew, actionDropped)
	}
	return out, nil
}

// runPurchasesRecord records an approved plan. A new purchase becomes a
// reference asset with its link; a matched asset with no link gains it, and
// one whose link differs keeps it unless the row says swap. Everything
// recorded or changed is marked Needs review; an asset already on the
// purchase's listing is left exactly as it is. Every call that makes the
// server look the listing up waits --interval after the one before, so a run
// over hundreds of purchases isn't throttled by the store.
func runPurchasesRecord(ctx context.Context, e *Env, f *purchasesFlags) error {
	if len(e.Args) != 2 {
		return axi.Usagef([]string{"dmsn purchases record plan.tsv"}, "purchases record takes one plan file")
	}
	if f.interval < 0 {
		return axi.Usagef(nil, "--interval cannot be negative")
	}
	rows, err := readPurchasePlan(e.Args[1])
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	// Read the links now rather than trust the plan's copy, which may be
	// older than an edit made since it was drafted.
	var recorded assetLinksResponse
	if err := client.GetJSON(ctx, "/api/assets/links", nil, &recorded); err != nil {
		return err
	}
	links := map[uint]string{}
	for _, a := range recorded.Assets {
		links[a.AssetID] = a.SourceURL
	}

	// Set after each call that may have looked the store up. A link the library
	// already holds is answered without a lookup, so a rerun doesn't wait on it.
	var lastLookup time.Time
	pace := func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(lastLookup.Add(f.interval))):
			return nil
		}
	}

	counts := map[string]int{}
	report := []map[string]any{}
	add := func(row purchasePlanRow, outcome string, assetID uint, detail string) {
		counts[outcome]++
		var id, d any
		if assetID != 0 {
			id = assetID
		}
		if detail != "" {
			d = detail
		}
		report = append(report, map[string]any{"package_id": row.ID, "name": row.Name, "outcome": outcome, "asset_id": id, "detail": d})
	}

	for _, row := range rows {
		switch row.Action {
		case actionDropped:
			counts["dropped"]++
			continue
		case actionNew:
			if err := pace(); err != nil {
				return err
			}
			var out struct {
				Outcome        string `json:"outcome"`
				AssetID        uint   `json:"asset_id"`
				MetadataFailed bool   `json:"metadata_failed"`
			}
			body := map[string]any{"source_url": row.Link, "name": row.Name, "needs_review": true}
			err := client.PostJSON(ctx, "/api/assets/references", body, &out)
			if out.Outcome != "exists" {
				lastLookup = time.Now()
			}
			if err != nil {
				if failsEveryRow(err) {
					return err
				}
				add(row, "failed", 0, axi.AsError(err).Message)
				continue
			}
			outcome := firstNonEmpty(recordOutcomes[out.Outcome], out.Outcome)
			detail := ""
			if out.MetadataFailed {
				counts["no_store_data"]++
				detail = "store lookup failed: linked, but no store metadata or cover"
			}
			add(row, outcome, out.AssetID, detail)
			continue
		}

		// matched or swap
		current, ok := links[row.AssetID]
		switch {
		case !ok:
			add(row, "failed", row.AssetID, "no such asset in this library")
			continue
		case current != "" && sameListing(current, row.Link):
			add(row, "unchanged", row.AssetID, "")
			continue
		case current != "" && row.Action == actionMatched:
			add(row, "kept", row.AssetID, "kept its own link "+current)
			continue
		}
		outcome := "linked"
		if current != "" {
			outcome = "swapped"
		}
		noStoreData, err := linkAsset(ctx, client, row.AssetID, row.Link, current != "", pace)
		lastLookup = time.Now()
		if err != nil {
			if failsEveryRow(err) {
				return err
			}
			add(row, "failed", row.AssetID, axi.AsError(err).Message)
			continue
		}
		links[row.AssetID] = row.Link
		detail := ""
		if noStoreData != "" {
			counts["no_store_data"]++
			detail = "store lookup failed: linked, but no store metadata or cover (" + noStoreData + ")"
		}
		add(row, outcome, row.AssetID, detail)
	}

	table := axi.Table{
		Key: "recorded", Columns: []string{"package_id", "name", "outcome", "asset_id", "detail"},
		Truncatable: []string{"detail"}, Rows: report, Total: -1,
		EmptyNote: "every row was dropped",
	}
	summary := Block{Key: "record", Fields: []toon.KV{
		{Key: "rows", Value: len(rows)},
		{Key: "added", Value: counts[recordOutcomes["created"]]},
		{Key: "already_in_library", Value: counts[recordOutcomes["exists"]]},
		{Key: "linked", Value: counts["linked"]},
		{Key: "swapped", Value: counts["swapped"]},
		{Key: "unchanged", Value: counts["unchanged"]},
		{Key: "kept_own_link", Value: counts["kept"]},
		{Key: "dropped", Value: counts["dropped"]},
		{Key: "failed", Value: counts["failed"]},
		{Key: "no_store_data", Value: counts["no_store_data"]},
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

// linkAsset marks an existing asset Needs review, sets its store link, then
// asks the server to look the listing up and fill the asset's blank fields.
// Needs review comes first: an asset linked but not flagged would read as
// unchanged on a rerun and never be flagged. The link is set before the lookup
// so a failed lookup still leaves the asset linked, as a new reference asset
// is; that failure comes back as noStoreData, not err.
//
// A swap also blanks the old store's name, product ID and version: the lookup
// fills only blank fields, so without this the asset would keep the old
// store's identity beside its new Unity link.
func linkAsset(ctx context.Context, client *Client, assetID uint, link string, swap bool, pace func() error) (noStoreData string, err error) {
	id := strconv.FormatUint(uint64(assetID), 10)
	review := map[string]any{"asset_ids": []uint{assetID}, "action": "unreview"}
	if err := client.PostJSON(ctx, "/api/assets/bulk", review, nil); err != nil {
		return "", err
	}
	patch := map[string]any{"source_url": link}
	if swap {
		patch["store"], patch["store_product_id"], patch["latest_store_version"] = "", "", ""
	}
	if err := client.PatchJSON(ctx, "/api/assets/"+id, patch, nil); err != nil {
		return "", err
	}
	if err := pace(); err != nil {
		return "", err
	}
	if err := client.PostJSON(ctx, "/api/assets/"+id+"/metadata/fetch", nil, nil); err != nil {
		if failsEveryRow(err) {
			return "", err
		}
		return axi.AsError(err).Message, nil
	}
	return "", nil
}
