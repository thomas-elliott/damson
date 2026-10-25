package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/preview"
)

// previewQueueResponse is the snapshot shape returned by GET /api/previews —
// the per-version rollups still being processed (Previews, newest first) plus a
// retained tail of recently-completed rollups (Finished). Total counts only the
// in-flight set so the nav badge reflects outstanding work, not history.
type previewQueueResponse struct {
	Previews []preview.Event `json:"previews"`
	Finished []preview.Event `json:"finished"`
	Total    int             `json:"total"`
}

// queueFinishedWindow / queueFinishedLimit bound the retained "finished" tail
// shown at the bottom of the import and processing queues: completed rows drop
// off after a week, or once fifty newer ones push them out — whichever comes
// first.
const (
	queueFinishedWindow = 7 * 24 * time.Hour
	queueFinishedLimit  = 50
)

// getPreviewQueue returns the current library's preview rollups: the in-flight
// set (versions with any queued/processing file) and a retained tail of
// recently-finished rollups for the queue's "finished" section.
func (h *Handler) getPreviewQueue(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	events, err := preview.InFlight(c.Request.Context(), h.db, lib.ID)
	if err != nil {
		respond500(c, err)
		return
	}
	finished, err := preview.Finished(c.Request.Context(), h.db, lib.ID,
		time.Now().Add(-queueFinishedWindow), queueFinishedLimit)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, previewQueueResponse{Previews: events, Finished: finished, Total: len(events)})
}

// postAssetPreviewRequeue regenerates previews for an asset's current version:
// every supported file is reset to queued and the version is handed to the
// preview pool. Requeue respects the library's previews_enabled toggle (409 when
// off) — the same gate that governs automatic previewing on import.
func (h *Handler) postAssetPreviewRequeue(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
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
	queued, err := preview.Queue(c.Request.Context(), h.db, h.previewPool, preview.Job{LibraryID: lib.ID, VersionID: *asset.CurrentVersionID})
	if !respondPreviewQueue(c, err) {
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"asset_id":   asset.ID,
		"version_id": *asset.CurrentVersionID,
		"queued":     queued,
	})
}

// postFilePreviewRequeue regenerates the preview for a single file of an asset's
// current version. The file must belong to that version (library-gated) and
// have a backend renderer; otherwise 404 / 409.
func (h *Handler) postFilePreviewRequeue(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	fileID, ok := parseIDParam(c, "fileId")
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

	// The file must belong to this asset's current version — a cross-version or
	// cross-library id is a 404, not a silent no-op.
	var file models.AssetFile
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND asset_version_id = ?", fileID, *asset.CurrentVersionID).
		First(&file).Error; err != nil {
		respondGORM(c, err, "file not found")
		return
	}
	if !preview.Supports(file.RelativePath, file.DetectedType) {
		respond409(c, "file type has no backend preview")
		return
	}
	_, err := preview.Queue(c.Request.Context(), h.db, h.previewPool, preview.Job{LibraryID: lib.ID, VersionID: *asset.CurrentVersionID, FileID: file.ID})
	if !respondPreviewQueue(c, err) {
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"asset_id":   asset.ID,
		"version_id": *asset.CurrentVersionID,
		"file_id":    file.ID,
	})
}

// respondPreviewQueue answers a failed preview.Queue and reports whether the
// handler may go on. A library with previews off is a 409, so a requeue can't
// resurrect previews an admin turned off.
func respondPreviewQueue(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, preview.ErrDisabled):
		respond409(c, "previews are disabled for this library")
		return false
	case err != nil:
		respond500(c, err)
		return false
	}
	return true
}

// getPreviewEvents streams library-wide preview rollup transitions as SSE,
// mirroring getImportEvents: subscribe, replay the current in-flight snapshot,
// then forward events until the client disconnects.
func (h *Handler) getPreviewEvents(c *gin.Context) {
	if h.previewBroadcaster == nil {
		respond500(c, errPreviewBroadcasterNotConfigured)
		return
	}
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// Subscribe before the snapshot read so events firing during it aren't lost.
	ch, unsub := h.previewBroadcaster.Subscribe(lib.ID)
	defer unsub()

	snapshot, err := preview.InFlight(c.Request.Context(), h.db, lib.ID)
	if err != nil {
		slog.Warn("preview events: snapshot", "err", err, "library_id", lib.ID)
	}
	for _, ev := range snapshot {
		if !writePreviewSSE(c.Writer, ev) {
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
			if !writePreviewSSE(c.Writer, ev) {
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

// writePreviewSSE serialises one preview rollup as a `data:` SSE frame. Returns
// false if the writer rejected the bytes so the caller bails out of the loop.
func writePreviewSSE(w gin.ResponseWriter, ev preview.Event) bool {
	payload, err := json.Marshal(ev)
	if err != nil {
		slog.Warn("preview events: marshal", "err", err, "version_id", ev.VersionID)
		return true
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return false
	}
	return true
}
