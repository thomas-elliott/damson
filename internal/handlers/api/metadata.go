package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/tags"
	"github.com/thomas-elliott/damson/internal/services/thumbnail"
)

// metadataService is the storefront-metadata dependency the handlers use.
// *metadata.Registry satisfies it in production; tests inject a fake so they
// don't perform real outbound HTTP (which the SSRF guard would block anyway).
type metadataService interface {
	Fetch(ctx context.Context, rawURL string) (*metadata.Metadata, error)
	FetchImage(ctx context.Context, rawURL string) ([]byte, string, error)
	SearchStores(ctx context.Context, query string) ([]metadata.Candidate, []string)
}

// metadataView is the client-facing shape of fetched metadata — the normalised
// fields minus the raw payload.
type metadataView struct {
	Title              string               `json:"title,omitempty"`
	Description        string               `json:"description,omitempty"`
	Publisher          string               `json:"publisher,omitempty"`
	PublisherURL       string               `json:"publisher_url,omitempty"`
	Price              string               `json:"price,omitempty"`
	Currency           string               `json:"currency,omitempty"`
	LicenceHint        string               `json:"licence_hint,omitempty"`
	CoverURL           string               `json:"cover_url,omitempty"`
	ImageURLs          []string             `json:"image_urls,omitempty"`
	Tags               []string             `json:"tags,omitempty"`
	CategoryPath       string               `json:"category_path,omitempty"`
	Store              string               `json:"store,omitempty"`
	ListingURL         string               `json:"listing_url,omitempty"`
	ReleasedAt         *time.Time           `json:"released_at,omitempty"`
	StoreUpdatedAt     *time.Time           `json:"store_updated_at,omitempty"`
	LatestStoreVersion string               `json:"latest_store_version,omitempty"`
	Engine             string               `json:"engine,omitempty"`
	MinEngineVersion   string               `json:"min_engine_version,omitempty"`
	Rating             float64              `json:"rating,omitempty"`
	RatingCount        int                  `json:"rating_count,omitempty"`
	Attributes         []metadata.Attribute `json:"attributes,omitempty"`
}

func metadataViewFrom(m *metadata.Metadata) *metadataView {
	if m == nil {
		return nil
	}
	hint := m.LicenceHint
	if hint == "" && m.LicenceText != "" {
		hint = "custom"
	}
	return &metadataView{
		Title:              m.Title,
		Description:        m.Description,
		Publisher:          m.Publisher,
		PublisherURL:       m.PublisherURL,
		Price:              m.Price,
		Currency:           m.Currency,
		LicenceHint:        hint,
		CoverURL:           m.CoverURL,
		ImageURLs:          m.ImageURLs,
		Tags:               m.Tags,
		CategoryPath:       m.CategoryPath,
		Store:              m.Store,
		ListingURL:         m.ListingURL,
		ReleasedAt:         m.ReleasedAt,
		StoreUpdatedAt:     m.StoreUpdatedAt,
		LatestStoreVersion: m.LatestStoreVersion,
		Engine:             m.Engine,
		MinEngineVersion:   m.MinEngineVersion,
		Rating:             m.Rating,
		RatingCount:        m.RatingCount,
		Attributes:         m.Attributes,
	}
}

type candidateView struct {
	Title      string `json:"title"`
	Publisher  string `json:"publisher,omitempty"`
	Price      string `json:"price,omitempty"`
	Currency   string `json:"currency,omitempty"`
	CoverURL   string `json:"cover_url,omitempty"`
	ListingURL string `json:"listing_url"`
	Store      string `json:"store"`
}

func candidateViewsFrom(cands []metadata.Candidate) []candidateView {
	out := make([]candidateView, 0, len(cands))
	for _, c := range cands {
		out = append(out, candidateView{
			Title:      c.Title,
			Publisher:  c.Publisher,
			Price:      c.Price,
			Currency:   c.Currency,
			CoverURL:   c.CoverURL,
			ListingURL: c.ListingURL,
			Store:      c.Store,
		})
	}
	return out
}

