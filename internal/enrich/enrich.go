// Package enrich applies a storefront listing to an asset. The import's
// enrichment stage, a person's metadata fetch and apply, recording a reference
// and background store lookups all write through a Plan.
package enrich

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"maps"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/licence"
	"github.com/thomas-elliott/damson/internal/metadata"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/storage"
	"github.com/thomas-elliott/damson/internal/tags"
	"github.com/thomas-elliott/damson/internal/thumbnail"
)

// Conflict names a field whose fetched value differs from the asset's current
// value, so overwriting it needs a person to confirm it.
type Conflict struct {
	Field    string `json:"field"`
	Current  string `json:"current"`
	Proposed string `json:"proposed"`
}

// ImageFetcher downloads a listing's cover and gallery images.
type ImageFetcher interface {
	FetchImage(ctx context.Context, rawURL string) ([]byte, string, error)
}

// Deps are what applying a listing needs. Without Storage or Images, the
// cover and gallery are left alone.
type Deps struct {
	DB      *gorm.DB
	Storage storage.Service
	Images  ImageFetcher
}

// Options say how a listing may change the asset.
type Options struct {
	// Confirm names the fields a person chose to overwrite. Every other field
	// is filled only when blank, and a differing value is a Conflict.
	Confirm map[string]bool
	// Refresh confirms every field a person hasn't changed since the last
	// lookup, in place of Confirm, and marks the asset Needs review when it
	// changes one.
	Refresh bool
	// SourceURL, when set, becomes the asset's store link.
	SourceURL string
	// UserID is who the change is audited under. A system apply (nil) is left
	// to its caller to audit.
	UserID *uint
	// By says who writes the licence; see licence.Replace.
	By licence.Writer
}

// Result is what an apply changed, and what it kept because the asset
// already held a different value.
type Result struct {
	Applied   []string
	Conflicts []Conflict
}

// Plan is a listing ready to apply. Prepare does the reads and downloads, so
// Write only writes, inside the caller's transaction.
type Plan struct {
	asset   *models.Asset
	meta    *metadata.Metadata
	opts    Options
	prev    *metadata.SourceRecord
	confirm map[string]bool

	before, updates map[string]any
	applied         []string
	conflicts       []Conflict

	licence *models.Licence
	cover   *thumbnail.Result
	gallery []models.AssetImage
	tags    []string
}

// Prepare works out what meta changes on the asset, and downloads the cover
// and gallery images it will store. It updates asset to the values it will
// write.
func Prepare(ctx context.Context, deps Deps, asset *models.Asset, meta *metadata.Metadata, opts Options) (*Plan, error) {
	p := &Plan{
		asset: asset, meta: meta, opts: opts,
		prev:    SourceRecordOf(asset),
		confirm: opts.Confirm,
		before:  map[string]any{}, updates: map[string]any{},
		applied: []string{}, conflicts: []Conflict{},
	}
	apply := meta
	if opts.Refresh {
		var err error
		if p.confirm, err = refreshConfirm(ctx, deps.DB, asset); err != nil {
			return nil, err
		}
		apply = withoutUnchangedCover(asset, meta)
	}

	if opts.SourceURL != "" && opts.SourceURL != asset.SourceURL {
		p.set("source_url", asset.SourceURL, opts.SourceURL)
		asset.SourceURL = opts.SourceURL
	}
	// Unlike the fill-if-blank fields below, a name is never blank, so a
	// differing store title is always a confirmable conflict, never a silent fill.
	if apply.Title != "" && asset.Name != apply.Title {
		if p.confirm["name"] {
			p.set("name", asset.Name, apply.Title)
			asset.Name = apply.Title
		} else {
			p.conflicts = append(p.conflicts, Conflict{Field: "name", Current: asset.Name, Proposed: apply.Title})
		}
	}
	if apply.Description != "" && asset.Description != apply.Description {
		if asset.Description == "" || p.confirm["description"] {
			p.set("description", asset.Description, apply.Description)
			asset.Description = apply.Description
		} else {
			p.conflicts = append(p.conflicts, Conflict{Field: "description", Current: asset.Description, Proposed: apply.Description})
		}
	}
	fieldBefore, fieldUpdates, fieldConflicts := fieldUpdates(asset, apply, p.confirm)
	p.conflicts = append(p.conflicts, fieldConflicts...)
	maps.Copy(p.before, fieldBefore)
	for k, v := range fieldUpdates {
		p.updates[k] = v
		p.applied = append(p.applied, k)
	}

	p.licence = listingLicence(apply)
	if apply.CoverURL != "" {
		if asset.ThumbnailObjectKey != "" && !p.confirm["thumbnail"] {
			p.conflicts = append(p.conflicts, Conflict{Field: "thumbnail", Current: "existing thumbnail", Proposed: apply.CoverURL})
		} else {
			p.cover = fetchCover(ctx, deps, apply.CoverURL)
		}
	}
	gallery, err := fetchGallery(ctx, deps, asset.ID, apply.ImageURLs)
	if err != nil {
		return nil, err
	}
	p.gallery = gallery
	p.tags = tagList(apply)
	return p, nil
}

func (p *Plan) set(column string, was, now any) {
	p.before[column] = was
	p.updates[column] = now
	p.applied = append(p.applied, column)
}

