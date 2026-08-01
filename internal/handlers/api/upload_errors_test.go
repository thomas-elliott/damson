package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestClassifyUploadError_DistinguishesCauses is the core of the F2 fix: every
// one of these used to collapse onto 400 "missing 'file' form field" with the
// cause thrown away.
func TestClassifyUploadError_DistinguishesCauses(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		ctxErr     error
		wantStatus int
		wantBody   string
		// wantCauseHas keeps the underlying error reachable in the log.
		wantCauseHas  string
		wantTransport bool
	}{
		{
			name:          "over the cap",
			err:           &http.MaxBytesError{Limit: 2 << 20},
			wantStatus:    http.StatusRequestEntityTooLarge,
			wantBody:      "upload exceeds the maximum allowed size",
			wantCauseHas:  "2097152",
			wantTransport: true,
		},
		{
			name:          "client walked away mid-transfer",
			err:           io.ErrUnexpectedEOF,
			ctxErr:        context.Canceled,
			wantStatus:    statusClientClosedRequest,
			wantBody:      "upload aborted",
			wantCauseHas:  "client aborted upload",
			wantTransport: true,
		},
		{
			name:          "connection reset",
			err:           fmt.Errorf("read tcp: %w", syscall.ECONNRESET),
			wantStatus:    statusClientClosedRequest,
			wantBody:      "upload aborted",
			wantCauseHas:  "connection reset",
			wantTransport: true,
		},
		{
			name:          "scratch volume full",
			err:           fmt.Errorf("multipart: write /tmp/multipart-123: %w", syscall.ENOSPC),
			wantStatus:    http.StatusInsufficientStorage,
			wantBody:      "server storage exhausted",
			wantCauseHas:  "scratch space exhausted",
			wantTransport: true,
		},
		{
			name:         "no file part at all",
			err:          http.ErrMissingFile,
			wantStatus:   http.StatusBadRequest,
			wantBody:     "missing 'file' form field",
			wantCauseHas: "missing 'file' form field",
		},
		{
			name:          "body truncated",
			err:           io.ErrUnexpectedEOF,
			wantStatus:    http.StatusBadRequest,
			wantBody:      "upload ended before the file was complete",
			wantCauseHas:  "truncated upload body",
			wantTransport: true,
		},
		{
			name:         "bare EOF",
			err:          io.EOF,
			wantStatus:   http.StatusBadRequest,
			wantBody:     "upload ended before the file was complete",
			wantCauseHas: "truncated upload body",
		},
		{
			name:         "malformed multipart framing",
			err:          errors.New("multipart: NextPart: bufio: buffer full"),
			wantStatus:   http.StatusBadRequest,
			wantBody:     "malformed upload body",
			wantCauseHas: "bufio: buffer full",
		},
		{
			name:          "deadline exceeded",
			err:           context.DeadlineExceeded,
			wantStatus:    http.StatusRequestTimeout,
			wantBody:      "upload timed out",
			wantCauseHas:  "deadline exceeded",
			wantTransport: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyUploadError(tc.err, tc.ctxErr)
			if got.status != tc.wantStatus {
				t.Errorf("status = %d, want %d", got.status, tc.wantStatus)
			}
			if got.body != tc.wantBody {
				t.Errorf("body = %q, want %q", got.body, tc.wantBody)
			}
			if got.transport != tc.wantTransport {
				t.Errorf("transport = %v, want %v", got.transport, tc.wantTransport)
			}
			if got.cause == nil {
				t.Fatal("cause is nil — the whole point is that the cause reaches the log")
			}
			if !strings.Contains(got.cause.Error(), tc.wantCauseHas) {
				t.Errorf("cause = %q, want it to contain %q", got.cause, tc.wantCauseHas)
			}
			// The pre-change behaviour: everything became this one 400. Any
			// case still landing there is a case that lost its diagnosis.
			if tc.wantStatus != http.StatusBadRequest && got.body == "missing 'file' form field" {
				t.Errorf("%s must not be reported as a missing form field", tc.name)
			}
		})
	}
}

// TestClassifyUploadError_CapBeatsDisconnect pins the branch order. Tripping
// MaxBytesReader can also kill the connection, so checking the context first
// would report a 413 as a client abort and hide the real reason.
func TestClassifyUploadError_CapBeatsDisconnect(t *testing.T) {
	got := classifyUploadError(&http.MaxBytesError{Limit: 1024}, context.Canceled)
	if got.status != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413 — the cap is the reason, not the disconnect", got.status)
	}
}

