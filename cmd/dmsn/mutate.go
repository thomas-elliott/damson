package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/tagpath"
	"github.com/thomas-elliott/damson/internal/toon"
)

// editableFields are the asset fields `asset edit` can set, in the order they
// appear in --help. Each maps to a PATCH /api/assets/:id body key.
var editableFields = []string{
	"name", "description", "source-url", "location-path",
	"publisher", "publisher-url", "store", "store-product-id",
	"released-at", "store-updated-at", "latest-store-version",
	"engine", "min-engine-version", "price", "price-currency",
	"rating", "rating-count",
}

// bodyKey converts a flag name to its JSON field.
func bodyKey(flagName string) string { return strings.ReplaceAll(flagName, "-", "_") }

type editFlags struct {
	values map[string]*string
	clear  string
}

func newEditFlags() *editFlags {
	return &editFlags{values: make(map[string]*string, len(editableFields))}
}

func (f *editFlags) register(fs *flag.FlagSet) {
	for _, name := range editableFields {
		v := new(string)
		f.values[name] = v
		fs.StringVar(v, name, "", "set "+bodyKey(name))
	}
	fs.StringVar(&f.clear, "clear", "", "comma-separated fields to clear to null")
}

// body builds the PATCH payload from whichever flags were actually set. The
// API treats an absent key as "leave alone" and an empty string as "clear",
// so --clear is expressed as an explicit empty string.
func (f *editFlags) body(e *Env) (map[string]any, error) {
	body := map[string]any{}
	for _, name := range editableFields {
		if e.flagWasSet(name) {
			body[bodyKey(name)] = *f.values[name]
		}
	}
	for _, raw := range axi.ParseFields(f.clear) {
		name := strings.TrimSpace(raw)
		if !slices.Contains(editableFields, name) {
			return nil, axi.Usagef(
				[]string{"clearable fields: " + strings.Join(editableFields, ",")},
				"cannot clear unknown field %q", name)
		}
		if e.flagWasSet(name) {
			return nil, axi.Usagef(nil, "--%s and --clear %s contradict each other", name, name)
		}
		body[bodyKey(name)] = ""
	}
	if len(body) == 0 {
		return nil, axi.Usagef(
			[]string{"dmsn asset edit <id> --name <name>", "dmsn asset edit <id> --clear description"},
			"nothing to change: set at least one field")
	}
	return body, nil
}

