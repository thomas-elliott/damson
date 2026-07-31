package api

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/imports"
)

// assetDetailView is the shape returned by GET /api/assets/:id and is the
// data the detail panel renders from. Lists of versions, files, tags,
// licence summary.
type assetDetailView struct {
	ID                   uint      `json:"id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	SourceURL            string    `json:"source_url"`
	NeedsReview          bool      `json:"needs_review"`
	CurrentVersionID     *uint     `json:"current_version_id,omitempty"`
	ThumbnailObjectKey   string    `json:"thumbnail_object_key,omitempty"`
	ThumbnailContentType string    `json:"thumbnail_content_type,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	assetMetadataFieldsView
	Licence        *licenceView        `json:"licence,omitempty"`
	SourceMetadata *sourceMetadataView `json:"source_metadata,omitempty"`
	Tags           []tagView           `json:"tags"`
	Versions       []assetVersionView  `json:"versions"`
	Files          []assetFileView     `json:"files"` // current version's files
	Images         []assetImageView    `json:"images"`
	Projects       []projectPinView    `json:"projects"`
	Collections    []collectionRefView `json:"collections"`
}

type assetImageView struct {
	ID          uint   `json:"id"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type,omitempty"`
}

type assetVersionView struct {
	ID                       uint      `json:"id"`
	VersionLabel             string    `json:"version_label"`
	StoreVersion             string    `json:"store_version"`
	Notes                    string    `json:"notes"`
	ImportDate               time.Time `json:"import_date"`
	PackageKind              string    `json:"package_kind"`
	FileCount                int       `json:"file_count"`
	SizeBytes                int64     `json:"size_bytes"`
	HasOriginal              bool      `json:"has_original"`
	OriginalPackageObjectKey string    `json:"original_package_object_key,omitempty"`
	IsCurrent                bool      `json:"is_current"`
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
		respond500(c, err)
		return
	}

	view := assetDetailView{
		ID:                      asset.ID,
		Name:                    asset.Name,
		Description:             asset.Description,
		SourceURL:               asset.SourceURL,
		NeedsReview:             asset.NeedsReview,
		CurrentVersionID:        asset.CurrentVersionID,
		ThumbnailObjectKey:      asset.ThumbnailObjectKey,
		ThumbnailContentType:    asset.ThumbnailContentType,
		CreatedAt:               asset.CreatedAt,
		UpdatedAt:               asset.UpdatedAt,
		assetMetadataFieldsView: assetMetadataFieldsViewFrom(&asset),
		SourceMetadata:          sourceMetadataViewFrom(asset.SourceMetadataJSON),
		Tags:                    []tagView{},
		Versions:                []assetVersionView{},
		Files:                   []assetFileView{},
		Images:                  []assetImageView{},
		Projects:                []projectPinView{},
		Collections:             []collectionRefView{},
	}

	// Versions with per-version file_count + size.
	type vRow struct {
		ID                       uint
		VersionLabel             string
		StoreVersion             string
		Notes                    string
		ImportDate               time.Time
		PackageKind              string
		HasOriginal              bool
		OriginalPackageObjectKey string
		FileCount                int
		SizeBytes                int64
	}
	var versions []vRow
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_versions AS v").
		Select(`v.id, v.version_label, v.store_version, v.notes, v.import_date, v.package_kind,
			CASE WHEN v.original_package_object_key IS NOT NULL AND v.original_package_object_key <> '' AND v.original_package_purged = false THEN true ELSE false END AS has_original,
			v.original_package_object_key,
			COALESCE(COUNT(f.id), 0) AS file_count,
			COALESCE(SUM(f.size_bytes), 0) AS size_bytes`).
		Joins("LEFT JOIN asset_files f ON f.asset_version_id = v.id").
		Where("v.asset_id = ?", asset.ID).
		Group("v.id").
		Order("v.import_date DESC").
		Scan(&versions).Error
	if err != nil {
		respond500(c, err)
		return
	}
	// Semver ordering isn't portably expressible in SQL; re-sort the
	// import-date-ordered rows here, versioned rows first (descending) and
	// unversioned rows kept in import-date order after them.
	sort.SliceStable(versions, func(i, j int) bool {
		a, aok := imports.ParseStoreVersion(versions[i].StoreVersion)
		b, bok := imports.ParseStoreVersion(versions[j].StoreVersion)
		if aok != bok {
			return aok
		}
		if !aok {
			return false
		}
		return imports.CompareStoreVersions(a, b) > 0
	})
	for _, v := range versions {
		originalKey := ""
		if v.HasOriginal {
			originalKey = v.OriginalPackageObjectKey
		}
		view.Versions = append(view.Versions, assetVersionView{
			ID:                       v.ID,
			VersionLabel:             v.VersionLabel,
			StoreVersion:             v.StoreVersion,
			Notes:                    v.Notes,
			ImportDate:               v.ImportDate,
			PackageKind:              v.PackageKind,
			FileCount:                v.FileCount,
			SizeBytes:                v.SizeBytes,
			HasOriginal:              v.HasOriginal,
			OriginalPackageObjectKey: originalKey,
			IsCurrent:                asset.CurrentVersionID != nil && *asset.CurrentVersionID == v.ID,
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
			respond500(c, err)
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

	// Gallery images (asset-level), ordered for a stable strip.
	var images []models.AssetImage
	if err := h.db.WithContext(c.Request.Context()).
		Where("asset_id = ?", asset.ID).
		Order("sort_order, id").
		Find(&images).Error; err != nil {
		respond500(c, err)
		return
	}
	for _, img := range images {
		view.Images = append(view.Images, assetImageView{
			ID:          img.ID,
			ObjectKey:   img.ObjectKey,
			ContentType: img.ContentType,
		})
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
		respond500(c, err)
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
		respond500(c, err)
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
		respond500(c, err)
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
	assetMetadataFieldsPatch
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
		if err := req.assetMetadataFieldsPatch.apply(&asset, before, updates); err != nil {
			return errBadRequestf("%s", err)
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
	c.JSON(http.StatusOK, assetPatchResponse{
		ID:                      out.ID,
		Name:                    out.Name,
		Description:             out.Description,
		SourceURL:               out.SourceURL,
		assetMetadataFieldsView: assetMetadataFieldsViewFrom(&out),
	})
}

// assetPatchResponse echoes the asset's editable fields after a PATCH.
type assetPatchResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SourceURL   string `json:"source_url"`
	assetMetadataFieldsView
}

func (h *Handler) deleteAssetVersion(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	vid, ok := parseIDParam(c, "vid")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		var version models.AssetVersion
		if err := tx.Where("id = ? AND asset_id = ?", vid, asset.ID).First(&version).Error; err != nil {
			return err
		}

		var pins int64
		if err := tx.Model(&models.ProjectAssetPin{}).
			Where("pinned_version_id = ?", version.ID).Count(&pins).Error; err != nil {
			return err
		}
		if pins > 0 {
			return errConflictf("version is pinned by %d project(s); unpin before deleting", pins)
		}

		var total int64
		if err := tx.Model(&models.AssetVersion{}).
			Where("asset_id = ?", asset.ID).Count(&total).Error; err != nil {
			return err
		}
		if total <= 1 {
			return errConflictf("cannot delete the only version; delete the asset instead")
		}

		// Repoint before deleting so the current_version_id FK (ON DELETE SET
		// NULL) can't null out a version the asset still has.
		if asset.CurrentVersionID != nil && *asset.CurrentVersionID == version.ID {
			next, err := nextCurrentVersion(tx, asset.ID, version.ID)
			if err != nil {
				return err
			}
			if err := tx.Model(&models.Asset{}).Where("id = ?", asset.ID).
				Update("current_version_id", next).Error; err != nil {
				return err
			}
		}

		if err := tx.Delete(&models.AssetVersion{}, version.ID).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.version.delete",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff: audit.Deleted(map[string]any{
				"version_id":    version.ID,
				"store_version": version.StoreVersion,
				"version_label": version.VersionLabel,
			}),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset or version not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func nextCurrentVersion(tx *gorm.DB, assetID, excludeID uint) (*uint, error) {
	var versions []models.AssetVersion
	if err := tx.Where("asset_id = ? AND id <> ?", assetID, excludeID).Find(&versions).Error; err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, nil
	}
	sort.SliceStable(versions, func(i, j int) bool {
		a, aok := imports.ParseStoreVersion(versions[i].StoreVersion)
		b, bok := imports.ParseStoreVersion(versions[j].StoreVersion)
		if aok != bok {
			return aok
		}
		if !aok {
			return versions[i].ImportDate.After(versions[j].ImportDate)
		}
		return imports.CompareStoreVersions(a, b) > 0
	})
	return &versions[0].ID, nil
}

func versionDisplayName(v models.AssetVersion) string {
	if v.VersionLabel != "" {
		return v.VersionLabel
	}
	if v.StoreVersion != "" {
		return v.StoreVersion
	}
	return "unversioned"
}

type versionPatchRequest struct {
	VersionLabel *string `json:"version_label,omitempty"`
	StoreVersion *string `json:"store_version,omitempty"`
	Notes        *string `json:"notes,omitempty"`
}

// patchAssetVersion updates the editable fields on a version — the store
// version and notes — writing an audit row in the same transaction. An empty
// store_version clears it.
func (h *Handler) patchAssetVersion(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	vid, ok := parseIDParam(c, "vid")
	if !ok {
		return
	}
	var req versionPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}

	var normStore string
	if req.StoreVersion != nil {
		trimmed := strings.TrimSpace(*req.StoreVersion)
		if trimmed != "" {
			if _, valid := imports.ParseStoreVersion(trimmed); !valid {
				respond400(c, asError("store_version must be a dotted number like 1.1"))
				return
			}
			normStore = strings.TrimLeft(trimmed, "vV")
		}
	}

	var out models.AssetVersion
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		var version models.AssetVersion
		if err := tx.Where("id = ? AND asset_id = ?", vid, asset.ID).First(&version).Error; err != nil {
			return err
		}
		before := map[string]any{}
		updates := map[string]any{}
		if req.VersionLabel != nil {
			label := strings.TrimSpace(*req.VersionLabel)
			before["version_label"] = version.VersionLabel
			version.VersionLabel = label
			updates["version_label"] = label
		}
		if req.StoreVersion != nil {
			if normStore != "" {
				var dupes int64
				if err := tx.Model(&models.AssetVersion{}).
					Where("asset_id = ? AND store_version = ? AND id <> ?", asset.ID, normStore, version.ID).
					Count(&dupes).Error; err != nil {
					return err
				}
				if dupes > 0 {
					return errConflictf("another version already has store version %s", normStore)
				}
			}
			before["store_version"] = version.StoreVersion
			version.StoreVersion = normStore
			updates["store_version"] = normStore
		}
		if req.Notes != nil {
			before["notes"] = version.Notes
			version.Notes = *req.Notes
			updates["notes"] = version.Notes
		}
		out = version
		if len(updates) == 0 {
			return nil
		}
		if err := tx.Model(&models.AssetVersion{}).Where("id = ?", version.ID).Updates(updates).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.version.update",
			TargetType: "asset",
			TargetID:   &version.AssetID,
			Diff:       audit.Updated(before, updates),
		})
	})
	if err != nil {
		respondGORM(c, err, "version not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":            out.ID,
		"version_label": out.VersionLabel,
		"store_version": out.StoreVersion,
		"notes":         out.Notes,
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
		return deleteAssetTx(tx, lib.ID, u.ID, id)
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteAssetTx(tx *gorm.DB, libID, userID, assetID uint) error {
	var asset models.Asset
	if err := tx.Where("id = ? AND library_id = ?", assetID, libID).First(&asset).Error; err != nil {
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
	if err := tx.Where("asset_id = ?", asset.ID).Delete(&models.AssetImage{}).Error; err != nil {
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
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "asset.delete",
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff:       audit.Deleted(before),
	})
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
