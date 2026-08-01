package models

import (
	"encoding/json"
	"time"
)

// Upload session lifecycle. Only UploadOpen accepts chunks.
const (
	UploadOpen      = "open"
	UploadCompleted = "completed"
	UploadAborted   = "aborted"
	UploadExpired   = "expired"
)

type UploadSession struct {
	ID        uint   `gorm:"primarykey;auto_increment;not_null"`
	LibraryID uint   `gorm:"not null;index"`
	UserID    uint   `gorm:"not null"`
	Status    string `gorm:"not null;default:open"`

	Filename    string `gorm:"not null"`
	ContentType string `gorm:"not null;default:''"`
	SizeBytes   int64  `gorm:"not null"`
	Sha256      string `gorm:"column:sha256;not null"`
	ChunkSize   int64  `gorm:"not null"`

	ObjectPrefix    string          `gorm:"not null"`
	BackendUploadID string          `gorm:"not null;default:''"`
	PartsJSON       json.RawMessage `gorm:"type:jsonb;not null;default:'[]'"`
	HashState       []byte
	ReceivedBytes   int64 `gorm:"not null;default:0"`
	NextChunk       int   `gorm:"not null;default:0"`

	TargetAssetID      *uint
	StoreVersion       string `gorm:"not null;default:''"`
	SourceKind         string `gorm:"not null;default:''"`
	SourceRelativePath string `gorm:"not null;default:''"`

	AssetID  *uint
	ImportID *uint

	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (UploadSession) TableName() string { return "upload_sessions" }
