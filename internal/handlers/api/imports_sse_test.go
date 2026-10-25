package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/services/ingest"
)

// TestGetImportEvents_StreamsBroadcasterEvents verifies that the SSE
// endpoint emits a snapshot frame for an existing in-flight import on
// connect and forwards a subsequent broadcaster event. Run against a real
// HTTP server so c.Stream / Flusher work; httptest.NewRecorder doesn't
// implement http.Flusher in a way Gin's streaming code path can drive.
func TestGetImportEvents_StreamsBroadcasterEvents(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)

	// Seed an in-flight import row so the snapshot has something to send.
	asset := struct {
		ID          uint
		LibraryID   uint
		Name        string
		NeedsReview bool
	}{}
	_ = asset
	if err := db.Exec("INSERT INTO assets (library_id, name, needs_review) VALUES (?, ?, ?)", lib.ID, "Pending package", true).Error; err != nil {
		t.Fatal(err)
	}
	var assetID uint
	if err := db.Raw("SELECT id FROM assets WHERE library_id = ? LIMIT 1", lib.ID).Scan(&assetID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO asset_imports (asset_id, stage, updated_at) VALUES (?, ?, ?)", assetID, ingest.StageExtracting, time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	var importID uint
	if err := db.Raw("SELECT id FROM asset_imports WHERE asset_id = ? LIMIT 1", assetID).Scan(&importID).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/imports/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	defer resp.Body.Close()
	if got, want := resp.Header.Get("Content-Type"), "text/event-stream"; !strings.HasPrefix(got, want) {
		t.Fatalf("content-type: got %q, want prefix %q", got, want)
	}

	// First frame should be the seeded extracting row from ReplaySnapshot.
	first := readEvent(t, resp.Body)
	if first.ID != importID || first.Stage != ingest.StageExtracting {
		t.Fatalf("snapshot frame: got %+v", first)
	}

	// Now publish a transition through the broadcaster and confirm we
	// receive it on the same stream.
	go func() {
		// Small delay to ensure the reader is parked on the channel.
		time.Sleep(20 * time.Millisecond)
		h.broadcaster.Publish(lib.ID, ingest.Event{
			ID:        importID,
			AssetID:   assetID,
			AssetName: "Pending package",
			Stage:     ingest.StageThumbnailing,
			UpdatedAt: time.Now().UTC(),
		})
	}()
	second := readEvent(t, resp.Body)
	if second.ID != importID || second.Stage != ingest.StageThumbnailing {
		t.Fatalf("live frame: got %+v", second)
	}
}

// readEvent reads one `data: {...}` frame from an SSE stream. Heartbeats
// (lines starting with `:`) and blank separators are skipped. The read
// happens on a goroutine so the test can bail with a clear failure message
// if no event arrives within the timeout (instead of the test framework's
// generic 30s panic).
func readEvent(t *testing.T, body io.Reader) ingest.Event {
	t.Helper()
	type result struct {
		ev  ingest.Event
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		br := bufio.NewReader(body)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				resCh <- result{err: err}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimPrefix(line, "data: ")
				var ev ingest.Event
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					resCh <- result{err: err}
					return
				}
				resCh <- result{ev: ev}
				return
			}
		}
	}()
	select {
	case r := <-resCh:
		if r.err != nil {
			t.Fatalf("read sse event: %v", r.err)
		}
		return r.ev
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for SSE event")
		return ingest.Event{}
	}
}
