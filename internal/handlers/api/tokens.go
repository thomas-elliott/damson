package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
)

const (
	maxTokenLabelLen       = 100
	maxActiveTokensPerUser = 25
)

type apiTokenView struct {
	ID          uint       `json:"id"`
	Label       string     `json:"label"`
	Prefix      string     `json:"prefix"`
	Scope       string     `json:"scope"`
	LibraryID   uint       `json:"library_id"`
	LibraryName string     `json:"library_name,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

type apiTokenListResponse struct {
	Tokens []apiTokenView `json:"tokens"`
}

// createTokenResponse carries the secret. It is the only response that ever
// does, and the value is not recoverable afterwards.
type createTokenResponse struct {
	apiTokenView
	Token string `json:"token"`
}

func (h *Handler) getAPITokens(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type row struct {
		models.APIToken
		LibraryName string
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("api_tokens AS t").
		Select("t.*, l.name AS library_name").
		Joins("LEFT JOIN libraries l ON l.id = t.library_id").
		Where("t.user_id = ?", u.ID).
		Order("t.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
		return
	}

	out := make([]apiTokenView, len(rows))
	for i, r := range rows {
		out[i] = tokenView(r.APIToken, r.LibraryName)
	}
	c.JSON(http.StatusOK, apiTokenListResponse{Tokens: out})
}

type createTokenRequest struct {
	Label     string     `json:"label"`
	Scope     string     `json:"scope"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (h *Handler) postAPIToken(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		respond400(c, asError("label is required"))
		return
	}
	if len(label) > maxTokenLabelLen {
		respond400(c, asError("label is too long"))
		return
	}
	scope, err := auth.NormaliseScope(req.Scope)
	if err != nil {
		respond400(c, err)
		return
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now().UTC()) {
		respond400(c, asError("expires_at must be in the future"))
		return
	}
	if u.CurrentLibraryID == nil {
		respondForbidden(c, "no current library")
		return
	}
	libraryID := *u.CurrentLibraryID

	var active int64
	if err := h.db.WithContext(c.Request.Context()).
		Model(&models.APIToken{}).
		Where("user_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())", u.ID).
		Count(&active).Error; err != nil {
		respond500(c, err)
		return
	}
	if active >= maxActiveTokensPerUser {
		respond409(c, "token limit reached — revoke an existing token first")
		return
	}

	var lib models.Library
	err = h.db.WithContext(c.Request.Context()).
		Table("library_memberships AS m").
		Select("l.id, l.name").
		Joins("JOIN libraries l ON l.id = m.library_id").
		Where("m.library_id = ? AND m.user_id = ?", libraryID, u.ID).
		Take(&lib).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondForbidden(c, "not a member of the current library")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}

	secret, hash, prefix, err := auth.GenerateToken()
	if err != nil {
		respond500(c, err)
		return
	}

	tok := models.APIToken{
		UserID:    u.ID,
		LibraryID: libraryID,
		Label:     label,
		TokenHash: hash,
		Prefix:    prefix,
		Scope:     scope,
		ExpiresAt: req.ExpiresAt,
	}
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&tok).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &libraryID,
			UserID:     &u.ID,
			ActionType: "token.create",
			TargetType: "api_token",
			TargetID:   &tok.ID,
			Diff: audit.Created(map[string]any{
				"label":      label,
				"scope":      scope,
				"prefix":     prefix,
				"library_id": libraryID,
			}),
		})
	})
	if err != nil {
		respond500(c, err)
		return
	}

	c.JSON(http.StatusCreated, createTokenResponse{
		apiTokenView: tokenView(tok, lib.Name),
		Token:        secret,
	})
}

// deleteAPIToken revokes rather than deletes, so the audit trail keeps a
// target to point at. Already-revoked tokens return 204 unchanged.
func (h *Handler) deleteAPIToken(c *gin.Context) {
	u := auth.GetUser(c)
	if u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var tok models.APIToken
	err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND user_id = ?", id, u.ID).
		Take(&tok).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "token not found")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}
	if tok.RevokedAt != nil {
		c.Status(http.StatusNoContent)
		return
	}

	now := time.Now().UTC()
	libraryID := tok.LibraryID
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.APIToken{}).
			Where("id = ?", tok.ID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &libraryID,
			UserID:     &u.ID,
			ActionType: "token.revoke",
			TargetType: "api_token",
			TargetID:   &tok.ID,
			Diff: audit.Deleted(map[string]any{
				"label":  tok.Label,
				"scope":  tok.Scope,
				"prefix": tok.Prefix,
			}),
		})
	})
	if err != nil {
		respond500(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func tokenView(t models.APIToken, libraryName string) apiTokenView {
	return apiTokenView{
		ID:          t.ID,
		Label:       t.Label,
		Prefix:      t.Prefix,
		Scope:       t.Scope,
		LibraryID:   t.LibraryID,
		LibraryName: libraryName,
		CreatedAt:   t.CreatedAt,
		LastUsedAt:  t.LastUsedAt,
		ExpiresAt:   t.ExpiresAt,
		RevokedAt:   t.RevokedAt,
	}
}
