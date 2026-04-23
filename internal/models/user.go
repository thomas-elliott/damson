package models

import "time"

type User struct {
	ID               uint   `gorm:"primarykey;auto_increment;not_null"`
	Subject          string `gorm:"uniqueIndex;not null"`
	Email            string `gorm:"not null"`
	DisplayName      string `gorm:"not null"`
	Role             string `gorm:"not null;default:user"`
	CurrentLibraryID *uint
	PasswordHash     *string    // set only in local auth mode
	TokensValidAfter *time.Time // reject JWTs issued before this instant
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (User) TableName() string { return "users" }
