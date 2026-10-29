package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// auditEntryView is the wire shape for a single audit row. The user's
// display name is joined in so the SPA doesn't have to N+1 lookup users.
// LibraryName is populated only on the sysadmin endpoint when scope=all so
// the SPA can render which library a row came from.
type auditEntryView struct {
	ID              uint   `json:"id"`
	LibraryID       *uint  `json:"library_id,omitempty"`
	LibraryName     string `json:"library_name,omitempty"`
	UserID          *uint  `json:"user_id,omitempty"`
	UserDisplayName string `json:"user_display_name,omitempty"`
	UserEmail       string `json:"user_email,omitempty"`
	ActionType      string `json:"action_type"`
	TargetType      string `json:"target_type"`
	TargetID        *uint  `json:"target_id,omitempty"`
	// TargetName is the target's current name, empty when the row has been
	// hard-deleted or the target type has none, so the SPA can fall back to
	// the id. A soft-deleted target keeps its name.
	TargetName string          `json:"target_name,omitempty"`
	Diff       json.RawMessage `json:"diff,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// auditTargetNameSQL resolves an audit row's target to its current name, one
// primary-key lookup per row on the page. A target belongs to the row's own
// library, or is server-level on the sysadmin feed.
const auditTargetNameSQL = `COALESCE(CASE a.target_type
	WHEN 'asset' THEN (SELECT name FROM assets WHERE id = a.target_id)
	WHEN 'project' THEN (SELECT name FROM projects WHERE id = a.target_id)
	WHEN 'collection' THEN (SELECT name FROM collections WHERE id = a.target_id)
	WHEN 'tag' THEN (SELECT path FROM tags WHERE id = a.target_id)
	WHEN 'library' THEN (SELECT name FROM libraries WHERE id = a.target_id)
	WHEN 'user' THEN (SELECT COALESCE(NULLIF(display_name, ''), email) FROM users WHERE id = a.target_id)
	WHEN 'api_token' THEN (SELECT NULLIF(label, '') FROM api_tokens WHERE id = a.target_id)
END, '') AS target_name`

type auditListResponse struct {
	Entries []auditEntryView `json:"entries"`
	Total   int64            `json:"total"`
}

// auditFilters holds the shared filter set across the four audit endpoints
// (library, per-asset, per-project, sysadmin). Each filter ANDs with the
// rest, mirroring the assets list semantics.
type auditFilters struct {
	UserID *uint
	// ActionTypes comes from a comma-separated action_type; a row matches any
	// entry, and an entry ending in ".*" matches every type under that prefix.
	ActionTypes []string
	TargetType  string
	TargetID    *uint
	Since       *time.Time
	Until       *time.Time
	// Search is q: a case-insensitive substring of the user's name or email,
	// or of the action type.
	Search string
}

// maxAuditActionTypes bounds the action_type list: each entry is a bind
// parameter, and the chips need far fewer.
const maxAuditActionTypes = 50

// parseAuditFilters reads the shared filters, answering 400 and returning
// false when action_type lists more than maxAuditActionTypes entries.
func parseAuditFilters(c *gin.Context) (auditFilters, bool) {
	f := auditFilters{
		TargetType: strings.TrimSpace(c.Query("target_type")),
		Search:     strings.TrimSpace(c.Query("q")),
	}
	for _, a := range strings.Split(c.Query("action_type"), ",") {
		if a = strings.TrimSpace(a); a != "" {
			f.ActionTypes = append(f.ActionTypes, a)
		}
	}
	if len(f.ActionTypes) > maxAuditActionTypes {
		respond400(c, fmt.Errorf("action_type lists more than %d types", maxAuditActionTypes))
		return f, false
	}
	if s := c.Query("user_id"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil && n > 0 {
			v := uint(n)
			f.UserID = &v
		}
	}
	if s := c.Query("target_id"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil && n > 0 {
			v := uint(n)
			f.TargetID = &v
		}
	}
	if s := c.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.Since = &t
		}
	}
	if s := c.Query("until"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.Until = &t
		}
	}
	return f, true
}

// applyAuditFilters layers the shared WHERE chain onto a base query that has
// already applied any scope-specific predicates (e.g. library_id, target)
// and joined users as u.
func applyAuditFilters(q *gorm.DB, f auditFilters) *gorm.DB {
	if f.Search != "" {
		like := "%" + escapeLikePattern(f.Search) + "%"
		q = q.Where(`(u.display_name ILIKE ? ESCAPE '\' OR u.email ILIKE ? ESCAPE '\' OR a.action_type ILIKE ? ESCAPE '\')`, like, like, like)
	}
	if f.UserID != nil {
		q = q.Where("a.user_id = ?", *f.UserID)
	}
	if len(f.ActionTypes) > 0 {
		conds := make([]string, len(f.ActionTypes))
		args := make([]any, len(f.ActionTypes))
		for i, a := range f.ActionTypes {
			if prefix, ok := strings.CutSuffix(a, ".*"); ok {
				conds[i], args[i] = "starts_with(a.action_type, ?)", prefix+"."
			} else {
				conds[i], args[i] = "a.action_type = ?", a
			}
		}
		q = q.Where("("+strings.Join(conds, " OR ")+")", args...)
	}
	if f.TargetType != "" {
		q = q.Where("a.target_type = ?", f.TargetType)
	}
	if f.TargetID != nil {
		q = q.Where("a.target_id = ?", *f.TargetID)
	}
	if f.Since != nil {
		q = q.Where("a.created_at >= ?", *f.Since)
	}
	if f.Until != nil {
		q = q.Where("a.created_at <= ?", *f.Until)
	}
	return q
}

