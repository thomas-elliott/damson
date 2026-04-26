// Package audit writes audit_log rows. Every library-scoped write handler
// from M3 onwards calls Write in the same transaction as its domain
// mutation — audit is mandatory per docs/archive/technical-design.md §8.3.
package audit

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// Entry is what handlers pass to Write. LibraryID is nullable only for
// server-level events (library create/delete, user role change, server
// settings update). Library-scoped events must set it.
type Entry struct {
	LibraryID  *uint
	UserID     *uint
	ActionType string
	TargetType string
	TargetID   *uint
	Diff       any
}

// Write inserts an audit_log row on the supplied tx. Pass the transaction
// the domain mutation is running under — a failed audit write must fail the
// enclosing action, so audit never rides outside the transaction.
func Write(tx *gorm.DB, e Entry) error {
	if e.ActionType == "" || e.TargetType == "" {
		return fmt.Errorf("audit: action_type and target_type required")
	}
	row := models.AuditLog{
		LibraryID:  e.LibraryID,
		UserID:     e.UserID,
		ActionType: e.ActionType,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
	}
	if e.Diff != nil {
		b, err := json.Marshal(e.Diff)
		if err != nil {
			return fmt.Errorf("audit: marshal diff: %w", err)
		}
		row.DiffJSON = b
	}
	return tx.Create(&row).Error
}

// Created returns a diff payload for creates: {"created": <after>}.
func Created(after any) map[string]any { return map[string]any{"created": after} }

// Updated returns a diff payload for updates: {"before": ..., "after": ...}.
func Updated(before, after any) map[string]any {
	return map[string]any{"before": before, "after": after}
}

// Deleted returns a diff payload for deletes: {"deleted": <before>}.
func Deleted(before any) map[string]any { return map[string]any{"deleted": before} }
