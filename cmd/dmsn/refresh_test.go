package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
)

// captureProgress sends progress lines to a buffer and polls quickly.
func captureProgress(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevPoll := progressOut, batchPollInterval
	progressOut, batchPollInterval = &buf, time.Millisecond
	t.Cleanup(func() { progressOut, batchPollInterval = prevOut, prevPoll })
	return &buf
}

// refreshServer answers a refresh request with a batch of two queued
// lookups, then reports one run per poll.
func refreshServer(t *testing.T, gotQuery *url.Values, gotBody *map[string]any, polls *atomic.Int32) {
	t.Helper()
	withServer(t, map[string]http.HandlerFunc{
		"POST /api/store-lookups": func(w http.ResponseWriter, r *http.Request) {
			*gotQuery = r.URL.Query()
			_ = json.NewDecoder(r.Body).Decode(gotBody)
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"batch_id":9,"refresh":true,"total":2,"queued":2,"skipped_no_link":1,"problems":[]}`)
		},
		"GET /api/store-lookups/batches/9": func(w http.ResponseWriter, r *http.Request) {
			switch polls.Add(1) {
			case 1:
				fmt.Fprint(w, `{"batch_id":9,"total":2,"queued":1,"done":1,"changed":1,"problems":[]}`)
			default:
				fmt.Fprint(w, `{"batch_id":9,"total":2,"queued":0,"done":1,"failed":1,"changed":1,
				  "problems":[{"asset_id":5,"name":"Gone","status":"failed","error":"unity: no product 5"}]}`)
			}
		},
	})
}

func TestBulkRefresh_FiltersWaitAndReportProgress(t *testing.T) {
	isolate(t)
	progress := captureProgress(t)
	var query url.Values
	var body map[string]any
	var polls atomic.Int32
	refreshServer(t, &query, &body, &polls)

	out, code := exec(t, "bulk", "refresh", "--store", "generic", "--source-host", "assetstore.unity.com")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if query.Get("store") != "generic" || query.Get("source_host") != "assetstore.unity.com" || query.Has("limit") {
		t.Fatalf("query = %v, want the filters and no paging", query)
	}
	if body["refresh"] != true || body["asset_ids"] != nil {
		t.Fatalf("body = %v, want a refresh selected by the filters", body)
	}
	for _, want := range []string{"changed: 1", "failed: 1", "skipped_no_link: 1", "unity: no product 5"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimSpace(progress.String()), "\n")
	want := []string{
		"store lookups: 0 of 2 run, 0 failed, 0 no store data",
		"store lookups: 1 of 2 run, 0 failed, 0 no store data",
		"store lookups: 2 of 2 run, 1 failed, 0 no store data",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("progress =\n%s\nwant\n%s", progress, strings.Join(want, "\n"))
	}
	if strings.Contains(out, "store lookups:") {
		t.Fatal("progress must go to stderr, not the result")
	}
}

func TestBulkRefresh_NoWaitReturnsTheBatch(t *testing.T) {
	isolate(t)
	captureProgress(t)
	var query url.Values
	var body map[string]any
	var polls atomic.Int32
	refreshServer(t, &query, &body, &polls)

	out, code := exec(t, "bulk", "refresh", "--assets", "3,4", "--no-wait")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if polls.Load() != 0 {
		t.Fatal("--no-wait must not poll the batch")
	}
	if fmt.Sprint(body["asset_ids"]) != "[3 4]" {
		t.Fatalf("body = %v, want the named assets", body)
	}
	if !strings.Contains(out, "batch: 9") || !strings.Contains(out, "dmsn bulk refresh --batch 9") {
		t.Fatalf("output must give the batch and how to follow it:\n%s", out)
	}

	out, code = exec(t, "bulk", "refresh", "--batch", "9")
	if code != axi.ExitOK || !strings.Contains(out, "failed: 1") {
		t.Fatalf("exit = %d, following the batch: %s", code, out)
	}
}

func TestAssetRefresh_RefreshesOneAsset(t *testing.T) {
	isolate(t)
	captureProgress(t)
	var query url.Values
	var body map[string]any
	var polls atomic.Int32
	refreshServer(t, &query, &body, &polls)

	out, code := exec(t, "asset", "refresh", "7")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if fmt.Sprint(body["asset_ids"]) != "[7]" || body["refresh"] != true {
		t.Fatalf("body = %v, want a refresh of asset 7", body)
	}
}

func TestBulkRefresh_NeedsAssetsOrAFilter(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"bulk", "refresh"},
		{"bulk", "refresh", "--assets", "1", "--store", "generic"},
	} {
		if out, code := exec(t, args...); code != axi.ExitUsage {
			t.Errorf("%v: exit = %d, want a usage error: %s", args, code, out)
		}
	}
}
