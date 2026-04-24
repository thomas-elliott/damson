package models

import "time"

type ServerSettings struct {
	ID                  uint  `gorm:"primarykey;default:1"`
	MaxUploadBytes      int64 `gorm:"not null;default:2147483648"`
	LicenceKBAutoUpdate bool  `gorm:"not null;default:false"`
	UpdatedAt           time.Time
	UpdatedByUserID     *uint
}

func (ServerSettings) TableName() string { return "server_settings" }

type LibrarySettings struct {
	LibraryID               uint   `gorm:"primarykey"`
	ThemeDefault            string `gorm:"not null;default:dark"`
	GridDensityDefault      string `gorm:"not null;default:medium"`
	RetainOriginalPackages  bool   `gorm:"not null;default:true"`
	DedupeEnabled           bool   `gorm:"not null;default:true"`
	ThumbnailsRasterEnabled bool   `gorm:"not null;default:true"`
	ThumbnailsAudioEnabled  bool   `gorm:"not null;default:true"`
	ThumbnailsFontEnabled   bool   `gorm:"not null;default:true"`
	ThumbnailsUnityPreview  bool   `gorm:"not null;default:true"`
	AuditRetentionDays      *int
	UpdatedAt               time.Time
	UpdatedByUserID         *uint
}

func (LibrarySettings) TableName() string { return "library_settings" }
