package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

// auditEntryView is the wire shape for a single audit row. The user's
// display name is joined in so the SPA doesn't have to N+1 lookup users.
type auditEntryView struct {
	ID              uint            `json:"id"`
	LibraryID       *uint           `json:"library_id,omitempty"`
	UserID          *uint           `json:"user_id,omitempty"`
	UserDisplayName string          `json:"user_display_name,omitempty"`
	UserEmail       string          `json:"user_email,omitempty"`
	ActionType      string          `json:"action_type"`
	TargetType      string          `json:"target_type"`
	TargetID        *uint           `json:"target_id,omitempty"`
	Diff            json.RawMessage `json:"diff,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

type auditListResponse struct {
	Entries []auditEntryView `json:"entries"`
	Total   int64            `json:"total"`
}

// auditFilters holds the shared filter set across the four audit endpoints
// (library, per-asset, per-project, sysadmin). Each filter ANDs with the
// rest, mirroring the assets list semantics.
type auditFilters struct {
	UserID     *uint
	ActionType string
	TargetType string
	TargetID   *uint
	Since      *time.Time
	Until      *time.Time
}

func parseAuditFilters(c *gin.Context) auditFilters {
	f := auditFilters{
		ActionType: strings.TrimSpace(c.Query("action_type")),
		TargetType: strings.TrimSpace(c.Query("target_type")),
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
	return f
}

// applyAuditFilters layers the shared WHERE chain onto a base query that has
// already applied any scope-specific predicates (e.g. library_id, target).
func applyAuditFilters(q *gorm.DB, f auditFilters) *gorm.DB {
	if f.UserID != nil {
		q = q.Where("a.user_id = ?", *f.UserID)
	}
	if f.ActionType != "" {
		q = q.Where("a.action_type = ?", f.ActionType)
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
// pagination, and the user join.
func loadAuditPage(c *gin.Context, base *gorm.DB, f auditFilters, limit, offset int) ([]auditEntryView, int64, error) {
	q := applyAuditFilters(base, f)

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type row struct {
		ID              uint
		LibraryID       *uint
		UserID          *uint
		UserDisplayName string
		UserEmail       string
		ActionType      string
		TargetType      string
		TargetID        *uint
		DiffJSON        []byte
		CreatedAt       time.Time
	}
	var rows []row
	err := q.Session(&gorm.Session{}).
		Select("a.id, a.library_id, a.user_id, COALESCE(u.display_name, '') AS user_display_name, COALESCE(u.email, '') AS user_email, a.action_type, a.target_type, a.target_id, a.diff_json, a.created_at").
		Joins("LEFT JOIN users u ON u.id = a.user_id").
		Order("a.created_at DESC").
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
			UserID:          r.UserID,
			UserDisplayName: r.UserDisplayName,
			UserEmail:       r.UserEmail,
			ActionType:      r.ActionType,
			TargetType:      r.TargetType,
			TargetID:        r.TargetID,
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
		respond500(c)
		return
	}
	f := parseAuditFilters(c)
	limit, offset := parsePagination(c)

	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a").
		Where("a.library_id = ?", lib.ID)
	entries, total, err := loadAuditPage(c, base, f, limit, offset)
	if err != nil {
		respond500(c)
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
		respond500(c)
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
		respond500(c)
		return
	}

	f := parseAuditFilters(c)
	limit, offset := parsePagination(c)
	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a").
		Where("a.library_id = ? AND a.target_type = ? AND a.target_id = ?", lib.ID, "asset", asset.ID)
	entries, total, err := loadAuditPage(c, base, f, limit, offset)
	if err != nil {
		respond500(c)
		return
	}
	c.JSON(http.StatusOK, auditListResponse{Entries: entries, Total: total})
}

// getProjectAudit returns audit entries scoped to one project. Same
// library-scope guard as the asset variant.
func (h *Handler) getProjectAudit(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c)
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
		respond500(c)
		return
	}

	f := parseAuditFilters(c)
	limit, offset := parsePagination(c)
	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a").
		Where("a.library_id = ? AND a.target_type = ? AND a.target_id = ?", lib.ID, "project", project.ID)
	entries, total, err := loadAuditPage(c, base, f, limit, offset)
	if err != nil {
		respond500(c)
		return
	}
	c.JSON(http.StatusOK, auditListResponse{Entries: entries, Total: total})
}

// getAdminAudit returns server-level audit entries (library_id IS NULL).
// Restricted to sysadmin per docs/archive/technical-design.md §13. We don't merge
// in cross-library library-scoped events for v1 — the library audit page
// covers those, and a "global ledger" view is a v2 concern.
func (h *Handler) getAdminAudit(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	f := parseAuditFilters(c)
	limit, offset := parsePagination(c)
	base := h.db.WithContext(c.Request.Context()).
		Table("audit_log AS a").
		Where("a.library_id IS NULL")
	entries, total, err := loadAuditPage(c, base, f, limit, offset)
	if err != nil {
		respond500(c)
		return
	}
	c.JSON(http.StatusOK, auditListResponse{Entries: entries, Total: total})
}

// requireSysadmin gates handlers that operate on server-wide state. The
// matching membership-level helper is requireAdmin in helpers.go.
func requireSysadmin(c *gin.Context) bool {
	u := auth.GetUser(c)
	if u == nil || u.Role != "sysadmin" {
		respondForbidden(c, "sysadmin required")
		return false
	}
	return true
}
