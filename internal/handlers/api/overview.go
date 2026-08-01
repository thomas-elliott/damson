package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
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

	assets := func() *gorm.DB {
		return db.Model(&models.Asset{}).Where("library_id = ?", lib.ID)
	}
	imports := func() *gorm.DB {
		return db.Table("asset_imports AS i").
			Joins("JOIN assets a ON a.id = i.asset_id").
			Where("a.library_id = ?", lib.ID)
	}

	for _, count := range []struct {
		into  *int64
		query func() *gorm.DB
	}{
		{&resp.Counts.Assets, assets},
		{&resp.Counts.NeedsReview, func() *gorm.DB { return assets().Where("needs_review") }},
		{&resp.Counts.UpdatesAvailable, func() *gorm.DB {
			return db.Table("assets AS a").Where("a.library_id = ?", lib.ID).Where(updatesAvailableSQL())
		}},
		{&resp.Counts.Collections, func() *gorm.DB {
			return db.Model(&models.Collection{}).Where("library_id = ?", lib.ID)
		}},
		{&resp.Counts.Tags, func() *gorm.DB {
			return db.Model(&models.Tag{}).Where("library_id = ?", lib.ID)
		}},
		{&resp.Counts.Projects, func() *gorm.DB {
			return db.Model(&models.Project{}).Where("library_id = ?", lib.ID)
		}},
		{&resp.Counts.ImportsInFlight, func() *gorm.DB {
			return imports().Where("i.stage NOT IN (?, ?)", "done", "failed")
		}},
		{&resp.Counts.ImportsFailed, func() *gorm.DB {
			return imports().Where("i.stage = ?", "failed")
		}},
	} {
		if err := count.query().Count(count.into).Error; err != nil {
			respond500(c, err)
			return
		}
	}

	if err := imports().
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
