package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
)

type adminUserView struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// getAdminUsers lists every user on the server. Sysadmin only — sensitive
// surface that shouldn't be exposed to library members.
func (h *Handler) getAdminUsers(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	var users []models.User
	if err := h.db.WithContext(c.Request.Context()).
		Order("display_name").
		Find(&users).Error; err != nil {
		respond500(c)
		return
	}
	out := make([]adminUserView, len(users))
	for i, u := range users {
		out[i] = adminUserView{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
		}
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

type patchUserRequest struct {
	Role     *string `json:"role,omitempty"`
	Password *string `json:"password,omitempty"` // local mode only; resets the user's credential
}

// patchAdminUser changes a user's role and/or resets their password
// (local-mode only). Refuses to demote the last sysadmin so a server
// can't end up sysadmin-less.
func (h *Handler) patchAdminUser(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req patchUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	var u models.User
	err := h.db.WithContext(c.Request.Context()).First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "user not found")
		return
	}
	if err != nil {
		respond500(c)
		return
	}

	actor := auth.GetUser(c)
	beforeRole := u.Role

	if req.Role != nil {
		role := strings.ToLower(strings.TrimSpace(*req.Role))
		if role != "user" && role != "sysadmin" {
			respond400(c, asError("role must be 'user' or 'sysadmin'"))
			return
		}
		if u.Role == "sysadmin" && role != "sysadmin" {
			var sysCount int64
			if err := h.db.WithContext(c.Request.Context()).
				Model(&models.User{}).
				Where("role = ?", "sysadmin").
				Count(&sysCount).Error; err != nil {
				respond500(c)
				return
			}
			if sysCount <= 1 {
				respond409(c, "cannot demote the last sysadmin")
				return
			}
		}
		u.Role = role
	}

	if req.Password != nil {
		if u.PasswordHash == nil {
			respond400(c, asError("user has no local password to reset"))
			return
		}
		if err := auth.SetLocalUserPassword(c.Request.Context(), h.db, u.ID, *req.Password); err != nil {
			respond400(c, err)
			return
		}
		// Reload to pick up the freshly-rotated TokensValidAfter.
		if err := h.db.WithContext(c.Request.Context()).First(&u, id).Error; err != nil {
			respond500(c)
			return
		}
	}

	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if req.Role != nil {
			if err := tx.Save(&u).Error; err != nil {
				return err
			}
		}
		var actorID *uint
		if actor != nil {
			actorID = &actor.ID
		}
		diff := map[string]any{}
		if req.Role != nil {
			diff = audit.Updated(map[string]any{"role": beforeRole}, map[string]any{"role": u.Role})
		}
		if req.Password != nil {
			if existing, ok := diff["after"].(map[string]any); ok {
				existing["password_reset"] = true
			} else {
				diff = map[string]any{"password_reset": true}
			}
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  nil,
			UserID:     actorID,
			ActionType: "admin.user.update",
			TargetType: "user",
			TargetID:   &u.ID,
			Diff:       diff,
		})
	})
	if err != nil {
		respond500(c)
		return
	}
	c.JSON(http.StatusOK, adminUserView{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
	})
}

type serverSettingsView struct {
	MaxUploadBytes      int64 `json:"max_upload_bytes"`
	LicenceKBAutoUpdate bool  `json:"licence_kb_auto_update"`
}

func (h *Handler) getServerSettings(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	s, err := loadOrCreateServerSettings(h.db.WithContext(c.Request.Context()))
	if err != nil {
		respond500(c)
		return
	}
	c.JSON(http.StatusOK, serverSettingsView{
		MaxUploadBytes:      s.MaxUploadBytes,
		LicenceKBAutoUpdate: s.LicenceKBAutoUpdate,
	})
}

type serverSettingsPatchRequest struct {
	MaxUploadBytes      *int64 `json:"max_upload_bytes,omitempty"`
	LicenceKBAutoUpdate *bool  `json:"licence_kb_auto_update,omitempty"`
}

func (h *Handler) patchServerSettings(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	var req serverSettingsPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	actor := auth.GetUser(c)
	var out models.ServerSettings
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		s, err := loadOrCreateServerSettings(tx)
		if err != nil {
			return err
		}
		before := serverSettingsView{
			MaxUploadBytes:      s.MaxUploadBytes,
			LicenceKBAutoUpdate: s.LicenceKBAutoUpdate,
		}
		if req.MaxUploadBytes != nil {
			s.MaxUploadBytes = *req.MaxUploadBytes
		}
		if req.LicenceKBAutoUpdate != nil {
			s.LicenceKBAutoUpdate = *req.LicenceKBAutoUpdate
		}
		var actorID *uint
		if actor != nil {
			s.UpdatedByUserID = &actor.ID
			actorID = &actor.ID
		}
		if err := tx.Save(s).Error; err != nil {
			return err
		}
		out = *s
		after := serverSettingsView{
			MaxUploadBytes:      s.MaxUploadBytes,
			LicenceKBAutoUpdate: s.LicenceKBAutoUpdate,
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  nil,
			UserID:     actorID,
			ActionType: "admin.server_settings.update",
			TargetType: "server",
			Diff:       audit.Updated(before, after),
		})
	})
	if err != nil {
		respond500(c)
		return
	}
	c.JSON(http.StatusOK, serverSettingsView{
		MaxUploadBytes:      out.MaxUploadBytes,
		LicenceKBAutoUpdate: out.LicenceKBAutoUpdate,
	})
}

// loadOrCreateServerSettings returns the singleton server-settings row,
// creating it with defaults if missing. Same loadOrCreate pattern as the
// per-library variant in settings.go.
func loadOrCreateServerSettings(tx *gorm.DB) (*models.ServerSettings, error) {
	var s models.ServerSettings
	err := tx.First(&s).Error
	if err == nil {
		return &s, nil
	}
	if !isNotFound(err) {
		return nil, err
	}
	s = models.ServerSettings{
		ID:                  1,
		MaxUploadBytes:      2147483648, // 2 GiB
		LicenceKBAutoUpdate: false,
	}
	if err := tx.Create(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}
