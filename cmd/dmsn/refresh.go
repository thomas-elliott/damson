package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// progressOut takes the lines a long command prints as it goes, so they stay
// out of the result on stdout. Tests swap it.
var progressOut io.Writer = os.Stderr

// batchPollInterval is how often a waiting refresh asks the server for its
// batch's counts.
var batchPollInterval = 2 * time.Second

// lookupBatch is the server's report on a batch of store lookups.
type lookupBatch struct {
	BatchID       uint `json:"batch_id"`
	Refresh       bool `json:"refresh"`
	Total         int  `json:"total"`
	Queued        int  `json:"queued"`
	Done          int  `json:"done"`
	Failed        int  `json:"failed"`
	Changed       int  `json:"changed"`
	Unchanged     int  `json:"unchanged"`
	NoStoreData   int  `json:"no_store_data"`
	Conflicted    int  `json:"conflicted"`
	SkippedNoLink int  `json:"skipped_no_link"`
	Problems      []struct {
		AssetID   uint   `json:"asset_id"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		Result    string `json:"result"`
		Error     string `json:"error"`
		Conflicts []struct {
			Field string `json:"field"`
		} `json:"conflicts"`
	} `json:"problems"`
}

// progressLine is one line of a waiting batch's progress.
func (b lookupBatch) progressLine() string {
	return fmt.Sprintf("store lookups: %d of %d run, %d failed, %d no store data",
		b.Total-b.Queued, b.Total, b.Failed, b.NoStoreData)
}

func batchPath(id uint) string { return "/api/store-lookups/batches/" + strconv.FormatUint(uint64(id), 10) }

// waitForBatch polls the batch until nothing in it is queued, printing a
// progress line whenever the counts move. Stopping dmsn stops only the
// waiting; the server goes on with the lookups.
func waitForBatch(ctx context.Context, client *Client, b lookupBatch) (lookupBatch, error) {
	last := ""
	for {
		if line := b.progressLine(); line != last {
			fmt.Fprintln(progressOut, line)
			last = line
		}
		if b.Queued == 0 {
			return b, nil
		}
		select {
		case <-ctx.Done():
			return b, ctx.Err()
		case <-time.After(batchPollInterval):
		}
		skipped := b.SkippedNoLink
		if err := client.GetJSON(ctx, batchPath(b.BatchID), nil, &b); err != nil {
			return b, err
		}
		b.SkippedNoLink = skipped
	}
}

// renderBatch prints a batch's counts and, as a table, each lookup that
// failed, got only generic details, or kept a field a person had changed.
func renderBatch(e *Env, b lookupBatch, waited bool) error {
	fields := []toon.KV{
		{Key: "batch", Value: b.BatchID},
		{Key: "assets", Value: b.Total},
		{Key: "queued", Value: b.Queued},
		{Key: "changed", Value: b.Changed},
		{Key: "unchanged", Value: b.Unchanged},
		{Key: "no_store_data", Value: b.NoStoreData},
		{Key: "failed", Value: b.Failed},
		{Key: "kept_a_persons_field", Value: b.Conflicted},
	}
	if b.SkippedNoLink > 0 {
		fields = append(fields, toon.KV{Key: "skipped_no_link", Value: b.SkippedNoLink})
	}
	rows := make([]map[string]any, 0, len(b.Problems))
	for _, p := range b.Problems {
		var kept []string
		for _, c := range p.Conflicts {
			kept = append(kept, c.Field)
		}
		rows = append(rows, map[string]any{
			"asset_id": p.AssetID, "name": p.Name, "outcome": firstNonEmpty(p.Result, p.Status),
			"kept": blankToNil(strings.Join(kept, " ")), "error": blankToNil(p.Error),
		})
	}
	table := axi.Table{
		Key: "problems", Columns: []string{"asset_id", "name", "outcome", "kept", "error"},
		Truncatable: []string{"error"}, Rows: rows, Total: -1,
		EmptyNote: "no lookup failed or kept a field",
	}
	var help []string
	if !waited && b.Queued > 0 {
		help = append(help, fmt.Sprintf("%s bulk refresh --batch %d", binaryName, b.BatchID))
	}
	return axi.RenderComposite(e.Out, []Block{{Key: "refresh", Fields: fields}}, nil, []axi.Table{table}, help, e.Globals.options())
}

// runAssetRefresh refreshes one asset's store details and waits for the
// result.
func runAssetRefresh(ctx context.Context, e *Env, args []string) error {
	id, err := oneID(args, "asset refresh", "asset id", binaryName+" asset refresh <id>")
	if err != nil {
		return err
	}
	return queueRefresh(ctx, e, map[string]any{"asset_ids": []any{mustUint(id)}}, nil, true)
}

// runBulkRefresh refreshes the named or matching assets, or reports on a batch
// queued earlier.
func runBulkRefresh(ctx context.Context, e *Env, assets, tag string, filters *assetFilters, noWait bool, batch uint) error {
	if batch != 0 {
		client, err := e.Client()
		if err != nil {
			return err
		}
		var b lookupBatch
		if err := client.GetJSON(ctx, batchPath(batch), nil, &b); err != nil {
			return err
		}
		if !noWait {
			if b, err = waitForBatch(ctx, client, b); err != nil {
				return err
			}
		}
		return renderBatch(e, b, !noWait)
	}

	filters.tag = tag
	query, err := filters.selectionQuery()
	if err != nil {
		return err
	}
	body := map[string]any{}
	switch {
	case strings.TrimSpace(assets) != "":
		if len(query) > 0 {
			return axi.Usagef(nil, "give --assets or filters, not both")
		}
		ids, err := parseIDList(assets)
		if err != nil {
			return err
		}
		body["asset_ids"] = ids
	case len(query) == 0:
		return axi.Usagef([]string{
			binaryName + " bulk refresh --assets 1,2,3",
			binaryName + " bulk refresh --store generic --source-host assetstore.unity.com",
		}, "bulk refresh needs --assets or at least one filter")
	}
	return queueRefresh(ctx, e, body, query, !noWait)
}

// queueRefresh asks the server to refresh the assets, then waits for the batch
// or reports it straight away.
func queueRefresh(ctx context.Context, e *Env, body map[string]any, query url.Values, wait bool) error {
	client, err := e.Client()
	if err != nil {
		return err
	}
	body["refresh"] = true
	path := "/api/store-lookups"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	var b lookupBatch
	if err := client.PostJSON(ctx, path, body, &b); err != nil {
		return err
	}
	if wait {
		if b, err = waitForBatch(ctx, client, b); err != nil {
			return err
		}
	}
	return renderBatch(e, b, wait)
}
