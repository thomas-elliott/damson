package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/storage"
	"github.com/thomas-elliott/damson/internal/storage/cleanup"
)

type storageCleanupCandidateView struct {
	Prefix       string    `json:"prefix"`
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	Filename     string    `json:"filename,omitempty"`
	LibraryID    uint      `json:"library_id,omitempty"`
	AssetID      uint      `json:"asset_id,omitempty"`
	VersionID    uint      `json:"version_id,omitempty"`
	Kind         string    `json:"kind,omitempty"`
}

type storageCleanupScanView struct {
	MinAgeSeconds int64                         `json:"min_age_seconds"`
	TotalCount    int                           `json:"total_count"`
	TotalBytes    int64                         `json:"total_bytes"`
	Candidates    []storageCleanupCandidateView `json:"candidates"`
}

func (h *Handler) postStorageCleanupScan(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	if h.storage == nil {
		respond500(c, errors.New("storage not configured"))
		return
	}
	cands, err := cleanup.FindOrphans(c.Request.Context(), h.storage, h.db, h.orphanMinAge, time.Now())
	if err != nil {
		respond500(c, err)
		return
	}
	view := storageCleanupScanView{
		MinAgeSeconds: int64(h.orphanMinAge.Seconds()),
		Candidates:    make([]storageCleanupCandidateView, 0, len(cands)),
	}
	for _, cand := range cands {
		view.Candidates = append(view.Candidates, storageCleanupCandidateView{
			Prefix:       string(cand.Prefix),
			Key:          cand.Key,
			Size:         cand.Size,
			LastModified: cand.LastModified,
			Filename:     cand.Meta.Filename,
			LibraryID:    cand.Meta.LibraryID,
			AssetID:      cand.Meta.AssetID,
			VersionID:    cand.Meta.VersionID,
			Kind:         cand.Meta.Kind,
		})
		view.TotalBytes += cand.Size
	}
	view.TotalCount = len(view.Candidates)
	c.JSON(http.StatusOK, view)
}

type storageCleanupDeleteRequest struct {
	Keys []struct {
		Prefix string `json:"prefix"`
		Key    string `json:"key"`
	} `json:"keys"`
}

type storageCleanupSkippedView struct {
	Prefix string `json:"prefix"`
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type storageCleanupDeleteView struct {
	DeletedCount int                         `json:"deleted_count"`
	FreedBytes   int64                       `json:"freed_bytes"`
	Skipped      []storageCleanupSkippedView `json:"skipped"`
}

func (h *Handler) postStorageCleanupDelete(c *gin.Context) {
	if !requireSysadmin(c) {
		return
	}
	if h.storage == nil {
		respond500(c, errors.New("storage not configured"))
		return
	}
	var req storageCleanupDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	keys := make([]cleanup.Key, 0, len(req.Keys))
	for _, k := range req.Keys {
		keys = append(keys, cleanup.Key{Prefix: storage.Prefix(k.Prefix), Key: k.Key})
	}
	var actorID *uint
	if u := auth.GetUser(c); u != nil {
		actorID = &u.ID
	}
	res, err := cleanup.DeleteOrphans(c.Request.Context(), h.storage, h.db, keys, h.orphanMinAge, time.Now(), actorID)
	if err != nil {
		respond500(c, err)
		return
	}
	view := storageCleanupDeleteView{
		DeletedCount: len(res.Deleted),
		FreedBytes:   res.FreedBytes,
		Skipped:      make([]storageCleanupSkippedView, 0, len(res.Skipped)),
	}
	for _, s := range res.Skipped {
		view.Skipped = append(view.Skipped, storageCleanupSkippedView{Prefix: string(s.Prefix), Key: s.Key, Reason: s.Reason})
	}
	c.JSON(http.StatusOK, view)
}
