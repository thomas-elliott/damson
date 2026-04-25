package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// respond500 writes the canonical generic 500 body. Don't leak internals.
func respond500(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// respond400 logs the real error and responds with err.Error() per
// CONVENTIONS.md ("400 includes err.Error()").
func respond400(c *gin.Context, err error) {
	slog.Warn("400 bad request", "path", c.FullPath(), "err", err)
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func respond404(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": msg})
}

func respond409(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": msg})
}

func respondForbidden(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": msg})
}

// Sentinel errors that handlers use to smuggle "respond with this status"
// signals out of GORM transactions. Wrap with fmt.Errorf("%w: ...") to add
// a message; respondGORM surfaces err.Error() for 400 / 409.
var (
	errBadRequest = errors.New("bad request")
	errConflict   = errors.New("conflict")
	errNotFound   = errors.New("not found")
)

// errBadRequestf / errConflictf return a sentinel-wrapped error suitable
// for returning out of a Transaction(func). respondGORM picks them up and
// maps to the right HTTP status with the supplied message.
func errBadRequestf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{errBadRequest}, args...)...)
}

func errConflictf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{errConflict}, args...)...)
}

// respondGORM maps a GORM/transaction error to the right HTTP status. Use it
// from handlers that funnel domain logic through a Transaction(func) — a
// single call replaces the repeated isNotFound/stringError ladder.
//
// notFoundMsg is the message surfaced when err matches a not-found sentinel
// — handlers care about saying "asset not found" vs "project not found".
func respondGORM(c *gin.Context, err error, notFoundMsg string) {
	switch {
	case err == nil:
		return
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, errNotFound):
		respond404(c, notFoundMsg)
	case errors.Is(err, errBadRequest):
		// Strip the "bad request: " prefix the wrapping introduced.
		respond400(c, errors.New(trimSentinelPrefix(err.Error(), "bad request: ")))
	case errors.Is(err, errConflict):
		respond409(c, trimSentinelPrefix(err.Error(), "conflict: "))
	default:
		respond500(c)
	}
}

func trimSentinelPrefix(msg, prefix string) string {
	if len(msg) > len(prefix) && msg[:len(prefix)] == prefix {
		return msg[len(prefix):]
	}
	return msg
}

// isNotFound reports whether err is the canonical GORM record-not-found.
func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// requireAdmin returns nil iff the membership attached to the context has
// role='admin'; otherwise it aborts with 403 and returns false.
func requireAdmin(c *gin.Context) bool {
	m := getMembership(c)
	if m == nil || m.Role != "admin" {
		respondForbidden(c, "library admin required")
		return false
	}
	return true
}
