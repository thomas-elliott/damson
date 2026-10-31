package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/ingest"
	"github.com/thomas-elliott/damson/internal/models"
)

// importRow is the wire shape returned by GET /api/imports — the
// queue/admin viewers care about stage + error + a human-readable filename.
// We build the filename from the asset's first file row when available; the
// asset name otherwise.
type importRow struct {
	ID                 uint      `json:"id"`
	AssetID            uint      `json:"asset_id"`
	AssetName          string    `json:"asset_name"`
	Stage              string    `json:"stage"`
	Error              string    `json:"error,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
	CreatedAt          time.Time `json:"created_at"`
	NeedsReview        bool      `json:"needs_review"`
	Description        string    `json:"description,omitempty"`
	ThumbnailObjectKey string    `json:"thumbnail_object_key,omitempty"`
	SourceURL          string    `json:"source_url,omitempty"`
	HasMetadata        bool      `json:"has_metadata"`
}

type importListResponse struct {
	Imports  []importRow `json:"imports"`
	Finished []importRow `json:"finished"`
	Total    int64       `json:"total"`
}

// importScanRow is the raw projection for both queue sections.
type importScanRow struct {
	ID                 uint
	AssetID            uint
	AssetName          string
	Stage              string
	Error              string
	UpdatedAt          time.Time
	CreatedAt          time.Time
	NeedsReview        bool
	Description        string
	ThumbnailObjectKey string
	SourceURL          string
	HasMetadata        bool
}

const importDescriptionChars = 400

var importRowSelect = fmt.Sprintf(
	"i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at, a.created_at, "+
		"a.needs_review, left(a.description, %d) AS description, a.thumbnail_object_key, "+
		"a.source_url, (a.source_metadata_json IS NOT NULL) AS has_metadata",
	importDescriptionChars,
)

func (r importScanRow) toRow() importRow {
	return importRow{
		ID:                 r.ID,
		AssetID:            r.AssetID,
		AssetName:          r.AssetName,
		Stage:              r.Stage,
		Error:              r.Error,
		UpdatedAt:          r.UpdatedAt,
		CreatedAt:          r.CreatedAt,
		NeedsReview:        r.NeedsReview,
		Description:        r.Description,
		ThumbnailObjectKey: r.ThumbnailObjectKey,
		SourceURL:          r.SourceURL,
		HasMetadata:        r.HasMetadata,
	}
}

// libraryImports is the tenancy-scoped base for every import query: imports
// are reached through their asset, so the library filter is a join condition
// rather than a column on the row. One definition, so the five call sites
// cannot drift apart on the one join that must not.
func libraryImports(db *gorm.DB, libraryID uint) *gorm.DB {
	return db.Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("a.library_id = ?", libraryID)
}

// getImports returns imports for the current library. The default (queue) view
// is the *review backlog*: anything still in flight, failed, or whose asset
// still needs review — regardless of import stage, so a completed import whose
// asset hasn't been reviewed stays visible. It also returns a retained tail of
// recently-finished imports (done + reviewed) for the queue's "finished"
// section. `?stage=<name>` returns that single stage; `?stage=all` returns
// everything (no finished tail in either case).
func (h *Handler) getImports(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	limit, offset := parsePagination(c)
	stageFilter := strings.TrimSpace(c.Query("stage"))

	q := libraryImports(h.db.WithContext(c.Request.Context()), lib.ID)

	switch stageFilter {
	case "":
		// Review backlog: not-done and not dismissed, or still needs review.
		q = q.Where("(i.stage <> ? AND i.dismissed_at IS NULL) OR a.needs_review", "done")
	case "all":
		// no stage filter
	default:
		q = q.Where("i.stage = ?", stageFilter)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		respond500(c, err)
		return
	}

	var rows []importScanRow
	err := q.Session(&gorm.Session{}).
		Select(importRowSelect).
		Order("i.updated_at DESC, i.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
		return
	}

	out := make([]importRow, len(rows))
	for i, r := range rows {
		out[i] = r.toRow()
	}

	resp := importListResponse{Imports: out, Total: total}
	if stageFilter == "" {
		finished, ferr := h.finishedImports(c, lib.ID)
		if ferr != nil {
			respond500(c, ferr)
			return
		}
		resp.Finished = finished
	}
	c.JSON(http.StatusOK, resp)
}

// finishedImports is the retained tail for the queue's "finished" section:
// done imports whose asset has been reviewed, touched within the retention
// window, newest first, capped.
func (h *Handler) finishedImports(c *gin.Context, libraryID uint) ([]importRow, error) {
	var rows []importScanRow
	err := libraryImports(h.db.WithContext(c.Request.Context()), libraryID).
		Where("i.stage = ? AND NOT a.needs_review AND i.updated_at >= ?",
			"done", time.Now().Add(-queueFinishedWindow)).
		Select(importRowSelect).
		Order("i.updated_at DESC").
		Limit(queueFinishedLimit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]importRow, len(rows))
	for i, r := range rows {
		out[i] = r.toRow()
	}
	return out, nil
}

// getImport returns one import row by id, library-gated via the linked asset.
func (h *Handler) getImport(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var r importScanRow
	err := libraryImports(h.db.WithContext(c.Request.Context()), lib.ID).
		Where("i.id = ?", id).
		Select(importRowSelect).
		Scan(&r).Error
	if err != nil {
		respond500(c, err)
		return
	}
	if r.ID == 0 {
		respond404(c, "import not found")
		return
	}
	c.JSON(http.StatusOK, r.toRow())
}

// postImportRetry re-runs the import pipeline against the failed import's
// retained source blob. Only `failed` rows can be retried; `done` rows are
// rejected with 409. A retry re-extracts into the same version in place and
// creates only a fresh asset_imports row in the `uploaded` stage, which it
// hands to the worker pool.
func (h *Handler) postImportRetry(c *gin.Context) {
	if h.storage == nil {
		respond500(c, errStorageNotConfigured)
		return
	}
	if h.importPool == nil {
		respond500(c, errImportPoolNotConfigured)
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

	var imp models.AssetImport
	err := h.db.WithContext(c.Request.Context()).First(&imp, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond404(c, "import not found")
		return
	}
	if err != nil {
		respond500(c, err)
		return
	}
	var asset models.Asset
	if err := h.db.WithContext(c.Request.Context()).First(&asset, imp.AssetID).Error; err != nil {
		respond500(c, err)
		return
	}
	if asset.LibraryID != lib.ID {
		respond404(c, "import not found")
		return
	}
	if imp.Stage != ingest.StageFailed {
		respond409(c, "import is not in failed state")
		return
	}

	// Locate a source blob from the failed import's own version.
	// Unity uploads always retain the original archive on the version row;
	// loose uploads need an asset_files row to recover the bytes.
	var version models.AssetVersion
	verr := gorm.ErrRecordNotFound
	if imp.AssetVersionID != nil {
		verr = h.db.WithContext(c.Request.Context()).
			Where("id = ? AND asset_id = ?", *imp.AssetVersionID, asset.ID).
			First(&version).Error
	}
	if verr != nil && !errors.Is(verr, gorm.ErrRecordNotFound) {
		respond500(c, verr)
		return
	}

	var sourceKey string
	if verr == nil && version.PackageKind == models.PackageKindUnityPackage {
		if version.HasDownloadableOriginal() {
			sourceKey = version.OriginalPackageObjectKey
		}
	} else if verr == nil {
		// Loose case: try the version's retained key first, then fall back
		// to the first asset_file row.
		if version.OriginalPackageObjectKey != "" {
			sourceKey = version.OriginalPackageObjectKey
		} else {
			var f models.AssetFile
			if err := h.db.WithContext(c.Request.Context()).
				Where("asset_version_id = ?", version.ID).
				First(&f).Error; err == nil {
				sourceKey = f.ObjectKey
			}
		}
	}
	if sourceKey == "" {
		respond409(c, "no source blob to retry")
		return
	}

	var newImportID uint
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Retry re-imports into the failed import's version — the pipeline
		// re-extracts into it in place, so no new version row is created.
		if version.OriginalPackageObjectKey == "" {
			if err := tx.Model(&models.AssetVersion{}).
				Where("id = ?", version.ID).
				Update("original_package_object_key", sourceKey).Error; err != nil {
				return err
			}
		}
		nextImp := models.AssetImport{
			AssetID:        asset.ID,
			AssetVersionID: &version.ID,
			Stage:          ingest.StageUploaded,
			UpdatedAt:      time.Now().UTC(),
		}
		if err := tx.Create(&nextImp).Error; err != nil {
			return err
		}
		newImportID = nextImp.ID
		if err := audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "asset.import.retry",
			TargetType: "asset",
			TargetID:   &asset.ID,
			Diff:       audit.Created(map[string]any{"import_id": nextImp.ID, "retried_import_id": imp.ID}),
		}); err != nil {
			return err
		}
		return h.importPool.Enqueue(tx, ingest.Job{ImportID: newImportID, LibraryID: lib.ID})
	})
	if err != nil {
		respond500(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"asset_id":  asset.ID,
		"import_id": newImportID,
	})
}

// getImportEvents streams library-wide import stage transitions to the
// caller as Server-Sent Events. On connect the handler emits a snapshot of
// every non-terminal import row so a fresh client paints the live state
// without waiting for the next transition; thereafter it forwards events
// from the broadcaster until the client disconnects.
func (h *Handler) getImportEvents(c *gin.Context) {
	if h.broadcaster == nil {
		respond500(c, errBroadcasterNotConfigured)
		return
	}
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	streamImportEvents(c, h, lib.ID)
}

// sseHeartbeat is the interval between SSE comment frames. Keeps proxies
// (nginx, traefik) from idling out the connection. Plenty of slack —
// neither stage transitions nor heartbeats are time-sensitive.
const sseHeartbeat = 25 * time.Second

type serverStoppingKey struct{}

// WithServerStopping returns ctx carrying stopping, whose Done the SSE streams
// watch so they end when the server shuts down. Pass the result as the
// http.Server's BaseContext. It is a value, not a parent: cancelling stopping
// ends the streams and leaves every other request to finish.
func WithServerStopping(ctx, stopping context.Context) context.Context {
	return context.WithValue(ctx, serverStoppingKey{}, stopping)
}

// serverStopping is closed once the server begins to shut down, and nil (never
// ready) when the server set no stopping context, as in tests.
func serverStopping(ctx context.Context) <-chan struct{} {
	if stopping, ok := ctx.Value(serverStoppingKey{}).(context.Context); ok {
		return stopping.Done()
	}
	return nil
}

func streamImportEvents(c *gin.Context, h *Handler, libraryID uint) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// Subscribe before replay so events that fire during the snapshot read
	// don't fall in a gap.
	ch, unsub := h.broadcaster.Subscribe(libraryID)
	defer unsub()

	snapshot, err := ingest.ReplaySnapshot(c.Request.Context(), h.db, libraryID)
	if err != nil {
		slog.Warn("import events: replay snapshot", "err", err, "library_id", libraryID)
	}
	for _, ev := range snapshot {
		if !writeSSEEvent(c.Writer, ev) {
			return
		}
	}
	c.Writer.Flush()

	ticker := time.NewTicker(sseHeartbeat)
	defer ticker.Stop()
	ctx := c.Request.Context()
	stopping := serverStopping(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopping:
			return
		case ev := <-ch:
			if !writeSSEEvent(c.Writer, ev) {
				return
			}
			c.Writer.Flush()
		case <-ticker.C:
			if _, err := io.WriteString(c.Writer, ": keepalive\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

// writeSSEEvent serializes one import event as a `data:` SSE frame.
// Returns false if the underlying writer rejected the bytes — callers
// should bail out of the stream loop on false.
func writeSSEEvent(w gin.ResponseWriter, ev ingest.Event) bool {
	payload, err := json.Marshal(ev)
	if err != nil {
		slog.Warn("import events: marshal", "err", err, "import_id", ev.ID)
		return true // skip this one, don't tear down the stream
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return false
	}
	return true
}
