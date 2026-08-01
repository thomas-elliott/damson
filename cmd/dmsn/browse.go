package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/importstage"
	"github.com/thomas-elliott/damson/internal/toon"
)

type tagRow struct {
	ID         uint   `json:"id"`
	Path       string `json:"path"`
	ParentID   *uint  `json:"parent_id"`
	AssetCount int    `json:"asset_count"`
}

type collectionRow struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AssetCount  int       `json:"asset_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type importRow struct {
	ID          uint      `json:"id"`
	AssetID     uint      `json:"asset_id"`
	AssetName   string    `json:"asset_name"`
	Stage       string    `json:"stage"`
	Error       string    `json:"error"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
	NeedsReview bool      `json:"needs_review"`
}

func init() {
	tagPage := &pageFlags{}
	var tagPrefix string
	register(&command{
		name:    "tags",
		summary: "list the library's tag tree",
		usage: "usage: dmsn tags [--prefix PATH] [--limit N] [--offset N]\n\n" +
			"Paths are `/`-separated; the tree is flat on the wire.",
		flags: func(fs *flag.FlagSet) {
			tagPage.register(fs)
			fs.StringVar(&tagPrefix, "prefix", "", "only tags at or under this path")
		},
		run: func(ctx context.Context, e *Env) error { return runTags(ctx, e, tagPage, tagPrefix) },
	})

	collPage := &pageFlags{}
	register(&command{
		name:    "collections",
		summary: "list collections",
		usage:   "usage: dmsn collections [--limit N] [--offset N]",
		flags:   collPage.register,
		run:     func(ctx context.Context, e *Env) error { return runCollections(ctx, e, collPage) },
	})

	importPage := &pageFlags{}
	var stage string
	register(&command{
		name:    "imports",
		summary: "list imports and their stage",
		usage: "usage: dmsn imports [--stage STAGE] [--limit N] [--offset N]\n\n" +
			"Without --stage this is the review backlog: anything in flight,\n" +
			"failed, or done-but-unreviewed. --stage all shows everything.\n\n" +
			"stages: " + strings.Join(importstage.All(), ", "),
		flags: func(fs *flag.FlagSet) {
			importPage.register(fs)
			fs.StringVar(&stage, "stage", "", "filter by stage, or `all`")
		},
		run: func(ctx context.Context, e *Env) error { return runImports(ctx, e, importPage, stage) },
	})
}

func runTags(ctx context.Context, e *Env, page *pageFlags, prefixFlag string) error {
	if err := page.validate(); err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	var resp struct {
		Tags []tagRow `json:"tags"`
	}
	if err := client.GetJSON(ctx, "/api/tags", nil, &resp); err != nil {
		return err
	}

	// GET /api/tags returns the whole tree in one shot, so the prefix filter
	// and pagination are applied here rather than as query parameters.
	prefix := strings.TrimSpace(prefixFlag)
	matched := make([]tagRow, 0, len(resp.Tags))
	for _, t := range resp.Tags {
		if prefix == "" || t.Path == prefix || strings.HasPrefix(t.Path, prefix+"/") {
			matched = append(matched, t)
		}
	}
	total := int64(len(matched))
	windowed := window(matched, page)

	rows := make([]map[string]any, 0, len(windowed))
	for _, t := range windowed {
		rows = append(rows, map[string]any{
			"id": t.ID, "path": t.Path, "asset_count": t.AssetCount,
			"parent_id": orNil(t.ParentID),
		})
	}

	filters := ""
	if prefix != "" {
		filters = "prefix=" + prefix
	}
	table := axi.Table{
		Key:       "tags",
		Columns:   []string{"id", "path", "asset_count"},
		Allowed:   []string{"id", "path", "asset_count", "parent_id"},
		Rows:      rows,
		Total:     int(total),
		EmptyNote: emptyNote("tag", filters, int64(len(resp.Tags))),
	}
	help := []string{binaryName + " assets --tag <path>"}
	help = append(help, page.pageHelp("tags", len(rows), total)...)
	return axi.RenderTable(e.Out, table, help, e.Globals.options())
}

func runCollections(ctx context.Context, e *Env, page *pageFlags) error {
	if err := page.validate(); err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}
	var resp struct {
		Collections []collectionRow `json:"collections"`
	}
	if err := client.GetJSON(ctx, "/api/collections", nil, &resp); err != nil {
		return err
	}

	total := int64(len(resp.Collections))
	windowed := window(resp.Collections, page)
	rows := make([]map[string]any, 0, len(windowed))
	for _, c := range windowed {
		rows = append(rows, map[string]any{
			"id": c.ID, "name": c.Name, "asset_count": c.AssetCount,
			"description": blankToNil(c.Description),
			"created_at":  c.CreatedAt, "updated_at": c.UpdatedAt,
		})
	}

	table := axi.Table{
		Key:         "collections",
		Columns:     []string{"id", "name", "asset_count"},
		Allowed:     []string{"id", "name", "asset_count", "description", "created_at", "updated_at"},
		Truncatable: []string{"description"},
		Rows:        rows,
		Total:       int(total),
		EmptyNote:   "no collections yet — create one with `" + binaryName + " collection new <name>`",
	}
	help := []string{binaryName + " collection <id>"}
	help = append(help, page.pageHelp("collections", len(rows), total)...)
	return axi.RenderTable(e.Out, table, help, e.Globals.options())
}

