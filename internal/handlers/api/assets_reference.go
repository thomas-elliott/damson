package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/enrich"
	"github.com/thomas-elliott/damson/internal/services/licence"
	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/tags"
)

// Outcomes of recording an asset by its location.
const (
	referenceCreated = "created"
	referenceUpdated = "updated"
	referenceExists  = "exists"
)

// maxReferenceTags bounds the tags one record attaches. A scan gives one per
// folder between its root and the pack, so this is far above any real depth.
const maxReferenceTags = 32

// maxLocationPathLen bounds a recorded path. Damson never reads it, so the cap
// only keeps a pasted mistake from landing in the search vector.
const maxLocationPathLen = 4096

// errReferenceNameRequired is returned when neither the path, the store link
// nor the caller supplies a name. It maps to 422 so a client can tell "ask for
// a name" apart from a malformed request.
var errReferenceNameRequired = errors.New("name is required: the path and store link give no name")

type referenceRecordRequest struct {
	LocationPath string `json:"location_path"`
	SourceURL    string `json:"source_url"`
	Name         string `json:"name"`
	NeedsReview  bool   `json:"needs_review"`
	// Publisher and Tags, like Name, apply only when the record creates the
	// asset; a re-record of a known path leaves them as they are.
	Publisher string   `json:"publisher"`
	Tags      []string `json:"tags"`
	// QueueLookup queues the store lookup to run in the background instead of
	// making it in the request, adding it to LookupBatchID when that is set and
	// to a new batch otherwise. It fills blanks only, as the immediate one does.
	QueueLookup   bool `json:"queue_lookup"`
	LookupBatchID uint `json:"lookup_batch_id"`
}

