package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
)

type projectView struct {
	ID                 uint      `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	TargetDistribution string    `json:"target_distribution"`
	Glyph              string    `json:"glyph"`
	GlyphColor         string    `json:"glyph_color"`
	AssetCount         int       `json:"asset_count"`
	UpdatesAvailable   int       `json:"updates_available"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// getProjects lists projects in the library with pin count + updates-available
// count (pins whose pinned_version_id is not the asset's current_version_id).
func (h *Handler) getProjects(c *gin.Context) {
	lib := getLibrary(c)
	type row struct {
		ID                 uint
		Name               string
		Description        string
		TargetDistribution string
		Glyph              string
		GlyphColor         string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		AssetCount         int
		UpdatesAvailable   int
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("projects AS p").
		Select(`p.id, p.name, p.description, p.target_distribution, p.glyph, p.glyph_color, p.created_at, p.updated_at,
			COALESCE(COUNT(pin.id), 0) AS asset_count,
			COALESCE(SUM(CASE WHEN a.current_version_id IS NOT NULL AND a.current_version_id <> pin.pinned_version_id THEN 1 ELSE 0 END), 0) AS updates_available`).
		Joins("LEFT JOIN project_asset_pins pin ON pin.project_id = p.id").
		Joins("LEFT JOIN assets a ON a.id = pin.asset_id").
		Where("p.library_id = ?", lib.ID).
		Group("p.id").
		Order("p.name").
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
		return
	}
	out := make([]projectView, len(rows))
	for i, r := range rows {
		out[i] = projectView{
			ID:                 r.ID,
			Name:               r.Name,
			Description:        r.Description,
			TargetDistribution: r.TargetDistribution,
			Glyph:              r.Glyph,
			GlyphColor:         r.GlyphColor,
			AssetCount:         r.AssetCount,
			UpdatesAvailable:   r.UpdatesAvailable,
			CreatedAt:          r.CreatedAt,
			UpdatedAt:          r.UpdatedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}

type projectWriteRequest struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	TargetDistribution string `json:"target_distribution,omitempty"`
	Glyph              string `json:"glyph,omitempty"`
	GlyphColor         string `json:"glyph_color,omitempty"`
}

var allowedDistributions = map[string]bool{
	"commercial":  true,
	"open-source": true,
	"personal":    true,
	"unspecified": true,
}

// postProject creates a new project in the library.
func (h *Handler) postProject(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	var req projectWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		respond400(c, asError("name required"))
		return
	}
	dist := req.TargetDistribution
	if dist == "" {
		dist = "unspecified"
	}
	if !allowedDistributions[dist] {
		respond400(c, asError("invalid target_distribution"))
		return
	}
	p := models.Project{
		LibraryID:          lib.ID,
		Name:               name,
		Description:        req.Description,
		TargetDistribution: dist,
		Glyph:              req.Glyph,
		GlyphColor:         req.GlyphColor,
		CreatedByUserID:    &u.ID,
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var existing models.Project
		if err := tx.Where("library_id = ? AND LOWER(name) = LOWER(?)", lib.ID, name).First(&existing).Error; err == nil {
			return errConflictf("project name already exists")
		} else if !isNotFound(err) {
			return err
		}
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "project.create",
			TargetType: "project",
			TargetID:   &p.ID,
			Diff:       audit.Created(map[string]any{"id": p.ID, "name": p.Name, "target_distribution": p.TargetDistribution}),
		})
	})
	if err != nil {
		respondGORM(c, err, "project not found")
		return
	}
	c.JSON(http.StatusCreated, projectView{
		ID:                 p.ID,
		Name:               p.Name,
		Description:        p.Description,
		TargetDistribution: p.TargetDistribution,
		Glyph:              p.Glyph,
		GlyphColor:         p.GlyphColor,
		AssetCount:         0,
		CreatedAt:          p.CreatedAt,
		UpdatedAt:          p.UpdatedAt,
	})
}

type projectDetailView struct {
	projectView
	Pins []projectPinDetail `json:"pins"`
}

type projectPinDetail struct {
	PinID           uint      `json:"pin_id"`
	AssetID         uint      `json:"asset_id"`
	AssetName       string    `json:"asset_name"`
	PinnedVersionID *uint     `json:"pinned_version_id"`
	VersionLabel    string    `json:"version_label"`
	IsReference     bool      `json:"is_reference,omitempty"`
	LatestVersionID *uint     `json:"latest_version_id,omitempty"`
	LatestLabel     string    `json:"latest_label,omitempty"`
	HasUpdate       bool      `json:"has_update"`
	LicenceStatus   string    `json:"licence_status"`
	LicenceSPDX     string    `json:"licence_spdx,omitempty"`
	SizeBytes       int64     `json:"size_bytes"`
	PinnedAt        time.Time `json:"pinned_at"`
}

