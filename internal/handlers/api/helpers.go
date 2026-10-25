package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
)

// Sentinels for the "should never happen" context/config guards so a swept
// respond500 always carries a non-nil, greppable cause into the request log.
var (
	errRequestContextIncomplete        = errors.New("required request context missing")
	errStorageNotConfigured            = errors.New("storage not configured")
	errImportPoolNotConfigured         = errors.New("import pool not configured")
	errBroadcasterNotConfigured        = errors.New("broadcaster not configured")
	errPreviewBroadcasterNotConfigured = errors.New("preview broadcaster not configured")
)

// respond500 attaches the real cause to c.Errors (surfaced on the request
// log line by requestLogger) and writes the canonical generic 500 body.
// Don't leak internals in the response; the cause goes to the logs only.
func respond500(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// respond400 attaches the real error (surfaced on the request log line) and
// responds with err.Error() per CONVENTIONS.md ("400 includes err.Error()").
func respond400(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

// respond400Msg is respond400 for the cases where the client-facing message
// must stay stable but the diagnostic detail belongs in the log.
func respond400Msg(c *gin.Context, cause error, msg string) {
	if cause != nil {
		_ = c.Error(cause)
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": msg})
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
		respond500(c, err)
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

func parseIDParam(c *gin.Context, key string) (uint, bool) {
	s := c.Param(key)
	n, err := parsePositiveID(s)
	if err != nil {
		respond400Msg(c, fmt.Errorf("invalid %s %q: %w", key, s, err), "invalid "+key)
		return 0, false
	}
	return n, true
}

func parsePositiveID(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, errors.New("must be a positive integer")
	}
	return uint(n), nil
}

// Page sizes for parsePagination.
const (
	defaultLimit = 50
	maxLimit     = 200
)

func parsePagination(c *gin.Context) (limit, offset int) {
	limit = defaultLimit
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= maxLimit {
			limit = n
		}
	}
	if s := c.Query("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

// requireSysadmin gates handlers that operate on server-wide state. The
// matching membership-level helper is requireAdmin, above.
func requireSysadmin(c *gin.Context) bool {
	u := auth.GetUser(c)
	if u == nil || u.Role != "sysadmin" {
		respondForbidden(c, "sysadmin required")
		return false
	}
	return true
}

// isUniqueViolation reports whether err is Postgres's unique_violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func respond502(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": msg})
}

func respond503(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": msg})
}
