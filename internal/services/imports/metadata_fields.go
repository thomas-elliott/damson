package imports

import (
	"math"
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
// Both apply paths go through this so their semantics can't drift.
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
		current := strings.TrimSpace(derefString(*f.current))
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
	// The deprecated flag is the store's state, not something a person writes,
	// so every lookup that could tell sets it, whatever it held before.
	if d := meta.StoreDeprecated; d != nil && *d != asset.StoreDeprecated {
		before["store_deprecated"] = asset.StoreDeprecated
		asset.StoreDeprecated = *d
		updates["store_deprecated"] = *d
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

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// StoreUpdateAvailable reports whether latestStoreVersion is newer than every
// store version the asset already holds, and returns the highest version held.
//
// Comparison is semantic, not textual: "1.10" is newer than "1.9". A version
// either side that isn't dotted-numeric can't be ordered, so it is ignored
// rather than guessed at — an unorderable latest version means "no update
// known", never a false positive.
func StoreUpdateAvailable(latestStoreVersion string, heldVersions []string) (available bool, highestHeld string) {
	latest, ok := ParseStoreVersion(latestStoreVersion)
	if !ok {
		return false, ""
	}
	var best []int
	for _, held := range heldVersions {
		parts, ok := ParseStoreVersion(held)
		if !ok {
			continue
		}
		if best == nil || CompareStoreVersions(parts, best) > 0 {
			best = parts
			highestHeld = strings.TrimSpace(strings.TrimLeft(held, "vV"))
		}
	}
	// An asset with no orderable version at all has nothing to compare, so it
	// isn't reported as out of date; the user hasn't told us what they have.
	if best == nil {
		return false, ""
	}
	return CompareStoreVersions(latest, best) > 0, highestHeld
}

// ChangedSince reports which of the asset's storefront columns, and its name
// and description, no longer hold what last (the previous lookup) gave them:
// someone changed them since. A blank column is never changed, since a lookup
// fills it anyway; a column holding a value the lookup didn't give is.
func ChangedSince(asset *models.Asset, last *metadata.Metadata) map[string]bool {
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
	// The column keeps two decimal places, and the store may send more.
	changed["rating"] = asset.Rating != nil && math.Round(*asset.Rating*100) != math.Round(last.Rating*100)
	changed["rating_count"] = asset.RatingCount != nil && *asset.RatingCount != last.RatingCount
	return changed
}