func init() {
	edit := newEditFlags()
	var assetYes, needsReview bool
	register(&command{
		name:    "asset",
		summary: "show or modify one asset",
		usage: "usage: dmsn asset <id>\n" +
			"       dmsn asset edit <id> [--name …] [--clear FIELD,…]\n" +
			"       dmsn asset record [--location-path PATH] [--source-url URL] [--name NAME] [--needs-review]\n" +
			"       dmsn asset review|unreview <id>\n" +
			"       dmsn asset tag|untag <id> <path>...\n" +
			"       dmsn asset rm <id> --yes\n\n" +
			"editable: " + strings.Join(editableFields, ", ") + "\n\n" +
			"review, tag and untag are idempotent and report whether anything\n" +
			"changed. rm is destructive and requires --yes.\n\n" +
			"record adds an asset by where it lives, with nothing imported: a path,\n" +
			"a store link, or both. Recording a known path updates that asset; a\n" +
			"store link already in the library names the asset that has it.",
		flags: func(fs *flag.FlagSet) {
			edit.register(fs)
			fs.BoolVar(&assetYes, "yes", false, "confirm a destructive action")
			fs.BoolVar(&needsReview, "needs-review", false, "record: mark the asset Needs review")
		},
		run: func(ctx context.Context, e *Env) error {
			if len(e.Args) > 0 && e.Args[0] == "record" {
				return runAssetRecord(ctx, e, edit, needsReview)
			}
			if e.flagWasSet("needs-review") {
				return axi.Usagef([]string{"dmsn asset review <id>"},
					"--needs-review only applies to asset record")
			}
			return runAssetVerb(ctx, e, edit, assetYes)
		},
	})

	var tagYes bool
	register(&command{
		name:    "tag",
		summary: "create or delete a tag",
		usage: "usage: dmsn tag new <path>\n" +
			"       dmsn tag rm <path> --yes\n\n" +
			"Paths are `/`-separated; missing ancestors are created.",
		flags: func(fs *flag.FlagSet) { fs.BoolVar(&tagYes, "yes", false, "confirm deletion") },
		run:   func(ctx context.Context, e *Env) error { return runTagVerb(ctx, e, tagYes) },
	})

	var collName, collDesc, collAssets string
	var collYes bool
	register(&command{
		name:    "collection",
		summary: "show or modify a collection",
		usage: "usage: dmsn collection <id>\n" +
			"       dmsn collection new <name> [--description TEXT]\n" +
			"       dmsn collection edit <id> [--name NAME] [--description TEXT]\n" +
			"       dmsn collection rm <id> --yes\n" +
			"       dmsn collection add|remove <id> --assets 1,2,3",
		flags: func(fs *flag.FlagSet) {
			fs.StringVar(&collName, "name", "", "collection name")
			fs.StringVar(&collDesc, "description", "", "collection description")
			fs.StringVar(&collAssets, "assets", "", "comma-separated asset ids")
			fs.BoolVar(&collYes, "yes", false, "confirm deletion")
		},
		run: func(ctx context.Context, e *Env) error {
			return runCollectionVerb(ctx, e, collName, collDesc, collAssets, collYes)
		},
	})

	var bulkAssets, bulkTag string
	var bulkYes bool
	register(&command{
		name:    "bulk",
		summary: "apply one action to many assets",
		usage: "usage: dmsn bulk tag|untag --assets 1,2,3 --tag <path>\n" +
			"       dmsn bulk review|unreview --assets 1,2,3\n" +
			"       dmsn bulk rm --assets 1,2,3 --yes\n\n" +
			"One transaction, audited, capped at 1000 assets. Reports `affected`.",
		flags: func(fs *flag.FlagSet) {
			fs.StringVar(&bulkAssets, "assets", "", "comma-separated asset ids")
			fs.StringVar(&bulkTag, "tag", "", "tag path, for tag/untag")
			fs.BoolVar(&bulkYes, "yes", false, "confirm a destructive action")
		},
		run: func(ctx context.Context, e *Env) error {
			return runBulk(ctx, e, bulkAssets, bulkTag, bulkYes)
		},
	})

	register(&command{
		name:    "import",
		summary: "retry a failed import",
		usage:   "usage: dmsn import retry <id>",
		run:     runImportVerb,
	})
}

// runAssetVerb dispatches `asset` — a bare id reads, a leading verb mutates.
func runAssetVerb(ctx context.Context, e *Env, edit *editFlags, yes bool) error {
	if len(e.Args) == 0 {
		return axi.Usagef(
			[]string{"dmsn asset <id>", "dmsn assets"},
			"asset needs an id or a verb")
	}
	verb := e.Args[0]
	rest := e.Args[1:]

	switch verb {
	case "edit":
		return runAssetEdit(ctx, e, edit, rest)
	case "review", "unreview":
		return runAssetReview(ctx, e, verb, rest)
	case "tag", "untag":
		return runAssetTag(ctx, e, verb, rest)
	case "rm":
		return runAssetRemove(ctx, e, rest, yes)
	default:
		return runAsset(ctx, e)
	}
}

