package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// captureJSONLogs installs the production logger, writing to a buffer, and
// returns a func yielding the decoded records.
func captureJSONLogs(t *testing.T) func() []map[string]any {
	t.Helper()
	t.Setenv("DAMSON_LOG_LEVEL", "debug")
	t.Setenv("DAMSON_LOG_FORMAT", "json")
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(newLogger(&buf))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return func() []map[string]any {
		var out []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if line == "" {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				t.Fatalf("log line is not JSON: %v (%q)", err, line)
			}
			out = append(out, rec)
		}
		return out
	}
}

// TestJSONLogsUseSlogsStandardKeys pins the message and timestamp to slog's
// own msg / time, which is where ops logs and Logs Drilldown look for them.
func TestJSONLogsUseSlogsStandardKeys(t *testing.T) {
	records := captureJSONLogs(t)
	slog.Info("hello", "status", 200)

	recs := records()
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	rec := recs[0]

	if got := rec[slog.MessageKey]; got != "hello" {
		t.Errorf("msg = %v, want %q", got, "hello")
	}
	if _, ok := rec[slog.TimeKey]; !ok {
		t.Errorf("time missing; got keys %v", keysOf(rec))
	}
	for _, old := range []string{"_msg", "_time"} {
		if _, ok := rec[old]; ok {
			t.Errorf("%q must not appear; got keys %v", old, keysOf(rec))
		}
	}
}

