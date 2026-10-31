package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/ingest"
	"github.com/thomas-elliott/damson/internal/models"
)

// triageRow is one asset in the triage queue. The queue is asset-shaped, not
// import-shaped: every asset marked needs review is in it however it got into
// the library, and Import is set only while the asset's latest import still
// wants attention (in flight, or failed and not dismissed).
type triageRow struct {
	AssetID            uint          `json:"asset_id"`
	AssetName          string        `json:"asset_name"`
	NeedsReview        bool          `json:"needs_review"`
	IsReference        bool          `json:"is_reference"`
	Description        string        `json:"description,omitempty"`
	ThumbnailObjectKey string        `json:"thumbnail_object_key,omitempty"`
	Matched            bool          `json:"matched"`
	Import             *triageImport `json:"import,omitempty"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

type triageImport struct {
	ID    uint   `json:"id"`
	Stage string `json:"stage"`
	Error string `json:"error,omitempty"`
}

type triageResponse struct {
	Assets   []triageRow `json:"assets"`
	Reviewed []triageRow `json:"reviewed"`
	Total    int64       `json:"total"`
}

type triageScanRow struct {
	AssetID            uint
	AssetName          string
	NeedsReview        bool
	IsReference        bool
	Description        string
	ThumbnailObjectKey string
	Matched            bool
	ImportID           *uint
	ImportStage        *string
	ImportError        *string
	ImportOpen         bool
	UpdatedAt          time.Time
}

func (r triageScanRow) toRow() triageRow {
	row := triageRow{
		AssetID:            r.AssetID,
		AssetName:          r.AssetName,
		NeedsReview:        r.NeedsReview,
		IsReference:        r.IsReference,
		Description:        r.Description,
		ThumbnailObjectKey: r.ThumbnailObjectKey,
		Matched:            r.Matched,
		UpdatedAt:          r.UpdatedAt,
	}
	if r.ImportOpen && r.ImportID != nil && r.ImportStage != nil {
		row.Import = &triageImport{ID: *r.ImportID, Stage: *r.ImportStage}
		if r.ImportError != nil {
			row.Import.Error = *r.ImportError
		}
	}
	return row
}

// importOpen says the asset's latest import still wants attention: it is in
// flight, or it failed and nobody has dismissed it. An earlier failure that a
// retry superseded is not the latest, so it drops out on its own.
const importOpen = "(li.id IS NOT NULL AND li.stage <> 'done' AND li.dismissed_at IS NULL)"

var triageSelect = fmt.Sprintf(
	"a.id AS asset_id, a.name AS asset_name, a.needs_review, a.is_reference, "+
		"left(a.description, %d) AS description, a.thumbnail_object_key, "+
		"COALESCE((a.source_metadata_json->>'matched')::boolean, false) AS matched, "+
		"li.id AS import_id, li.stage AS import_stage, li.error AS import_error, "+
		"%s AS import_open, GREATEST(a.updated_at, COALESCE(li.updated_at, a.updated_at)) AS updated_at",
	importDescriptionChars, importOpen,
)

// libraryTriage is the tenancy-scoped base for the triage queries: the
// library's assets, each with its latest import if it has one.
func libraryTriage(db *gorm.DB, libraryID uint) *gorm.DB {
	return db.Table("assets AS a").
		Joins("LEFT JOIN LATERAL (SELECT id, stage, error, updated_at, dismissed_at "+
			"FROM asset_imports WHERE asset_id = a.id ORDER BY id DESC LIMIT 1) li ON true").
		Where("a.library_id = ?", libraryID)
}

// getTriage returns the triage queue: every asset that needs review, plus any
// whose latest import is in flight or failed and not dismissed. Open imports
// sort first, since they are what changes while the page is up. Reviewed is
// a retained tail of assets reviewed or dismissed within the queue's window,
// read from the audit log because the asset keeps no reviewed-at.
func (h *Handler) getTriage(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	limit, offset := parsePagination(c)
	db := h.db.WithContext(c.Request.Context())

	q := libraryTriage(db, lib.ID).Where("a.needs_review OR " + importOpen)
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		respond500(c, err)
		return
	}
	var rows []triageScanRow
	if err := q.Session(&gorm.Session{}).
		Select(triageSelect).
		Order(importOpen + " DESC, updated_at DESC, a.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		respond500(c, err)
		return
	}

	var reviewed []triageScanRow
	if err := libraryTriage(db, lib.ID).
		Joins("JOIN (SELECT target_id, max(created_at) AS at FROM audit_log "+
			"WHERE library_id = ? AND target_type = 'asset' AND action_type IN ? AND created_at >= ? "+
			"GROUP BY target_id) r ON r.target_id = a.id",
			lib.ID, []string{"asset.review", "asset.import.dismiss"}, time.Now().Add(-queueFinishedWindow)).
		Where("NOT a.needs_review AND NOT " + importOpen).
		Select(triageSelect).
		Order("r.at DESC, a.id DESC").
		Limit(queueFinishedLimit).
		Scan(&reviewed).Error; err != nil {
		respond500(c, err)
		return
	}

	resp := triageResponse{Assets: make([]triageRow, len(rows)), Reviewed: make([]triageRow, len(reviewed)), Total: total}
	for i, r := range rows {
		resp.Assets[i] = r.toRow()
	}
	for i, r := range reviewed {
		resp.Reviewed[i] = r.toRow()
	}
	c.JSON(http.StatusOK, resp)
}

// postImportDismiss takes a failed import out of the triage queue. The import
// row stays for the history, and the asset is marked reviewed in the same
// step, since a needs-review asset would otherwise keep its place in the
// queue and the dismissal would look like it did nothing. Only the asset's
// latest import can be dismissed: an earlier failure is already out of the
// queue. 409 unless the import failed.
func (h *Handler) postImportDismiss(c *gin.Context) {
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

	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var imp models.AssetImport
		if err := tx.Table("asset_imports AS i").
			Select("i.*").
			Joins("JOIN assets a ON a.id = i.asset_id").
			Where("i.id = ? AND a.library_id = ?", id, lib.ID).
			Take(&imp).Error; err != nil {
			return err
		}
		if imp.Stage != ingest.StageFailed {
			return errConflictf("only a failed import can be dismissed")
		}
		if imp.DismissedAt != nil {
			return nil
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.AssetImport{}).Where("id = ?", imp.ID).
			Update("dismissed_at", now).Error; err != nil {
			return err
		}
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", imp.AssetID, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		before := map[string]any{"import_id": imp.ID, "dismissed": false, "needs_review": asset.NeedsReview}
		if asset.NeedsReview {
			if err := tx.Model(&asset).Update("needs_review", false).Error; err != nil {
				return err
			}
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.import.dismiss",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff: audit.Updated(before,
				map[string]any{"import_id": imp.ID, "dismissed": true, "needs_review": false}),
		})
	})
	if err != nil {
		respondGORM(c, err, "import not found")
		return
	}
	c.Status(http.StatusNoContent)
}
