// Package library holds the rules that belong to a library as a whole.
package library

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// Settings loads a library's settings, creating its row with the defaults on
// first read.
func Settings(tx *gorm.DB, libID uint) (*models.LibrarySettings, error) {
	var s models.LibrarySettings
	err := tx.Where("library_id = ?", libID).First(&s).Error
	if err == nil {
		return &s, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	s = models.LibrarySettings{
		LibraryID:               libID,
		GridDensityDefault:      "medium",
		RetainOriginalPackages:  true,
		DedupeEnabled:           true,
		ThumbnailsRasterEnabled: true,
		ThumbnailsAudioEnabled:  true,
		ThumbnailsFontEnabled:   true,
		ThumbnailsUnityPreview:  true,
		PreviewsEnabled:         true,
		AutoEnrichEnabled:       true,
		PreferredStore:          metadata.StoreUnity,
	}
	// A concurrent first read may create the row first; then read theirs.
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&s)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		if err := tx.Where("library_id = ?", libID).First(&s).Error; err != nil {
			return nil, err
		}
	}
	return &s, nil
}