func (h *Handler) getProject(c *gin.Context) {
	lib := getLibrary(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var p models.Project
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).
		First(&p).Error; err != nil {
		if isNotFound(err) {
			respond404(c, "project not found")
			return
		}
		respond500(c, err)
		return
	}

	type pinRow struct {
		PinID           uint
		AssetID         uint
		AssetName       string
		PinnedVersionID *uint
		VersionLabel    string
		IsReference     bool
		CurrentVersion  *uint
		CurrentLabel    string
		LicenceType     string
		LicenceStatus   string
		SizeBytes       int64
		PinnedAt        time.Time
	}
	var pins []pinRow
	err := h.db.WithContext(c.Request.Context()).
		Table("project_asset_pins AS pin").
		Select(`pin.id AS pin_id, pin.asset_id, a.name AS asset_name,
			pin.pinned_version_id, COALESCE(v.version_label, '') AS version_label, a.is_reference,
			a.current_version_id AS current_version,
			cv.version_label AS current_label,
			l.detected_type AS licence_type,
			`+licenceStatusSQL("l")+` AS licence_status,
			COALESCE((SELECT SUM(f.size_bytes) FROM asset_files f WHERE f.asset_version_id = pin.pinned_version_id), 0) AS size_bytes,
			pin.pinned_at`, permissiveLicences).
		Joins("JOIN assets a ON a.id = pin.asset_id").
		Joins("LEFT JOIN asset_versions v ON v.id = pin.pinned_version_id").
		Joins("LEFT JOIN asset_versions cv ON cv.id = a.current_version_id").
		Joins("LEFT JOIN licences l ON l.id = a.licence_id").
		Where("pin.project_id = ?", p.ID).
		Order("a.name").
		Scan(&pins).Error
	if err != nil {
		respond500(c, err)
		return
	}
	out := projectDetailView{
		projectView: projectView{
			ID:                 p.ID,
			Name:               p.Name,
			Description:        p.Description,
			TargetDistribution: p.TargetDistribution,
			Glyph:              p.Glyph,
			GlyphColor:         p.GlyphColor,
			CreatedAt:          p.CreatedAt,
			UpdatedAt:          p.UpdatedAt,
			AssetCount:         len(pins),
		},
		Pins: make([]projectPinDetail, 0, len(pins)),
	}
	for _, r := range pins {
		detail := projectPinDetail{
			PinID:           r.PinID,
			AssetID:         r.AssetID,
			AssetName:       r.AssetName,
			PinnedVersionID: r.PinnedVersionID,
			VersionLabel:    r.VersionLabel,
			IsReference:     r.IsReference,
			LatestVersionID: r.CurrentVersion,
			LatestLabel:     r.CurrentLabel,
			HasUpdate: r.CurrentVersion != nil && r.PinnedVersionID != nil &&
				*r.CurrentVersion != *r.PinnedVersionID,
			LicenceSPDX:   r.LicenceType,
			LicenceStatus: r.LicenceStatus,
			SizeBytes:     r.SizeBytes,
			PinnedAt:      r.PinnedAt,
		}
		if detail.HasUpdate {
			out.UpdatesAvailable++
		}
		out.Pins = append(out.Pins, detail)
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) patchProject(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req projectWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	var out models.Project
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var p models.Project
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&p).Error; err != nil {
			return err
		}
		before := map[string]any{}
		updates := map[string]any{}
		if name := strings.TrimSpace(req.Name); name != "" {
			before["name"] = p.Name
			p.Name = name
			updates["name"] = name
		}
		if req.Description != "" {
			before["description"] = p.Description
			p.Description = req.Description
			updates["description"] = req.Description
		}
		if req.TargetDistribution != "" {
			if !allowedDistributions[req.TargetDistribution] {
				return errBadRequestf("invalid target_distribution")
			}
			before["target_distribution"] = p.TargetDistribution
			p.TargetDistribution = req.TargetDistribution
			updates["target_distribution"] = req.TargetDistribution
		}
		if req.Glyph != "" {
			before["glyph"] = p.Glyph
			p.Glyph = req.Glyph
			updates["glyph"] = req.Glyph
		}
		if req.GlyphColor != "" {
			before["glyph_color"] = p.GlyphColor
			p.GlyphColor = req.GlyphColor
			updates["glyph_color"] = req.GlyphColor
		}
		if len(updates) > 0 {
			if err := tx.Model(&p).Updates(updates).Error; err != nil {
				return err
			}
		}
		out = p
		if len(updates) == 0 {
			return nil
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "project.update",
			TargetType: "project",
			TargetID:   &p.ID,
			Diff:       audit.Updated(before, updates),
		})
	})
	if err != nil {
		respondGORM(c, err, "project not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                  out.ID,
		"name":                out.Name,
		"description":         out.Description,
		"target_distribution": out.TargetDistribution,
		"glyph":               out.Glyph,
		"glyph_color":         out.GlyphColor,
	})
}

func (h *Handler) deleteProject(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var p models.Project
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&p).Error; err != nil {
			return err
		}
		// Explicit pin cleanup — SQLite test path doesn't honour the FK
		// cascade that Postgres enforces, so belt-and-braces delete pins
		// first. Idempotent in both engines.
		if err := tx.Where("project_id = ?", p.ID).Delete(&models.ProjectAssetPin{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&p).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "project.delete",
			TargetType: "project",
			TargetID:   &p.ID,
			Diff:       audit.Deleted(map[string]any{"id": p.ID, "name": p.Name}),
		})
	})
	if err != nil {
		respondGORM(c, err, "project not found")
		return
	}
	c.Status(http.StatusNoContent)
}

