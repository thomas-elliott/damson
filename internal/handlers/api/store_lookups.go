package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// errStoreLookupsUnavailable is the 503 for a server with no background queue.
const errStoreLookupsUnavailable = "background store lookups not configured"

type storeLookupsRequest struct {
	// AssetIDs names the assets to look up. When empty, the request's query
	// string selects them with the same filters as GET /api/assets.
	AssetIDs []uint `json:"asset_ids"`
	// Refresh lets each lookup replace fields a person hasn't changed. Without
	// it a lookup fills blanks only.
	Refresh bool `json:"refresh"`
	// BatchID adds the lookups to an existing batch, so one run's lookups can
	// be followed together.
	BatchID uint `json:"batch_id"`
}

// storeLookupProblem is one lookup the caller should hear about: it failed,
// only the generic scraper answered, or it kept a field a person had changed.
type storeLookupProblem struct {
	AssetID   uint               `json:"asset_id"`
	Name      string             `json:"name"`
	Status    string             `json:"status"`
	Result    string             `json:"result,omitempty"`
	Error     string             `json:"error,omitempty"`
	Conflicts []metadataConflict `json:"conflicts,omitempty"`
}

type storeLookupBatchResponse struct {
	BatchID     uint                 `json:"batch_id"`
	Refresh     bool                 `json:"refresh"`
	Total       int                  `json:"total"`
	Queued      int                  `json:"queued"`
	Done        int                  `json:"done"`
	Failed      int                  `json:"failed"`
	Changed     int                  `json:"changed"`
	Unchanged   int                  `json:"unchanged"`
	NoStoreData int                  `json:"no_store_data"`
	Conflicted  int                  `json:"conflicted"`
	Problems    []storeLookupProblem `json:"problems"`
	// SkippedNoLink counts assets the request named that have no store link,
	// so nothing was queued for them. Only the queuing response carries it.
	SkippedNoLink int `json:"skipped_no_link,omitempty"`
}

// postStoreLookups queues a background store lookup for each named or
// matching asset that has a store link, and answers straight away with the
// batch's counts. The server runs the lookups one at a time.
func (h *Handler) postStoreLookups(c *gin.Context) {
	if h.storeLookups == nil || h.metadata == nil {
		respond503(c, errStoreLookupsUnavailable)
		return
	}
	lib := getLibrary(c)
	u := auth.GetUser(c)
	if lib == nil || u == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	var req storeLookupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	db := h.db.WithContext(c.Request.Context())

	// Every matching asset, with whether it has a link to look up.
	type candidate struct {
		ID        uint
		SourceURL string
	}
	filters := parseAssetFilters(c)
	filters.Sort = "" // an order is not a filter
	var found []candidate
	switch {
	case len(req.AssetIDs) > 0:
		if err := db.Model(&models.Asset{}).Select("id, source_url").
			Where("library_id = ? AND id IN ?", lib.ID, req.AssetIDs).Order("id").Scan(&found).Error; err != nil {
			respond500(c, err)
			return
		}
		if len(found) != len(uniqueIDs(req.AssetIDs)) {
			respond404(c, "asset not found")
			return
		}
	case filters != (assetListFilters{}):
		q := h.applyAssetFilters(db, lib.ID, filters)
		if err := q.Select("a.id, a.source_url").Order("a.id").Scan(&found).Error; err != nil {
			respond500(c, err)
			return
		}
	default:
		// Without this, an empty request would look up the whole library.
		respond400(c, asError("give asset_ids, or at least one filter in the query string"))
		return
	}

	var (
		batch   models.StoreLookupBatch
		queued  []uint
		skipped int
	)
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		if batch, err = lookupBatchTx(tx, lib.ID, u.ID, req.BatchID, req.Refresh); err != nil {
			return err
		}
		for _, a := range found {
			if a.SourceURL == "" {
				skipped++
				continue
			}
			id, err := createStoreLookupTx(tx, batch, a.ID)
			if err != nil {
				return err
			}
			queued = append(queued, id)
		}
		return nil
	})
	if err != nil {
		respondGORM(c, err, "batch not found")
		return
	}
	for _, id := range queued {
		h.storeLookups.EnqueueStoreLookup(id)
	}

	resp, err := h.storeLookupBatchStatus(c.Request.Context(), batch)
	if err != nil {
		respond500(c, err)
		return
	}
	resp.SkippedNoLink = skipped
	c.JSON(http.StatusAccepted, resp)
}

