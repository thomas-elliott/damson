package enrich

import (
	"context"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// An apply that fails part-way writes nothing and returns the error, rather
// than leaving the fields it reached written and reporting them applied.
// Here the last write, the audit row, fails: its user does not exist.
func TestApply_FailingPartWayWritesNothing(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedEnrichAsset(t, db, "Pack", false)
	missingUser := uint(999999)
	meta := &metadata.Metadata{
		Store: metadata.StoreUnity, Description: "From the store.", Publisher: "P",
		LicenceHint: "MIT", Tags: []string{"nature"},
	}

	res, err := Apply(context.Background(), Deps{DB: db}, asset, meta, Options{UserID: &missingUser, By: licence.ByPerson})
	if err == nil {
		t.Fatalf("Apply = %+v, want the failed audit write returned", res)
	}

	var got models.Asset
	db.First(&got, asset.ID)
	if got.Description != "" || got.Publisher != nil || got.LicenceID != nil {
		t.Fatalf("asset %+v: want nothing written", got)
	}
	var licences, links int64
	db.Model(&models.Licence{}).Where("asset_id = ?", asset.ID).Count(&licences)
	db.Model(&models.AssetTag{}).Where("asset_id = ?", asset.ID).Count(&links)
	if licences != 0 || links != 0 {
		t.Fatalf("%d licences, %d tag links: want none", licences, links)
	}
}
