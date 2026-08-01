package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/services/imports"
)

// maxPreflightEntries bounds one preflight request. A cache sync sends its
// whole manifest, so the cap is generous — but it is a cap, and exceeding it
// is an error rather than a silent truncation.
const maxPreflightEntries = 2000

// Preflight statuses. This slice owns the name-only half; the Unity cache
// import adds already_imported and updated once versions carry a source path
// and size.
const (
	preflightNew       = "new"
	preflightNameMatch = "name_match"
)

type preflightEntry struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
}

type preflightRequest struct {
	Entries []preflightEntry `json:"entries"`
}

type preflightResult struct {
	Index        int    `json:"index"`
	Filename     string `json:"filename"`
	Status       string `json:"status"`
	AssetID      uint   `json:"asset_id,omitempty"`
	AssetName    string `json:"asset_name,omitempty"`
	VersionCount int    `json:"version_count,omitempty"`
}

type preflightResponse struct {
	Results      []preflightResult `json:"results"`
	LibraryTotal int               `json:"library_total"`
}

// postImportPreflight answers, for a batch of filenames, whether the library
// already holds something by that name — the same normalised-title matcher
// POST /api/imports uses to return suggested_match after an upload, asked
// before the bytes move.
//
// Results correlate to the request by index, not by filename: a manifest may
// legitimately contain the same basename twice from different directories.
func (h *Handler) postImportPreflight(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}

	var req preflightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	if len(req.Entries) == 0 {
		respond400(c, asError("entries is required"))
		return
	}
	if len(req.Entries) > maxPreflightEntries {
		respond400(c, errTooManyPreflightEntries(len(req.Entries)))
		return
	}

	// One read of the library for the whole batch. MatchAssetByName reads
	// every asset per call, which is quadratic across a manifest.
	matcher, err := imports.NewNameMatcher(h.db.WithContext(c.Request.Context()), lib.ID)
	if err != nil {
		respond500(c, err)
		return
	}

	matchedIDs := make([]uint, 0, len(req.Entries))
	results := make([]preflightResult, len(req.Entries))
	for i, e := range req.Entries {
		filename := strings.TrimSpace(e.Filename)
		results[i] = preflightResult{Index: i, Filename: filename, Status: preflightNew}
		if filename == "" {
			continue
		}
		name, _ := imports.ParseImportFilename(filepath.Base(filename))
		if a := matcher.Match(name); a != nil {
			results[i].Status = preflightNameMatch
			results[i].AssetID = a.ID
			results[i].AssetName = a.Name
			matchedIDs = append(matchedIDs, a.ID)
		}
	}

	if len(matchedIDs) > 0 {
		counts, err := h.versionCounts(c, matchedIDs)
		if err != nil {
			respond500(c, err)
			return
		}
		for i := range results {
			if results[i].AssetID != 0 {
				results[i].VersionCount = counts[results[i].AssetID]
			}
		}
	}

	c.JSON(http.StatusOK, preflightResponse{Results: results, LibraryTotal: matcher.Total()})
}

// versionCounts returns held-version counts for the matched assets in one
// grouped query, rather than one query per match.
func (h *Handler) versionCounts(c *gin.Context, assetIDs []uint) (map[uint]int, error) {
	type row struct {
		AssetID uint
		N       int
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("asset_versions").
		Select("asset_id, COUNT(*) AS n").
		Where("asset_id IN ?", assetIDs).
		Group("asset_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]int, len(rows))
	for _, r := range rows {
		out[r.AssetID] = r.N
	}
	return out, nil
}

func errTooManyPreflightEntries(n int) error {
	return fmt.Errorf("too many entries: %d sent, %d is the maximum per request", n, maxPreflightEntries)
}
