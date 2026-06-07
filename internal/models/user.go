package models

import "time"

type User struct {
	ID uint `gorm:"primarykey;auto_increment;not_null"`
	// Email is the account's canonical address and the cross-IdP join key:
	// unique among non-blank values (a freshly-created OIDC user with no email
	// claim sits at "" until they complete profile setup — see needs_profile).
	// External IdP identities live in UserIdentity, not here.
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
