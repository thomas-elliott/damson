package models

import (
	"encoding/json"
	"time"
)

type Asset struct {
	ID                   uint   `gorm:"primarykey;auto_increment;not_null"`
	LibraryID            uint   `gorm:"not null;index"`
	Name                 string `gorm:"not null"`
	Description          string
	SourceURL            string
	SourceMetadataJSON   json.RawMessage `gorm:"type:jsonb"`
	LicenceID            *uint
	CurrentVersionID     *uint
	ThumbnailObjectKey   string
	ThumbnailContentType string
	NeedsReview          bool `gorm:"not null;default:true"`
	CreatedByUserID      *uint
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (Asset) TableName() string { return "assets" }

type AssetVersion struct {
	ID                       uint   `gorm:"primarykey;auto_increment;not_null"`
	AssetID                  uint   `gorm:"not null;index:idx_version_asset_date;uniqueIndex:idx_version_asset_label,priority:1"`
	VersionLabel             string `gorm:"not null;uniqueIndex:idx_version_asset_label,priority:2"`
	Notes                    string
	ImportDate               time.Time `gorm:"not null;default:current_timestamp;index:idx_version_asset_date,sort:desc"`
	OriginalPackageObjectKey string
	PackageKind              string // 'unitypackage' | 'zip' | 'loose' | ''
	CreatedByUserID          *uint
}

func (AssetVersion) TableName() string { return "asset_versions" }

type AssetFile struct {
	ID             uint   `gorm:"primarykey;auto_increment;not_null"`
	AssetVersionID uint   `gorm:"not null;uniqueIndex:idx_file_version_path;index"`
	RelativePath   string `gorm:"not null;uniqueIndex:idx_file_version_path"`
	SizeBytes      int64  `gorm:"not null"`
	ContentHash    string `gorm:"not null;index"`
	ObjectKey      string `gorm:"not null"`
	DetectedType   string
	UnityGUID      string
	// Backend-rendered preview derivative (e.g. a flattened/rasterised image),
	// stored content-addressed in the thumb namespace. Empty when the file has
	// no backend preview.
	PreviewObjectKey   string
	PreviewContentType string
	// PreviewStatus is the first-class state of this file's backend preview:
	// unsupported | queued | processing | done | failed. Drives the preview
	// processing queue and requeue; see services/preview for the constants.
	PreviewStatus string `gorm:"not null;default:unsupported"`
	CreatedAt     time.Time
}

func (AssetFile) TableName() string { return "asset_files" }

type AssetImport struct {
	ID        uint   `gorm:"primarykey;auto_increment;not_null"`
	AssetID   uint   `gorm:"not null;index"`
	Stage     string `gorm:"not null;index"`
	Error     string
	UpdatedAt time.Time
}

func (AssetImport) TableName() string { return "asset_imports" }
