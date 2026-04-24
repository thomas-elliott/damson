package models

import "time"

type Tag struct {
	ID        uint   `gorm:"primarykey;auto_increment;not_null"`
	LibraryID uint   `gorm:"not null;uniqueIndex:idx_tag_library_path"`
	Path      string `gorm:"not null;uniqueIndex:idx_tag_library_path"`
	ParentID  *uint  `gorm:"index"`
	CreatedAt time.Time
}

func (Tag) TableName() string { return "tags" }

type AssetTag struct {
	AssetID   uint `gorm:"primaryKey"`
	TagID     uint `gorm:"primaryKey;index"`
	CreatedAt time.Time
}

func (AssetTag) TableName() string { return "asset_tags" }
