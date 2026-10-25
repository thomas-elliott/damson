package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/enrich"
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
// only the generic scraper answered, or, in a refresh, it kept a field a
// person had changed.
type storeLookupProblem struct {
	AssetID   uint              `json:"asset_id"`
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	Result    string            `json:"result,omitempty"`
	Error     string            `json:"error,omitempty"`
	Conflicts []enrich.Conflict `json:"conflicts,omitempty"`
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
	// The asset list drops a filter it can't read, which only widens a list.
	// Here it would widen the lookups, and a refresh can set Needs review, so
	// it is refused.
	for _, f := range []struct {
		name string
		read bool
	}{
		{"released_after", filters.ReleasedAfter != nil},
		{"tag_id", filters.TagID != nil},
		{"project_id", filters.ProjectID != nil},
		{"collection_id", filters.CollectionID != nil},
		{"needs_review", filters.NeedsReview != nil},
		{"store_deprecated", filters.StoreDeprecated != nil},
		{"updates_available", optionalBool(c.Query("updates_available")) != nil},
		{"not_downloaded", optionalBool(c.Query("not_downloaded")) != nil},
	} {
		if v := strings.TrimSpace(c.Query(f.name)); v != "" && !f.read {
			respond400(c, fmt.Errorf("can't read %s %q", f.name, v))
			return
		}
	}
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
			if err := h.queueStoreLookupTx(tx, batch, a.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		respondGORM(c, err, "batch not found")
		return
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

// queueStoreLookupTx adds one queued lookup to the batch and enqueues its
// job in the same transaction, so neither outlives the other.
func (h *Handler) queueStoreLookupTx(tx *gorm.DB, batch models.StoreLookupBatch, assetID uint) error {
	lk := models.StoreLookup{BatchID: batch.ID, LibraryID: batch.LibraryID, AssetID: assetID, Status: models.StoreLookupQueued}
	if err := tx.Create(&lk).Error; err != nil {
		return err
	}
	return h.storeLookups.EnqueueStoreLookup(tx, lk.ID)
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
		var conflicts []enrich.Conflict
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
		// Only a refresh replaces fields, so only there is a kept field news: a
		// fill-blanks lookup keeps every value already set, by design.
		kept := batch.Refresh && len(conflicts) > 0
		if kept {
			resp.Conflicted++
		} else {
			conflicts = nil
		}
		if r.Status == models.StoreLookupFailed || r.Result == models.StoreLookupNoStoreData || kept {
			resp.Problems = append(resp.Problems, storeLookupProblem{
				AssetID: r.AssetID, Name: r.Name, Status: r.Status, Result: r.Result, Error: r.Error, Conflicts: conflicts,
			})
		}
	}
	return resp, nil
}
