package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/imaging"
	"github.com/thomas-elliott/damson/internal/library"
	"github.com/thomas-elliott/damson/internal/licence"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/preview"
	"github.com/thomas-elliott/damson/internal/storage"
	"github.com/thomas-elliott/damson/internal/thumbnail"
)

// rescanResponse summarises what a rescan touched so the SPA can toast the
// outcome without a second round-trip.
type rescanResponse struct {
	AssetID        uint   `json:"asset_id"`
	VersionID      uint   `json:"version_id"`
	LicenceUpdated bool   `json:"licence_updated"`
	DetectedType   string `json:"detected_type,omitempty"`
	ThumbnailFresh bool   `json:"thumbnail_regenerated"`
	PreviewsQueued int    `json:"previews_queued"`
}

// postAssetRescan re-runs the housekeeping passes over an asset's current
// version without creating a new one: a licence re-detect (non-destructive — a
// manually reviewed licence is never overwritten), a thumbnail regenerate, and
// a preview requeue. It reuses the same building blocks as import so the result
// matches a fresh upload. Each pass is best-effort: a failure in one is logged
// and the others still run.
func (h *Handler) postAssetRescan(c *gin.Context) {
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

	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).
		First(&asset).Error; err != nil {
		respondGORM(c, err, "asset not found")
		return
	}
	if asset.CurrentVersionID == nil {
		respond409(c, "asset has no current version")
		return
	}
	versionID := *asset.CurrentVersionID

	var files []models.AssetFile
	if err := h.db.WithContext(c.Request.Context()).
		Where("asset_version_id = ?", versionID).
		Find(&files).Error; err != nil {
		respond500(c, err)
		return
	}

	resp := rescanResponse{AssetID: asset.ID, VersionID: versionID}

	licUpdated, detectedType := h.rescanLicence(c, &asset, files)
	resp.LicenceUpdated = licUpdated
	resp.DetectedType = detectedType

	resp.ThumbnailFresh = h.rescanThumbnail(c, lib.ID, &asset, files)

	resp.PreviewsQueued = h.rescanPreviews(c, lib.ID, versionID)

	if err := audit.Write(h.db.WithContext(c.Request.Context()), audit.Entry{
		LibraryID:  &lib.ID,
		UserID:     &u.ID,
		ActionType: "asset.rescan",
		TargetType: "asset",
		TargetID:   &asset.ID,
		Diff: audit.Created(map[string]any{
			"licence_updated":       resp.LicenceUpdated,
			"thumbnail_regenerated": resp.ThumbnailFresh,
			"previews_queued":       resp.PreviewsQueued,
		}),
	}); err != nil {
		slog.Warn("rescan: audit write", "err", err, "asset_id", asset.ID)
	}

	c.JSON(http.StatusOK, resp)
}

// rescanLicence re-detects the licence from the current version's files and
// persists it. As an automatic write it never replaces a reviewed licence.
// Returns whether it wrote a licence and the asset's licence type.
func (h *Handler) rescanLicence(c *gin.Context, asset *models.Asset, files []models.AssetFile) (bool, string) {
	candidate := pickLicenceCandidate(files)
	if candidate == nil {
		return false, ""
	}
	blob, err := h.storage.Get(c.Request.Context(), storage.Blobs, candidate.ObjectKey)
	if err != nil {
		slog.Warn("rescan: fetch licence blob", "err", err, "object_key", candidate.ObjectKey)
		return false, ""
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
	det := licence.Detect(string(buf[:n]))
	attrsJSON, err := json.Marshal(det.Attributes)
	if err != nil {
		slog.Warn("rescan: marshal licence attributes", "err", err, "asset_id", asset.ID)
		return false, det.DetectedType
	}
	l := models.Licence{
		DetectedType:        det.DetectedType,
		DetectionConfidence: det.DetectionConfidence,
		CanonicalDiff:       det.CanonicalDiff,
		OriginalText:        string(buf[:n]),
		AttributesJSON:      attrsJSON,
	}
	var (
		prev    *models.Licence
		written bool
	)
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		prev, written, err = licence.Replace(tx, asset.ID, &l, licence.Automatic)
		return err
	})
	if err != nil {
		slog.Warn("rescan: persist licence", "err", err, "asset_id", asset.ID)
		return false, det.DetectedType
	}
	if !written {
		return false, prev.DetectedType
	}
	asset.LicenceID = &l.ID
	return true, det.DetectedType
}

