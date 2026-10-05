package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/models"
)

// assetMetadataFieldsView is the storefront-metadata block of an asset's detail
// response. Every field is nullable and omitted when unset, so the client can
// tell "the store didn't say" from "the store said zero".
type assetMetadataFieldsView struct {
	Publisher          *string    `json:"publisher,omitempty"`
	PublisherURL       *string    `json:"publisher_url,omitempty"`
	Store              *string    `json:"store,omitempty"`
	StoreProductID     *string    `json:"store_product_id,omitempty"`
	ReleasedAt         *time.Time `json:"released_at,omitempty"`
	StoreUpdatedAt     *time.Time `json:"store_updated_at,omitempty"`
	LatestStoreVersion *string    `json:"latest_store_version,omitempty"`
	Engine             *string    `json:"engine,omitempty"`
	MinEngineVersion   *string    `json:"min_engine_version,omitempty"`
	Price              *string    `json:"price,omitempty"`
	PriceCurrency      *string    `json:"price_currency,omitempty"`
	Rating             *float64   `json:"rating,omitempty"`
	RatingCount        *int       `json:"rating_count,omitempty"`
	StoreDeprecated    bool       `json:"store_deprecated"`
	// UpdateAvailable is true when latest_store_version is newer than every
	// store version the asset holds. Derived, never stored: it depends on the
	// asset's versions, which change independently of the metadata fetch.
	UpdateAvailable bool `json:"update_available,omitempty"`
}

func assetMetadataFieldsViewFrom(a *models.Asset) assetMetadataFieldsView {
	return assetMetadataFieldsView{
		Publisher:          a.Publisher,
		PublisherURL:       a.PublisherURL,
		Store:              a.Store,
		StoreProductID:     a.StoreProductID,
		ReleasedAt:         a.ReleasedAt,
		StoreUpdatedAt:     a.StoreUpdatedAt,
		LatestStoreVersion: a.LatestStoreVersion,
		Engine:             a.Engine,
		MinEngineVersion:   a.MinEngineVersion,
		Price:              a.Price,
		PriceCurrency:      a.PriceCurrency,
		Rating:             a.Rating,
		RatingCount:        a.RatingCount,
		StoreDeprecated:    a.StoreDeprecated,
	}
}

// assetMetadataFieldsPatch carries hand-corrections to the storefront fields.
// Every field is a string so one rule covers all of them: absent leaves the
// column alone, and an empty string clears it to NULL. Dates and numbers are
// parsed and rejected with a 400 rather than silently dropped.
type assetMetadataFieldsPatch struct {
	Publisher          *string `json:"publisher,omitempty"`
	PublisherURL       *string `json:"publisher_url,omitempty"`
	Store              *string `json:"store,omitempty"`
	StoreProductID     *string `json:"store_product_id,omitempty"`
	ReleasedAt         *string `json:"released_at,omitempty"`
	StoreUpdatedAt     *string `json:"store_updated_at,omitempty"`
	LatestStoreVersion *string `json:"latest_store_version,omitempty"`
	Engine             *string `json:"engine,omitempty"`
	MinEngineVersion   *string `json:"min_engine_version,omitempty"`
	Price              *string `json:"price,omitempty"`
	PriceCurrency      *string `json:"price_currency,omitempty"`
	Rating             *string `json:"rating,omitempty"`
	RatingCount        *string `json:"rating_count,omitempty"`
}

