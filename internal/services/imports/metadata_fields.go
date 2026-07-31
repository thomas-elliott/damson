package imports

import (
	"strconv"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// FieldConflict names a storefront column whose fetched value differs from the
// asset's current value, so overwriting it needs explicit confirmation.
type FieldConflict struct {
	Field    string
	Current  string
	Proposed string
}

// MetadataFieldUpdates computes the storefront-column writes for one fetch.
// A blank column is filled silently; a column that already holds a different
// value is reported as a conflict unless confirm[field] is set, in which case
// it is overwritten. asset is mutated for every applied field so the caller's
// response reflects the new state.
//
// Both apply paths — the manual fetch handler and auto-enrichment — go through
// this so their semantics can't drift.
func MetadataFieldUpdates(asset *models.Asset, meta *metadata.Metadata, confirm map[string]bool) (before, updates map[string]any, conflicts []FieldConflict) {
	before = map[string]any{}
	updates = map[string]any{}

	text := []struct {
		column   string
		current  **string
		proposed string
	}{
		{"publisher", &asset.Publisher, meta.Publisher},
		{"publisher_url", &asset.PublisherURL, meta.PublisherURL},
		{"store", &asset.Store, meta.Store},
		{"store_product_id", &asset.StoreProductID, meta.StoreID},
		{"latest_store_version", &asset.LatestStoreVersion, meta.LatestStoreVersion},
		{"engine", &asset.Engine, meta.Engine},
		{"min_engine_version", &asset.MinEngineVersion, meta.MinEngineVersion},
		{"price", &asset.Price, meta.Price},
		{"price_currency", &asset.PriceCurrency, meta.Currency},
	}
	for _, f := range text {
		proposed := strings.TrimSpace(f.proposed)
		if proposed == "" {
			continue
		}
		current := strings.TrimSpace(derefStr(*f.current))
		if current == proposed {
			continue
		}
		if current != "" {
			if !confirm[f.column] {
				conflicts = append(conflicts, FieldConflict{Field: f.column, Current: current, Proposed: proposed})
				continue
			}
			before[f.column] = current
		}
		v := proposed
		*f.current = &v
		updates[f.column] = v
	}

	dates := []struct {
		column   string
		current  **time.Time
		proposed *time.Time
	}{
		{"released_at", &asset.ReleasedAt, meta.ReleasedAt},
		{"store_updated_at", &asset.StoreUpdatedAt, meta.StoreUpdatedAt},
	}
	for _, f := range dates {
		if f.proposed == nil {
			continue
		}
		if cur := *f.current; cur != nil {
			if cur.Equal(*f.proposed) {
				continue
			}
			if !confirm[f.column] {
				conflicts = append(conflicts, FieldConflict{
					Field:    f.column,
					Current:  cur.UTC().Format(time.RFC3339),
					Proposed: f.proposed.UTC().Format(time.RFC3339),
				})
				continue
			}
			before[f.column] = cur.UTC().Format(time.RFC3339)
		}
		v := f.proposed.UTC()
		*f.current = &v
		updates[f.column] = v
	}

	if meta.Rating > 0 {
		if cur := asset.Rating; cur == nil || *cur != meta.Rating {
			switch {
			case cur != nil && !confirm["rating"]:
				conflicts = append(conflicts, FieldConflict{
					Field:    "rating",
					Current:  strconv.FormatFloat(*cur, 'f', -1, 64),
					Proposed: strconv.FormatFloat(meta.Rating, 'f', -1, 64),
				})
			default:
				if cur != nil {
					before["rating"] = *cur
				}
				v := meta.Rating
				asset.Rating = &v
				updates["rating"] = v
			}
		}
	}
	if meta.RatingCount > 0 {
		if cur := asset.RatingCount; cur == nil || *cur != meta.RatingCount {
			switch {
			case cur != nil && !confirm["rating_count"]:
				conflicts = append(conflicts, FieldConflict{
					Field:    "rating_count",
					Current:  strconv.Itoa(*cur),
					Proposed: strconv.Itoa(meta.RatingCount),
				})
			default:
				if cur != nil {
					before["rating_count"] = *cur
				}
				v := meta.RatingCount
				asset.RatingCount = &v
				updates["rating_count"] = v
			}
		}
	}
	return before, updates, conflicts
}

// MetadataTagList is the full tag set one fetch contributes: the store's flat
// keyword tags plus its category as a single hierarchical path. tags.Attach
// normalises each path, so they are passed through as the store wrote them.
func MetadataTagList(meta *metadata.Metadata) []string {
	out := make([]string, 0, len(meta.Tags)+1)
	out = append(out, meta.Tags...)
	if p := strings.TrimSpace(meta.CategoryPath); p != "" {
		out = append(out, p)
	}
	return out
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
