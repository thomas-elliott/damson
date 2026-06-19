package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
)

// assetDetailView is the shape returned by GET /api/assets/:id and is the
// data the detail panel renders from. Lists of versions, files, tags,
// licence summary.
type assetDetailView struct {
	ID                   uint                `json:"id"`
	Name                 string              `json:"name"`
	Description          string              `json:"description"`
	SourceURL            string              `json:"source_url"`
	NeedsReview          bool                `json:"needs_review"`
	CurrentVersionID     *uint               `json:"current_version_id,omitempty"`
	ThumbnailObjectKey   string              `json:"thumbnail_object_key,omitempty"`
	ThumbnailContentType string              `json:"thumbnail_content_type,omitempty"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
	Licence              *licenceView        `json:"licence,omitempty"`
	Tags                 []tagView           `json:"tags"`
	Versions             []assetVersionView  `json:"versions"`
	Files                []assetFileView     `json:"files"` // current version's files
	Projects             []projectPinView    `json:"projects"`
	Collections          []collectionRefView `json:"collections"`
}

type assetVersionView struct {
	ID           uint      `json:"id"`
	VersionLabel string    `json:"version_label"`
	Notes        string    `json:"notes"`
	ImportDate   time.Time `json:"import_date"`
	PackageKind  string    `json:"package_kind"`
	FileCount    int       `json:"file_count"`
	SizeBytes    int64     `json:"size_bytes"`
	HasOriginal  bool      `json:"has_original"`
	IsCurrent    bool      `json:"is_current"`
}

type assetFileView struct {
	ID               uint   `json:"id"`
	RelativePath     string `json:"relative_path"`
	SizeBytes        int64  `json:"size_bytes"`
	ContentHash      string `json:"content_hash"`
	ObjectKey        string `json:"object_key"`
	DetectedType     string `json:"detected_type,omitempty"`
	PreviewObjectKey string `json:"preview_object_key,omitempty"`
	PreviewStatus    string `json:"preview_status,omitempty"`
}

type projectPinView struct {
	ProjectID       uint   `json:"project_id"`
	ProjectName     string `json:"project_name"`
	PinnedVersionID uint   `json:"pinned_version_id"`
	VersionLabel    string `json:"version_label"`
}

type collectionRefView struct {
	CollectionID   uint   `json:"collection_id"`
	CollectionName string `json:"collection_name"`
}

// getAssetDetail returns everything the detail panel renders — asset +
// current-version files + all versions + tags + licence + project pins +
// collections.
func (h *Handler) getAssetDetail(c *gin.Context) {
	lib := getLibrary(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).
		First(&asset).Error; err != nil {
		if isNotFound(err) {
			respond404(c, "asset not found")
			return
		}
		respond500(c)
		return
	}

	view := assetDetailView{
		ID:                   asset.ID,
		Name:                 asset.Name,
		Description:          asset.Description,
		SourceURL:            asset.SourceURL,
		NeedsReview:          asset.NeedsReview,
		CurrentVersionID:     asset.CurrentVersionID,
		ThumbnailObjectKey:   asset.ThumbnailObjectKey,
		ThumbnailContentType: asset.ThumbnailContentType,
		CreatedAt:            asset.CreatedAt,
		UpdatedAt:            asset.UpdatedAt,
		Tags:                 []tagView{},
		Versions:             []assetVersionView{},
		Files:                []assetFileView{},
		Projects:             []projectPinView{},
		Collections:          []collectionRefView{},
	}

	// Versions with per-version file_count + size.
	type vRow struct {
		ID           uint
		VersionLabel string
		Notes        string
		ImportDate   time.Time
		PackageKind  string
		HasOriginal  bool
		FileCount    int
		SizeBytes    int64
	}
	var versions []vRow
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_versions AS v").
		Select(`v.id, v.version_label, v.notes, v.import_date, v.package_kind,
			CASE WHEN v.original_package_object_key IS NOT NULL AND v.original_package_object_key <> '' THEN true ELSE false END AS has_original,
			COALESCE(COUNT(f.id), 0) AS file_count,
			COALESCE(SUM(f.size_bytes), 0) AS size_bytes`).
		Joins("LEFT JOIN asset_files f ON f.asset_version_id = v.id").
		Where("v.asset_id = ?", asset.ID).
		Group("v.id").
		Order("v.import_date DESC").
		Scan(&versions).Error
	if err != nil {
		respond500(c)
		return
	}
	for _, v := range versions {
		view.Versions = append(view.Versions, assetVersionView{
			ID:           v.ID,
			VersionLabel: v.VersionLabel,
			Notes:        v.Notes,
			ImportDate:   v.ImportDate,
			PackageKind:  v.PackageKind,
			FileCount:    v.FileCount,
			SizeBytes:    v.SizeBytes,
			HasOriginal:  v.HasOriginal,
			IsCurrent:    asset.CurrentVersionID != nil && *asset.CurrentVersionID == v.ID,
		})
	}

	// Files for the current version only — the detail panel scopes to the
	// current version, and the version selector triggers a separate request
	// if the user wants to inspect an older one.
	if asset.CurrentVersionID != nil {
		var files []models.AssetFile
		if err := h.db.WithContext(c.Request.Context()).
			Where("asset_version_id = ?", *asset.CurrentVersionID).
			Order("relative_path").
			Find(&files).Error; err != nil {
			respond500(c)
			return
		}
		for _, f := range files {
			view.Files = append(view.Files, assetFileView{
				ID:               f.ID,
				RelativePath:     f.RelativePath,
				SizeBytes:        f.SizeBytes,
				ContentHash:      f.ContentHash,
				ObjectKey:        f.ObjectKey,
				DetectedType:     f.DetectedType,
				PreviewObjectKey: f.PreviewObjectKey,
				PreviewStatus:    f.PreviewStatus,
			})
		}
	}

	// Tags.
	type tRow struct {
		ID   uint
		Path string
	}
	var tagRows []tRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("tags AS t").
		Select("t.id, t.path").
		Joins("JOIN asset_tags at ON at.tag_id = t.id").
		Where("at.asset_id = ?", asset.ID).
		Order("t.path").
		Scan(&tagRows).Error; err != nil {
		respond500(c)
		return
	}
	for _, t := range tagRows {
		view.Tags = append(view.Tags, tagView{ID: t.ID, Path: t.Path})
	}

	// Licence summary.
	if asset.LicenceID != nil {
		var lic models.Licence
		if err := h.db.WithContext(c.Request.Context()).First(&lic, *asset.LicenceID).Error; err == nil {
			lv := licenceViewFrom(&lic)
			view.Licence = &lv
		}
	}

	// Project pins.
	type pRow struct {
		ProjectID       uint
		ProjectName     string
		PinnedVersionID uint
		VersionLabel    string
	}
	var pins []pRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("project_asset_pins AS pin").
		Select("pin.project_id, p.name AS project_name, pin.pinned_version_id, v.version_label").
		Joins("JOIN projects p ON p.id = pin.project_id").
		Joins("JOIN asset_versions v ON v.id = pin.pinned_version_id").
		Where("pin.asset_id = ? AND p.library_id = ?", asset.ID, lib.ID).
		Scan(&pins).Error; err != nil {
		respond500(c)
		return
	}
	for _, p := range pins {
		view.Projects = append(view.Projects, projectPinView{
			ProjectID:       p.ProjectID,
			ProjectName:     p.ProjectName,
			PinnedVersionID: p.PinnedVersionID,
			VersionLabel:    p.VersionLabel,
		})
	}

	// Collections.
	type cRow struct {
		CollectionID   uint
		CollectionName string
	}
	var collRows []cRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("collection_assets AS ca").
		Select("ca.collection_id, c.name AS collection_name").
		Joins("JOIN collections c ON c.id = ca.collection_id").
		Where("ca.asset_id = ? AND c.library_id = ?", asset.ID, lib.ID).
		Scan(&collRows).Error; err != nil {
		respond500(c)
		return
	}
	for _, cv := range collRows {
		view.Collections = append(view.Collections, collectionRefView{
			CollectionID:   cv.CollectionID,
			CollectionName: cv.CollectionName,
		})
	}

	c.JSON(http.StatusOK, view)
}

type assetPatchRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	SourceURL   *string `json:"source_url,omitempty"`
}

// patchAsset updates editable fields on an asset. Writes an audit row in
// the same transaction.
func (h *Handler) patchAsset(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req assetPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		respond400(c, asError("name must not be blank"))
		return
	}
	if req.SourceURL != nil {
		if cleaned, err := validateSourceURL(*req.SourceURL); err != nil {
			respond400(c, err)
			return
		} else {
			req.SourceURL = &cleaned
		}
	}

	var out models.Asset
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		// Build the column-set map from set fields only — keeps writes
		// narrow so audit diffs reflect intent rather than full-row rewrites.
		before := map[string]any{}
		updates := map[string]any{}
		if req.Name != nil {
			before["name"] = asset.Name
			asset.Name = strings.TrimSpace(*req.Name)
			updates["name"] = asset.Name
		}
		if req.Description != nil {
			before["description"] = asset.Description
			asset.Description = *req.Description
			updates["description"] = asset.Description
		}
		if req.SourceURL != nil {
			before["source_url"] = asset.SourceURL
			asset.SourceURL = strings.TrimSpace(*req.SourceURL)
			updates["source_url"] = asset.SourceURL
		}
		if len(updates) > 0 {
			if err := tx.Model(&asset).Updates(updates).Error; err != nil {
				return err
			}
		}
		out = asset
		if len(updates) == 0 {
			return nil
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.update",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Updated(before, updates),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          out.ID,
		"name":        out.Name,
		"description": out.Description,
		"source_url":  out.SourceURL,
	})
}

// deleteAsset removes an asset and all its versions/files/tags/pins. Blob
// storage is untouched — content-addressed, GC is a later job.
func (h *Handler) deleteAsset(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		before := map[string]any{"id": asset.ID, "name": asset.Name}
		// Explicit child cleanup — SQLite test path doesn't honour FK
		// cascades that Postgres enforces. No-op when Postgres has
		// already cascaded.
		var versionIDs []uint
		if err := tx.Model(&models.AssetVersion{}).
			Where("asset_id = ?", asset.ID).
			Pluck("id", &versionIDs).Error; err != nil {
			return err
		}
		if len(versionIDs) > 0 {
			if err := tx.Where("asset_version_id IN ?", versionIDs).
				Delete(&models.AssetFile{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("asset_id = ?", asset.ID).Delete(&models.AssetVersion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("asset_id = ?", asset.ID).Delete(&models.AssetTag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("asset_id = ?", asset.ID).Delete(&models.CollectionAsset{}).Error; err != nil {
			return err
		}
		if err := tx.Where("asset_id = ?", asset.ID).Delete(&models.ProjectAssetPin{}).Error; err != nil {
			return err
		}
		if err := tx.Where("asset_id = ?", asset.ID).Delete(&models.Licence{}).Error; err != nil {
			return err
		}
		if err := tx.Where("asset_id = ?", asset.ID).Delete(&models.AssetImport{}).Error; err != nil {
			return err
		}
		// Break the asset.current_version_id FK before deleting the asset row.
		asset.CurrentVersionID = nil
		asset.LicenceID = nil
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		if err := tx.Delete(&asset).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.delete",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Deleted(before),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// postAssetReview marks an asset reviewed (clears needs_review). Idempotent.
func (h *Handler) postAssetReview(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		if !asset.NeedsReview {
			// Idempotent; no audit entry to avoid log spam.
			return nil
		}
		before := map[string]any{"needs_review": true}
		asset.NeedsReview = false
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.review",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Updated(before, map[string]any{"needs_review": false}),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// asError wraps a string into an error so handler call sites stay terse:
//
//	respond400(c, asError("name must not be blank"))
//
// Returns a plain errors.New value — nothing to type-check against.
func asError(s string) error { return errors.New(s) }

// validateSourceURL trims, accepts the empty string ("clear the field"), and
// otherwise requires an absolute http(s) URL. javascript:, data:, and other
// scheme attacks land here as a 400 — the field is rendered as an <a href>
// in the SPA so an unfiltered scheme is a stored XSS vector.
func validateSourceURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", asError("source_url is not a valid URL")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", asError("source_url must be http(s)")
	}
	if u.Host == "" {
		return "", asError("source_url must include a host")
	}
	return s, nil
}