// apply validates the patch and writes its set fields into updates/before,
// mutating asset so the caller's response reflects the new values. A returned
// error is a 400 — the value was set but unusable.
func (p assetMetadataFieldsPatch) apply(asset *models.Asset, before, updates map[string]any) error {
	text := []struct {
		column string
		in     *string
		out    **string
	}{
		{"publisher", p.Publisher, &asset.Publisher},
		{"publisher_url", p.PublisherURL, &asset.PublisherURL},
		{"store", p.Store, &asset.Store},
		{"store_product_id", p.StoreProductID, &asset.StoreProductID},
		{"latest_store_version", p.LatestStoreVersion, &asset.LatestStoreVersion},
		{"engine", p.Engine, &asset.Engine},
		{"min_engine_version", p.MinEngineVersion, &asset.MinEngineVersion},
		{"price", p.Price, &asset.Price},
		{"price_currency", p.PriceCurrency, &asset.PriceCurrency},
	}
	for _, f := range text {
		if f.in == nil {
			continue
		}
		before[f.column] = derefString(*f.out)
		v := nilIfBlank(*f.in)
		*f.out = v
		updates[f.column] = v
	}

	dates := []struct {
		column string
		in     *string
		out    **time.Time
	}{
		{"released_at", p.ReleasedAt, &asset.ReleasedAt},
		{"store_updated_at", p.StoreUpdatedAt, &asset.StoreUpdatedAt},
	}
	for _, f := range dates {
		if f.in == nil {
			continue
		}
		v, err := parsePatchTime(f.column, *f.in)
		if err != nil {
			return err
		}
		before[f.column] = *f.out
		*f.out = v
		updates[f.column] = v
	}

	if p.Rating != nil {
		v, err := parsePatchRating(*p.Rating)
		if err != nil {
			return err
		}
		before["rating"] = asset.Rating
		asset.Rating = v
		updates["rating"] = v
	}
	if p.RatingCount != nil {
		v, err := parsePatchCount(*p.RatingCount)
		if err != nil {
			return err
		}
		before["rating_count"] = asset.RatingCount
		asset.RatingCount = v
		updates["rating_count"] = v
	}
	return nil
}

// parsePatchTime accepts an RFC 3339 timestamp or a bare date; empty clears.
func parsePatchTime(column, raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			utc := t.UTC()
			return &utc, nil
		}
	}
	return nil, fmt.Errorf("%s must be an RFC 3339 timestamp or a YYYY-MM-DD date", column)
}

func parsePatchRating(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("rating must be a number")
	}
	if v < 0 || v > 5 {
		return nil, fmt.Errorf("rating must be between 0 and 5")
	}
	return &v, nil
}

func parsePatchCount(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("rating_count must be a whole number")
	}
	if v < 0 {
		return nil, fmt.Errorf("rating_count must not be negative")
	}
	return &v, nil
}

// nilIfBlank maps an empty or whitespace-only value to NULL, so clearing a
// hand-corrected field is expressible without a separate delete call.
func nilIfBlank(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// assetFacet is one distinct value of a filterable metadata column, with the
// number of assets carrying it, so the sidebar can show counts the way the tag
// and project facets already do.
type assetFacet struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type assetFacetsResponse struct {
	Publishers []assetFacet `json:"publishers"`
	Stores     []assetFacet `json:"stores"`
	Engines    []assetFacet `json:"engines"`
}

// assetFacetLimit caps each facet list. Publisher is in the search vector, so a
// value past the cap is still findable by search.
const assetFacetLimit = 50

// getAssetFacets returns the distinct publisher, store and engine values in the
// current library. Read-only and library-scoped.
func (h *Handler) getAssetFacets(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	out := assetFacetsResponse{
		Publishers: []assetFacet{},
		Stores:     []assetFacet{},
		Engines:    []assetFacet{},
	}
	for _, f := range []struct {
		column string
		into   *[]assetFacet
	}{
		{"publisher", &out.Publishers},
		{"store", &out.Stores},
		{"engine", &out.Engines},
	} {
		var rows []assetFacet
		err := h.db.WithContext(c.Request.Context()).
			Table("assets").
			Select(f.column+" AS value, COUNT(*) AS count").
			Where("library_id = ? AND "+f.column+" IS NOT NULL AND "+f.column+" <> ''", lib.ID).
			Group(f.column).
			Order("count DESC, value ASC").
			Limit(assetFacetLimit).
			Scan(&rows).Error
		if err != nil {
			respond500(c, err)
			return
		}
		if rows != nil {
			*f.into = rows
		}
	}
	c.JSON(http.StatusOK, out)
}
