// Package tags holds the shared tag-path normalisation and asset-tag attach
// logic used by the API handlers and the import enrichment pipeline, so both
// create tags, reuse existing ones, and link them to assets the same way.
package tags

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/tagpath"
)

// MaxPathLength caps the byte length of a normalised tag path.
const MaxPathLength = tagpath.MaxLength

// NormalisePath trims whitespace, lowercases, collapses repeated slashes, and
// strips leading/trailing slashes. Returns "" when the result is empty or
// exceeds MaxPathLength so callers surface a 400.
//
// The rule lives in internal/tagpath so the CLI can apply it without pulling
// GORM into the client binary; two copies would drift.
func NormalisePath(s string) string { return tagpath.Normalise(s) }

// CreateOrReusePath resolves the leaf tag for a `/`-separated path, creating any
// missing ancestor rows for the library. Returns tagID == 0 for a blank or
// oversized path (a no-op the caller can treat as a bad request). tx must be an
// open transaction.
func CreateOrReusePath(tx *gorm.DB, libraryID uint, path string) (uint, error) {
	p := NormalisePath(path)
	if p == "" {
		return 0, nil
	}
	segs := strings.Split(p, "/")
	var parentID *uint
	var tagID uint
	for i := range segs {
		sub := strings.Join(segs[:i+1], "/")
		var t models.Tag
		err := tx.Where("library_id = ? AND path = ?", libraryID, sub).First(&t).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			t = models.Tag{LibraryID: libraryID, Path: sub, ParentID: parentID}
			if err := tx.Create(&t).Error; err != nil {
				return 0, err
			}
		}
		tagID = t.ID
		parentID = &t.ID
	}
	return tagID, nil
}

// LinkTag idempotently links a tag to an asset and audits only when a new link
// is created (so re-runs and enrichment don't emit duplicate audit noise). tx
// must be an open transaction.
func LinkTag(tx *gorm.DB, libraryID uint, userID *uint, assetID, tagID uint) (bool, error) {
	link := models.AssetTag{AssetID: assetID, TagID: tagID}
	res := tx.Where("asset_id = ? AND tag_id = ?", assetID, tagID).
		Attrs(&link).
		FirstOrCreate(&link)
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	return true, audit.Write(tx, audit.Entry{
		LibraryID:  &libraryID,
		UserID:     userID,
		ActionType: "asset.tag",
		TargetType: "asset",
		TargetID:   &assetID,
		Diff:       audit.Created(map[string]any{"tag_id": tagID}),
	})
}

// Attach create-or-reuses the tag at path and links it to the asset in one
// step. Returns the leaf tag id (0 for a blank/oversized path) and whether a
// new link was created. tx must be an open transaction.
func Attach(tx *gorm.DB, libraryID uint, userID *uint, assetID uint, path string) (uint, bool, error) {
	tagID, err := CreateOrReusePath(tx, libraryID, path)
	if err != nil || tagID == 0 {
		return 0, false, err
	}
	linked, err := LinkTag(tx, libraryID, userID, assetID, tagID)
	return tagID, linked, err
}