type referenceRecordResponse struct {
	Outcome      string `json:"outcome"`
	AssetID      uint   `json:"asset_id"`
	Name         string `json:"name"`
	LocationPath string `json:"location_path,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
	// MetadataFailed says the store lookup for the link failed, so the asset
	// has its link but not the store's details: none at all, or only what the
	// generic scraper read from the page.
	MetadataFailed bool `json:"metadata_failed,omitempty"`
	// LookupBatchID is the batch a queued lookup joined, for following it.
	LookupBatchID uint `json:"lookup_batch_id,omitempty"`
}

// normaliseLocationPath applies the spec's sameness rule: two paths are the
// same when they match exactly after trimming surrounding whitespace and one
// trailing slash. An empty result means "no path".
func normaliseLocationPath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	if len(p) > maxLocationPathLen {
		return "", asError("location_path is too long")
	}
	if strings.ContainsFunc(p, unicode.IsControl) {
		return "", asError("location_path must not contain control characters")
	}
	return p, nil
}

// fileExtension matches a trailing extension worth dropping from a name. It
// must contain a letter, so a folder like `Pack v1.2` keeps its version.
var fileExtension = regexp.MustCompile(`\.[A-Za-z0-9]*[A-Za-z][A-Za-z0-9]*$`)

// nameFromLocationPath is the last segment of the path minus any file
// extension: `…/Synty/POLYGON_Fantasy_Kingdom.unitypackage` →
// `POLYGON_Fantasy_Kingdom`.
func nameFromLocationPath(p string) string {
	seg := p
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		seg = p[i+1:]
	}
	if trimmed := fileExtension.ReplaceAllString(seg, ""); trimmed != "" {
		seg = trimmed
	}
	return strings.TrimSpace(seg)
}

// applyReferenceListing fills the recorded asset's blanks from its listing
// and saves the lookup's source record, together or not at all.
func (h *Handler) applyReferenceListing(ctx context.Context, asset *models.Asset, meta *metadata.Metadata, link string, userID uint) error {
	plan, err := enrich.Prepare(ctx, h.enrichDeps(), asset, meta, enrich.Options{UserID: &userID, By: licence.ByPerson})
	if err != nil {
		return err
	}
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res, err := plan.Write(tx)
		if err != nil {
			return err
		}
		return plan.SaveRecord(tx, link, res.Applied, enrich.ConflictFields(res.Conflicts))
	})
}

// referenceName picks a new reference asset's name: the caller's, else the
// store's title, else the path's last segment, else the page's title. A title
// from the generic scraper is the page's title, not a store's.
func referenceName(given, path string, meta *metadata.Metadata) string {
	if given != "" {
		return given
	}
	var title string
	if meta != nil {
		title = strings.TrimSpace(meta.Title)
		if title != "" && meta.Store != metadata.StoreGeneric {
			return title
		}
	}
	if n := nameFromLocationPath(path); n != "" {
		return n
	}
	return title
}

// postReferenceAsset records an asset by where it lives — a path on the NFS
// store and/or a store link — with no imported data. A known path updates its
// asset; a link-only record whose link is already on an asset creates nothing
// and names that asset. A store link is looked up and fills blank fields only.
func (h *Handler) postReferenceAsset(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	var req referenceRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	path, err := normaliseLocationPath(req.LocationPath)
	if err != nil {
		respond400(c, err)
		return
	}
	link, err := validateSourceURL(req.SourceURL)
	if err != nil {
		respond400(c, err)
		return
	}
	if path == "" && link == "" {
		respond400(c, asError("give a location_path, a source_url, or both"))
		return
	}
	if len(req.Tags) > maxReferenceTags {
		respond400(c, asError("too many tags"))
		return
	}
	// Check the tags before a store lookup is spent on a record that would fail.
	for _, t := range req.Tags {
		if tags.NormalisePath(t) == "" {
			respond400(c, fmt.Errorf("tag %q is blank or longer than %d bytes", t, tags.MaxPathLength))
			return
		}
	}
	publisher := strings.TrimSpace(req.Publisher)
	if req.QueueLookup && (h.storeLookups == nil || h.metadata == nil) {
		respond503(c, errStoreLookupsUnavailable)
		return
	}

	ctx := c.Request.Context()
	db := h.db.WithContext(ctx)

	// A link-only record whose link is already in the library creates nothing,
	// so answer it before spending a store lookup.
	if path == "" {
		var existing models.Asset
		err := db.Where("library_id = ? AND source_url = ?", lib.ID, link).
			Order("id").First(&existing).Error
		if err == nil {
			c.JSON(http.StatusOK, referenceResponseFrom(referenceExists, &existing))
			return
		}
		if !isNotFound(err) {
			respond500(c, err)
			return
		}
	}

	// The lookup runs outside the transaction: it is network-bound, and a link
	// it can't read is still recorded.
	var meta *metadata.Metadata
	metadataFailed := false
	if link != "" && h.metadata != nil && !req.QueueLookup {
		if m, ferr := h.metadata.Fetch(ctx, link); ferr != nil {
			slog.Warn("reference: store lookup failed", "url", link, "err", ferr, "library_id", lib.ID)
			metadataFailed = true
		} else {
			// A generic-scraper answer still fills blanks, but it isn't the
			// store's details, so it is reported as a failed lookup.
			meta = m
			metadataFailed = m.StoreLookupFailed
		}
	}

	var (
		asset   models.Asset
		outcome string
		batch   models.StoreLookupBatch
	)
	// recordTx records or updates the asset. It and the queued lookup commit
	// together, so a recorded asset is never left without the lookup a rerun
	// (which answers exists) would not queue again.
	recordTx := func(tx *gorm.DB) error {
		if path != "" {
			err := tx.Where("library_id = ? AND location_path = ?", lib.ID, path).First(&asset).Error
			if err == nil {
				outcome = referenceUpdated
				return updateReferenceTx(tx, lib.ID, u.ID, &asset, link, req.NeedsReview)
			}
			if !isNotFound(err) {
				return err
			}
		} else {
			// Serialise link-only records of the same link, then check again, so
			// two at once can't both pass the check above and create two assets.
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?, hashtext(?))", int32(lib.ID), link).Error; err != nil {
				return err
			}
			err := tx.Where("library_id = ? AND source_url = ?", lib.ID, link).Order("id").First(&asset).Error
			if err == nil {
				outcome = referenceExists
				return nil
			}
			if !isNotFound(err) {
				return err
			}
		}
		name := referenceName(strings.TrimSpace(req.Name), path, meta)
		if name == "" {
			return errReferenceNameRequired
		}
		outcome = referenceCreated
		asset = models.Asset{
			LibraryID:       lib.ID,
			Name:            name,
			SourceURL:       link,
			IsReference:     true,
			NeedsReview:     req.NeedsReview,
			CreatedByUserID: &u.ID,
		}
		if path != "" {
			asset.LocationPath = &path
		}
		if publisher != "" {
			asset.Publisher = &publisher
		}
		if err := tx.Create(&asset).Error; err != nil {
			return err
		}
		// GORM leaves a zero-value field with a column default out of the
		// INSERT, and needs_review defaults to true, so false is written here.
		if !req.NeedsReview {
			asset.NeedsReview = false
			if err := tx.Model(&asset).Update("needs_review", false).Error; err != nil {
				return err
			}
		}
		if err := audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.reference.create",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff: audit.Created(map[string]any{
				"name":          asset.Name,
				"location_path": path,
				"source_url":    link,
				"publisher":     publisher,
				"needs_review":  asset.NeedsReview,
			}),
		}); err != nil {
			return err
		}
		for _, t := range req.Tags {
			if _, _, err := tags.Attach(tx, lib.ID, &u.ID, asset.ID, t); err != nil {
				return err
			}
		}
		return nil
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := recordTx(tx); err != nil {
			return err
		}
		if !req.QueueLookup || link == "" || outcome == referenceExists {
			return nil
		}
		var err error
		if batch, err = lookupBatchTx(tx, lib.ID, u.ID, req.LookupBatchID, false); err != nil {
			return err
		}
		return h.queueStoreLookupTx(tx, batch, asset.ID)
	})
	switch {
	case errors.Is(err, errReferenceNameRequired):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	case isUniqueViolation(err):
		// Another request recorded the same path between the lookup and the
		// insert; recording again now updates it.
		respond409(c, "that path was recorded at the same moment; record it again to update it")
		return
	case err != nil:
		respondGORM(c, err, "asset or lookup batch not found")
		return
	}
	if meta != nil && outcome != referenceExists {
		if err := h.applyReferenceListing(ctx, &asset, meta, link, u.ID); err != nil {
			slog.Warn("reference: apply listing", "asset_id", asset.ID, "err", err)
			metadataFailed = true
		}
	}

	status := http.StatusOK
	if outcome == referenceCreated {
		status = http.StatusCreated
	}
	resp := referenceResponseFrom(outcome, &asset)
	// Only an asset this record created or updated is missing what the lookup
	// would have filled.
	resp.MetadataFailed = metadataFailed && outcome != referenceExists
	resp.LookupBatchID = batch.ID
	c.JSON(status, resp)
}

type recordedLocation struct {
	AssetID      uint   `json:"asset_id"`
	LocationPath string `json:"location_path"`
}

// getAssetLocations lists every path recorded in the library, unpaged: one
// short row per asset stays small even for thousands of packs. ?under= keeps
// only that folder and what lies beneath it, so a scan of one folder never
// reports another's paths as gone.
func (h *Handler) getAssetLocations(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	under, err := normaliseLocationPath(c.Query("under"))
	if err != nil {
		respond400(c, err)
		return
	}
	q := h.db.WithContext(c.Request.Context()).Model(&models.Asset{}).
		Select("id AS asset_id, location_path").
		Where("library_id = ? AND location_path IS NOT NULL", lib.ID)
	if under != "" && under != "/" {
		// starts_with rather than LIKE, so the `_` common in pack names isn't
		// read as a wildcard.
		q = q.Where("(location_path = ? OR starts_with(location_path, ?))", under, under+"/")
	}
	locations := []recordedLocation{}
	if err := q.Order("location_path").Scan(&locations).Error; err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"locations": locations})
}

type assetLink struct {
	AssetID        uint   `json:"asset_id"`
	Name           string `json:"name"`
	Publisher      string `json:"publisher"`
	SourceURL      string `json:"source_url"`
	Store          string `json:"store"`
	StoreProductID string `json:"store_product_id"`
}

// getAssetLinks lists every asset in the library with its store link and
// store product ID, stored and reference alike, unpaged: a purchase run
// matches its whole purchase list against the whole library at once.
func (h *Handler) getAssetLinks(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	links := []assetLink{}
	err := h.db.WithContext(c.Request.Context()).Model(&models.Asset{}).
		Select(`id AS asset_id, name, COALESCE(publisher, '') AS publisher, COALESCE(source_url, '') AS source_url,
			COALESCE(store, '') AS store, COALESCE(store_product_id, '') AS store_product_id`).
		Where("library_id = ?", lib.ID).
		Order("id").Scan(&links).Error
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"assets": links})
}

// updateReferenceTx applies a re-record of a known path: a store link given now
// replaces any earlier one, and Needs review is set when asked for. Nothing the
// user edited is touched; the store lookup that follows fills blanks only.
func updateReferenceTx(tx *gorm.DB, libID, userID uint, asset *models.Asset, link string, needsReview bool) error {
	before := map[string]any{}
	updates := map[string]any{}
	if link != "" && link != asset.SourceURL {
		before["source_url"] = asset.SourceURL
		updates["source_url"] = link
		asset.SourceURL = link
	}
	if needsReview && !asset.NeedsReview {
		before["needs_review"] = false
		updates["needs_review"] = true
		asset.NeedsReview = true
	}
	if len(updates) == 0 {
		return nil
	}
	if err := tx.Model(&models.Asset{}).Where("id = ?", asset.ID).Updates(updates).Error; err != nil {
		return err
	}
	return audit.Write(tx, audit.Entry{
		LibraryID:  &libID,
		UserID:     &userID,
		ActionType: "asset.reference.update",
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff:       audit.Updated(before, updates),
	})
}

func referenceResponseFrom(outcome string, a *models.Asset) referenceRecordResponse {
	return referenceRecordResponse{
		Outcome:      outcome,
		AssetID:      a.ID,
		Name:         a.Name,
		LocationPath: derefString(a.LocationPath),
		SourceURL:    a.SourceURL,
	}
}
