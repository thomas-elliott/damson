package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/importsource"
	"github.com/thomas-elliott/damson/internal/services/imports"
)

// maxPreflightEntries bounds one preflight request. A cache sync sends its
// whole manifest, so the cap is generous — but it is a cap, and exceeding it
// is an error rather than a silent truncation.
const maxPreflightEntries = 2000

// Preflight statuses come from the leaf vocabulary the CLI shares, so the two
// sides cannot drift.
const (
	preflightAlreadyImported = importsource.StatusAlreadyImported
	preflightUpdated         = importsource.StatusUpdated
	preflightNameMatch       = importsource.StatusNameMatch
	preflightNew             = importsource.StatusNew
)

type preflightEntry struct {
	Filename     string `json:"filename"`
	SizeBytes    int64  `json:"size_bytes"`
	SourceKind   string `json:"source_kind"`
	RelativePath string `json:"relative_path"`
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
	HeldBytes    int64  `json:"held_bytes,omitempty"`
}

type preflightResponse struct {
	Results      []preflightResult `json:"results"`
	LibraryTotal int               `json:"library_total"`
}

// postImportPreflight answers, for a batch of entries, what the library
// already holds — asked before the bytes move.
//
// An entry carrying a source path is answered precisely: the same path at the
// same size is already_imported, the same path at a different size is updated.
// An entry without one falls back to the normalised-title matcher POST
// /api/imports uses for suggested_match, which can only say name_match — it
// cannot tell a duplicate from a newer version.
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

	db := h.db.WithContext(c.Request.Context())

	// One read of the library for the whole batch. MatchAssetByName reads
	// every asset per call, which is quadratic across a manifest.
	matcher, err := imports.NewNameMatcher(db, lib.ID)
	if err != nil {
		respond500(c, err)
		return
	}

	paths, err := preflightPaths(req.Entries)
	if err != nil {
		slog.Warn("preflight: invalid source path", "err", err, "library_id", lib.ID)
		respond400(c, err)
		return
	}
	sourceMatches, err := imports.SourceMatches(db, lib.ID, imports.SourceKindUnityCache, distinctPaths(paths))
	if err != nil {
		respond500(c, err)
		return
	}

	matchedIDs := make([]uint, 0, len(req.Entries))
	results := make([]preflightResult, len(req.Entries))
	for i, e := range req.Entries {
		filename := strings.TrimSpace(e.Filename)
		results[i] = preflightResult{Index: i, Filename: filename, Status: preflightNew}

		if path := paths[i]; path != "" {
			if m, ok := sourceMatches[path]; ok {
				results[i].Status = preflightUpdated
				if m.SizeBytes == e.SizeBytes {
					results[i].Status = preflightAlreadyImported
				}
				results[i].AssetID = m.AssetID
				results[i].AssetName = m.AssetName
				results[i].HeldBytes = m.SizeBytes
				matchedIDs = append(matchedIDs, m.AssetID)
				continue
			}
		}

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

// preflightPaths validates each entry's source hint, returning the normalised
// relative path per entry index and "" where the entry carries none. Validated
// with the same rule POST /api/imports applies, so an entry preflight accepts
// cannot then be rejected at upload.
func preflightPaths(entries []preflightEntry) ([]string, error) {
	paths := make([]string, len(entries))
	for i, e := range entries {
		clean, err := imports.ValidateSource(e.SourceKind, e.RelativePath)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		paths[i] = clean
	}
	return paths, nil
}

// distinctPaths collapses the per-entry paths into the lookup set, dropping
// blanks and duplicates so one manifest is one query.
func distinctPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, dupe := seen[p]; dupe {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func errTooManyPreflightEntries(n int) error {
	return fmt.Errorf("too many entries: %d sent, %d is the maximum per request", n, maxPreflightEntries)
}