func runAssetEdit(ctx context.Context, e *Env, edit *editFlags, args []string) error {
	id, err := oneID(args, "asset edit", "asset id", "dmsn asset edit <id> --name <name>")
	if err != nil {
		return err
	}
	body, err := edit.body(e)
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	var out assetDetailResponse
	if err := client.PatchJSON(ctx, "/api/assets/"+id, body, &out); err != nil {
		return err
	}

	changed := make([]string, 0, len(body))
	for k := range body {
		changed = append(changed, k)
	}
	sort.Strings(changed)
	return e.Block("asset", []toon.KV{
		{Key: "id", Value: out.ID},
		{Key: "name", Value: out.Name},
		{Key: "changed", Value: len(changed)},
		{Key: "fields", Value: toon.Text(strings.Join(changed, ","))},
	}, []string{fmt.Sprintf("dmsn asset %d", out.ID)})
}

// recordOutcomes names each record result for the reader: a known path is
// updated, never added again.
var recordOutcomes = map[string]string{
	"created": "added",
	"updated": "updated",
	"exists":  "already_in_library",
}

func runAssetRecord(ctx context.Context, e *Env, edit *editFlags, needsReview bool) error {
	usage := []string{
		"dmsn asset record --location-path /mnt/assets/Publisher/Pack",
		"dmsn asset record --source-url https://assetstore.unity.com/packages/… --name <name>",
	}
	if len(e.Args) > 1 {
		return axi.Usagef(usage, "asset record takes flags, not arguments; got %q", e.Args[1])
	}
	recordFlags := []string{"location-path", "source-url", "name"}
	for _, name := range append(slices.Clone(editableFields), "clear", "yes") {
		if e.flagWasSet(name) && !slices.Contains(recordFlags, name) {
			return axi.Usagef(usage, "--%s does not apply to asset record; set it afterwards with asset edit", name)
		}
	}
	body := map[string]any{"needs_review": needsReview}
	for _, name := range recordFlags {
		if v := strings.TrimSpace(*edit.values[name]); v != "" {
			body[bodyKey(name)] = v
		}
	}
	if body["location_path"] == nil && body["source_url"] == nil {
		return axi.Usagef(usage, "asset record needs --location-path, --source-url, or both")
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	var out struct {
		Outcome      string `json:"outcome"`
		AssetID      uint   `json:"asset_id"`
		Name         string `json:"name"`
		LocationPath string `json:"location_path"`
		SourceURL    string `json:"source_url"`
	}
	if err := client.PostJSON(ctx, "/api/assets/references", body, &out); err != nil {
		var ae *axi.Error
		if errors.As(err, &ae) && ae.Code == axi.CodeUsage && len(ae.Help) == 0 {
			ae.Help = usage
		}
		return err
	}
	return e.Block("record", []toon.KV{
		{Key: "outcome", Value: firstNonEmpty(recordOutcomes[out.Outcome], out.Outcome)},
		{Key: "asset_id", Value: out.AssetID},
		{Key: "name", Value: out.Name},
		{Key: "location_path", Value: blankToNil(out.LocationPath)},
		{Key: "source_url", Value: blankToNil(out.SourceURL)},
	}, []string{fmt.Sprintf("dmsn asset %d", out.AssetID)})
}

func runAssetReview(ctx context.Context, e *Env, verb string, args []string) error {
	id, err := oneID(args, "asset "+verb, "asset id", "dmsn asset "+verb+" <id>")
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}

	// The endpoint toggles rather than sets, so read first and act only if the
	// asset is not already in the requested state — that makes the verb
	// idempotent and lets it report honestly whether anything changed.
	var before assetDetailResponse
	if err := client.GetJSON(ctx, "/api/assets/"+id, nil, &before); err != nil {
		return err
	}
	want := verb == "review"
	already := before.NeedsReview != want
	if !already {
		if err := client.PostJSON(ctx, "/api/assets/"+id+"/review", nil, nil); err != nil {
			return err
		}
	}
	return e.Block(verb, []toon.KV{
		{Key: "asset_id", Value: before.ID},
		{Key: "name", Value: before.Name},
		{Key: "changed", Value: !already},
	}, []string{fmt.Sprintf("dmsn asset %s", id)})
}

