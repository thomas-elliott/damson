package main

import (
	"context"
	"os"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// homeCommand is what a bare `dmsn` runs. Per AXI principle 8 it shows live
// state rather than a help screen, so an agent that types the binary name
// learns what it is connected to in one call.
var homeCommand = &command{
	name:    "",
	summary: "show the current library at a glance",
	usage:   "usage: dmsn",
	hidden:  true,
	run:     runHome,
}

// overviewResponse mirrors GET /api/overview. Decoded into the CLI's own
// struct rather than importing internal/handlers/api, which would drag gin
// into the client binary.
type overviewResponse struct {
	Library struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"library"`
	Counts struct {
		Assets           int `json:"assets"`
		NeedsReview      int `json:"needs_review"`
		UpdatesAvailable int `json:"updates_available"`
		Collections      int `json:"collections"`
		Tags             int `json:"tags"`
		Projects         int `json:"projects"`
		ImportsInFlight  int `json:"imports_in_flight"`
		ImportsFailed    int `json:"imports_failed"`
	} `json:"counts"`
	RecentImports []struct {
		ID        uint      `json:"id"`
		AssetID   uint      `json:"asset_id"`
		AssetName string    `json:"asset_name"`
		Stage     string    `json:"stage"`
		Error     string    `json:"error"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"recent_imports"`
}

func runHome(ctx context.Context, e *Env) error {
	self := Block{Key: binaryName, Fields: []toon.KV{
		{Key: "version", Value: clientVersion()},
		{Key: "path", Value: displayPath(selfPath())},
		{Key: "about", Value: toon.Text("manage a damson asset library: upload, tag, collect, curate")},
	}}

	client, err := e.Client()
	if err != nil {
		// Not signed in is a state, not a failure: say so, and say what to do.
		return e.Records([]Block{self, disconnected(err)}, []string{
			"dmsn login --server <url> --token <token>",
			"dmsn help",
		})
	}

	var ov overviewResponse
	if err := client.GetJSON(ctx, "/api/overview", nil, &ov); err != nil {
		return e.Records([]Block{self, disconnected(err)}, []string{
			"dmsn whoami",
			"dmsn login --server <url> --token <token>",
		})
	}

	library := Block{Key: "library", Fields: []toon.KV{
		{Key: "name", Value: ov.Library.Name},
		{Key: "server", Value: client.BaseURL},
		{Key: "role", Value: ov.Library.Role},
		{Key: "assets", Value: ov.Counts.Assets},
		{Key: "needs_review", Value: ov.Counts.NeedsReview},
		{Key: "updates_available", Value: ov.Counts.UpdatesAvailable},
		{Key: "collections", Value: ov.Counts.Collections},
		{Key: "tags", Value: ov.Counts.Tags},
		{Key: "projects", Value: ov.Counts.Projects},
		{Key: "imports_in_flight", Value: ov.Counts.ImportsInFlight},
		{Key: "imports_failed", Value: ov.Counts.ImportsFailed},
	}}

	rows := make([]map[string]any, 0, len(ov.RecentImports))
	for _, r := range ov.RecentImports {
		rows = append(rows, map[string]any{
			"asset_id":   r.AssetID,
			"asset_name": r.AssetName,
			"stage":      r.Stage,
			"error":      blankToNil(r.Error),
			"updated_at": r.UpdatedAt,
		})
	}
	table := axi.Table{
		Key:         "recent_imports",
		Columns:     []string{"asset_id", "asset_name", "stage"},
		Truncatable: []string{"error"},
		Rows:        rows,
		Total:       -1,
		EmptyNote:   emptyImportsNote(ov),
	}
	// One call, so --json yields one parseable document rather than three.
	return axi.RenderComposite(e.Out, []Block{self, library}, []axi.Table{table},
		homeHelp(ov), e.Globals.options())
}

// emptyImportsNote makes "nothing here" a statement rather than a shrug: an
// empty library and a busy one with no recent activity are different facts.
func emptyImportsNote(ov overviewResponse) string {
	if ov.Counts.Assets == 0 {
		return "nothing imported yet — start with `dmsn upload <path>`"
	}
	return axi.Count(ov.Counts.Assets, "asset") + " in the library, none imported recently"
}

// homeHelp offers the work that is actually outstanding first, so the next
// command follows from the state rather than from a fixed menu.
func homeHelp(ov overviewResponse) []string {
	var help []string
	if ov.Counts.ImportsFailed > 0 {
		help = append(help, "dmsn imports --stage failed")
	}
	if ov.Counts.NeedsReview > 0 {
		help = append(help, "dmsn assets --needs-review")
	}
	if ov.Counts.UpdatesAvailable > 0 {
		help = append(help, "dmsn assets --updates-available")
	}
	help = append(help, "dmsn assets", "dmsn upload <path> --as asset", "dmsn help")
	return help
}

func disconnected(err error) Block {
	return Block{Key: "library", Fields: []toon.KV{
		{Key: "connected", Value: false},
		{Key: "reason", Value: toon.Text(axi.AsError(err).Message)},
	}}
}

// selfPath resolves the running binary, falling back to the bare name when
// the OS won't say.
func selfPath() string {
	p, err := os.Executable()
	if err != nil || p == "" {
		return binaryName
	}
	return p
}
