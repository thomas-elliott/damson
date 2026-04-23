package models

import "time"

type Library struct {
	ID          uint   `gorm:"primarykey;auto_increment;not_null"`
	Name        string `gorm:"not null"`
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Library) TableName() string { return "libraries" }

type LibraryMembership struct {
	ID        uint   `gorm:"primarykey;auto_increment;not_null"`
	LibraryID uint   `gorm:"not null;uniqueIndex:idx_library_user"`
	UserID    uint   `gorm:"not null;uniqueIndex:idx_library_user;index"`
	Role      string `gorm:"not null"` // 'admin' | 'member'
	CreatedAt time.Time
}

func (LibraryMembership) TableName() string { return "library_memberships" }