func runAssetTag(ctx context.Context, e *Env, verb string, args []string) error {
	if len(args) < 2 {
		return axi.Usagef(
			[]string{"dmsn asset " + verb + " <id> <path>..."},
			"asset %s takes an id and at least one tag path", verb)
	}
	id, err := parsePositiveID(args[0], "asset id")
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}

	// Attaching is naturally idempotent — tags are unique per library path —
	// so this reports which paths were newly present rather than pretending
	// each call changed something.
	var before assetDetailResponse
	if err := client.GetJSON(ctx, "/api/assets/"+id, nil, &before); err != nil {
		return err
	}
	// The server normalises paths (lowercase, collapsed slashes), so compare
	// on the normalised form — otherwise every attach looks new.
	held := map[string]uint{}
	for _, t := range before.Tags {
		held[tagpath.Normalise(t.Path)] = t.ID
	}

	changed := 0
	rows := make([]map[string]any, 0, len(args)-1)
	for _, path := range args[1:] {
		tagID, has := held[tagpath.Normalise(path)]
		action := "unchanged"
		switch {
		case verb == "tag" && !has:
			if err := client.PostJSON(ctx, "/api/assets/"+id+"/tags", map[string]any{"path": path}, nil); err != nil {
				return err
			}
			action, changed = "attached", changed+1
		case verb == "untag" && has:
			if err := client.Delete(ctx, fmt.Sprintf("/api/assets/%s/tags/%d", id, tagID)); err != nil {
				return err
			}
			action, changed = "detached", changed+1
		}
		rows = append(rows, map[string]any{"path": path, "action": action})
	}

	table := axi.Table{
		Key: "tags", Columns: []string{"path", "action"}, Rows: rows, Total: -1,
		EmptyNote: "no tag paths given",
	}
	summary := Block{Key: verb, Fields: []toon.KV{
		{Key: "asset_id", Value: before.ID},
		{Key: "changed", Value: changed},
	}}
	return axi.RenderComposite(e.Out, []Block{summary}, nil, []axi.Table{table},
		[]string{fmt.Sprintf("dmsn asset %s", id)}, e.Globals.options())
}

func runAssetRemove(ctx context.Context, e *Env, args []string, yes bool) error {
	id, err := oneID(args, "asset rm", "asset id", "dmsn asset rm <id> --yes")
	if err != nil {
		return err
	}
	if !yes {
		return confirmRequired(fmt.Sprintf("dmsn asset rm %s --yes", id),
			"deleting asset %s also deletes its versions and files", id)
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, "/api/assets/"+id); err != nil {
		return err
	}
	return e.Block("deleted", []toon.KV{
		{Key: "asset_id", Value: mustUint(id)},
	}, []string{"dmsn assets"})
}

func runTagVerb(ctx context.Context, e *Env, yes bool) error {
	if len(e.Args) == 0 {
		return axi.Usagef([]string{"dmsn tags", "dmsn tag new <path>"}, "tag needs a verb")
	}
	verb, rest := e.Args[0], e.Args[1:]
	if len(rest) != 1 {
		return axi.Usagef([]string{"dmsn tag " + verb + " <path>"}, "tag %s takes one path", verb)
	}
	path := strings.TrimSpace(rest[0])
	if path == "" {
		return axi.Usagef(nil, "tag path must not be blank")
	}
	client, err := e.Client()
	if err != nil {
		return err
	}

	switch verb {
	case "new":
		var out tagRow
		if err := client.PostJSON(ctx, "/api/tags", map[string]any{"path": path}, &out); err != nil {
			return err
		}
		return e.Block("tag", []toon.KV{
			{Key: "id", Value: out.ID},
			{Key: "path", Value: out.Path},
		}, []string{"dmsn tags", fmt.Sprintf("dmsn assets --tag %s", path)})
	case "rm":
		if !yes {
			return confirmRequired(fmt.Sprintf("dmsn tag rm %s --yes", path),
				"deleting tag %q detaches it from every asset that carries it", path)
		}
		id, err := e.tagIDForPath(ctx, client, path)
		if err != nil {
			return err
		}
		if err := client.Delete(ctx, fmt.Sprintf("/api/tags/%d", id)); err != nil {
			return err
		}
		return e.Block("deleted", []toon.KV{
			{Key: "tag", Value: path},
			{Key: "id", Value: id},
		}, []string{"dmsn tags"})
	default:
		return axi.Usagef([]string{"dmsn tag new <path>", "dmsn tag rm <path> --yes"},
			"unknown tag verb %q", verb)
	}
}

