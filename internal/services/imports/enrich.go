package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

// enrichAsset auto-fills blank fields from a storefront listing after import,
// gated on the per-library toggle and a conservative title-match. It is
// best-effort: any failure is logged and swallowed so enrichment never fails
// the import. When it can't confidently match, it stores the search
// candidates for one-click review and applies nothing.
func enrichAsset(ctx context.Context, deps Deps, jc jobContext) {
	if deps.Metadata == nil {
		return
	}
	settings, err := loadOrCreateLibrarySettings(deps.DB.WithContext(ctx), jc.libraryID)
	if err != nil {
		slog.Warn("enrich: load settings", "err", err, "library_id", jc.libraryID)
		return
	}
	if !settings.AutoEnrichEnabled {
		return
	}

	var asset models.Asset
	if err := deps.DB.WithContext(ctx).First(&asset, jc.asset.ID).Error; err != nil {
		slog.Warn("enrich: reload asset", "err", err, "asset_id", jc.asset.ID)
		return
	}

	cands, err := deps.Metadata.Search(ctx, asset.Name)
	if err != nil {
		slog.Warn("enrich: search", "err", err, "asset_id", asset.ID)
		return
	}
	if len(cands) == 0 {
		return
	}

	match := exactMatchCandidate(asset.Name, cands)
	rec := metadata.SourceRecord{
		FetchedAt:  time.Now().UTC(),
		Candidates: cands,
	}

	if match != nil {
		if meta, ferr := deps.Metadata.Fetch(ctx, match.ListingURL); ferr != nil || meta == nil {
			if ferr != nil {
				slog.Warn("enrich: fetch matched hit", "err", ferr, "url", match.ListingURL)
			}
		} else {
			enrichFillBlanks(ctx, deps, &asset, meta)
			rec.Store = meta.Store
			rec.ListingURL = firstNonEmptyStr(meta.ListingURL, match.ListingURL)
			rec.Confidence = 1.0
			rec.Matched = true
			rec.Metadata = meta
		}
	}

	persistEnrichRecord(ctx, deps, asset.ID, rec)
	flagNeedsReview(ctx, deps, asset.ID)
}

// enrichFillBlanks fills only empty fields — description, licence, and the
// cover thumbnail — mirroring the manual fetch's fill-blanks semantics but
// without the conflict/confirm path.
func enrichFillBlanks(ctx context.Context, deps Deps, asset *models.Asset, meta *metadata.Metadata) {
	if asset.Description == "" && meta.Description != "" {
		if err := deps.DB.WithContext(ctx).Model(asset).Where("id = ?", asset.ID).
			Update("description", meta.Description).Error; err != nil {
			slog.Warn("enrich: set description", "err", err, "asset_id", asset.ID)
		}
	}
	if asset.LicenceID == nil {
		if lic := buildEnrichLicence(meta); lic != nil {
			enrichPersistLicence(ctx, deps, asset, lic)
		}
	}
	if asset.ThumbnailObjectKey == "" && meta.CoverURL != "" {
		enrichSetThumbnail(ctx, deps, asset, meta.CoverURL)
	}
}

func buildEnrichLicence(meta *metadata.Metadata) *models.Licence {
	var detType, text string
	var attrs licence.Attributes
	var conf float32
	var canon string
	switch {
	case meta.LicenceText != "":
		r := licence.Detect(meta.LicenceText)
		detType, attrs, conf, canon, text = r.DetectedType, r.Attributes, r.DetectionConfidence, r.CanonicalDiff, meta.LicenceText
	case meta.LicenceHint != "":
		detType = meta.LicenceHint
		attrs = licence.AttributesForType(detType)
		text = "Licence: " + meta.LicenceHint + " (from " + storeOrSource(meta.Store) + " listing)"
	default:
		return nil
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		attrsJSON = []byte("{}")
	}
	return &models.Licence{
		DetectedType:        detType,
		DetectionConfidence: conf,
		CanonicalDiff:       canon,
		OriginalText:        text,
		AttributesJSON:      attrsJSON,
	}
}

