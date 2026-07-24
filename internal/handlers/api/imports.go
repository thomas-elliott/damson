package api

import (
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

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/imports"
)

// importRow is the wire shape returned by GET /api/imports — the
// queue/admin viewers care about stage + error + a human-readable filename.
// We build the filename from the asset's first file row when available; the
// asset name otherwise.
type importRow struct {
	ID          uint      `json:"id"`
	AssetID     uint      `json:"asset_id"`
	AssetName   string    `json:"asset_name"`
	Stage       string    `json:"stage"`
	Error       string    `json:"error,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
	NeedsReview bool      `json:"needs_review"`
}

type importListResponse struct {
	Imports  []importRow `json:"imports"`
	Finished []importRow `json:"finished"`
	Total    int64       `json:"total"`
}

// importScanRow is the raw projection for both queue sections.
type importScanRow struct {
	ID          uint
	AssetID     uint
	AssetName   string
	Stage       string
	Error       string
	UpdatedAt   time.Time
	CreatedAt   time.Time
	NeedsReview bool
}

const importRowSelect = "i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at, a.created_at, a.needs_review"

func (r importScanRow) toRow() importRow {
	return importRow{
		ID:          r.ID,
		AssetID:     r.AssetID,
		AssetName:   r.AssetName,
		Stage:       r.Stage,
		Error:       r.Error,
		UpdatedAt:   r.UpdatedAt,
		CreatedAt:   r.CreatedAt,
		NeedsReview: r.NeedsReview,
	}
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

	q := h.db.WithContext(c.Request.Context()).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("a.library_id = ?", lib.ID)

	switch stageFilter {
	case "":
		// Review backlog: not-done, or done-but-still-needs-review.
		q = q.Where("i.stage <> ? OR a.needs_review", "done")
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
		Order("i.updated_at DESC").
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
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("a.library_id = ? AND i.stage = ? AND NOT a.needs_review AND i.updated_at >= ?",
			libraryID, "done", time.Now().Add(-queueFinishedWindow)).
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

	type row struct {
		ID          uint
		AssetID     uint
		AssetName   string
		Stage       string
		Error       string
		UpdatedAt   time.Time
		CreatedAt   time.Time
		NeedsReview bool
	}
	var r row
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_imports AS i").
		Joins("JOIN assets a ON a.id = i.asset_id").
		Where("i.id = ? AND a.library_id = ?", id, lib.ID).
		Select("i.id, i.asset_id, a.name AS asset_name, i.stage, i.error, i.updated_at, a.created_at, a.needs_review").
		Scan(&r).Error
	if err != nil {
		respond500(c, err)
		return
	}
	if r.ID == 0 {
		respond404(c, "import not found")
		return
	}
	c.JSON(http.StatusOK, importRow{
		ID:          r.ID,
		AssetID:     r.AssetID,
		AssetName:   r.AssetName,
		Stage:       r.Stage,
		Error:       r.Error,
		UpdatedAt:   r.UpdatedAt,
		CreatedAt:   r.CreatedAt,
		NeedsReview: r.NeedsReview,
	})
}

// postImportRetry re-runs the import pipeline against the previous import's
// retained source blob. Only `failed` rows can be retried; `done` rows are
// rejected with 409. A retry creates a brand-new asset_version (label v2,
// v3, …) plus a fresh asset_imports row in the `uploaded` stage and hands
// it to the worker pool — same code path as a first-time upload.
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
	if imp.Stage != imports.StageFailed {
		respond409(c, "import is not in failed state")
		return
	}

	// Locate a source blob from the failed import's most-recent version.
	// Unity uploads always retain the original archive on the version row;
	// loose uploads need an asset_files row to recover the bytes.
	var version models.AssetVersion
	verr := h.db.WithContext(c.Request.Context()).
		Where("asset_id = ?", asset.ID).
		Order("import_date DESC").
		First(&version).Error
	if verr != nil && !errors.Is(verr, gorm.ErrRecordNotFound) {
		respond500(c, verr)
		return
	}

	var sourceKey string
	if verr == nil && version.PackageKind == packageKindUnityPackage {
		if version.OriginalPackageObjectKey != "" && !version.OriginalPackagePurged {
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
		if version.OriginalPackageObjectKey == "" {
			if err := tx.Model(&models.AssetVersion{}).
				Where("id = ?", version.ID).
				Update("original_package_object_key", sourceKey).Error; err != nil {
				return err
			}
		}
		nextImp := models.AssetImport{
			AssetID:   asset.ID,
			Stage:     imports.StageUploaded,
			UpdatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&nextImp).Error; err != nil {
			return err
		}
		newImportID = nextImp.ID
		return nil
	})
	if err != nil {
		respond500(c, err)
		return
	}

	h.importPool.Enqueue(imports.Job{ImportID: newImportID, LibraryID: lib.ID})

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
	streamImportEvents(c, h, lib.ID, 0)
}

// getImportEventsByID is the single-import variant of getImportEvents.
// Useful for the asset detail panel to follow one upload from queued to
// done without subscribing to the entire library feed.
func (h *Handler) getImportEventsByID(c *gin.Context) {
	if h.broadcaster == nil {
		respond500(c, errBroadcasterNotConfigured)
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
	// Library-gate the import id up front so we don't open a stream that
	// will only ever receive other libraries' events.
	_, found, err := imports.LoadEvent(c.Request.Context(), h.db, lib.ID, id)
	if err != nil {
		respond500(c, err)
		return
	}
	if !found {
		respond404(c, "import not found")
		return
	}
	streamImportEvents(c, h, lib.ID, id)
}

// sseHeartbeat is the interval between SSE comment frames. Keeps proxies
// (nginx, traefik) from idling out the connection. Plenty of slack —
// neither stage transitions nor heartbeats are time-sensitive.
const sseHeartbeat = 25 * time.Second

func streamImportEvents(c *gin.Context, h *Handler, libraryID, filterImportID uint) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// Subscribe before replay so events that fire during the snapshot read
	// don't fall in a gap.
	ch, unsub := h.broadcaster.Subscribe(libraryID)
	defer unsub()

	snapshot, err := imports.ReplaySnapshot(c.Request.Context(), h.db, libraryID)
	if err != nil {
		slog.Warn("import events: replay snapshot", "err", err, "library_id", libraryID)
	}
	for _, ev := range snapshot {
		if filterImportID != 0 && ev.ID != filterImportID {
			continue
		}
		if !writeSSEEvent(c.Writer, ev) {
			return
		}
	}
	c.Writer.Flush()

	ticker := time.NewTicker(sseHeartbeat)
	defer ticker.Stop()
	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if filterImportID != 0 && ev.ID != filterImportID {
				continue
			}
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
func writeSSEEvent(w gin.ResponseWriter, ev imports.Event) bool {
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