// tagIDForPath resolves a path to an id, since DELETE /api/tags takes an id
// while the CLI speaks in paths throughout.
func (e *Env) tagIDForPath(ctx context.Context, client *Client, path string) (uint, error) {
	var resp struct {
		Tags []tagRow `json:"tags"`
	}
	if err := client.GetJSON(ctx, "/api/tags", nil, &resp); err != nil {
		return 0, err
	}
	want := tagpath.Normalise(path)
	for _, t := range resp.Tags {
		if tagpath.Normalise(t.Path) == want {
			return t.ID, nil
		}
	}
	return 0, &axi.Error{
		Code:    axi.CodeNotFound,
		Message: fmt.Sprintf("no tag with path %q", path),
		Help:    []string{"dmsn tags"},
	}
}

func runCollectionVerb(ctx context.Context, e *Env, name, desc, assets string, yes bool) error {
	if len(e.Args) == 0 {
		return axi.Usagef([]string{"dmsn collections", "dmsn collection <id>"}, "collection needs an id or a verb")
	}
	verb, rest := e.Args[0], e.Args[1:]
	if _, isID := parsePositiveID(verb, "collection id"); isID == nil {
		return runCollection(ctx, e)
	}
	client, err := e.Client()
	if err != nil {
		return err
	}

	switch verb {
	case "new":
		return runCollectionNew(ctx, e, client, rest, desc)
	case "edit":
		return runCollectionEdit(ctx, e, client, rest, name, desc)
	case "rm":
		return runCollectionRemove(ctx, e, client, rest, yes)
	case "add", "remove":
		return runCollectionMembers(ctx, e, client, verb, rest, assets)
	default:
		return axi.Usagef(
			[]string{"dmsn collection <id>", "dmsn collection new <name>", "dmsn collections"},
			"unknown collection verb %q", verb)
	}
}

func runCollectionNew(ctx context.Context, e *Env, client *Client, args []string, desc string) error {
	if len(args) != 1 {
		return axi.Usagef([]string{`dmsn collection new "Name"`}, "collection new takes one name")
	}
	body := map[string]any{"name": args[0]}
	if desc != "" {
		body["description"] = desc
	}
	var out collectionRow
	if err := client.PostJSON(ctx, "/api/collections", body, &out); err != nil {
		return err
	}
	return e.Block("collection", []toon.KV{
		{Key: "id", Value: out.ID}, {Key: "name", Value: out.Name},
	}, []string{fmt.Sprintf("dmsn collection add %d --assets <ids>", out.ID)})
}

func runCollectionEdit(ctx context.Context, e *Env, client *Client, args []string, name, desc string) error {
	id, err := oneID(args, "collection edit", "collection id", "dmsn collection edit <id> --name <name>")
	if err != nil {
		return err
	}
	// An unset flag is absent from the body, so the API leaves that column
	// alone; a flag set to "" is a deliberate clear.
	body := map[string]any{}
	if e.flagWasSet("name") {
		body["name"] = name
	}
	if e.flagWasSet("description") {
		body["description"] = desc
	}
	if len(body) == 0 {
		return axi.Usagef(
			[]string{"dmsn collection edit <id> --name <name>", "dmsn collection edit <id> --description <text>"},
			"nothing to change: set --name or --description")
	}
	var out collectionRow
	if err := client.PatchJSON(ctx, "/api/collections/"+id, body, &out); err != nil {
		return err
	}
	return e.Block("collection", []toon.KV{
		{Key: "id", Value: out.ID}, {Key: "name", Value: out.Name},
		{Key: "changed", Value: len(body)},
	}, []string{fmt.Sprintf("dmsn collection %d", out.ID)})
}