// lookupBatchTx returns the batch to add lookups to: the one named, which
// must be the library's and of the same kind, or a new one.
func lookupBatchTx(tx *gorm.DB, libID, userID, batchID uint, refresh bool) (models.StoreLookupBatch, error) {
	var batch models.StoreLookupBatch
	if batchID != 0 {
		if err := tx.Where("id = ? AND library_id = ?", batchID, libID).First(&batch).Error; err != nil {
			return batch, err
		}
		if batch.Refresh != refresh {
			return batch, errBadRequestf("batch %d is a %s batch", batchID, lookupKind(batch.Refresh))
		}
		return batch, nil
	}
	batch = models.StoreLookupBatch{LibraryID: libID, CreatedByUserID: &userID, Refresh: refresh}
	return batch, tx.Create(&batch).Error
}

func lookupKind(refresh bool) string {
	if refresh {
		return "refresh"
	}
	return "fill-blanks"
}

// createStoreLookupTx adds one queued lookup to the batch. The caller enqueues
// its job after the transaction commits, so the job never runs ahead of it.
func createStoreLookupTx(tx *gorm.DB, batch models.StoreLookupBatch, assetID uint) (uint, error) {
	lk := models.StoreLookup{BatchID: batch.ID, LibraryID: batch.LibraryID, AssetID: assetID, Status: models.StoreLookupQueued}
	if err := tx.Create(&lk).Error; err != nil {
		return 0, err
	}
	return lk.ID, nil
}