type pinRequest struct {
	AssetID   uint  `json:"asset_id" binding:"required"`
	VersionID *uint `json:"version_id,omitempty"`
}

// postProjectPin pins an asset-version into a project. If VersionID is
// omitted, pins the asset's current_version_id. Idempotent: re-pinning the
// same (project, asset) updates the pinned_version_id (that's the "upgrade"
// path).
func (h *Handler) postProjectPin(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	projID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req pinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	var pin models.ProjectAssetPin
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var p models.Project
		if err := tx.Where("id = ? AND library_id = ?", projID, lib.ID).First(&p).Error; err != nil {
			return err
		}
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", req.AssetID, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		if asset.IsReference {
			if req.VersionID != nil {
				return errBadRequestf("a reference asset has no versions to pin")
			}
			return pinReferenceTx(tx, lib.ID, u.ID, &p, &asset, &pin)
		}
		versionID := req.VersionID
		if versionID == nil {
			if asset.CurrentVersionID == nil {
				return errBadRequestf("asset has no current version")
			}
			versionID = asset.CurrentVersionID
		}
		// Verify the version belongs to the asset.
		var v models.AssetVersion
		if err := tx.Where("id = ? AND asset_id = ?", *versionID, asset.ID).First(&v).Error; err != nil {
			return errBadRequestf("version does not belong to asset")
		}
		// Existing pin?
		var existing models.ProjectAssetPin
		err := tx.Where("project_id = ? AND asset_id = ?", p.ID, asset.ID).First(&existing).Error
		if err != nil && !isNotFound(err) {
			return err
		}
		if isNotFound(err) {
			pin = models.ProjectAssetPin{
				ProjectID:       p.ID,
				AssetID:         asset.ID,
				PinnedVersionID: versionID,
				PinnedAt:        time.Now().UTC(),
				PinnedByUserID:  &u.ID,
			}
			if err := tx.Create(&pin).Error; err != nil {
				return err
			}
			return audit.Write(tx, audit.Entry{
				LibraryID:  &lib.ID,
				UserID:     &u.ID,
				ActionType: "project.pin",
				TargetType: "project",
				TargetID:   &p.ID,
				Diff:       audit.Created(map[string]any{"asset_id": asset.ID, "version_id": *versionID}),
			})
		}
		pin = existing
		if pin.PinnedVersionID != nil && *pin.PinnedVersionID == *versionID {
			return nil // idempotent no-op
		}
		prev := pin.PinnedVersionID
		pin.PinnedVersionID = versionID
		pin.PinnedAt = time.Now().UTC()
		pin.PinnedByUserID = &u.ID
		if err := tx.Save(&pin).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "project.pin.upgrade",
			TargetType: "project",
			TargetID:   &p.ID,
			Diff: audit.Updated(
				map[string]any{"asset_id": asset.ID, "version_id": prev},
				map[string]any{"asset_id": asset.ID, "version_id": *versionID}),
		})
	})
	if err != nil {
		respondGORM(c, err, "project or asset not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pin_id":            pin.ID,
		"project_id":        pin.ProjectID,
		"asset_id":          pin.AssetID,
		"pinned_version_id": pin.PinnedVersionID,
	})
}

// pinReferenceTx pins a reference asset, which has no versions, so the pin
// carries none. Pinning it again is a no-op.
func pinReferenceTx(tx *gorm.DB, libID, userID uint, p *models.Project, asset *models.Asset, pin *models.ProjectAssetPin) error {
	err := tx.Where("project_id = ? AND asset_id = ?", p.ID, asset.ID).First(pin).Error
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		return err
	}
	*pin = models.ProjectAssetPin{
		ProjectID:      p.ID,
		AssetID:        asset.ID,
		PinnedAt:       time.Now().UTC(),
		PinnedByUserID: &userID,
	}
	if err := tx.Create(pin).Error; err != nil {
		return err
	}
	return audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "project.pin",
		TargetType: "project",
		TargetID:   &p.ID,
		Diff:       audit.Created(map[string]any{"asset_id": asset.ID, "version_id": nil}),
	})
}

func (h *Handler) deleteProjectPin(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	projID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	pinID, ok := parseIDParam(c, "pinId")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var p models.Project
		if err := tx.Where("id = ? AND library_id = ?", projID, lib.ID).First(&p).Error; err != nil {
			return err
		}
		var pin models.ProjectAssetPin
		if err := tx.Where("id = ? AND project_id = ?", pinID, p.ID).First(&pin).Error; err != nil {
			return err
		}
		if err := tx.Delete(&pin).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "project.unpin",
			TargetType: "project",
			TargetID:   &p.ID,
			Diff:       audit.Deleted(map[string]any{"asset_id": pin.AssetID, "version_id": pin.PinnedVersionID}),
		})
	})
	if err != nil {
		respondGORM(c, err, "project or pin not found")
		return
	}
	c.Status(http.StatusNoContent)
}
