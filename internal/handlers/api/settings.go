package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

type librarySettingsView struct {
	LibraryID               uint   `json:"library_id"`
	ThemeDefault            string `json:"theme_default"`
	GridDensityDefault      string `json:"grid_density_default"`
	RetainOriginalPackages  bool   `json:"retain_original_packages"`
	DedupeEnabled           bool   `json:"dedupe_enabled"`
	ThumbnailsRasterEnabled bool   `json:"thumbnails_raster_enabled"`
	ThumbnailsAudioEnabled  bool   `json:"thumbnails_audio_enabled"`
	ThumbnailsFontEnabled   bool   `json:"thumbnails_font_enabled"`
	ThumbnailsUnityPreview  bool   `json:"thumbnails_unity_preview"`
	AutoEnrichEnabled       bool   `json:"auto_enrich_enabled"`
	PreferredStore          string `json:"preferred_store"`
	AuditRetentionDays      *int   `json:"audit_retention_days"`
}

// librarySettingsResponse is what the endpoints return: the library-owned
// settings plus the server-level upload cap. Kept distinct from
// librarySettingsView, which feeds the audit diff and must therefore contain
// only fields this endpoint can change.
type librarySettingsResponse struct {
	librarySettingsView
	// MaxUploadBytes is read-only here so a member can size an upload before
	// transferring it. Writing it stays on the sysadmin route.
	MaxUploadBytes int64 `json:"max_upload_bytes"`
}

// settingsViewFrom builds the library-owned view. It is what the audit diff
// records, so it deliberately carries nothing the endpoint cannot change.
func settingsViewFrom(s *models.LibrarySettings) librarySettingsView {
	return librarySettingsView{
		LibraryID:               s.LibraryID,
		ThemeDefault:            s.ThemeDefault,
		GridDensityDefault:      s.GridDensityDefault,
		RetainOriginalPackages:  s.RetainOriginalPackages,
		DedupeEnabled:           s.DedupeEnabled,
		ThumbnailsRasterEnabled: s.ThumbnailsRasterEnabled,
		ThumbnailsAudioEnabled:  s.ThumbnailsAudioEnabled,
		ThumbnailsFontEnabled:   s.ThumbnailsFontEnabled,
		ThumbnailsUnityPreview:  s.ThumbnailsUnityPreview,
		AutoEnrichEnabled:       s.AutoEnrichEnabled,
		PreferredStore:          s.PreferredStore,
		AuditRetentionDays:      s.AuditRetentionDays,
	}
}

