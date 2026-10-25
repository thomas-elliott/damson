package api

import (
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
	"github.com/thomas-elliott/damson/internal/services/enrich"
	"github.com/thomas-elliott/damson/internal/services/library"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
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

type metadataFetchResponse struct {
	Store      string            `json:"store"`
	ListingURL string            `json:"listing_url"`
	Fetched    *metadataView     `json:"fetched"`
	Applied    []string          `json:"applied"`
	Conflicts  []enrich.Conflict `json:"conflicts"`
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

	opts := enrich.Options{UserID: &u.ID, By: licence.ByPerson}
	if setSourceURL {
		opts.SourceURL = target
	}
	plan, err := enrich.Prepare(c.Request.Context(), h.enrichDeps(), &asset, meta, opts)
	if err != nil {
		respond500(c, err)
		return
	}
	var res enrich.Result
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		if res, err = plan.Write(tx); err != nil {
			return err
		}
		// The record keeps the conflicts' fetched values, which the person can
		// still choose to apply.
		return plan.SaveRecord(tx, target, res.Applied, nil)
	})
	if err != nil {
		respond500(c, err)
		return
	}

	c.JSON(http.StatusOK, metadataFetchResponse{
		Store:      meta.Store,
		ListingURL: firstNonEmptyStr(meta.ListingURL, target),
		Fetched:    metadataViewFrom(meta),
		Applied:    res.Applied,
		Conflicts:  res.Conflicts,

		StoreLookupFailed: meta.StoreLookupFailed,
	})
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
		if s, serr := library.Settings(h.db.WithContext(c.Request.Context()), lib.ID); serr == nil {
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
	res, err := enrich.Apply(c.Request.Context(), h.enrichDeps(), &asset, rec.Metadata, enrich.Options{Confirm: confirm, UserID: &u.ID, By: licence.ByPerson})
	if err != nil {
		respond500(c, err)
		return
	}

	c.JSON(http.StatusOK, metadataFetchResponse{
		Store:      rec.Store,
		ListingURL: rec.ListingURL,
		Fetched:    metadataViewFrom(rec.Metadata),
		Applied:    res.Applied,
		Conflicts:  res.Conflicts,
	})
}

// enrichDeps are what applying a listing to an asset needs.
func (h *Handler) enrichDeps() enrich.Deps {
	d := enrich.Deps{DB: h.db, Storage: h.storage}
	if h.metadata != nil {
		d.Images = h.metadata
	}
	return d
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
