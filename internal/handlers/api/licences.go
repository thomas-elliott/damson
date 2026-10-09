package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/licence"
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
		Status:              LicenceStatusFor(l.DetectedType, attrs),
	}
}

// LicenceStatusFor maps an SPDX/custom type + attributes to the UI pill's
// semantic status. Kept in the handler package because it's the one place
// where business semantics for the badge live; the design-system owns only
// the visual tone mapping.
func LicenceStatusFor(detectedType string, attrs licence.Attributes) string {
	if detectedType == "" || detectedType == "unknown" {
		return "unk"
	}
	// Commercial-use blockers flag as `err`; anything with all affirmative
	// common-use attributes is `ok`; everything else is `warn`.
	if attrs.CommercialUse != nil && !*attrs.CommercialUse {
		return "err"
	}
	if slices.Contains(permissiveLicences, detectedType) {
		return "ok"
	}
	return "warn"
}

// permissiveLicences are the types LicenceStatusFor reports as ok unless
// their attributes bar commercial use.
var permissiveLicences = []string{
	"MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "Zlib", "ISC",
	"CC-BY-4.0", "CC-BY-3.0", "CC0-1.0", "OFL-1.1", "Unlicense",
}

// licenceStatusSQL is LicenceStatusFor as SQL over a licences row aliased
// alias (all NULL when the asset has no licence), so the grid, a project's
// pins and the licence-status filter compute the status inside the query.
// Its one placeholder takes permissiveLicences. Keep it in step with
// LicenceStatusFor; TestLicenceStatus_AgreesAcrossGridProjectFilterAndDetail
// checks the two agree.
func licenceStatusSQL(alias string) string {
	return `CASE
		WHEN COALESCE(` + alias + `.detected_type, '') IN ('', 'unknown') THEN 'unk'
		WHEN ` + alias + `.attributes_json->>'commercial_use' = 'false' THEN 'err'
		WHEN ` + alias + `.detected_type IN ? THEN 'ok'
		ELSE 'warn' END`
}

// getLicence returns the current licence row for an asset, or 404.
func (h *Handler) getLicence(c *gin.Context) {
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
	if asset.LicenceID == nil {
		respond404(c, "no licence")
		return
	}
	var l models.Licence
	if err := h.db.WithContext(c.Request.Context()).First(&l, *asset.LicenceID).Error; err != nil {
		if isNotFound(err) {
			respond404(c, "no licence")
			return
		}
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, licenceViewFrom(&l))
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

	// If the client asked for detection, run it *outside* the transaction —
	// the detector has no DB dependencies and is deterministic.
	detectedType := req.DetectedType
	attrs := req.Attributes
	canonicalDiff := ""
	var confidence float32
	if req.UseDetected || detectedType == "" {
		r := licence.Detect(req.OriginalText)
		detectedType = r.DetectedType
		canonicalDiff = r.CanonicalDiff
		confidence = r.DetectionConfidence
		if req.UseDetected {
			attrs = r.Attributes
		}
	}

	var out models.Licence
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		var before *licenceView
		if asset.LicenceID != nil {
			var existing models.Licence
			if err := tx.First(&existing, *asset.LicenceID).Error; err == nil {
				v := licenceViewFrom(&existing)
				before = &v
				if err := tx.Delete(&existing).Error; err != nil {
					return err
				}
			}
		}
		attrsJSON, err := json.Marshal(attrs)
		if err != nil {
			return err
		}
		l := models.Licence{
			AssetID:             asset.ID,
			DetectedType:        detectedType,
			DetectionConfidence: confidence,
			CanonicalDiff:       canonicalDiff,
			OriginalText:        req.OriginalText,
			AttributesJSON:      attrsJSON,
		}
		if err := tx.Create(&l).Error; err != nil {
			return err
		}
		asset.LicenceID = &l.ID
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		out = l
		after := licenceViewFrom(&l)
		action := "asset.licence.set"
		if before != nil {
			action = "asset.licence.update"
		}
		var diff any
		if before != nil {
			diff = audit.Updated(before, after)
		} else {
			diff = audit.Created(after)
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: action,
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       diff,
		})
	})
	if err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	c.JSON(http.StatusOK, licenceViewFrom(&out))
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
		"status":               LicenceStatusFor(r.DetectedType, r.Attributes),
	})
}

// detectLicenceFromFiles scans an asset's current-version file manifest for
// candidate LICENSE-like filenames, reads the first match from storage, and
// runs detection. Returns {} when no candidate is found.
func (h *Handler) getDetectLicenceFromFiles(c *gin.Context) {
	lib := getLibrary(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
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
	if asset.CurrentVersionID == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	var files []models.AssetFile
	if err := h.db.WithContext(c.Request.Context()).
		Where("asset_version_id = ?", *asset.CurrentVersionID).
		Find(&files).Error; err != nil {
		respond500(c, err)
		return
	}
	candidate := pickLicenceCandidate(files)
	if candidate == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	blob, err := h.storage.GetBlob(c.Request.Context(), candidate.ObjectKey)
	if err != nil {
		respond500(c, err)
		return
	}
	defer blob.Close()
	buf := make([]byte, 128*1024)
	n := 0
	for n < len(buf) {
		m, rerr := blob.Read(buf[n:])
		n += m
		if rerr != nil {
			break
		}
	}
	text := string(buf[:n])
	r := licence.Detect(text)
	c.JSON(http.StatusOK, gin.H{
		"candidate_path":       candidate.RelativePath,
		"original_text":        text,
		"detected_type":        r.DetectedType,
		"detection_confidence": r.DetectionConfidence,
		"canonical_diff":       r.CanonicalDiff,
		"attributes":           r.Attributes,
		"status":               LicenceStatusFor(r.DetectedType, r.Attributes),
	})
}

// pickLicenceCandidate scans the manifest for common licence filenames.
// Order mirrors docs/archive/technical-design.md §10.1: root-level LICENSE/COPYING
// variants, case-insensitive.
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