func uniqueIDs(ids []uint) map[uint]bool {
	out := make(map[uint]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// getStoreLookupBatch reports how far a batch has got.
func (h *Handler) getStoreLookupBatch(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var batch models.StoreLookupBatch
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).First(&batch).Error; err != nil {
		respondGORM(c, err, "batch not found")
		return
	}
	resp, err := h.storeLookupBatchStatus(c.Request.Context(), batch)
	if err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) storeLookupBatchStatus(ctx context.Context, batch models.StoreLookupBatch) (storeLookupBatchResponse, error) {
	type row struct {
		AssetID   uint
		Name      string
		Status    string
		Result    string
		Error     string
		Conflicts json.RawMessage
	}
	var rows []row
	err := h.db.WithContext(ctx).Table("store_lookups AS l").
		Select("l.asset_id, a.name, l.status, l.result, l.error, l.conflicts").
		Joins("JOIN assets a ON a.id = l.asset_id").
		Where("l.batch_id = ? AND l.library_id = ?", batch.ID, batch.LibraryID).
		Order("l.id").Scan(&rows).Error
	if err != nil {
		return storeLookupBatchResponse{}, err
	}
	resp := storeLookupBatchResponse{BatchID: batch.ID, Refresh: batch.Refresh, Total: len(rows), Problems: []storeLookupProblem{}}
	for _, r := range rows {
		var conflicts []metadataConflict
		_ = json.Unmarshal(r.Conflicts, &conflicts)
		switch r.Status {
		case models.StoreLookupQueued:
			resp.Queued++
		case models.StoreLookupFailed:
			resp.Failed++
		case models.StoreLookupDone:
			resp.Done++
		}
		switch r.Result {
		case models.StoreLookupChanged:
			resp.Changed++
		case models.StoreLookupUnchanged:
			resp.Unchanged++
		case models.StoreLookupNoStoreData:
			resp.NoStoreData++
		}
		if len(conflicts) > 0 {
			resp.Conflicted++
		}
		if r.Status == models.StoreLookupFailed || r.Result == models.StoreLookupNoStoreData || len(conflicts) > 0 {
			resp.Problems = append(resp.Problems, storeLookupProblem{
				AssetID: r.AssetID, Name: r.Name, Status: r.Status, Result: r.Result, Error: r.Error, Conflicts: conflicts,
			})
		}
	}
	return resp, nil
}

// RunStoreLookup runs one queued lookup: it looks the asset's store link up,
// waiting out any pause the store asks for, applies what it found, saves the
// source record and records the outcome on the row. An error is returned only
// for what a retry could fix; a lookup the store can't answer is recorded as
// failed and not retried.
func (h *Handler) RunStoreLookup(ctx context.Context, lookupID uint) error {
	db := h.db.WithContext(ctx)
	var lk models.StoreLookup
	if err := db.First(&lk, lookupID).Error; err != nil {
		if isNotFound(err) {
			return nil // the asset or batch was deleted since
		}
		return err
	}
	if lk.Status != models.StoreLookupQueued {
		return nil
	}
	var batch models.StoreLookupBatch
	if err := db.First(&batch, lk.BatchID).Error; err != nil {
		return err
	}
	var asset models.Asset
	if err := db.Where("id = ? AND library_id = ?", lk.AssetID, lk.LibraryID).First(&asset).Error; err != nil {
		return err
	}
	if asset.SourceURL == "" {
		return h.finishStoreLookup(ctx, &lk, models.StoreLookupFailed, "", nil, nil, "the asset has no store link")
	}
	if h.metadata == nil {
		return fmt.Errorf("store lookup %d: no metadata service", lk.ID)
	}

	meta, err := h.metadata.FetchWaiting(ctx, asset.SourceURL)
	if err != nil {
		if ctx.Err() != nil {
			return err // shut down or timed out: River tries again
		}
		slog.Warn("store lookup failed", "lookup_id", lk.ID, "asset_id", asset.ID, "url", asset.SourceURL, "err", err)
		return h.finishStoreLookup(ctx, &lk, models.StoreLookupFailed, "", nil, nil, err.Error())
	}

	var userID uint
	if batch.CreatedByUserID != nil {
		userID = *batch.CreatedByUserID
	}
	applied, conflicts := h.applyMetadata(ctx, lk.LibraryID, &asset, meta, nil, false, "", userID)
	h.persistSourceRecord(ctx, &asset, metadata.SourceRecord{
		Store:      meta.Store,
		ListingURL: firstNonEmptyStr(meta.ListingURL, asset.SourceURL),
		FetchedAt:  time.Now().UTC(),
		Matched:    true,
		Metadata:   meta,
	})

	result := models.StoreLookupUnchanged
	switch {
	case meta.StoreLookupFailed:
		result = models.StoreLookupNoStoreData
	case len(applied) > 0:
		result = models.StoreLookupChanged
	}
	return h.finishStoreLookup(ctx, &lk, models.StoreLookupDone, result, applied, conflicts, "")
}

func (h *Handler) finishStoreLookup(ctx context.Context, lk *models.StoreLookup, status, result string, applied []string, conflicts []metadataConflict, errMsg string) error {
	if applied == nil {
		applied = []string{}
	}
	if conflicts == nil {
		conflicts = []metadataConflict{}
	}
	appliedJSON, err := json.Marshal(applied)
	if err != nil {
		return err
	}
	conflictsJSON, err := json.Marshal(conflicts)
	if err != nil {
		return err
	}
	return h.db.WithContext(ctx).Model(lk).Updates(map[string]any{
		"status":    status,
		"result":    result,
		"applied":   json.RawMessage(appliedJSON),
		"conflicts": json.RawMessage(conflictsJSON),
		"error":     errMsg,
	}).Error
}

// ResumeStoreLookups re-enqueues every lookup still queued, for one whose
// enqueue failed or whose job was lost. River dedupes by args, so this is safe
// on every boot.
func (h *Handler) ResumeStoreLookups(ctx context.Context) (int, error) {
	if h.storeLookups == nil {
		return 0, nil
	}
	var ids []uint
	if err := h.db.WithContext(ctx).Model(&models.StoreLookup{}).
		Where("status = ?", models.StoreLookupQueued).Order("id").Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	for _, id := range ids {
		h.storeLookups.EnqueueStoreLookup(id)
	}
	return len(ids), nil
}
