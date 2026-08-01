package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"syscall"

	"github.com/gin-gonic/gin"
)

// Not an IANA status. nginx logs 499 for an abandoned request, so damson
// and the proxy in front of it agree on what an abort looks like.
const statusClientClosedRequest = 499

var errMissingFileField = errors.New("missing 'file' form field")

// msgUploadTooLarge is shared with the session endpoint, which rejects an
// oversized upload before a byte moves rather than by tripping the cap.
const msgUploadTooLarge = "upload exceeds the maximum allowed size"

type uploadFailure struct {
	status int
	body   string
	cause  error
	// transport is a different question from the status: a 400 can be a bad
	// body from the client or a fault in the storage layer. Only the former
	// keeps its status when the failure came from a storage put.
	transport bool
}

// classifyUploadError works out why reading an upload body failed, keeping the
// real error as the cause in every branch. A client that walks away surfaces as
// a generic read error, so ctxErr — the request context's error — is consulted
// separately; only it distinguishes an abort from a malformed body.
//
// Branch order is load-bearing: tripping the size cap can also kill the
// connection, so MaxBytesError has to be tested before the context.
func classifyUploadError(err error, ctxErr error) uploadFailure {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return uploadFailure{
			status:    http.StatusRequestEntityTooLarge,
			body:      msgUploadTooLarge,
			cause:     fmt.Errorf("upload exceeds the %d byte limit: %w", tooLarge.Limit, err),
			transport: true,
		}

	case errors.Is(ctxErr, context.Canceled),
		errors.Is(err, context.Canceled),
		errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.EPIPE):
		return uploadFailure{
			status:    statusClientClosedRequest,
			body:      "upload aborted",
			cause:     fmt.Errorf("client aborted upload: %w", err),
			transport: true,
		}

	case errors.Is(ctxErr, context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return uploadFailure{
			status:    http.StatusRequestTimeout,
			body:      "upload timed out",
			cause:     fmt.Errorf("upload deadline exceeded: %w", err),
			transport: true,
		}

	case errors.Is(err, syscall.ENOSPC), errors.Is(err, syscall.EDQUOT):
		return uploadFailure{
			status:    http.StatusInsufficientStorage,
			body:      "server storage exhausted",
			cause:     fmt.Errorf("upload scratch space exhausted: %w", err),
			transport: true,
		}

	case errors.Is(err, http.ErrMissingFile):
		return uploadFailure{
			status: http.StatusBadRequest,
			body:   errMissingFileField.Error(),
			cause:  errMissingFileField,
		}

	case errors.Is(err, io.ErrUnexpectedEOF):
		return uploadFailure{
			status:    http.StatusBadRequest,
			body:      "upload ended before the file was complete",
			cause:     fmt.Errorf("truncated upload body: %w", err),
			transport: true,
		}

	// Same answer, deliberately not the same case. From a body read a bare EOF
	// means truncation; from a storage backend it is far more likely an
	// internal bug, so it is not trusted as a transport failure.
	case errors.Is(err, io.EOF):
		return uploadFailure{
			status: http.StatusBadRequest,
			body:   "upload ended before the file was complete",
			cause:  fmt.Errorf("truncated upload body: %w", err),
		}

	default:
		return uploadFailure{
			status: http.StatusBadRequest,
			body:   "malformed upload body",
			cause:  fmt.Errorf("malformed upload body: %w", err),
		}
	}
}

// respondUploadError classifies err and responds, attaching the cause to
// c.Errors so it lands on the request log line.
func respondUploadError(c *gin.Context, err error, attrs ...any) {
	respondUploadFailure(c, classifyUploadError(err, c.Request.Context().Err()), attrs...)
}

// respondUploadOrServerError answers with the classified upload status when
// the transfer is what failed, and 500 with serverErr when it isn't.
func respondUploadOrServerError(c *gin.Context, err error, serverErr error, attrs ...any) {
	f := classifyUploadError(err, c.Request.Context().Err())
	if !f.transport {
		respond500(c, serverErr)
		return
	}
	respondUploadFailure(c, f, attrs...)
}

func respondUploadFailure(c *gin.Context, f uploadFailure, attrs ...any) {
	if f.cause != nil {
		_ = c.Error(f.cause)
	}
	if f.status >= 500 {
		slog.Error("import: upload failed", append([]any{"err", f.cause, "status", f.status}, attrs...)...)
	}
	c.AbortWithStatusJSON(f.status, gin.H{"error": f.body})
}