func runCollectionRemove(ctx context.Context, e *Env, client *Client, args []string, yes bool) error {
	id, err := oneID(args, "collection rm", "collection id", "dmsn collection rm <id> --yes")
	if err != nil {
		return err
	}
	if !yes {
		return confirmRequired(fmt.Sprintf("dmsn collection rm %s --yes", id),
			"deleting collection %s removes the grouping; the assets themselves stay", id)
	}
	if err := client.Delete(ctx, "/api/collections/"+id); err != nil {
		return err
	}
	return e.Block("deleted", []toon.KV{
		{Key: "collection_id", Value: mustUint(id)},
	}, []string{"dmsn collections"})
}

func runCollectionMembers(ctx context.Context, e *Env, client *Client, verb string, args []string, assets string) error {
	id, err := oneID(args, "collection "+verb, "collection id",
		"dmsn collection "+verb+" <id> --assets 1,2,3")
	if err != nil {
		return err
	}
	ids, err := parseIDList(assets)
	if err != nil {
		return err
	}
	return e.collectionMembership(ctx, client, verb, id, ids)
}

// collectionMembership adds or removes assets. Adding goes through the bulk
// endpoint, which is one audited transaction; removing has no bulk action, so
// it is a loop over the per-asset route.
func (e *Env) collectionMembership(ctx context.Context, client *Client, verb, id string, assetIDs []uint) error {
	if verb == "add" {
		var out struct {
			Affected int `json:"affected"`
		}
		body := map[string]any{
			"asset_ids":     assetIDs,
			"action":        "add_to_collection",
			"collection_id": mustUint(id),
		}
		if err := client.PostJSON(ctx, "/api/assets/bulk", body, &out); err != nil {
			return err
		}
		return e.Block("collection", []toon.KV{
			{Key: "id", Value: mustUint(id)},
			{Key: "requested", Value: len(assetIDs)},
			{Key: "affected", Value: out.Affected},
		}, []string{fmt.Sprintf("dmsn collection %s", id)})
	}

	removed := 0
	for _, a := range assetIDs {
		if err := client.Delete(ctx, fmt.Sprintf("/api/collections/%s/assets/%d", id, a)); err != nil {
			// A 404 means it was not a member — the end state is what was
			// asked for, so this stays a success.
			if axi.AsError(err).Code != axi.CodeNotFound {
				return err
			}
			continue
		}
		removed++
	}
	return e.Block("collection", []toon.KV{
		{Key: "id", Value: mustUint(id)},
		{Key: "requested", Value: len(assetIDs)},
		{Key: "affected", Value: removed},
	}, []string{fmt.Sprintf("dmsn collection %s", id)})
}

// bulkActions maps a CLI verb to the API action, and says whether it is
// destructive enough to need --yes.
var bulkActions = map[string]struct {
	action      string
	needsTag    bool
	destructive bool
}{
	"tag":      {action: "add_tag", needsTag: true},
	"untag":    {action: "remove_tag", needsTag: true},
	"review":   {action: "review"},
	"unreview": {action: "unreview"},
	"rm":       {action: "delete", destructive: true},
}