// sourceMetadataView is the persisted-enrichment summary the detail panel
// renders: provenance + the normalised metadata + any stored candidates.
type sourceMetadataView struct {
	Store      string          `json:"store,omitempty"`
	ListingURL string          `json:"listing_url,omitempty"`
	FetchedAt  time.Time       `json:"fetched_at,omitempty"`
	Matched    bool            `json:"matched"`
	Confidence float64         `json:"match_confidence,omitempty"`
	Metadata   *metadataView   `json:"metadata,omitempty"`
	Candidates []candidateView `json:"candidates,omitempty"`
}

func sourceMetadataViewFrom(raw json.RawMessage) *sourceMetadataView {
	if len(raw) == 0 {
		return nil
	}
	var rec metadata.SourceRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil
	}
	return &sourceMetadataView{
		Store:      rec.Store,
		ListingURL: rec.ListingURL,
		FetchedAt:  rec.FetchedAt,
		Matched:    rec.Matched,
		Confidence: rec.Confidence,
		Metadata:   metadataViewFrom(rec.Metadata),
		Candidates: candidateViewsFrom(rec.Candidates),
	}
}

// metadataConflict names a field whose fetched value differs from the asset's
// current value and so needs explicit user confirmation to overwrite.
type metadataConflict struct {
	Field    string `json:"field"`
	Current  string `json:"current"`
	Proposed string `json:"proposed"`
}

type metadataFetchResponse struct {
	Store      string             `json:"store"`
	ListingURL string             `json:"listing_url"`
	Fetched    *metadataView      `json:"fetched"`
	Applied    []string           `json:"applied"`
	Conflicts  []metadataConflict `json:"conflicts"`
	// StoreLookupFailed says the store's adapter failed and the generic
	// scraper answered; what it read is applied all the same.
	StoreLookupFailed bool `json:"store_lookup_failed,omitempty"`
}

type metadataFetchRequest struct {
	URL string `json:"url,omitempty"`
}

// postAssetMetadataFetch fetches storefront metadata for an asset's source URL
// (or a URL supplied in the body, which also sets source_url), fills blank
// fields, and returns applied fields plus would-overwrite conflicts. The raw
// payload + provenance always persist to source_metadata.
func (h *Handler) postAssetMetadataFetch(c *gin.Context) {
	if h.metadata == nil {
		respond503(c, "metadata service not configured")
		return
	}
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
		return
	}
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
	var req metadataFetchRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respond400(c, err)
			return
		}
	}

	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
		respondGORM(c, err, "asset not found")
		return
	}

	target := asset.SourceURL
	setSourceURL := false
	if req.URL != "" {
		cleaned, err := validateSourceURL(req.URL)
		if err != nil {
			respond400(c, err)
			return
		}
		target = cleaned
		setSourceURL = cleaned != asset.SourceURL
	}
	if target == "" {
		respond400(c, asError("asset has no source_url; supply one in the request body"))
		return
	}

	meta, err := h.metadata.Fetch(c.Request.Context(), target)
	if err != nil {
		slog.Warn("metadata fetch failed", "url", target, "err", err)
		respondLookupFailed(c, err, "could not fetch metadata from source")
		return
	}

	applied, conflicts := h.applyMetadata(c, lib.ID, &asset, meta, nil, setSourceURL, target, u.ID)

	rec := metadata.SourceRecord{
		Store:      meta.Store,
		ListingURL: firstNonEmptyStr(meta.ListingURL, target),
		FetchedAt:  time.Now().UTC(),
		Matched:    true,
		Metadata:   meta,
	}
	h.persistSourceRecord(c, &asset, rec)

	c.JSON(http.StatusOK, metadataFetchResponse{
		Store:      meta.Store,
		ListingURL: rec.ListingURL,
		Fetched:    metadataViewFrom(meta),
		Applied:    applied,
		Conflicts:  conflicts,

		StoreLookupFailed: meta.StoreLookupFailed,
	})
}

type galleryBackfillResponse struct {
	Stored int `json:"stored"`
	Total  int `json:"total"`
}

