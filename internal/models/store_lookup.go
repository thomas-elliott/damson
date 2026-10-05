package models

import (
	"encoding/json"
	"time"
)

// Store lookup lifecycle (migration 026). A lookup is queued until its job
// has run.
const (
	StoreLookupQueued = "queued"
	StoreLookupDone   = "done"
	StoreLookupFailed = "failed"
)

// What a done lookup did to its asset.
const (
	StoreLookupChanged     = "changed"
	StoreLookupUnchanged   = "unchanged"
	StoreLookupNoStoreData = "no_store_data"
)

// StoreLookupBatch groups the lookups one request queued. Refresh lookups may
// replace fields a person hasn't changed; the rest fill blanks only.
type StoreLookupBatch struct {
	ID              uint `gorm:"primarykey;auto_increment;not_null"`
	LibraryID       uint `gorm:"not null;index"`
	CreatedByUserID *uint
	Refresh         bool `gorm:"not null;default:false"`
	CreatedAt       time.Time
}

func (StoreLookupBatch) TableName() string { return "store_lookup_batches" }

// StoreLookup is one asset's background store lookup.
type StoreLookup struct {
	ID        uint            `gorm:"primarykey;auto_increment;not_null"`
	BatchID   uint            `gorm:"not null;index"`
	LibraryID uint            `gorm:"not null"`
	AssetID   uint            `gorm:"not null"`
	Status    string          `gorm:"not null;default:queued"`
	Result    string          `gorm:"not null;default:''"`
	Applied   json.RawMessage `gorm:"type:jsonb;not null;default:'[]'"`
	Conflicts json.RawMessage `gorm:"type:jsonb;not null;default:'[]'"`
	Error     string          `gorm:"not null;default:''"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (StoreLookup) TableName() string { return "store_lookups" }