func runCollection(ctx context.Context, e *Env) error {
	id, err := oneID(e.Args, "collection", "collection id", binaryName+" collection <id>")
	if err != nil {
		return err
	}
	client, err := e.Client()
	if err != nil {
		return err
	}

	var c collectionRow
	if err := client.GetJSON(ctx, "/api/collections/"+id, nil, &c); err != nil {
		return err
	}

	// The collection's members come from the asset list, which already
	// supports a collection filter — no second endpoint needed.
	q := url.Values{}
	q.Set("collection_id", id)
	q.Set("limit", "50")
	var assets assetListResponse
	if err := client.GetJSON(ctx, "/api/assets", q, &assets); err != nil {
		return err
	}

	rows := assetRows(assets.Assets)

	block := Block{Key: "collection", Fields: []toon.KV{
		{Key: "id", Value: c.ID},
		{Key: "name", Value: c.Name},
		{Key: "description", Value: blankToNil(c.Description)},
		{Key: "asset_count", Value: c.AssetCount},
		{Key: "created_at", Value: c.CreatedAt},
		{Key: "updated_at", Value: c.UpdatedAt},
	}}
	table := axi.Table{
		Key:       "assets",
		Columns:   assetColumns,
		Rows:      rows,
		Total:     int(assets.Total),
		EmptyNote: fmt.Sprintf("collection %q holds no assets", c.Name),
	}

	help := []string{
		fmt.Sprintf("%s collection add %d --assets <ids>", binaryName, c.ID),
		fmt.Sprintf("%s assets --collection %d", binaryName, c.ID),
	}
	if assets.Total > int64(len(rows)) {
		help = append([]string{fmt.Sprintf("%s assets --collection %d --limit %d", binaryName, c.ID, maxLimit)}, help...)
	}
	return axi.RenderComposite(e.Out, []Block{block}, nil, []axi.Table{table}, help, e.Globals.options())
}

func runImports(ctx context.Context, e *Env, page *pageFlags, stage string) error {
	client, err := e.Client()
	if err != nil {
		return err
	}
	q := url.Values{}
	if err := page.apply(q); err != nil {
		return err
	}
	if s := strings.TrimSpace(stage); s != "" {
		if s != "all" && !importstage.Valid(s) {
			return axi.Usagef(
				[]string{"stages: " + strings.Join(importstage.All(), ",") + ",all"},
				"unknown stage %q", s)
		}
		q.Set("stage", s)
	}

	var resp struct {
		Imports []importRow `json:"imports"`
		Total   int64       `json:"total"`
	}
	if err := client.GetJSON(ctx, "/api/imports", q, &resp); err != nil {
		return err
	}

	rows := make([]map[string]any, 0, len(resp.Imports))
	failed := 0
	for _, i := range resp.Imports {
		if i.Stage == "failed" {
			failed++
		}
		rows = append(rows, map[string]any{
			"id": i.ID, "asset_id": i.AssetID, "asset_name": i.AssetName,
			"stage": i.Stage, "error": blankToNil(i.Error),
			"needs_review": i.NeedsReview,
			"updated_at":   i.UpdatedAt, "created_at": i.CreatedAt,
		})
	}

	table := axi.Table{
		Key:         "imports",
		Columns:     []string{"id", "asset_name", "stage"},
		Allowed:     []string{"id", "asset_id", "asset_name", "stage", "error", "needs_review", "updated_at", "created_at"},
		Truncatable: []string{"error"},
		Rows:        rows,
		Total:       int(resp.Total),
		EmptyNote:   importsEmptyNote(strings.TrimSpace(stage)),
	}

	var help []string
	if failed > 0 {
		help = append(help,
			binaryName+" imports --stage failed --fields id,asset_name,error",
			binaryName+" import retry <id>")
	}
	help = append(help, page.pageHelp("imports", len(rows), resp.Total)...)
	help = append(help, binaryName+" imports --stage all")
	return axi.RenderTable(e.Out, table, help, e.Globals.options())
}

// importsEmptyNote distinguishes "the backlog is clear" from "nothing is in
// that stage" — the default view is a backlog, not a full listing.
func importsEmptyNote(stage string) string {
	switch stage {
	case "":
		return "the review backlog is clear — use `--stage all` to see finished imports"
	case "all":
		return "nothing has been imported yet"
	default:
		return fmt.Sprintf("no imports in stage %q — use `--stage all` to see every import", stage)
	}
}

// window applies client-side pagination for endpoints that return a whole
// collection in one response.
func window[T any](items []T, page *pageFlags) []T {
	if page.offset >= len(items) {
		return nil
	}
	end := page.offset + page.limit
	if end > len(items) {
		end = len(items)
	}
	return items[page.offset:end]
}
