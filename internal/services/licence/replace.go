package licence

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/thomas-elliott/damson/internal/models"
)

// Writer says who is writing an asset's licence.
type Writer int

const (
	// ByPerson is a person's own write, which replaces any licence.
	ByPerson Writer = iota
	// Automatic is import enrichment, a rescan or a store lookup, which never
	// replaces a licence a person has reviewed.
	Automatic
)

// Replace makes l the asset's licence and deletes the one it replaces. The
// new row starts unreviewed. An Automatic write leaves a reviewed licence in
// place and reports written false. It returns the licence that was the
// asset's before the call, or nil when it had none.
func Replace(tx *gorm.DB, assetID uint, l *models.Licence, by Writer) (prev *models.Licence, written bool, err error) {
	var asset models.Asset
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "licence_id").
		First(&asset, assetID).Error; err != nil {
		return nil, false, err
	}
	if asset.LicenceID != nil {
		var existing models.Licence
		switch err := tx.First(&existing, *asset.LicenceID).Error; {
		case err == nil:
			prev = &existing
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return nil, false, err
		}
	}
	if prev != nil {
		if prev.ReviewedAt != nil && by == Automatic {
			return prev, false, nil
		}
		if err := tx.Delete(prev).Error; err != nil {
			return nil, false, err
		}
	}
	l.ID = 0
	l.AssetID = assetID
	l.ReviewedAt, l.ReviewedByUserID = nil, nil
	if err := tx.Create(l).Error; err != nil {
		return nil, false, err
	}
	if err := tx.Model(&models.Asset{}).Where("id = ?", assetID).Update("licence_id", l.ID).Error; err != nil {
		return nil, false, err
	}
	return prev, true, nil
}
