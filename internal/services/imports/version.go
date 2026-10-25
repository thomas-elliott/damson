package imports

import (
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// ParseStoreVersion splits a dotted-integer store version into numeric
// components. ok is false when the string is empty or has a non-numeric
// segment, meaning it can't participate in semantic ordering.
func ParseStoreVersion(s string) (parts []int, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "vV")
	if s == "" {
		return nil, false
	}
	for _, seg := range strings.Split(s, ".") {
		n, err := strconv.Atoi(seg)
		if err != nil {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}

// CompareStoreVersions orders two parsed store versions component-wise,
// treating a missing trailing component as zero (so 1.0 == 1). Returns -1 when
// a sorts before b, 1 when after, 0 when equal.
func CompareStoreVersions(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
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

// ChooseCurrent points the asset at the version that should be current: the
// highest parseable store version, then the newest import date, then the
// newest row. Every version without a parseable store version ties below the
// parseable ones. A version whose latest import failed or is still running is
// passed over, except finishing: the version whose import is finishing in tx
// (0 for none). It returns the version chosen, or nil when there is none.
func ChooseCurrent(tx *gorm.DB, assetID, finishing uint) (*uint, error) {
	var versions []models.AssetVersion
	unfinished := tx.Table("asset_imports AS i").Select("1").
		Where("i.asset_version_id = asset_versions.id AND i.stage <> ?", StageDone).
		Where("NOT EXISTS (SELECT 1 FROM asset_imports j WHERE j.asset_version_id = i.asset_version_id AND j.id > i.id)")
	if err := tx.Where("asset_id = ?", assetID).
		Where("id = ? OR NOT EXISTS (?)", finishing, unfinished).
		Find(&versions).Error; err != nil {
		return nil, err
	}
	var current *uint
	if len(versions) > 0 {
		best := versions[0]
		for _, v := range versions[1:] {
			if newerThan(v, best) {
				best = v
			}
		}
		current = &best.ID
	}
	err := tx.Model(&models.Asset{}).Where("id = ? AND current_version_id IS DISTINCT FROM ?", assetID, current).
		Update("current_version_id", current).Error
	return current, err
}

// chooseCurrentAndCover runs ChooseCurrent, then gives the asset version's
// harvested cover when version is the one chosen. A storefront cover outlives
// later extractions.
func chooseCurrentAndCover(tx *gorm.DB, assetID, version, finishing uint, thumbKey, thumbType string) error {
	current, err := ChooseCurrent(tx, assetID, finishing)
	if err != nil || current == nil || *current != version || thumbKey == "" {
		return err
	}
	return tx.Model(&models.Asset{}).Where("id = ? AND NOT thumbnail_from_store", assetID).
		Updates(map[string]any{"thumbnail_object_key": thumbKey, "thumbnail_content_type": thumbType}).Error
}

// newerThan reports whether a ranks above b for ChooseCurrent.
func newerThan(a, b models.AssetVersion) bool {
	pa, aok := ParseStoreVersion(a.StoreVersion)
	pb, bok := ParseStoreVersion(b.StoreVersion)
	if aok != bok {
		return aok
	}
	if aok {
		if c := CompareStoreVersions(pa, pb); c != 0 {
			return c > 0
		}
	}
	if !a.ImportDate.Equal(b.ImportDate) {
		return a.ImportDate.After(b.ImportDate)
	}
	return a.ID > b.ID
}