// loadAuditPage returns a page of audit entries for the supplied base query.
// The base query must already have the scope filter applied (library_id,
// target=asset/project/etc.) — this helper layers the shared filter set,
// the user join, and pagination. When includeLibraryName is true the
// returned rows carry the library's name (joined via libraries) so the
// sysadmin "all" feed can render the source library.
func loadAuditPage(c *gin.Context, base *gorm.DB, f auditFilters, limit, offset int, includeLibraryName bool) ([]auditEntryView, int64, error) {
	q := applyAuditFilters(base.Joins("LEFT JOIN users u ON u.id = a.user_id"), f)

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type row struct {
		ID              uint
		LibraryID       *uint
		LibraryName     string
		UserID          *uint
		UserDisplayName string
		UserEmail       string
		ActionType      string
		TargetType      string
		TargetID        *uint
		TargetName      string
		DiffJSON        []byte
		CreatedAt       time.Time
	}
	selectCols := "a.id, a.library_id, a.user_id, COALESCE(u.display_name, '') AS user_display_name, COALESCE(u.email, '') AS user_email, a.action_type, a.target_type, a.target_id, " + auditTargetNameSQL + ", a.diff_json, a.created_at"
	if includeLibraryName {
		selectCols = "a.id, a.library_id, COALESCE(l.name, '') AS library_name, a.user_id, COALESCE(u.display_name, '') AS user_display_name, COALESCE(u.email, '') AS user_email, a.action_type, a.target_type, a.target_id, " + auditTargetNameSQL + ", a.diff_json, a.created_at"
	}
	var rows []row
	page := q.Session(&gorm.Session{}).
		Select(selectCols)
	if includeLibraryName {
		page = page.Joins("LEFT JOIN libraries l ON l.id = a.library_id")
	}
	// id breaks created_at ties, so a row never shows on two pages.
	err := page.Order("a.created_at DESC, a.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]auditEntryView, len(rows))
	for i, r := range rows {
		out[i] = auditEntryView{
			ID:              r.ID,
			LibraryID:       r.LibraryID,
			LibraryName:     r.LibraryName,
			UserID:          r.UserID,
			UserDisplayName: r.UserDisplayName,
			UserEmail:       r.UserEmail,
			ActionType:      r.ActionType,
			TargetType:      r.TargetType,
			TargetID:        r.TargetID,
			TargetName:      r.TargetName,
			Diff:            json.RawMessage(r.DiffJSON),
			CreatedAt:       r.CreatedAt,
		}
	}
	return out, total, nil
}

// getLibraryAudit returns audit entries for the current library. Library
// members can see this; admins gating happens elsewhere if it ever needs
// tightening.
func (h *Handler) getLibraryAudit(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	f, ok := parseAuditFilters(c)
	if !ok {
		return
	}
	limit, offset := parsePagination(c)

	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a").
		Where("a.library_id = ?", lib.ID)
	entries, total, err := loadAuditPage(c, base, f, limit, offset, false)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, auditListResponse{Entries: entries, Total: total})
}

// getAssetAudit returns audit entries scoped to one asset. The asset must
// belong to the current library; otherwise the per-asset query returns 404
// to avoid leaking cross-library existence.
func (h *Handler) getAssetAudit(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var asset models.Asset
	err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).
		First(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "asset not found")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}

	f, ok := parseAuditFilters(c)
	if !ok {
		return
	}
	limit, offset := parsePagination(c)
	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a").
		Where("a.library_id = ? AND a.target_type = ? AND a.target_id = ?", lib.ID, "asset", asset.ID)
	entries, total, err := loadAuditPage(c, base, f, limit, offset, false)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, auditListResponse{Entries: entries, Total: total})
}

// getProjectAudit returns audit entries scoped to one project. Same
// library-scope guard as the asset variant.
func (h *Handler) getProjectAudit(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var project models.Project
	err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).
		First(&project).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "project not found")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}

	f, ok := parseAuditFilters(c)
	if !ok {
		return
	}
	limit, offset := parsePagination(c)
	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a").
		Where("a.library_id = ? AND a.target_type = ? AND a.target_id = ?", lib.ID, "project", project.ID)
	entries, total, err := loadAuditPage(c, base, f, limit, offset, false)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, auditListResponse{Entries: entries, Total: total})
}

// getAdminAudit returns audit entries for the sysadmin feed. Two scopes:
//
//   - scope=server (default) — server-level events only (library_id IS NULL).
//     Library / user / server-settings lifecycle events live here.
//   - scope=all — server-level + every library-scoped event across every
//     library. This is the cross-library activity view; sysadmins use it
//     to spot bursts (e.g. "library X had 200 deletions in the last hour")
//     without joining each library individually.
//
// Restricted to sysadmin. The scope=all rows carry library_name so the UI
// can group by source library.
func (h *Handler) getAdminAudit(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	f, ok := parseAuditFilters(c)
	if !ok {
		return
	}
	limit, offset := parsePagination(c)
	scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	includeLibraryName := scope == "all"
	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a")
	if !includeLibraryName {
		base = base.Where("a.library_id IS NULL")
	}
	entries, total, err := loadAuditPage(c, base, f, limit, offset, includeLibraryName)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, auditListResponse{Entries: entries, Total: total})
}
