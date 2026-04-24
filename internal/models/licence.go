package models

import (
	"encoding/json"
	"time"
)

type Licence struct {
	ID                  uint   `gorm:"primarykey;auto_increment;not_null"`
	AssetID             uint   `gorm:"not null;index"`
	DetectedType        string // SPDX id, 'unity-eula', 'custom', 'unknown'
	DetectionConfidence float32
	CanonicalDiff       string
	OriginalText        string          `gorm:"not null"`
	AttributesJSON      json.RawMessage `gorm:"type:jsonb;not null;default:'{}'"`
	ReviewedByUserID    *uint
	ReviewedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (Licence) TableName() string { return "licences" }
