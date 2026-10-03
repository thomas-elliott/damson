package api

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// Outcomes of recording an asset by its location.
const (
	referenceCreated = "created"
	referenceUpdated = "updated"
	referenceExists  = "exists"
)

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
}

type referenceRecordResponse struct {
	Outcome      string `json:"outcome"`
	AssetID      uint   `json:"asset_id"`
	Name         string `json:"name"`
	LocationPath string `json:"location_path,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
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
	if link != "" && h.metadata != nil {
		if m, ferr := h.metadata.Fetch(ctx, link); ferr != nil {
			slog.Warn("reference: store lookup failed", "url", link, "err", ferr, "library_id", lib.ID)
		} else {
			meta = m
		}
	}

	var (
		asset   models.Asset
		outcome string
	)
	err = db.Transaction(func(tx *gorm.DB) error {
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
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.reference.create",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff: audit.Created(map[string]any{
				"name":          asset.Name,
				"location_path": path,
				"source_url":    link,
				"needs_review":  asset.NeedsReview,
			}),
		})
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
		respondGORM(c, err, "asset not found")
		return
	}

	if meta != nil && outcome != referenceExists {
		h.applyMetadata(c, lib.ID, &asset, meta, nil, false, "", u.ID)
		h.persistSourceRecord(c, &asset, metadata.SourceRecord{
			Store:      meta.Store,
			ListingURL: firstNonEmptyStr(meta.ListingURL, link),
			FetchedAt:  time.Now().UTC(),
			Matched:    true,
			Metadata:   meta,
		})
	}

	status := http.StatusOK
	if outcome == referenceCreated {
		status = http.StatusCreated
	}
	c.JSON(status, referenceResponseFrom(outcome, &asset))
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

// isUniqueViolation reports whether err is Postgres's unique_violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
