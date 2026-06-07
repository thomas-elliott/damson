package models

import "time"

// UserIdentity links an external IdP identity (issuer + subject) to a damson
// user. A user may have several — e.g. the same person signing in via Google
// and via Apple — which is how we reconcile multiple login providers onto one
// account. The (Issuer, Subject) pair is globally unique; resolution at login
// is: match the pair → user; else match the verified email → user (and link a
// new identity); else create a new user. Local-auth users carry no identity
// rows — they authenticate by email + password.
type UserIdentity struct {
	ID        uint   `gorm:"primarykey;auto_increment;not_null"`
	UserID    uint   `gorm:"not null;index"`
	Issuer    string `gorm:"not null;uniqueIndex:user_identities_issuer_subject_idx,priority:1"`
	Subject   string `gorm:"not null;uniqueIndex:user_identities_issuer_subject_idx,priority:2"`
	CreatedAt time.Time
}

func (UserIdentity) TableName() string { return "user_identities" }
