package enrich

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/metadata"
	"github.com/thomas-elliott/damson/internal/models"
)

// SourceRecordOf is the asset's stored source record, or nil when it has
// none or it holds no lookup's details.
func SourceRecordOf(asset *models.Asset) *metadata.SourceRecord {
	var rec metadata.SourceRecord
	if len(asset.SourceMetadataJSON) == 0 || json.Unmarshal(asset.SourceMetadataJSON, &rec) != nil || rec.Metadata == nil {
		return nil
	}
	return &rec
}

// SaveRecord stores the source record for the lookup this plan applied, from
// the listing at link. The record must say what lookups put on the asset. A
// generic answer only adds the blanks it filled to the record a store lookup
// left, and each kept field goes on holding the last record's value.
func (p *Plan) SaveRecord(tx *gorm.DB, link string, applied, kept []string) error {
	meta := p.meta
	if meta.StoreLookupFailed && p.prev != nil {
		copyRecordFields(p.prev.Metadata, meta, applied)
		return SaveSourceRecord(tx, p.asset, *p.prev)
	}
	saved := *meta
	if p.prev != nil {
		copyRecordFields(&saved, p.prev.Metadata, kept)
	}
	listing := meta.ListingURL
	if listing == "" {
		listing = link
	}
	return SaveSourceRecord(tx, p.asset, metadata.SourceRecord{
		Store:      meta.Store,
		ListingURL: listing,
		FetchedAt:  time.Now().UTC(),
		Matched:    true,
		Metadata:   &saved,
	})
}

// SaveSourceRecord writes rec as the asset's source record.
func SaveSourceRecord(tx *gorm.DB, asset *models.Asset, rec metadata.SourceRecord) error {
	blob, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := tx.Model(&models.Asset{}).Where("id = ?", asset.ID).
		Update("source_metadata_json", json.RawMessage(blob)).Error; err != nil {
		return err
	}
	asset.SourceMetadataJSON = blob
	return nil
}