// Write applies the plan inside tx. Any error means the caller's transaction
// must roll back: nothing it wrote stands.
func (p *Plan) Write(tx *gorm.DB) (Result, error) {
	asset := p.asset
	auditBefore, auditAfter := map[string]any{}, map[string]any{}
	if len(p.updates) > 0 {
		if err := tx.Model(&models.Asset{}).Where("id = ?", asset.ID).Updates(p.updates).Error; err != nil {
			return Result{}, err
		}
		maps.Copy(auditBefore, p.before)
		maps.Copy(auditAfter, p.updates)
	}

	if p.licence != nil {
		written, prevType, conflict, err := p.writeLicence(tx)
		switch {
		case err != nil:
			return Result{}, err
		case conflict != nil:
			p.conflicts = append(p.conflicts, *conflict)
		case written:
			p.applied = append(p.applied, "licence")
			auditBefore["licence"] = prevType
			auditAfter["licence"] = p.licence.DetectedType
		}
	}

	if p.cover != nil {
		auditBefore["thumbnail_object_key"] = asset.ThumbnailObjectKey
		if err := tx.Model(&models.Asset{}).Where("id = ?", asset.ID).Updates(map[string]any{
			"thumbnail_object_key":   p.cover.ObjectKey,
			"thumbnail_content_type": p.cover.ContentType,
			"thumbnail_from_store":   true,
		}).Error; err != nil {
			return Result{}, err
		}
		asset.ThumbnailObjectKey, asset.ThumbnailContentType, asset.ThumbnailFromStore = p.cover.ObjectKey, p.cover.ContentType, true
		p.applied = append(p.applied, "thumbnail")
		auditAfter["thumbnail_object_key"] = asset.ThumbnailObjectKey
	}

	if len(p.gallery) > 0 {
		if err := tx.Create(&p.gallery).Error; err != nil {
			return Result{}, err
		}
	}

	linked := false
	for _, t := range p.tags {
		_, added, err := tags.Attach(tx, asset.LibraryID, p.opts.UserID, asset.ID, t)
		if err != nil {
			return Result{}, err
		}
		linked = linked || added
	}
	if linked {
		p.applied = append(p.applied, "tags")
	}

	if p.opts.Refresh && len(p.applied) > 0 && !asset.NeedsReview {
		if err := tx.Model(&models.Asset{}).Where("id = ?", asset.ID).Update("needs_review", true).Error; err != nil {
			return Result{}, err
		}
		asset.NeedsReview = true
	}

	if len(auditAfter) > 0 && p.opts.UserID != nil {
		if err := audit.Write(tx, audit.Entry{
			LibraryID:  &asset.LibraryID,
			UserID:     p.opts.UserID,
			ActionType: "asset.metadata.apply",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Updated(auditBefore, auditAfter),
		}); err != nil {
			return Result{}, err
		}
	}
	return Result{Applied: p.applied, Conflicts: p.conflicts}, nil
}

// writeLicence gives the asset the listing's licence when it has none, or
// when a person confirmed replacing a different one. A differing licence it
// may not replace comes back as a conflict, as does a reviewed licence that
// licence.Replace keeps from an automatic write.
func (p *Plan) writeLicence(tx *gorm.DB) (written bool, prevType string, conflict *Conflict, err error) {
	asset, lic := p.asset, p.licence
	if asset.LicenceID != nil {
		var existing models.Licence
		switch err := tx.First(&existing, *asset.LicenceID).Error; {
		case errors.Is(err, gorm.ErrRecordNotFound):
		case err != nil:
			return false, "", nil, err
		case existing.DetectedType == lic.DetectedType:
			return false, "", nil, nil
		case !p.confirm["licence"]:
			return false, "", &Conflict{Field: "licence", Current: existing.DetectedType, Proposed: lic.DetectedType}, nil
		}
	}
	prev, written, err := licence.Replace(tx, asset.ID, lic, p.opts.By)
	if err != nil {
		return false, "", nil, err
	}
	if prev != nil {
		prevType = prev.DetectedType
	}
	if !written {
		return false, "", &Conflict{Field: "licence", Current: prevType, Proposed: lic.DetectedType}, nil
	}
	asset.LicenceID = &lic.ID
	return true, prevType, nil, nil
}

// Apply prepares and writes meta in one transaction. On an error nothing is
// written.
func Apply(ctx context.Context, deps Deps, asset *models.Asset, meta *metadata.Metadata, opts Options) (Result, error) {
	p, err := Prepare(ctx, deps, asset, meta, opts)
	if err != nil {
		return Result{}, err
	}
	var res Result
	err = deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res, err = p.Write(tx)
		return err
	})
	return res, err
}

// fetchCover downloads a listing's cover and stores it as a thumbnail through
// the raster pipeline, whose decode and re-encode refuse a body that is not an
// image. It returns nil when there is no usable cover.
func fetchCover(ctx context.Context, deps Deps, coverURL string) *thumbnail.Result {
	if deps.Storage == nil || deps.Images == nil {
		return nil
	}
	imgBytes, contentType, err := deps.Images.FetchImage(ctx, coverURL)
	if err != nil {
		slog.Warn("enrich: fetch cover", "err", err, "url", coverURL)
		return nil
	}
	gen := &thumbnail.Generator{Storage: deps.Storage}
	res, err := gen.Generate(ctx, thumbnail.Input{
		Reader:      bytes.NewReader(imgBytes),
		Filename:    "cover",
		ContentType: contentType,
	}, thumbnail.Toggles{Raster: true})
	if err != nil {
		slog.Warn("enrich: render cover", "err", err, "url", coverURL)
		return nil
	}
	if res.ObjectKey == "" {
		return nil
	}
	return &res
}

// ConflictFields names the fields in conflicts.
func ConflictFields(conflicts []Conflict) []string {
	fields := make([]string, len(conflicts))
	for i, c := range conflicts {
		fields[i] = c.Field
	}
	return fields
}
