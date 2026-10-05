package api

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// refreshFields is every field a lookup can write that a person could also
// have written. An explicit refresh may replace each one a person hasn't
// changed; the store's deprecated flag is not among them, since every lookup
// sets it.
var refreshFields = []string{
	"name", "description", "licence", "thumbnail",
	"publisher", "publisher_url", "store", "store_product_id", "latest_store_version",
	"engine", "min_engine_version", "price", "price_currency",
	"released_at", "store_updated_at", "rating", "rating_count",
}

// refreshConfirm is the confirm set an explicit refresh applies with: every
// field a person hasn't changed. Blank fields are filled whatever it says, and
// a field left out of it is kept and reported as a conflict.
func (h *Handler) refreshConfirm(ctx context.Context, asset *models.Asset) (map[string]bool, error) {
	changed, err := h.changedByPerson(ctx, asset)
	if err != nil {
		return nil, err
	}
	confirm := map[string]bool{}
	for _, f := range refreshFields {
		if !changed[f] {
			confirm[f] = true
		}
	}
	return confirm, nil
}

// changedByPerson reports which fields a person has changed. With a stored
// source record that is any field no longer holding what the last lookup gave
// it; with none, any field the audit log shows a person editing. A thumbnail
// is a person's unless a lookup set it, whatever the record says, because the
// record keeps only the cover's URL.
func (h *Handler) changedByPerson(ctx context.Context, asset *models.Asset) (map[string]bool, error) {
	var (
		changed map[string]bool
		err     error
	)
	if rec := sourceRecordOf(asset); rec != nil {
		changed = imports.ChangedSince(asset, rec.Metadata)
		changed["licence"], err = h.licenceChangedSince(ctx, asset, rec.Metadata)
	} else {
		changed, err = h.editedInAudit(ctx, asset.ID)
	}
	if err != nil {
		return nil, err
	}
	changed["thumbnail"] = asset.ThumbnailObjectKey != "" && !asset.ThumbnailFromStore
	return changed, nil
}

// licenceChangedSince reports whether the asset's licence is not the one the
// last lookup would have given it, or a person has reviewed it.
func (h *Handler) licenceChangedSince(ctx context.Context, asset *models.Asset, last *metadata.Metadata) (bool, error) {
	if asset.LicenceID == nil {
		return false, nil
	}
	var lic models.Licence
	if err := h.db.WithContext(ctx).First(&lic, *asset.LicenceID).Error; err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if lic.ReviewedByUserID != nil {
		return true, nil
	}
	want := buildLicenceFromMeta(last)
	return want == nil || want.DetectedType != lic.DetectedType, nil
}

// editedInAudit reports the fields the audit log shows a person editing on
// the asset: the columns of each edit, and the licence when it was set,
// changed or reviewed. A lookup's own writes are not a person's edits.
func (h *Handler) editedInAudit(ctx context.Context, assetID uint) (map[string]bool, error) {
	var rows []struct {
		ActionType string
		DiffJSON   json.RawMessage
	}
	err := h.db.WithContext(ctx).Model(&models.AuditLog{}).Select("action_type, diff_json").
		Where("target_type = 'asset' AND target_id = ? AND user_id IS NOT NULL", assetID).
		Where("action_type = 'asset.update' OR action_type LIKE 'asset.licence.%'").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	edited := map[string]bool{}
	for _, r := range rows {
		if strings.HasPrefix(r.ActionType, "asset.licence.") {
			edited["licence"] = true
			continue
		}
		var diff struct {
			After map[string]any `json:"after"`
		}
		if json.Unmarshal(r.DiffJSON, &diff) != nil {
			continue
		}
		for column := range diff.After {
			edited[column] = true
		}
	}
	return edited, nil
}

// withoutUnchangedCover leaves the cover out of a refresh when the asset
// already has a thumbnail and the store's cover is the one the last lookup
// gave. Fetching it again would replace the thumbnail with itself and count as
// a change, marking every refreshed asset Needs review.
func withoutUnchangedCover(asset *models.Asset, meta *metadata.Metadata) *metadata.Metadata {
	if asset.ThumbnailObjectKey == "" || meta.CoverURL == "" {
		return meta
	}
	if rec := sourceRecordOf(asset); rec == nil || rec.Metadata.CoverURL != meta.CoverURL {
		return meta
	}
	m := *meta
	m.CoverURL = ""
	return &m
}

// sourceRecordOf is the asset's stored source record, or nil when it has none
// or it holds no lookup's details.
func sourceRecordOf(asset *models.Asset) *metadata.SourceRecord {
	var rec metadata.SourceRecord
	if len(asset.SourceMetadataJSON) == 0 || json.Unmarshal(asset.SourceMetadataJSON, &rec) != nil || rec.Metadata == nil {
		return nil
	}
	return &rec
}

// copyRecordFields copies the named fields' values from src into dst. The
// refresh rule reads a field that differs from the stored record as a
// person's, so the record must hold what lookups last put on the asset, not
// what a lookup proposed and didn't write.
func copyRecordFields(dst, src *metadata.Metadata, fields []string) {
	for _, f := range fields {
		switch f {
		case "name":
			dst.Title = src.Title
		case "description":
			dst.Description = src.Description
		case "licence":
			dst.LicenceHint, dst.LicenceText = src.LicenceHint, src.LicenceText
		case "thumbnail":
			dst.CoverURL = src.CoverURL
		case "publisher":
			dst.Publisher = src.Publisher
		case "publisher_url":
			dst.PublisherURL = src.PublisherURL
		case "store":
			dst.Store = src.Store
		case "store_product_id":
			dst.StoreID = src.StoreID
		case "latest_store_version":
			dst.LatestStoreVersion = src.LatestStoreVersion
		case "engine":
			dst.Engine = src.Engine
		case "min_engine_version":
			dst.MinEngineVersion = src.MinEngineVersion
		case "price":
			dst.Price = src.Price
		case "price_currency":
			dst.Currency = src.Currency
		case "released_at":
			dst.ReleasedAt = src.ReleasedAt
		case "store_updated_at":
			dst.StoreUpdatedAt = src.StoreUpdatedAt
		case "rating":
			dst.Rating = src.Rating
		case "rating_count":
			dst.RatingCount = src.RatingCount
		}
	}
}