// runRequest drives one request through requestID + requestLogger into handler.
func runRequest(t *testing.T, req *http.Request, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(requestID())
	r.Use(requestLogger())
	r.Handle(req.Method, "/api/imports", handler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestRequestLoggerComposesMessage covers F4: a static msg="request" carries no
// information once it lands on the message.
func TestRequestLoggerComposesMessage(t *testing.T) {
	records := captureJSONLogs(t)

	req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	runRequest(t, req, func(c *gin.Context) { c.Status(http.StatusAccepted) })

	recs := records()
	if len(recs) != 1 {
		t.Fatalf("a non-upload request must log exactly once, got %d", len(recs))
	}
	msg, _ := recs[0][slog.MessageKey].(string)
	if !strings.HasPrefix(msg, "POST /api/imports → 202 in ") {
		t.Errorf("msg = %q, want the composed form", msg)
	}
	if msg == "request" {
		t.Errorf("msg must not be the old static string")
	}
	if got := recs[0]["status"]; got != float64(202) {
		t.Errorf("structured status = %v, want 202 (fields must stay queryable)", got)
	}
	if _, ok := recs[0]["phase"]; ok {
		t.Errorf("a JSON request must not emit an upload start line; got %v", keysOf(recs[0]))
	}
}

// TestRequestLoggerCountsBytesRead covers F5: bytes_in reports what was really
// read off the socket, so a transfer that died partway is distinguishable from
// one that never started.
func TestRequestLoggerCountsBytesRead(t *testing.T) {
	t.Run("partial read reports what arrived", func(t *testing.T) {
		records := captureJSONLogs(t)

		body := strings.Repeat("x", 4096)
		req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader(body))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=zzz")
		runRequest(t, req, func(c *gin.Context) {
			// Read a prefix, then bail — the shape of a handler that gives up
			// mid-transfer.
			_, _ = io.CopyN(io.Discard, c.Request.Body, 1000)
			c.Status(http.StatusBadRequest)
		})

		done := lastRecord(t, records())
		got, ok := done["bytes_in"].(float64)
		if !ok {
			t.Fatalf("bytes_in missing; got keys %v", keysOf(done))
		}
		if int64(got) != 1000 {
			t.Errorf("bytes_in = %v, want 1000 (bytes read, not Content-Length %d)", got, len(body))
		}
	})

	t.Run("bodyless request omits the field", func(t *testing.T) {
		records := captureJSONLogs(t)

		req := httptest.NewRequest(http.MethodGet, "/api/imports", nil)
		runRequest(t, req, func(c *gin.Context) { c.Status(http.StatusOK) })

		done := lastRecord(t, records())
		if _, ok := done["bytes_in"]; ok {
			t.Errorf("bytes_in must be absent when nothing was read; got %v", done["bytes_in"])
		}
	})
}

// TestRequestLoggerEmitsUploadStartLine covers the other half of F5: a request
// that never returns must still leave a trace.
func TestRequestLoggerEmitsUploadStartLine(t *testing.T) {
	records := captureJSONLogs(t)

	req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader("payload"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=zzz")
	runRequest(t, req, func(c *gin.Context) { c.Status(http.StatusAccepted) })

	recs := records()
	if len(recs) != 2 {
		t.Fatalf("an upload must log a start and a completion line, got %d: %v", len(recs), recs)
	}

	start, done := recs[0], recs[1]
	if got := start["phase"]; got != "start" {
		t.Errorf("start line phase = %v, want %q", got, "start")
	}
	if msg, _ := start[slog.MessageKey].(string); msg != "POST /api/imports → upload started" {
		t.Errorf("start msg = %q", msg)
	}
	if got := start["content_length"]; got != float64(len("payload")) {
		t.Errorf("start content_length = %v, want %d", got, len("payload"))
	}
	if _, ok := start["status"]; ok {
		t.Errorf("the start line cannot know the status; got %v", keysOf(start))
	}
	if start["request_id"] == "" || start["request_id"] != done["request_id"] {
		t.Errorf("start and completion must share a request_id: %v vs %v", start["request_id"], done["request_id"])
	}
}

// TestRequestLoggerFlagsClientAbort distinguishes "the client went away" from
// "the server broke" — indistinguishable before this change.
func TestRequestLoggerFlagsClientAbort(t *testing.T) {
	t.Run("cancelled request is flagged", func(t *testing.T) {
		records := captureJSONLogs(t)

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader("x")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		runRequest(t, req, func(c *gin.Context) {
			cancel()
			c.Status(http.StatusInternalServerError)
		})

		done := lastRecord(t, records())
		if got := done["client_aborted"]; got != true {
			t.Errorf("client_aborted = %v, want true", got)
		}
	})

	t.Run("clean request is not flagged", func(t *testing.T) {
		records := captureJSONLogs(t)

		req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader("x"))
		req.Header.Set("Content-Type", "application/json")
		runRequest(t, req, func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

		done := lastRecord(t, records())
		if _, ok := done["client_aborted"]; ok {
			t.Errorf("a server-side 500 must not be blamed on the client; got %v", done["client_aborted"])
		}
	})
}

func TestIsUploadStream(t *testing.T) {
	cases := []struct {
		method, contentType string
		want                bool
	}{
		{http.MethodPost, "multipart/form-data; boundary=zzz", true},
		{http.MethodPut, "application/octet-stream", true},
		{http.MethodPost, "application/json", false},
		{http.MethodGet, "multipart/form-data; boundary=zzz", false},
		{http.MethodDelete, "application/octet-stream", false},
		{http.MethodPost, "", false},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, "/api/imports", nil)
		if tc.contentType != "" {
			r.Header.Set("Content-Type", tc.contentType)
		}
		if got := isUploadStream(r); got != tc.want {
			t.Errorf("isUploadStream(%s, %q) = %v, want %v", tc.method, tc.contentType, got, tc.want)
		}
	}
}

func TestRequestMessage(t *testing.T) {
	got := requestMessage(http.MethodPost, "/api/imports", 202, 431*time.Millisecond)
	if want := "POST /api/imports → 202 in 431ms"; got != want {
		t.Errorf("requestMessage() = %q, want %q", got, want)
	}
}

func lastRecord(t *testing.T, recs []map[string]any) map[string]any {
	t.Helper()
	if len(recs) == 0 {
		t.Fatal("no log records emitted")
	}
	return recs[len(recs)-1]
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
