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
	"github.com/thomas-elliott/damson/internal/services/tags"
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
	metadata.SortByPreferredStore(cands, settings.PreferredStore)

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
	if meta.Title != "" && asset.Name != meta.Title {
		if err := deps.DB.WithContext(ctx).Model(asset).Where("id = ?", asset.ID).
			Update("name", meta.Title).Error; err != nil {
			slog.Warn("enrich: set name", "err", err, "asset_id", asset.ID)
		} else {
			asset.Name = meta.Title
		}
	}
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
	// Auto-enrichment has no confirm path, so a nil confirm map makes every
	// would-be overwrite a no-op — blanks only, same rule as the fields above.
	if _, updates, _ := MetadataFieldUpdates(asset, meta, nil); len(updates) > 0 {
		if err := deps.DB.WithContext(ctx).Model(asset).Where("id = ?", asset.ID).
			Updates(updates).Error; err != nil {
			slog.Warn("enrich: set metadata fields", "err", err, "asset_id", asset.ID)
		}
	}
	StoreGalleryImages(ctx, deps.DB, deps.Storage, deps.Metadata, asset.ID, meta.ImageURLs)
	enrichAttachTags(ctx, deps, asset, MetadataTagList(meta))
}

// enrichAttachTags adds fetched storefront tags to the asset. Additive and
// best-effort — auto-enrichment never removes tags and any failure is logged
// and swallowed. Attributed to no user (system enrichment).
func enrichAttachTags(ctx context.Context, deps Deps, asset *models.Asset, tagList []string) {
	if len(tagList) == 0 {
		return
	}
	err := deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, t := range tagList {
			if _, _, err := tags.Attach(tx, asset.LibraryID, nil, asset.ID, t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("enrich: attach tags", "err", err, "asset_id", asset.ID)
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
			"thumbnail_from_store":   true,
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
	a := normalisedTitleKey(assetName)
	b := normalisedTitleKey(candidateTitle)
	return a != "" && a == b
}

// exactMatchCandidate returns the first candidate whose title equals the asset
// name under normalisation. The merged list has no meaningful cross-store
// order, so every candidate is checked, not just the head.
func exactMatchCandidate(assetName string, cands []metadata.Candidate) *metadata.Candidate {
	for i := range cands {
		if titlesMatch(assetName, cands[i].Title) {
			return &cands[i]
		}
	}
	return nil
}

// MatchAssetByName returns an existing asset in the library whose name equals
// name under normalisation, excluding excludeID. Returns nil when nothing
// matches. A one-shot NameMatcher, so the single and batch paths cannot
// disagree about the answer or the tie-break.
func MatchAssetByName(db *gorm.DB, libraryID, excludeID uint, name string) (*NameMatch, error) {
	m, err := newNameMatcher(db, libraryID, excludeID)
	if err != nil {
		return nil, err
	}
	return m.Match(name), nil
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

// NameMatch identifies an asset that matched by name. Only the id and name are
// ever read from a match, and that is all the matcher loads — a storefront-
// enriched asset carries a source_metadata_json payload that answering a name
// question has no use for.
type NameMatch struct {
	ID   uint
	Name string
}

// NameMatcher answers "do you already hold something by this name?" for a
// whole batch against a single read of the library, rather than re-reading it
// per entry across a preflight of hundreds.
type NameMatcher struct {
	byTitle map[string]NameMatch
	total   int
}

// NewNameMatcher reads the library once and indexes it by normalised title.
// Where two assets normalise to the same title the lower id wins, so the
// result does not depend on scan order.
func NewNameMatcher(db *gorm.DB, libraryID uint) (*NameMatcher, error) {
	return newNameMatcher(db, libraryID, 0)
}

// newNameMatcher builds the index, optionally excluding one asset — the
// just-created row, when the caller is asking what its own name collides with.
func newNameMatcher(db *gorm.DB, libraryID, excludeID uint) (*NameMatcher, error) {
	q := db.Model(&models.Asset{}).
		Select("id", "name").
		Where("library_id = ?", libraryID)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var assets []NameMatch
	if err := q.Order("id ASC").Find(&assets).Error; err != nil {
		return nil, err
	}
	m := &NameMatcher{byTitle: make(map[string]NameMatch, len(assets)), total: len(assets)}
	for _, a := range assets {
		key := normalisedTitleKey(a.Name)
		if key == "" {
			continue
		}
		if _, seen := m.byTitle[key]; !seen {
			m.byTitle[key] = a
		}
	}
	return m, nil
}

// Match returns the asset whose name equals name under the same normalisation
// titlesMatch uses, or nil.
func (m *NameMatcher) Match(name string) *NameMatch {
	if m == nil {
		return nil
	}
	key := normalisedTitleKey(name)
	if key == "" {
		return nil
	}
	match, ok := m.byTitle[key]
	if !ok {
		return nil
	}
	return &match
}

// Total is the number of assets the matcher was built from.
func (m *NameMatcher) Total() int {
	if m == nil {
		return 0
	}
	return m.total
}

// normalisedTitleKey is titlesMatch's comparison key, extracted so the batch
// index and the single-shot matcher cannot drift apart.
func normalisedTitleKey(s string) string {
	return strings.Join(strings.Fields(normaliseTitle(s)), " ")
}