// TestRespondUploadError_AttachesCauseToGinErrors proves the cause reaches
// c.Errors, which is the only route by which requestLogger can print it.
func TestRespondUploadError_AttachesCauseToGinErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/imports", nil)

	respondUploadError(c, fmt.Errorf("read: %w", syscall.ECONNRESET), "library_id", uint(7))

	if len(c.Errors) != 1 {
		t.Fatalf("want exactly 1 attached error, got %d — an unattached cause never reaches the log", len(c.Errors))
	}
	if !strings.Contains(c.Errors[0].Error(), "connection reset") {
		t.Errorf("attached error = %q, want the underlying cause", c.Errors[0])
	}
	if w.Code != statusClientClosedRequest {
		t.Errorf("status = %d, want %d", w.Code, statusClientClosedRequest)
	}
	if strings.Contains(w.Body.String(), "connection reset") {
		t.Errorf("the cause must stay out of the response body; got %s", w.Body.String())
	}
}

// TestRespondUploadOrServerError_SplitsTransportFromServerFault is what the
// single classifier buys: one list of error shapes decides both the status and
// whether the status is trustworthy for a failure raised by storage rather
// than by the transfer.
func TestRespondUploadOrServerError_SplitsTransportFromServerFault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serverErr := errors.New("s3 put (bucket damson): dial tcp: no route to host")

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "client abort keeps its classified status",
			err:        fmt.Errorf("read tcp: %w", syscall.ECONNRESET),
			wantStatus: statusClientClosedRequest,
			wantBody:   "upload aborted",
		},
		{
			name:       "deadline keeps its classified status",
			err:        fmt.Errorf("upload part: %w", context.DeadlineExceeded),
			wantStatus: http.StatusRequestTimeout,
			wantBody:   "upload timed out",
		},
		{
			name:       "an arbitrary backend error is the server's fault",
			err:        errors.New("s3 put: InternalError"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal error",
		},
		{
			name:       "a bare EOF from storage is the server's fault",
			err:        fmt.Errorf("s3 put: %w", io.EOF),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/imports", nil)

			respondUploadOrServerError(c, tc.err, serverErr, "library_id", uint(3))

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Errorf("body = %s, want it to contain %q", w.Body.String(), tc.wantBody)
			}
			if len(c.Errors) == 0 {
				t.Fatal("no cause attached — the failure would reach the log as a bare status")
			}
			if strings.Contains(w.Body.String(), "no route to host") ||
				strings.Contains(w.Body.String(), "InternalError") {
				t.Errorf("the backend detail must stay out of the response body; got %s", w.Body.String())
			}
		})
	}
}

// TestPostImport_MissingFileFieldReportsCause is the end-to-end half: a
// multipart body with no "file" part must still name itself in the log.
func TestPostImport_MissingFileFieldReportsCause(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	body, ctype := buildMultipart(t, "not_the_file_field", "x.bin", []byte("hello"))
	w := postMultipart(r, "/api/imports", body, ctype)

	requireStatus(t, w, http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), "missing 'file' form field") {
		t.Errorf("body = %s, want the missing-field message preserved", w.Body.String())
	}
}

// TestPostImport_TruncatedBodyIsNotBlamedOnAMissingField covers the case that
// motivated the change: a transfer that dies partway is a different failure
// from a client that forgot the form field, and must not be reported as one.
func TestPostImport_TruncatedBodyIsNotBlamedOnAMissingField(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	_ = seedLibraryAndSwitch(t, h, u)

	r := setupRouter(h, u)
	full, ctype := buildMultipart(t, "file", "big.bin", bytes.Repeat([]byte("a"), 8<<10))

	// Cut the body mid-part: the framing is valid up to the truncation point,
	// which is exactly what a dropped connection leaves behind.
	truncated := full.Bytes()[:full.Len()/2]
	req := httptest.NewRequest(http.MethodPost, "/api/imports", bytes.NewReader(truncated))
	req.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if strings.Contains(w.Body.String(), "missing 'file' form field") {
		t.Errorf("a truncated transfer must not be reported as a missing form field; got %s", w.Body.String())
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestParseIDParam_KeepsClientMessageAndLogsDetail guards the split
// respond400Msg exists for: stable body, diagnostic cause.
func TestParseIDParam_KeepsClientMessageAndLogsDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/assets/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	if _, ok := parseIDParam(c, "id"); ok {
		t.Fatal("parseIDParam accepted a non-numeric id")
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"error":"invalid id"}` {
		t.Errorf("body = %s, want the message unchanged", got)
	}
	if len(c.Errors) != 1 {
		t.Fatalf("want the offending value attached as a cause, got %d errors", len(c.Errors))
	}
	if !strings.Contains(c.Errors[0].Error(), `"abc"`) {
		t.Errorf("cause = %q, want it to quote the rejected value", c.Errors[0])
	}
}
