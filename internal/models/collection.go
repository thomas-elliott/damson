package models

import "time"

type Collection struct {
	ID              uint   `gorm:"primarykey;auto_increment;not_null"`
	LibraryID       uint   `gorm:"not null;index"`
	Name            string `gorm:"not null"`
	Description     string
	CreatedByUserID *uint
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Collection) TableName() string { return "collections" }

type CollectionAsset struct {
	CollectionID  uint `gorm:"primaryKey"`
	AssetID       uint `gorm:"primaryKey;index"`
	AddedAt       time.Time
	AddedByUserID *uint
}

func (CollectionAsset) TableName() string { return "collection_assets" }
