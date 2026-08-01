package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// overviewRecentImports caps the recent-import tail. Small on purpose: this
// endpoint runs on every agent session start.
const overviewRecentImports = 5

type overviewCounts struct {
	Assets           int64 `json:"assets"`
	NeedsReview      int64 `json:"needs_review"`
	UpdatesAvailable int64 `json:"updates_available"`
	Collections      int64 `json:"collections"`
	Tags             int64 `json:"tags"`
	Projects         int64 `json:"projects"`
	ImportsInFlight  int64 `json:"imports_in_flight"`
	ImportsFailed    int64 `json:"imports_failed"`
}

type overviewImport struct {
	ID        uint      `json:"id"`
	AssetID   uint      `json:"asset_id"`
	AssetName string    `json:"asset_name"`
	Stage     string    `json:"stage"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type overviewLibrary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type overviewResponse struct {
	Library       overviewLibrary  `json:"library"`
	Counts        overviewCounts   `json:"counts"`
	RecentImports []overviewImport `json:"recent_imports"`
}

// getOverview collapses the home view into one round trip. Composing it from
// the existing list routes would be five requests, three of which transfer
// payloads only to discard them.
func (h *Handler) getOverview(c *gin.Context) {
	lib := getLibrary(c)
	mem := getMembership(c)
	if lib == nil || mem == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	db := h.db.WithContext(c.Request.Context())

	resp := overviewResponse{
		Library:       overviewLibrary{ID: lib.ID, Name: lib.Name, Role: mem.Role},
		RecentImports: []overviewImport{},
	}

	// Eight counts as eight round trips would be eight scans; aggregated with
	// FILTER, each table is visited once. updates_available matters most — it
	// cannot be indexed, so it rides on the scan assets already needs.
	var assetCounts struct {
		Assets           int64
		NeedsReview      int64
		UpdatesAvailable int64
		Collections      int64
		Tags             int64
		Projects         int64
	}
	err := db.Table("assets AS a").
		Select(`count(*) AS assets,
			count(*) FILTER (WHERE a.needs_review) AS needs_review,
			count(*) FILTER (WHERE `+updatesAvailableSQL()+`) AS updates_available,
			(SELECT count(*) FROM collections WHERE library_id = ?) AS collections,
			(SELECT count(*) FROM tags WHERE library_id = ?) AS tags,
			(SELECT count(*) FROM projects WHERE library_id = ?) AS projects`,
			lib.ID, lib.ID, lib.ID).
		Where("a.library_id = ?", lib.ID).
		Scan(&assetCounts).Error
	if err != nil {
		respond500(c, err)
		return
	}

	var importCounts struct {
		ImportsInFlight int64
		ImportsFailed   int64
	}
	err = libraryImports(db, lib.ID).
		Select(`count(*) FILTER (WHERE i.stage NOT IN ('done', 'failed')) AS imports_in_flight,
			count(*) FILTER (WHERE i.stage = 'failed') AS imports_failed`).
		Scan(&importCounts).Error
	if err != nil {
		respond500(c, err)
		return
	}

	resp.Counts = overviewCounts{
		Assets:           assetCounts.Assets,
		NeedsReview:      assetCounts.NeedsReview,
		UpdatesAvailable: assetCounts.UpdatesAvailable,
		Collections:      assetCounts.Collections,
		Tags:             assetCounts.Tags,
		Projects:         assetCounts.Projects,
		ImportsInFlight:  importCounts.ImportsInFlight,
		ImportsFailed:    importCounts.ImportsFailed,
	}

	if err := libraryImports(db, lib.ID).
		Select("i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at").
		Order("i.updated_at DESC").
		Limit(overviewRecentImports).
		Scan(&resp.RecentImports).Error; err != nil {
		respond500(c, err)
		return
	}
	if resp.RecentImports == nil {
		resp.RecentImports = []overviewImport{}
	}

	c.JSON(http.StatusOK, resp)
}