// rescanThumbnail re-derives the asset's main thumbnail from the current
// version's files, working from extracted files rather than the archive so it
// holds up after the original package is purged. It first tries a fresh render
// of a directly-renderable file (image/audio/font, honouring the library
// toggles); failing that — the common Unity case, where the renderable textures
// are types the thumbnail worker doesn't decode — it reuses a file's already
// rendered preview derivative. Returns whether it set a thumbnail.
//
// A store-sourced cover is never regenerated over: the storefront cover always
// wins, so a rescan/preview requeue leaves it untouched.
func (h *Handler) rescanThumbnail(c *gin.Context, libraryID uint, asset *models.Asset, files []models.AssetFile) bool {
	if asset.ThumbnailFromStore {
		return false
	}
	settings, err := library.Settings(h.db.WithContext(c.Request.Context()), libraryID)
	if err != nil {
		slog.Warn("rescan: load settings", "err", err, "library_id", libraryID)
		return false
	}

	if candidate := pickThumbnailCandidate(files); candidate != nil {
		if blob, ferr := h.storage.Get(c.Request.Context(), storage.Blobs, candidate.ObjectKey); ferr != nil {
			slog.Warn("rescan: fetch thumbnail blob", "err", ferr, "object_key", candidate.ObjectKey)
		} else {
			gen := &thumbnail.Generator{Storage: h.storage}
			res, gerr := gen.Generate(c.Request.Context(), thumbnail.Input{
				Reader:      blob,
				Filename:    candidate.RelativePath,
				ContentType: candidate.DetectedType,
			}, thumbnail.Toggles{
				Raster: settings.ThumbnailsRasterEnabled,
				Audio:  settings.ThumbnailsAudioEnabled,
				Font:   settings.ThumbnailsFontEnabled,
			})
			blob.Close()
			if gerr != nil {
				slog.Warn("rescan: generate thumbnail", "err", gerr, "path", candidate.RelativePath)
			} else if res.ObjectKey != "" {
				return h.setThumbnail(c.Request.Context(), asset, res.ObjectKey, res.ContentType, false)
			}
		}
	}

	// Fallback: reuse a rendered preview derivative as the card thumbnail.
	if pv := pickPreviewDerivative(files); pv != nil {
		return h.setThumbnail(c.Request.Context(), asset, pv.PreviewObjectKey, pv.PreviewContentType, false)
	}
	return false
}

// setThumbnail persists the asset's thumbnail pointer (and whether it came from
// a storefront cover) and mirrors it onto the in-memory asset so a follow-on
// audit diff sees the new value.
func (h *Handler) setThumbnail(ctx context.Context, asset *models.Asset, key, contentType string, fromStore bool) bool {
	if err := h.db.WithContext(ctx).Model(asset).
		Where("id = ?", asset.ID).
		Updates(map[string]any{
			"thumbnail_object_key":   key,
			"thumbnail_content_type": contentType,
			"thumbnail_from_store":   fromStore,
		}).Error; err != nil {
		slog.Warn("rescan: persist thumbnail", "err", err, "asset_id", asset.ID)
		return false
	}
	asset.ThumbnailObjectKey = key
	asset.ThumbnailContentType = contentType
	asset.ThumbnailFromStore = fromStore
	return true
}

// rescanPreviews requeues backend previews for the current version, skipped
// quietly when the library has previews off. Returns how many files were
// queued.
func (h *Handler) rescanPreviews(c *gin.Context, libraryID, versionID uint) int {
	queued, err := preview.Queue(c.Request.Context(), h.db, h.previewPool, preview.Job{LibraryID: libraryID, VersionID: versionID})
	if err != nil && !errors.Is(err, preview.ErrDisabled) {
		slog.Warn("rescan: queue previews", "err", err, "version_id", versionID)
	}
	return queued
}

// Base scores for a thumbnail source. Higher means more useful: any image
// the decoders read (imaging.IsImage) wins over an audio or font render.
const imageThumbnailScore = 3

var nonImageThumbnailExts = map[string]int{
	".wav": 1, ".mp3": 1, ".ogg": 1, ".oga": 1,
	".ttf": 1, ".otf": 1,
}

// pickThumbnailCandidate chooses the best file to derive the asset thumbnail
// from: a renderable type, preferring rasters and preview-flavoured filenames
// (preview/icon/thumbnail/screenshot). Returns nil when nothing is renderable.
func pickThumbnailCandidate(files []models.AssetFile) *models.AssetFile {
	var best *models.AssetFile
	bestScore := 0
	for i := range files {
		f := &files[i]
		base := nonImageThumbnailExts[strings.ToLower(filepath.Ext(f.RelativePath))]
		if imaging.IsImage(f.RelativePath) {
			base = imageThumbnailScore
		}
		if base == 0 {
			continue
		}
		score := base
		name := strings.ToLower(f.RelativePath)
		if strings.Contains(name, "preview") || strings.Contains(name, "thumb") ||
			strings.Contains(name, "icon") || strings.Contains(name, "screenshot") {
			score += 2
		}
		if score > bestScore {
			bestScore = score
			best = f
		}
	}
	return best
}

// pickPreviewDerivative chooses a file whose backend preview already rendered,
// to reuse as the asset thumbnail when nothing renders directly. Prefers
// preview-flavoured filenames. Returns nil when no file has a derivative.
func pickPreviewDerivative(files []models.AssetFile) *models.AssetFile {
	var best *models.AssetFile
	bestScore := -1
	for i := range files {
		f := &files[i]
		if f.PreviewObjectKey == "" {
			continue
		}
		score := 0
		name := strings.ToLower(f.RelativePath)
		if strings.Contains(name, "preview") || strings.Contains(name, "thumb") ||
			strings.Contains(name, "icon") || strings.Contains(name, "screenshot") {
			score = 1
		}
		if score > bestScore {
			bestScore = score
			best = f
		}
	}
	return best
}
