package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/licence"
	"github.com/thomas-elliott/damson/internal/models"
)

type licenceView struct {
	ID                  uint               `json:"id"`
	AssetID             uint               `json:"asset_id"`
	DetectedType        string             `json:"detected_type"`
	DetectionConfidence float32            `json:"detection_confidence"`
	CanonicalDiff       string             `json:"canonical_diff,omitempty"`
	OriginalText        string             `json:"original_text"`
	Attributes          licence.Attributes `json:"attributes"`
	ReviewedByUserID    *uint              `json:"reviewed_by_user_id,omitempty"`
	ReviewedAt          *time.Time         `json:"reviewed_at,omitempty"`
	Status              string             `json:"status"` // derived: ok/warn/err/unk
}

// licenceViewFrom builds a view model from a DB row. The `status` field is
// derived so clients don't have to replicate the mapping.
func licenceViewFrom(l *models.Licence) licenceView {
	var attrs licence.Attributes
	if len(l.AttributesJSON) > 0 {
		_ = json.Unmarshal(l.AttributesJSON, &attrs)
	}
	return licenceView{
		ID:                  l.ID,
		AssetID:             l.AssetID,
		DetectedType:        l.DetectedType,
		DetectionConfidence: l.DetectionConfidence,
		CanonicalDiff:       l.CanonicalDiff,
		OriginalText:        l.OriginalText,
		Attributes:          attrs,
		ReviewedByUserID:    l.ReviewedByUserID,
		ReviewedAt:          l.ReviewedAt,
		Status:              licence.StatusFor(l.DetectedType, attrs),
	}
}

type putLicenceRequest struct {
	OriginalText string             `json:"original_text" binding:"required"`
	DetectedType string             `json:"detected_type,omitempty"` // override; if blank, runs Detect()
	Attributes   licence.Attributes `json:"attributes,omitempty"`
	// If UseDetected is true and DetectedType is blank, the response uses the
	// detector-derived type+attributes rather than the client's. Clients use
	// this on first save when a freshly-extracted LICENSE file is being
	// confirmed.
	UseDetected bool `json:"use_detected,omitempty"`
}

// putLicence creates or replaces an asset's licence record. Always preserves
// original_text verbatim; attributes can come from the client (override) or
// from the detector (use_detected=true, detected_type blank).
func (h *Handler) putLicence(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req putLicenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	if strings.TrimSpace(req.OriginalText) == "" {
		respond400(c, asError("original_text must not be blank"))
		return
	}

	out, err := requestedLicence(req.OriginalText, req.DetectedType, req.Attributes, req.UseDetected)
	if err != nil {
		respond500(c, err)
		return
	}
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		return setLicenceTx(tx, lib.ID, u.ID, id, &out)
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.JSON(http.StatusOK, licenceViewFrom(&out))
}

// requestedLicence builds the licence a person asked for. With no detected
// type, or with useDetected, it runs the detector over the text; useDetected
// also takes the detector's attributes over the client's.
func requestedLicence(text, detectedType string, attrs licence.Attributes, useDetected bool) (models.Licence, error) {
	l := models.Licence{DetectedType: detectedType, OriginalText: text}
	if useDetected || detectedType == "" {
		r := licence.Detect(text)
		l.DetectedType = r.DetectedType
		l.CanonicalDiff = r.CanonicalDiff
		l.DetectionConfidence = r.DetectionConfidence
		if useDetected {
			attrs = r.Attributes
		}
	}
	var err error
	l.AttributesJSON, err = json.Marshal(attrs)
	return l, err
}

// setLicenceTx is a person's licence write: it replaces the asset's licence
// with l, clearing any review, and audits the change.
func setLicenceTx(tx *gorm.DB, libID, userID, assetID uint, l *models.Licence) error {
	var asset models.Asset
	if err := tx.Select("id").Where("id = ? AND library_id = ?", assetID, libID).First(&asset).Error; err != nil {
		return err
	}
	prev, _, err := licence.Replace(tx, asset.ID, l, licence.ByPerson)
	if err != nil {
		return err
	}
	action := "asset.licence.set"
	var diff any = audit.Created(licenceViewFrom(l))
	if prev != nil {
		action = "asset.licence.update"
		diff = audit.Updated(licenceViewFrom(prev), licenceViewFrom(l))
	}
	return audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: action,
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff:       diff,
	})
}

// postLicenceReview marks the asset's current licence as reviewed.
func (h *Handler) postLicenceReview(c *gin.Context) {
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
		if asset.LicenceID == nil {
			return errBadRequestf("asset has no licence")
		}
		var l models.Licence
		if err := tx.First(&l, *asset.LicenceID).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		l.ReviewedAt = &now
		l.ReviewedByUserID = &u.ID
		if err := tx.Save(&l).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.licence.review",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Updated(nil, map[string]any{"reviewed_at": now}),
		})
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// postDetectLicence runs the detector against client-supplied text without
// persisting anything — used by the licence editor to preview detection
// before the user commits.
func (h *Handler) postDetectLicence(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	r := licence.Detect(req.Text)
	c.JSON(http.StatusOK, gin.H{
		"detected_type":        r.DetectedType,
		"detection_confidence": r.DetectionConfidence,
		"canonical_diff":       r.CanonicalDiff,
		"attributes":           r.Attributes,
		"status":               licence.StatusFor(r.DetectedType, r.Attributes),
	})
}

// pickLicenceCandidate scans the manifest for common licence filenames.
// It looks for root-level LICENSE/COPYING variants, case-insensitively.
func pickLicenceCandidate(files []models.AssetFile) *models.AssetFile {
	names := []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.LESSER", "UNLICENSE"}
	// Prefer root-level (no `/` in the remainder after the package folder).
	var best *models.AssetFile
	for i := range files {
		f := &files[i]
		base := strings.ToUpper(lastPathSegment(f.RelativePath))
		for _, n := range names {
			if strings.EqualFold(base, n) {
				if best == nil || segmentDepth(f.RelativePath) < segmentDepth(best.RelativePath) {
					best = f
				}
			}
		}
	}
	return best
}

func lastPathSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func segmentDepth(p string) int {
	return strings.Count(p, "/")
}
