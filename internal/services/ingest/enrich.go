package ingest

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/enrich"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// enrichAsset auto-fills blank fields from a storefront listing after import,
// gated on the per-library toggle and a conservative title-match. It renames
// the asset to the listing's title and fills blanks only. It is best-effort:
// any failure is logged and swallowed so enrichment never fails the import.
// When it can't confidently match, it stores the search candidates for
// one-click review and applies nothing.
func enrichAsset(ctx context.Context, deps Deps, jc jobContext) {
	if deps.Metadata == nil {
		return
	}
	settings := jc.settings
	if settings == nil || !settings.AutoEnrichEnabled {
		return
	}

	var asset models.Asset
	if err := deps.DB.WithContext(ctx).First(&asset, jc.asset.ID).Error; err != nil {
		slog.Warn("enrich: reload asset", "err", err, "asset_id", jc.asset.ID)
		return
	}
	before, err := assetSnapshot(ctx, deps.DB, asset.ID)
	if err != nil {
		slog.Warn("enrich: audit snapshot", "err", err, "asset_id", asset.ID)
	}
	defer auditSystemUpdate(ctx, deps.DB, jc.libraryID, asset.ID, before)

	cands, err := deps.Metadata.Search(ctx, asset.Name)
	if err != nil {
		slog.Warn("enrich: search", "err", err, "asset_id", asset.ID)
		return
	}
	if len(cands) == 0 {
		return
	}
	metadata.SortByPreferredStore(cands, settings.PreferredStore)

	match, exact := exactMatchCandidate(asset.Name, derefString(asset.Publisher), cands)
	rec := metadata.SourceRecord{
		FetchedAt:  time.Now().UTC(),
		Candidates: cands,
	}

	var plan *enrich.Plan
	if match != nil {
		if meta, ferr := deps.Metadata.Fetch(ctx, match.ListingURL); ferr != nil || meta == nil {
			if ferr != nil {
				slog.Warn("enrich: fetch matched hit", "err", ferr, "url", match.ListingURL)
			}
		} else {
			plan, err = enrich.Prepare(ctx, enrich.Deps{DB: deps.DB, Storage: deps.Storage, Images: deps.Metadata}, &asset, meta,
				enrich.Options{Confirm: map[string]bool{"name": true}, By: licence.Automatic})
			if err != nil {
				slog.Warn("enrich: prepare", "err", err, "asset_id", asset.ID)
				return
			}
			rec.Store = meta.Store
			rec.ListingURL = meta.ListingURL
			if rec.ListingURL == "" {
				rec.ListingURL = match.ListingURL
			}
			rec.Confidence = nearMatchConfidence
			if exact {
				rec.Confidence = 1.0
			}
			rec.Matched = true
			rec.Metadata = meta
		}
	}

	err = deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if plan != nil {
			if _, err := plan.Write(tx); err != nil {
				return err
			}
		}
		if err := enrich.SaveSourceRecord(tx, &asset, rec); err != nil {
			return err
		}
		return tx.Model(&models.Asset{}).Where("id = ?", asset.ID).Update("needs_review", true).Error
	})
	if err != nil {
		slog.Warn("enrich: apply", "err", err, "asset_id", asset.ID)
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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

// exactMatchCandidate returns the best candidate for the asset. A title match
// alone still qualifies, but a known publisher both breaks ties among title
// matches and lets a near-title match qualify when the publisher agrees
// exactly. The merged list has no meaningful cross-store order, so every
// candidate is checked, not just the head.
func exactMatchCandidate(assetName, assetPublisher string, cands []metadata.Candidate) (*metadata.Candidate, bool) {
	var titleOnly *metadata.Candidate
	var publisherNear *metadata.Candidate
	for i := range cands {
		title := titlesMatch(assetName, cands[i].Title)
		publisher := publishersMatch(assetPublisher, cands[i].Publisher)
		switch {
		case title && publisher:
			return &cands[i], true
		case title:
			if titleOnly == nil {
				titleOnly = &cands[i]
			}
		case publisher && titlesNear(assetName, cands[i].Title):
			if publisherNear == nil {
				publisherNear = &cands[i]
			}
		}
	}
	if titleOnly != nil {
		return titleOnly, true
	}
	return publisherNear, false
}

// publishersMatch compares publishers under the same normalisation titles use.
// A blank on either side never matches, so an asset with no publisher falls
// through to title-only matching.
func publishersMatch(assetPublisher, candidatePublisher string) bool {
	a := normalisedTitleKey(assetPublisher)
	b := normalisedTitleKey(candidatePublisher)
	return a != "" && a == b
}

// titlesNear is the weaker gate the publisher hint unlocks: the shorter
// normalised title is a whole-word prefix of the longer, catching a listing
// that carries a subtitle or edition suffix the cached filename dropped.
// Bounded at minNearTitleKey characters so a short generic name cannot reach
// half a publisher's catalogue.
func titlesNear(assetName, candidateTitle string) bool {
	a := normalisedTitleKey(assetName)
	b := normalisedTitleKey(candidateTitle)
	if a == "" || b == "" || a == b {
		return false
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) < minNearTitleKey {
		return false
	}
	return strings.HasPrefix(b, a+" ")
}

const (
	// minNearTitleKey is the shortest normalised title that may qualify on the
	// weaker titlesNear gate.
	minNearTitleKey = 8

	// nearMatchConfidence is recorded for a match that only cleared the weaker
	// gate, so the provenance record never claims an exact title match it
	// did not make.
	nearMatchConfidence = 0.8
)

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
// Reference assets are left out: they take no files, so an upload can never
// land in one.
func newNameMatcher(db *gorm.DB, libraryID, excludeID uint) (*NameMatcher, error) {
	q := db.Model(&models.Asset{}).
		Select("id", "name").
		Where("library_id = ? AND is_reference = false", libraryID)
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
