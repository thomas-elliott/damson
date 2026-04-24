package models

import "time"

type Project struct {
	ID                 uint   `gorm:"primarykey;auto_increment;not_null"`
	LibraryID          uint   `gorm:"not null;index"`
	Name               string `gorm:"not null"`
	Description        string
	TargetDistribution string `gorm:"not null;default:unspecified"` // 'commercial' | 'open-source' | 'personal' | 'unspecified'
	Glyph              string
	GlyphColor         string
	CreatedByUserID    *uint
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (Project) TableName() string { return "projects" }

type ProjectAssetPin struct {
	ID              uint `gorm:"primarykey;auto_increment;not_null"`
	ProjectID       uint `gorm:"not null;uniqueIndex:idx_pin_project_asset"`
	AssetID         uint `gorm:"not null;uniqueIndex:idx_pin_project_asset;index"`
	PinnedVersionID uint `gorm:"not null"`
	PinnedAt        time.Time
	PinnedByUserID  *uint
}

func (ProjectAssetPin) TableName() string { return "project_asset_pins" }