// postAssetGalleryBackfill re-runs gallery image storage for an asset from its
// already-persisted source metadata, so assets enriched before the gallery
// existed get their images without a fresh storefront lookup. Idempotent:
// StoreGalleryImages skips source URLs already stored.
func (h *Handler) postAssetGalleryBackfill(c *gin.Context) {
	if h.metadata == nil {
		respond503(c, "metadata service not configured")
		return
	}
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
		return
	}
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	if len(asset.SourceMetadataJSON) == 0 {
		respond409(c, "no fetched metadata to backfill from; fetch first")
		return
	}
	var rec metadata.SourceRecord
	if err := json.Unmarshal(asset.SourceMetadataJSON, &rec); err != nil || rec.Metadata == nil {
		respond409(c, "stored metadata is unreadable; fetch again")
		return
	}

	ctx := c.Request.Context()
	before := h.countAssetImages(ctx, asset.ID)
	imports.StoreGalleryImages(ctx, h.db, h.storage, h.metadata, asset.ID, rec.Metadata.ImageURLs)
	after := h.countAssetImages(ctx, asset.ID)

	c.JSON(http.StatusOK, galleryBackfillResponse{Stored: after - before, Total: after})
}

func (h *Handler) countAssetImages(ctx context.Context, assetID uint) int {
	var n int64
	h.db.WithContext(ctx).Model(&models.AssetImage{}).Where("asset_id = ?", assetID).Count(&n)
	return int(n)
}

type metadataSearchResponse struct {
	Query      string          `json:"query"`
	Candidates []candidateView `json:"candidates"`
	// FailedStores names each store that errored or timed out, so a caller
	// can tell a throttled search from one that found nothing.
	FailedStores []string `json:"failed_stores,omitempty"`
}

func (h *Handler) getMetadataPreview(c *gin.Context) {
	if h.metadata == nil {
		respond503(c, "metadata service not configured")
		return
	}
	raw := strings.TrimSpace(c.Query("url"))
	if raw == "" {
		respond400(c, asError("url is required"))
		return
	}
	cleaned, err := validateSourceURL(raw)
	if err != nil {
		respond400(c, err)
		return
	}
	meta, err := h.metadata.Fetch(c.Request.Context(), cleaned)
	if err != nil {
		slog.Warn("metadata preview failed", "url", cleaned, "err", err)
		respondLookupFailed(c, err, "could not fetch listing")
		return
	}
	c.JSON(http.StatusOK, metadataViewFrom(meta))
}

func (h *Handler) getMetadataImage(c *gin.Context) {
	if h.metadata == nil {
		respond503(c, "metadata service not configured")
		return
	}
	raw := strings.TrimSpace(c.Query("url"))
	if raw == "" {
		respond400(c, asError("url is required"))
		return
	}
	cleaned, err := validateSourceURL(raw)
	if err != nil {
		respond400(c, err)
		return
	}
	data, contentType, err := h.metadata.FetchImage(c.Request.Context(), cleaned)
	if err != nil {
		slog.Warn("metadata image proxy failed", "url", cleaned, "err", err)
		respond502(c, "could not fetch image")
		return
	}
	if !strings.HasPrefix(contentType, "image/") {
		slog.Warn("metadata image proxy: non-image response", "url", cleaned, "content_type", contentType)
		respond502(c, "not an image")
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Data(http.StatusOK, contentType, data)
}

// getMetadataSearch fans out a name search across the storefront adapters and
// returns a merged candidate list for the match picker.
func (h *Handler) getMetadataSearch(c *gin.Context) {
	if h.metadata == nil {
		respond503(c, "metadata service not configured")
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		respond400(c, asError("q is required"))
		return
	}
	cands, failed := h.metadata.SearchStores(c.Request.Context(), q)
	if lib := getLibrary(c); lib != nil {
		if s, serr := loadOrCreateLibrarySettings(h.db.WithContext(c.Request.Context()), lib.ID); serr == nil {
			metadata.SortByPreferredStore(cands, s.PreferredStore)
		}
	}
	c.JSON(http.StatusOK, metadataSearchResponse{
		Query:        q,
		Candidates:   candidateViewsFrom(cands),
		FailedStores: failed,
	})
}

type metadataApplyRequest struct {
	Fields []string `json:"fields"`
}

// postAssetMetadataApply persists user-confirmed overwrites for the fields the
// fetch returned as conflicts, reading the fetched metadata back from the
// stored source_metadata record.
func (h *Handler) postAssetMetadataApply(c *gin.Context) {
	if h.metadata == nil {
		respond503(c, "metadata service not configured")
		return
	}
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
		return
	}
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
	var req metadataApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	if len(req.Fields) == 0 {
		respond400(c, asError("fields must not be empty"))
		return
	}

	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).First(&asset).Error; err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	if len(asset.SourceMetadataJSON) == 0 {
		respond409(c, "no fetched metadata to apply; fetch first")
		return
	}
	var rec metadata.SourceRecord
	if err := json.Unmarshal(asset.SourceMetadataJSON, &rec); err != nil || rec.Metadata == nil {
		respond409(c, "stored metadata is unreadable; fetch again")
		return
	}

	confirm := make(map[string]bool, len(req.Fields))
	for _, f := range req.Fields {
		confirm[f] = true
	}
	applied, conflicts := h.applyMetadata(c, lib.ID, &asset, rec.Metadata, confirm, false, "", u.ID)

	c.JSON(http.StatusOK, metadataFetchResponse{
		Store:      rec.Store,
		ListingURL: rec.ListingURL,
		Fetched:    metadataViewFrom(rec.Metadata),
		Applied:    applied,
		Conflicts:  conflicts,
	})
}

