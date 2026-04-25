package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
)

type tagView struct {
	ID         uint   `json:"id"`
	Path       string `json:"path"`
	ParentID   *uint  `json:"parent_id,omitempty"`
	AssetCount int    `json:"asset_count"`
}

// getTags returns the library's tag tree with per-tag asset counts. The tree
// is flat on the wire; the client assembles the hierarchy from `path` (`/`
// separated).
func (h *Handler) getTags(c *gin.Context) {
	lib := getLibrary(c)
	type row struct {
		ID         uint
		Path       string
		ParentID   *uint
		AssetCount int
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("tags AS t").
		Select("t.id, t.path, t.parent_id, COALESCE(COUNT(at.asset_id), 0) AS asset_count").
		Joins("LEFT JOIN asset_tags at ON at.tag_id = t.id").
		Where("t.library_id = ?", lib.ID).
		Group("t.id, t.path, t.parent_id").
		Order("t.path").
		Scan(&rows).Error
	if err != nil {
		respond500(c)
		return
	}
	out := make([]tagView, len(rows))
	for i, r := range rows {
		out[i] = tagView{ID: r.ID, Path: r.Path, ParentID: r.ParentID, AssetCount: r.AssetCount}
	}
	c.JSON(http.StatusOK, gin.H{"tags": out})
}

type createTagRequest struct {
	Path string `json:"path" binding:"required"`
}

// postTag creates a tag (and any missing ancestor rows) for the library.
// Paths are `/`-separated; `environment/trees/deciduous` creates/returns
// three rows if they didn't already exist.
func (h *Handler) postTag(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	path := normaliseTagPath(req.Path)
	if path == "" {
		respond400(c, asError("path must contain at least one segment"))
		return
	}
	var leaf *models.Tag
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		segs := strings.Split(path, "/")
		var parentID *uint
		for i := range segs {
			sub := strings.Join(segs[:i+1], "/")
			var t models.Tag
			err := tx.Where("library_id = ? AND path = ?", lib.ID, sub).First(&t).Error
			if err != nil && !isNotFound(err) {
				return err
			}
			if isNotFound(err) {
				t = models.Tag{LibraryID: lib.ID, Path: sub, ParentID: parentID}
				if err := tx.Create(&t).Error; err != nil {
					return err
				}
				if werr := audit.Write(tx, audit.Entry{
					LibraryID:  &lib.ID,
					UserID:     &u.ID,
					ActionType: "tag.create",
					TargetType: "tag",
					TargetID:   &t.ID,
					Diff:       audit.Created(map[string]any{"path": sub}),
				}); werr != nil {
					return werr
				}
			}
			cp := t
			leaf = &cp
			parentID = &t.ID
		}
		return nil
	})
	if err != nil {
		respond500(c)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":        leaf.ID,
		"path":      leaf.Path,
		"parent_id": leaf.ParentID,
	})
}

// deleteTag removes a tag and all its descendants (path prefix match).
// asset_tags rows cascade via FK.
func (h *Handler) deleteTag(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var tag models.Tag
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&tag).Error; err != nil {
			return err
		}
		// Delete self + descendants (path LIKE `p/%`).
		if err := tx.Where("library_id = ? AND (id = ? OR path LIKE ?)", lib.ID, tag.ID, tag.Path+"/%").
			Delete(&models.Tag{}).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "tag.delete",
			TargetType: "tag",
			TargetID:   &tag.ID,
			Diff:       audit.Deleted(map[string]any{"path": tag.Path}),
		})
	})
	if err != nil {
		respondGORM(c, err, "tag not found")
		return
	}
	c.Status(http.StatusNoContent)
}

type assetTagsRequest struct {
	// Either TagID (existing tag) or Path (create-or-reuse). TagID wins.
	TagID *uint  `json:"tag_id,omitempty"`
	Path  string `json:"path,omitempty"`
}

// postAssetTag attaches a tag to an asset. Accepts either an existing
// tag_id or a path (which is created via the same `postTag` logic if it
// doesn't exist yet).
func (h *Handler) postAssetTag(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	assetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req assetTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	var tagID uint
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", assetID, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		if req.TagID != nil {
			var t models.Tag
			if err := tx.Where("id = ? AND library_id = ?", *req.TagID, lib.ID).First(&t).Error; err != nil {
				return err
			}
			tagID = t.ID
		} else {
			path := normaliseTagPath(req.Path)
			if path == "" {
				return errBadRequestf("tag_id or path required (or path exceeds %d bytes)", maxTagPathLength)
			}
			// Reuse if present, else create with ancestors.
			segs := strings.Split(path, "/")
			var parentID *uint
			for i := range segs {
				sub := strings.Join(segs[:i+1], "/")
				var t models.Tag
				err := tx.Where("library_id = ? AND path = ?", lib.ID, sub).First(&t).Error
				if err != nil && !isNotFound(err) {
					return err
				}
				if isNotFound(err) {
					t = models.Tag{LibraryID: lib.ID, Path: sub, ParentID: parentID}
					if err := tx.Create(&t).Error; err != nil {
						return err
					}
				}
				tagID = t.ID
				parentID = &t.ID
			}
		}
		link := models.AssetTag{AssetID: asset.ID, TagID: tagID}
		// Idempotent: INSERT ... ON CONFLICT DO NOTHING via FirstOrCreate.
		if err := tx.Where("asset_id = ? AND tag_id = ?", asset.ID, tagID).
			Attrs(&link).
			FirstOrCreate(&link).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.tag",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Created(map[string]any{"tag_id": tagID}),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset or tag not found")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"asset_id": assetID, "tag_id": tagID})
}

// deleteAssetTag detaches a tag from an asset.
func (h *Handler) deleteAssetTag(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	assetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	tagID, ok := parseIDParam(c, "tagId")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Both asset and tag must belong to the current library.
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", assetID, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		var tag models.Tag
		if err := tx.Where("id = ? AND library_id = ?", tagID, lib.ID).First(&tag).Error; err != nil {
			return err
		}
		res := tx.Where("asset_id = ? AND tag_id = ?", asset.ID, tag.ID).Delete(&models.AssetTag{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // idempotent no-op; don't audit
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.untag",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Deleted(map[string]any{"tag_id": tag.ID}),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset or tag not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// maxTagPathLength caps the byte length of a normalised tag path. Generous
// enough to fit any legitimate hierarchy; tight enough that a malicious
// caller can't paste megabytes into the column.
const maxTagPathLength = 1024

// normaliseTagPath trims whitespace, lowercases, collapses multiple slashes,
// strips leading/trailing slashes. Returns "" when the result exceeds
// maxTagPathLength so callers surface a 400.
func normaliseTagPath(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "\\", "/")
	// Collapse repeated slashes + trim ends.
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	s = strings.Trim(s, "/")
	// Trim each segment, drop empties.
	parts := strings.Split(s, "/")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	joined := strings.Join(out, "/")
	if len(joined) > maxTagPathLength {
		return ""
	}
	return joined
}