func runBulk(ctx context.Context, e *Env, assets, tag string, yes bool) error {
	if len(e.Args) != 1 {
		return axi.Usagef(
			[]string{"dmsn bulk tag --assets 1,2,3 --tag <path>", "dmsn bulk review --assets 1,2,3"},
			"bulk takes exactly one verb")
	}
	spec, ok := bulkActions[e.Args[0]]
	if !ok {
		return axi.Usagef(
			[]string{"verbs: " + strings.Join(sortedKeys(bulkActions), ",")},
			"unknown bulk verb %q", e.Args[0])
	}
	ids, err := parseIDList(assets)
	if err != nil {
		return err
	}
	body := map[string]any{"asset_ids": ids, "action": spec.action}
	if spec.needsTag {
		if strings.TrimSpace(tag) == "" {
			return axi.Usagef(
				[]string{fmt.Sprintf("dmsn bulk %s --assets %s --tag <path>", e.Args[0], assets)},
				"--tag is required for %s", e.Args[0])
		}
		body["path"] = strings.TrimSpace(tag)
	}
	if spec.destructive && !yes {
		return confirmRequired(
			fmt.Sprintf("dmsn bulk %s --assets %s --yes", e.Args[0], assets),
			"this deletes %s and every version and file they hold", axi.Count(len(ids), "asset"))
	}

	client, err := e.Client()
	if err != nil {
		return err
	}
	var out struct {
		Affected int `json:"affected"`
	}
	if err := client.PostJSON(ctx, "/api/assets/bulk", body, &out); err != nil {
		return err
	}

	help := []string{"dmsn assets"}
	if out.Affected < len(ids) {
		// The gap is meaningful: an id that was already in the target state,
		// or not in this library, is counted as requested but not affected.
		help = append([]string{fmt.Sprintf("%d of %d were already in that state or not in this library",
			len(ids)-out.Affected, len(ids))}, help...)
	}
	return e.Block("bulk", []toon.KV{
		{Key: "action", Value: spec.action},
		{Key: "requested", Value: len(ids)},
		{Key: "affected", Value: out.Affected},
	}, help)
}

func runImportVerb(ctx context.Context, e *Env) error {
	if len(e.Args) != 2 || e.Args[0] != "retry" {
		return axi.Usagef([]string{"dmsn import retry <id>", "dmsn imports"}, "usage: import retry <id>")
	}
	id, err := parsePositiveID(e.Args[1], "import id")
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	var out importEnqueueResponse
	if err := client.PostJSON(ctx, "/api/imports/"+id+"/retry", nil, &out); err != nil {
		return err
	}
	return e.Block("retry", []toon.KV{
		{Key: "import_id", Value: out.ImportID},
		{Key: "asset_id", Value: out.AssetID},
	}, []string{"dmsn imports", fmt.Sprintf("dmsn asset %d", out.AssetID)})
}

// confirmRequired is the refusal for a destructive verb without --yes. AXI
// forbids interactive prompts, so the command exits 2 with the exact line
// that would have worked.
func confirmRequired(command, why string, args ...any) error {
	return &axi.Error{
		Code:    axi.CodeUsage,
		Message: fmt.Sprintf(why, args...) + "; re-run with --yes to confirm",
		Help:    []string{command},
	}
}

// oneID validates the single positional id a verb takes, so a missing or
// malformed id reads identically across every verb.
func oneID(args []string, verb, noun, usage string) (string, error) {
	if len(args) != 1 {
		return "", axi.Usagef([]string{usage}, "%s takes one %s", verb, noun)
	}
	return parsePositiveID(args[0], noun)
}

// parseIDList decodes --assets. An empty list is a usage error rather than a
// no-op mutation that reports zero and looks like it worked.
func parseIDList(raw string) ([]uint, error) {
	parts := axi.ParseFields(raw)
	if len(parts) == 0 {
		return nil, axi.Usagef(
			[]string{"--assets 1,2,3", "dmsn assets --fields id"},
			"--assets is required")
	}
	ids := make([]uint, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil || n == 0 {
			return nil, axi.Usagef(nil, "--assets must be positive numbers, got %q", p)
		}
		ids = append(ids, uint(n))
	}
	return ids, nil
}

// mustUint renders an already-validated numeric id as a number rather than a
// quoted string, so ids read the same everywhere.
func mustUint(id string) any {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return id
	}
	return uint(n)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