// applyMetadata applies fetched metadata to an asset. With confirm==nil it
// fills blank fields only and reports the rest as conflicts; with confirm set
// it overwrites exactly the confirmed fields. setSourceURL/target set the
// source URL when the fetch was triggered by a body URL.
func (h *Handler) applyMetadata(c *gin.Context, libID uint, asset *models.Asset, meta *metadata.Metadata, confirm map[string]bool, setSourceURL bool, target string, userID uint) ([]string, []metadataConflict) {
	ctx := c.Request.Context()
	applied := []string{}
	conflicts := []metadataConflict{}
	updates := map[string]any{}
	before := map[string]any{}

	if setSourceURL && target != "" && target != asset.SourceURL {
		before["source_url"] = asset.SourceURL
		updates["source_url"] = target
		asset.SourceURL = target
		applied = append(applied, "source_url")
	}

	// Unlike the fill-if-blank fields below, a name is never blank, so a
	// differing store title is always a confirmable conflict, never a silent fill.
	if meta.Title != "" && asset.Name != meta.Title {
		if confirm["name"] {
			before["name"] = asset.Name
			updates["name"] = meta.Title
			asset.Name = meta.Title
			applied = append(applied, "name")
		} else {
			conflicts = append(conflicts, metadataConflict{Field: "name", Current: asset.Name, Proposed: meta.Title})
		}
	}

	if meta.Description != "" && asset.Description != meta.Description {
		if asset.Description == "" || confirm["description"] {
			before["description"] = asset.Description
			updates["description"] = meta.Description
			asset.Description = meta.Description
			applied = append(applied, "description")
		} else {
			conflicts = append(conflicts, metadataConflict{Field: "description", Current: asset.Description, Proposed: meta.Description})
		}
	}

	fieldBefore, fieldUpdates, fieldConflicts := imports.MetadataFieldUpdates(asset, meta, confirm)
	for _, fc := range fieldConflicts {
		conflicts = append(conflicts, metadataConflict{Field: fc.Field, Current: fc.Current, Proposed: fc.Proposed})
	}
	for k, v := range fieldBefore {
		before[k] = v
	}
	for k, v := range fieldUpdates {
		updates[k] = v
		applied = append(applied, k)
	}

	auditBefore := map[string]any{}
	auditAfter := map[string]any{}
	if len(updates) > 0 {
		if err := h.db.WithContext(ctx).Model(asset).Where("id = ?", asset.ID).Updates(updates).Error; err != nil {
			slog.Warn("metadata: persist fields", "err", err, "asset_id", asset.ID)
		} else {
			for k, v := range before {
				auditBefore[k] = v
			}
			for k, v := range updates {
				auditAfter[k] = v
			}
		}
	}

	if applied2, prevType, newType, conflict := h.applyLicenceFromMeta(c, asset, meta, confirm); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if applied2 {
		applied = append(applied, "licence")
		auditBefore["licence"] = prevType
		auditAfter["licence"] = newType
	}

	prevThumb := asset.ThumbnailObjectKey
	if applied3, conflict := h.applyThumbnailFromMeta(c, asset, meta, confirm); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if applied3 {
		applied = append(applied, "thumbnail")
		auditBefore["thumbnail_object_key"] = prevThumb
		auditAfter["thumbnail_object_key"] = asset.ThumbnailObjectKey
	}

	imports.StoreGalleryImages(ctx, h.db, h.storage, h.metadata, asset.ID, meta.ImageURLs)

	if h.attachMetadataTags(ctx, libID, asset.ID, userID, imports.MetadataTagList(meta)) {
		applied = append(applied, "tags")
	}

	if len(auditAfter) > 0 {
		h.auditMetadata(c, libID, asset.ID, userID, "asset.metadata.apply", auditBefore, auditAfter)
	}

	return applied, conflicts
}

