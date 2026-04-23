package models

import (
	"encoding/json"
	"time"
)

type AuditLog struct {
	ID         uint `gorm:"primarykey;auto_increment;not_null"`
	LibraryID  *uint
	UserID     *uint
	ActionType string `gorm:"not null"`
	TargetType string `gorm:"not null"`
	TargetID   *uint
	DiffJSON   json.RawMessage `gorm:"type:jsonb"`
	CreatedAt  time.Time
}

func (AuditLog) TableName() string { return "audit_log" }
