package enrich

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/metadata"
	"github.com/thomas-elliott/damson/internal/models"
)

// fieldUpdates computes the storefront-column writes for one fetch.
// A blank column is filled silently; a column that already holds a different
// value is reported as a conflict unless confirm[field] is set, in which case
// it is overwritten. asset is mutated for every applied field so the caller's
// response reflects the new state.
func fieldUpdates(asset *models.Asset, meta *metadata.Metadata, confirm map[string]bool) (before, updates map[string]any, conflicts []Conflict) {
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
		current := strings.TrimSpace(derefString(*f.current))
		if current == proposed {
			continue
		}
		if current != "" {
			if !confirm[f.column] {
				conflicts = append(conflicts, Conflict{Field: f.column, Current: current, Proposed: proposed})
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
				conflicts = append(conflicts, Conflict{
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
		// The column keeps two decimal places, and the store may send more, so
		// an unchanged rating must not read as a new one.
		if cur := asset.Rating; cur == nil || roundRating(*cur) != roundRating(meta.Rating) {
			switch {
			case cur != nil && !confirm["rating"]:
				conflicts = append(conflicts, Conflict{
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
				conflicts = append(conflicts, Conflict{
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
	// The deprecated flag is the store's state, not something a person writes,
	// so every lookup that could tell sets it, whatever it held before.
	if d := meta.StoreDeprecated; d != nil && *d != asset.StoreDeprecated {
		before["store_deprecated"] = asset.StoreDeprecated
		asset.StoreDeprecated = *d
		updates["store_deprecated"] = *d
	}
	return before, updates, conflicts
}

// roundRating is a rating at the two decimal places the rating column keeps.
func roundRating(r float64) float64 { return math.Round(r*100) / 100 }

// tagList is the full tag set one fetch contributes: the store's flat
// keyword tags plus its category as a single hierarchical path. tags.Attach
// normalises each path, so they are passed through as the store wrote them.
func tagList(meta *metadata.Metadata) []string {
	out := make([]string, 0, len(meta.Tags)+1)
	out = append(out, meta.Tags...)
	if p := strings.TrimSpace(meta.CategoryPath); p != "" {
		out = append(out, p)
	}
	return out
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// changedSince reports which of the asset's storefront columns, and its name
// and description, no longer hold what last (the previous lookup) gave them:
// someone changed them since. A blank column is never changed, since a lookup
// fills it anyway; a column holding a value the lookup didn't give is.
func changedSince(asset *models.Asset, last *metadata.Metadata) map[string]bool {
	changed := map[string]bool{}
	text := map[string]struct{ current, last string }{
		"name":                 {asset.Name, last.Title},
		"description":          {asset.Description, last.Description},
		"publisher":            {derefString(asset.Publisher), last.Publisher},
		"publisher_url":        {derefString(asset.PublisherURL), last.PublisherURL},
		"store":                {derefString(asset.Store), last.Store},
		"store_product_id":     {derefString(asset.StoreProductID), last.StoreID},
		"latest_store_version": {derefString(asset.LatestStoreVersion), last.LatestStoreVersion},
		"engine":               {derefString(asset.Engine), last.Engine},
		"min_engine_version":   {derefString(asset.MinEngineVersion), last.MinEngineVersion},
		"price":                {derefString(asset.Price), last.Price},
		"price_currency":       {derefString(asset.PriceCurrency), last.Currency},
	}
	for column, v := range text {
		current := strings.TrimSpace(v.current)
		changed[column] = current != "" && current != strings.TrimSpace(v.last)
	}
	dates := map[string]struct{ current, last *time.Time }{
		"released_at":      {asset.ReleasedAt, last.ReleasedAt},
		"store_updated_at": {asset.StoreUpdatedAt, last.StoreUpdatedAt},
	}
	for column, v := range dates {
		changed[column] = v.current != nil && (v.last == nil || !v.current.Equal(*v.last))
	}
	changed["rating"] = asset.Rating != nil && roundRating(*asset.Rating) != roundRating(last.Rating)
	changed["rating_count"] = asset.RatingCount != nil && *asset.RatingCount != last.RatingCount
	return changed
}