// applyLicenceFromMeta creates a licence from the fetched hint/text when the
// asset has none, or reports a conflict when one exists and differs (unless
// confirmed). A reviewed licence is only ever replaced on explicit confirm.
// On apply it returns the previous and new detected types for the audit diff.
func (h *Handler) applyLicenceFromMeta(c *gin.Context, asset *models.Asset, meta *metadata.Metadata, confirm map[string]bool) (bool, string, string, *metadataConflict) {
	lic := buildLicenceFromMeta(meta)
	if lic == nil {
		return false, "", "", nil
	}
	prevType := ""
	if asset.LicenceID != nil {
		var existing models.Licence
		if err := h.db.WithContext(c.Request.Context()).First(&existing, *asset.LicenceID).Error; err == nil {
			prevType = existing.DetectedType
			if existing.DetectedType == lic.DetectedType {
				return false, "", "", nil
			}
			if !confirm["licence"] {
				return false, "", "", &metadataConflict{Field: "licence", Current: existing.DetectedType, Proposed: lic.DetectedType}
			}
		}
	}
	if err := h.persistLicence(c, asset, lic); err != nil {
		slog.Warn("metadata: persist licence", "err", err, "asset_id", asset.ID)
		return false, "", "", nil
	}
	return true, prevType, lic.DetectedType, nil
}

// applyThumbnailFromMeta downloads the cover image and stores it as the asset
// thumbnail via the raster thumbnail pipeline, whose decode + re-encode
// rejects non-image bodies rather than storing them. Fills when no thumbnail
// exists; conflicts otherwise unless confirmed.
func (h *Handler) applyThumbnailFromMeta(c *gin.Context, asset *models.Asset, meta *metadata.Metadata, confirm map[string]bool) (bool, *metadataConflict) {
	if meta.CoverURL == "" {
		return false, nil
	}
	if asset.ThumbnailObjectKey != "" && !confirm["thumbnail"] {
		return false, &metadataConflict{Field: "thumbnail", Current: "existing thumbnail", Proposed: meta.CoverURL}
	}
	imgBytes, contentType, err := h.metadata.FetchImage(c.Request.Context(), meta.CoverURL)
	if err != nil {
		slog.Warn("metadata: fetch cover", "err", err, "url", meta.CoverURL)
		return false, nil
	}
	gen := &thumbnail.Generator{Storage: h.storage}
	res, err := gen.Generate(c.Request.Context(), thumbnail.Input{
		Reader:      bytes.NewReader(imgBytes),
		Filename:    "cover",
		ContentType: contentType,
	}, thumbnail.Toggles{Raster: true})
	if err != nil || res.ObjectKey == "" {
		if err != nil {
			slog.Warn("metadata: render cover", "err", err, "url", meta.CoverURL)
		}
		return false, nil
	}
	if !h.setThumbnail(c, asset, res.ObjectKey, res.ContentType, true) {
		return false, nil
	}
	return true, nil
}