func enrichPersistLicence(ctx context.Context, deps Deps, asset *models.Asset, lic *models.Licence) {
	err := deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lic.AssetID = asset.ID
		if err := tx.Create(lic).Error; err != nil {
			return err
		}
		asset.LicenceID = &lic.ID
		return tx.Model(asset).Where("id = ?", asset.ID).Update("licence_id", lic.ID).Error
	})
	if err != nil {
		slog.Warn("enrich: persist licence", "err", err, "asset_id", asset.ID)
	}
}

func enrichSetThumbnail(ctx context.Context, deps Deps, asset *models.Asset, coverURL string) {
	imgBytes, contentType, err := deps.Metadata.FetchImage(ctx, coverURL)
	if err != nil {
		slog.Warn("enrich: fetch cover", "err", err, "url", coverURL)
		return
	}
	gen := &thumbnail.Generator{Storage: deps.Storage}
	res, err := gen.Generate(ctx, thumbnail.Input{
		Reader:      bytes.NewReader(imgBytes),
		Filename:    "cover",
		ContentType: contentType,
	}, thumbnail.Toggles{Raster: true})
	if err != nil || res.ObjectKey == "" {
		if err != nil {
			slog.Warn("enrich: render cover", "err", err, "url", coverURL)
		}
		return
	}
	if err := deps.DB.WithContext(ctx).Model(asset).Where("id = ?", asset.ID).
		Updates(map[string]any{
			"thumbnail_object_key":   res.ObjectKey,
			"thumbnail_content_type": res.ContentType,
		}).Error; err != nil {
		slog.Warn("enrich: set thumbnail", "err", err, "asset_id", asset.ID)
	}
}

func persistEnrichRecord(ctx context.Context, deps Deps, assetID uint, rec metadata.SourceRecord) {
	blob, err := json.Marshal(rec)
	if err != nil {
		slog.Warn("enrich: marshal source record", "err", err, "asset_id", assetID)
		return
	}
	if err := deps.DB.WithContext(ctx).Model(&models.Asset{}).Where("id = ?", assetID).
		Update("source_metadata_json", json.RawMessage(blob)).Error; err != nil {
		slog.Warn("enrich: persist source record", "err", err, "asset_id", assetID)
	}
}

func flagNeedsReview(ctx context.Context, deps Deps, assetID uint) {
	if err := deps.DB.WithContext(ctx).Model(&models.Asset{}).Where("id = ?", assetID).
		Update("needs_review", true).Error; err != nil {
		slog.Warn("enrich: flag needs_review", "err", err, "asset_id", assetID)
	}
}

var nonAlnumRE = regexp.MustCompile(`[^a-z0-9]+`)

// normaliseTitle lowercases and strips everything but alphanumerics, so
// "Fantasy Pack (Deluxe)" and "fantasy-pack-deluxe" compare equal.
func normaliseTitle(s string) string {
	return strings.Trim(nonAlnumRE.ReplaceAllString(strings.ToLower(s), " "), " ")
}

// titlesMatch is the high-confidence gate: the normalised asset name and
// candidate title must be exactly equal (after collapsing). Anything short of
// that abstains, per the spec's conservative auto-enrich posture.
func titlesMatch(assetName, candidateTitle string) bool {
	a := strings.Join(strings.Fields(normaliseTitle(assetName)), " ")
	b := strings.Join(strings.Fields(normaliseTitle(candidateTitle)), " ")
	return a != "" && a == b
}

func exactMatchCandidate(assetName string, cands []metadata.Candidate) *metadata.Candidate {
	for i := range cands {
		if titlesMatch(assetName, cands[i].Title) {
			return &cands[i]
		}
	}
	return nil
}

func storeOrSource(store string) string {
	if store == "" {
		return "source"
	}
	return store
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