// getLibrarySettings returns the current library's settings, creating the
// row with defaults if it's missing (it shouldn't be after migration 005,
// but callers get a sensible response either way).
func (h *Handler) getLibrarySettings(c *gin.Context) {
	lib := getLibrary(c)
	s, err := loadOrCreateLibrarySettings(h.db.WithContext(c.Request.Context()), lib.ID)
	if err != nil {
		respond500(c, err)
		return
	}
	view, err := h.withMaxUpload(c, settingsViewFrom(s))
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// withMaxUpload layers the server-level upload cap onto a library settings
// view. Exposed to members read-only so a client can size an upload before
// transferring it; writing it stays on the sysadmin route.
func (h *Handler) withMaxUpload(c *gin.Context, v librarySettingsView) (librarySettingsResponse, error) {
	out := librarySettingsResponse{librarySettingsView: v}
	server, err := loadOrCreateServerSettings(h.db.WithContext(c.Request.Context()))
	if err != nil {
		return out, err
	}
	out.MaxUploadBytes = server.MaxUploadBytes
	return out, nil
}

type librarySettingsPatchRequest struct {
	ThemeDefault            *string `json:"theme_default,omitempty"`
	GridDensityDefault      *string `json:"grid_density_default,omitempty"`
	RetainOriginalPackages  *bool   `json:"retain_original_packages,omitempty"`
	DedupeEnabled           *bool   `json:"dedupe_enabled,omitempty"`
	ThumbnailsRasterEnabled *bool   `json:"thumbnails_raster_enabled,omitempty"`
	ThumbnailsAudioEnabled  *bool   `json:"thumbnails_audio_enabled,omitempty"`
	ThumbnailsFontEnabled   *bool   `json:"thumbnails_font_enabled,omitempty"`
	ThumbnailsUnityPreview  *bool   `json:"thumbnails_unity_preview,omitempty"`
	AutoEnrichEnabled       *bool   `json:"auto_enrich_enabled,omitempty"`
	PreferredStore          *string `json:"preferred_store,omitempty"`
	AuditRetentionDays      *int    `json:"audit_retention_days,omitempty"`
}

// patchLibrarySettings is library-admin-only. Members get 403.
func (h *Handler) patchLibrarySettings(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	var req librarySettingsPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	if req.PreferredStore != nil && !metadata.ValidPreferredStore(*req.PreferredStore) {
		respond400(c, asError("invalid preferred_store"))
		return
	}
	var out models.LibrarySettings
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		s, err := loadOrCreateLibrarySettings(tx, lib.ID)
		if err != nil {
			return err
		}
		before := settingsViewFrom(s)
		if req.ThemeDefault != nil {
			s.ThemeDefault = *req.ThemeDefault
		}
		if req.GridDensityDefault != nil {
			s.GridDensityDefault = *req.GridDensityDefault
		}
		if req.RetainOriginalPackages != nil {
			s.RetainOriginalPackages = *req.RetainOriginalPackages
		}
		if req.DedupeEnabled != nil {
			s.DedupeEnabled = *req.DedupeEnabled
		}
		if req.ThumbnailsRasterEnabled != nil {
			s.ThumbnailsRasterEnabled = *req.ThumbnailsRasterEnabled
		}
		if req.ThumbnailsAudioEnabled != nil {
			s.ThumbnailsAudioEnabled = *req.ThumbnailsAudioEnabled
		}
		if req.ThumbnailsFontEnabled != nil {
			s.ThumbnailsFontEnabled = *req.ThumbnailsFontEnabled
		}
		if req.ThumbnailsUnityPreview != nil {
			s.ThumbnailsUnityPreview = *req.ThumbnailsUnityPreview
		}
		if req.AutoEnrichEnabled != nil {
			s.AutoEnrichEnabled = *req.AutoEnrichEnabled
		}
		if req.PreferredStore != nil {
			s.PreferredStore = *req.PreferredStore
		}
		if req.AuditRetentionDays != nil {
			s.AuditRetentionDays = req.AuditRetentionDays
		}
		s.UpdatedByUserID = &u.ID
		if err := tx.Save(s).Error; err != nil {
			return err
		}
		out = *s
		after := settingsViewFrom(s)
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "library.settings.update",
			TargetType: "library",
			TargetID:   &lib.ID,
			Diff:       audit.Updated(before, after),
		})
	})
	if err != nil {
		respond500(c, err)
		return
	}
	view, err := h.withMaxUpload(c, settingsViewFrom(&out))
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func loadOrCreateLibrarySettings(tx *gorm.DB, libID uint) (*models.LibrarySettings, error) {
	var s models.LibrarySettings
	err := tx.Where("library_id = ?", libID).First(&s).Error
	if err == nil {
		return &s, nil
	}
	if !isNotFound(err) {
		return nil, err
	}
	// Create with defaults.
	s = models.LibrarySettings{
		LibraryID:               libID,
		ThemeDefault:            "dark",
		GridDensityDefault:      "medium",
		RetainOriginalPackages:  true,
		DedupeEnabled:           true,
		ThumbnailsRasterEnabled: true,
		ThumbnailsAudioEnabled:  true,
		ThumbnailsFontEnabled:   true,
		ThumbnailsUnityPreview:  true,
		PreviewsEnabled:         true,
		AutoEnrichEnabled:       true,
		PreferredStore:          metadata.StoreUnity,
	}
	if err := tx.Create(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}