// buildLicenceFromMeta turns a fetched licence hint/text into an unsaved
// Licence row. Custom text is run through the detector; a bare SPDX/EULA hint
// gets its known attributes and a synthesised original_text (the model
// requires non-blank text). Returns nil when there's no licence signal.
func buildLicenceFromMeta(meta *metadata.Metadata) *models.Licence {
	text := meta.LicenceText
	var detType string
	var attrs licence.Attributes
	var conf float32
	var canon string
	switch {
	case text != "":
		r := licence.Detect(text)
		detType = r.DetectedType
		attrs = r.Attributes
		conf = r.DetectionConfidence
		canon = r.CanonicalDiff
	case meta.LicenceHint != "":
		detType = meta.LicenceHint
		attrs = licence.AttributesForType(detType)
		text = "Licence: " + meta.LicenceHint + " (from " + storeLabel(meta.Store) + " listing)"
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

func storeLabel(store string) string {
	if store == "" {
		return "source"
	}
	return store
}

// persistLicence replaces the asset's licence row with lic in one transaction,
// mirroring rescanLicence's create-then-point pattern.
func (h *Handler) persistLicence(c *gin.Context, asset *models.Asset, lic *models.Licence) error {
	return h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if asset.LicenceID != nil {
			if err := tx.Where("id = ?", *asset.LicenceID).Delete(&models.Licence{}).Error; err != nil {
				return err
			}
		}
		lic.AssetID = asset.ID
		if err := tx.Create(lic).Error; err != nil {
			return err
		}
		asset.LicenceID = &lic.ID
		return tx.Model(asset).Where("id = ?", asset.ID).Update("licence_id", lic.ID).Error
	})
}

// persistSourceRecord writes the enrichment envelope to source_metadata.
func (h *Handler) persistSourceRecord(c *gin.Context, asset *models.Asset, rec metadata.SourceRecord) {
	blob, err := json.Marshal(rec)
	if err != nil {
		slog.Warn("metadata: marshal source record", "err", err, "asset_id", asset.ID)
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Model(asset).
		Where("id = ?", asset.ID).
		Update("source_metadata_json", json.RawMessage(blob)).Error; err != nil {
		slog.Warn("metadata: persist source record", "err", err, "asset_id", asset.ID)
		return
	}
	asset.SourceMetadataJSON = blob
}

// attachMetadataTags adds every fetched tag to the asset, create-or-reusing
// each by path. Tags are additive (never removed by a re-fetch); returns true
// when at least one new link was made so the caller can report "tags" applied.
func (h *Handler) attachMetadataTags(ctx context.Context, libID, assetID, userID uint, tagList []string) bool {
	if len(tagList) == 0 {
		return false
	}
	linked := false
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, t := range tagList {
			_, added, err := tags.Attach(tx, libID, &userID, assetID, t)
			if err != nil {
				return err
			}
			linked = linked || added
		}
		return nil
	})
	if err != nil {
		slog.Warn("metadata: attach tags", "err", err, "asset_id", assetID)
		return false
	}
	return linked
}

func (h *Handler) auditMetadata(c *gin.Context, libID, assetID, userID uint, action string, before, after map[string]any) {
	if err := audit.Write(h.db.WithContext(c.Request.Context()), audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: action,
		TargetType: "asset",
		TargetID:   &assetID,
		Diff:       audit.Updated(before, after),
	}); err != nil {
		slog.Warn("metadata: audit write", "err", err, "asset_id", assetID)
	}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// respondLookupFailed answers a store lookup that failed. A store that asked
// for a pause is a 503 with Retry-After, so the person waiting knows to try
// again later rather than that the link is bad; anything else is a 502.
func respondLookupFailed(c *gin.Context, err error, msg string) {
	var limited *metadata.RateLimitedError
	if errors.As(err, &limited) {
		secs := max(1, int(math.Ceil(time.Until(limited.Until).Seconds())))
		c.Header("Retry-After", strconv.Itoa(secs))
		respond503(c, fmt.Sprintf("the store asked for a pause; try again in %d s", secs))
		return
	}
	respond502(c, msg)
}

func respond502(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": msg})
}

func respond503(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": msg})
}
