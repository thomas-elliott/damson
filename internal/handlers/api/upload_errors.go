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

const statusClientClosedRequest = 499

var errMissingFileField = errors.New("missing 'file' form field")

type uploadFailure struct {
	status int
	body   string
	cause  error
}

// classifyUploadError works out why reading an upload body failed, keeping the
// real error as the cause in every branch.
func classifyUploadError(err error, ctxErr error) uploadFailure {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return uploadFailure{
			status: http.StatusRequestEntityTooLarge,
			body:   "upload exceeds the maximum allowed size",
			cause:  fmt.Errorf("upload exceeds the %d byte limit: %w", tooLarge.Limit, err),
		}

	case errors.Is(ctxErr, context.Canceled),
		errors.Is(err, context.Canceled),
		errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.EPIPE):
		return uploadFailure{
			status: statusClientClosedRequest,
			body:   "upload aborted",
			cause:  fmt.Errorf("client aborted upload: %w", err),
		}

	case errors.Is(ctxErr, context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return uploadFailure{
			status: http.StatusRequestTimeout,
			body:   "upload timed out",
			cause:  fmt.Errorf("upload deadline exceeded: %w", err),
		}

	case errors.Is(err, syscall.ENOSPC), errors.Is(err, syscall.EDQUOT):
		return uploadFailure{
			status: http.StatusInsufficientStorage,
			body:   "server storage exhausted",
			cause:  fmt.Errorf("upload scratch space exhausted: %w", err),
		}

	case errors.Is(err, http.ErrMissingFile):
		return uploadFailure{
			status: http.StatusBadRequest,
			body:   errMissingFileField.Error(),
			cause:  errMissingFileField,
		}

	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
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
	f := classifyUploadError(err, c.Request.Context().Err())
	if f.cause != nil {
		_ = c.Error(f.cause)
	}
	if f.status >= 500 {
		slog.Error("import: upload failed", append([]any{"err", f.cause, "status", f.status}, attrs...)...)
	}
	c.AbortWithStatusJSON(f.status, gin.H{"error": f.body})
}

// isTransportFailure reports whether err is the transfer failing rather than
// the server failing, so it keeps its classified status instead of collapsing
// to a generic 500.
func isTransportFailure(err error, ctxErr error) bool {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge),
		errors.Is(ctxErr, context.Canceled),
		errors.Is(err, context.Canceled),
		errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.EPIPE),
		errors.Is(err, syscall.ENOSPC),
		errors.Is(err, syscall.EDQUOT),
		errors.Is(err, io.ErrUnexpectedEOF):
		return true
	}
	return false
}
