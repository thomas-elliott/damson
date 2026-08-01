package models

import "time"

type APIToken struct {
	ID         uint   `gorm:"primarykey;auto_increment;not_null"`
	UserID     uint   `gorm:"not null;index"`
	LibraryID  uint   `gorm:"not null"`
	Label      string `gorm:"not null;default:''"`
	TokenHash  string `gorm:"not null;uniqueIndex"`
	Prefix     string `gorm:"not null;default:''"`
	Scope      string `gorm:"not null;default:write"` // 'read' | 'write'
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
}

func (APIToken) TableName() string { return "api_tokens" }
