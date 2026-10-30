package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// The tag index shows a card for every second-level tag (`3d/environment`):
// the first level is too broad to browse by. A top-level tag with nothing
// under it gets a card of its own, so a library's own tags still show.
const (
	tagIndexCovers   = 3
	tagIndexChildren = 3
)

// tagIndexNodeSQL maps a tag path to its card: the first two steps, or the
// whole path when it has one.
const tagIndexNodeSQL = `CASE WHEN position('/' in t.path) = 0 THEN t.path
	ELSE split_part(t.path, '/', 1) || '/' || split_part(t.path, '/', 2) END`

type tagIndexCover struct {
	ID                 uint   `json:"id"`
	Name               string `json:"name"`
	Publisher          string `json:"publisher,omitempty"`
	ThumbnailObjectKey string `json:"thumbnail_object_key,omitempty"`
}

type tagIndexChild struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type tagIndexEntry struct {
	Path        string          `json:"path"`
	Count       int             `json:"count"`
	TopChildren []tagIndexChild `json:"top_children"`
	Covers      []tagIndexCover `json:"covers"`
}

// getTagIndex lists the tag index's cards, each with the number of assets
// under it (counted once however many of its tags they carry), its biggest
// child tags and a few covers, preferring assets that have a thumbnail.
func (h *Handler) getTagIndex(c *gin.Context) {
	lib := getLibrary(c)
	if lib == nil {
		respond500(c, errRequestContextIncomplete)
		return
	}
	db := h.db.WithContext(c.Request.Context())

	// Each asset under each card, once.
	nodeAssets := `SELECT DISTINCT ` + tagIndexNodeSQL + ` AS node, at.asset_id
		FROM tags t JOIN asset_tags at ON at.tag_id = t.id
		WHERE t.library_id = ?`

	var counts []struct {
		Node  string
		Count int
	}
	if err := db.Raw(`SELECT node, COUNT(*) AS count FROM (`+nodeAssets+`) na
		GROUP BY node ORDER BY node`, lib.ID).Scan(&counts).Error; err != nil {
		respond500(c, err)
		return
	}

	var covers []struct {
		Node               string
		ID                 uint
		Name               string
		Publisher          *string
		ThumbnailObjectKey *string
	}
	if err := db.Raw(`SELECT node, id, name, publisher, thumbnail_object_key FROM (
			SELECT na.node, a.id, a.name, a.publisher, a.thumbnail_object_key,
				row_number() OVER (PARTITION BY na.node
					ORDER BY COALESCE(a.thumbnail_object_key, '') <> '' DESC, a.created_at DESC, a.id DESC) AS rn
			FROM (`+nodeAssets+`) na JOIN assets a ON a.id = na.asset_id AND a.library_id = ?
		) ranked WHERE rn <= ? ORDER BY node, rn`, lib.ID, lib.ID, tagIndexCovers).Scan(&covers).Error; err != nil {
		respond500(c, err)
		return
	}

	var children []struct {
		Node  string
		Child string
		Count int
	}
	if err := db.Raw(`SELECT node, child, COUNT(*) AS count FROM (
			SELECT DISTINCT split_part(t.path, '/', 1) || '/' || split_part(t.path, '/', 2) AS node,
				split_part(t.path, '/', 3) AS child, at.asset_id
			FROM tags t JOIN asset_tags at ON at.tag_id = t.id
			WHERE t.library_id = ? AND split_part(t.path, '/', 3) <> ''
		) nc GROUP BY node, child ORDER BY node, count DESC, child`, lib.ID).Scan(&children).Error; err != nil {
		respond500(c, err)
		return
	}

	entries := make([]tagIndexEntry, 0, len(counts))
	byNode := make(map[string]int, len(counts))
	for _, r := range counts {
		byNode[r.Node] = len(entries)
		entries = append(entries, tagIndexEntry{
			Path: r.Node, Count: r.Count,
			TopChildren: []tagIndexChild{}, Covers: []tagIndexCover{},
		})
	}
	for _, r := range covers {
		e := &entries[byNode[r.Node]]
		e.Covers = append(e.Covers, tagIndexCover{
			ID: r.ID, Name: r.Name,
			Publisher: derefString(r.Publisher), ThumbnailObjectKey: derefString(r.ThumbnailObjectKey),
		})
	}
	for _, r := range children {
		i, ok := byNode[r.Node]
		if !ok || len(entries[i].TopChildren) == tagIndexChildren {
			continue
		}
		entries[i].TopChildren = append(entries[i].TopChildren, tagIndexChild{Name: r.Child, Count: r.Count})
	}

	// A top-level tag with deeper tags under it is shown through them.
	nested := map[string]bool{}
	for _, e := range entries {
		if top, _, ok := strings.Cut(e.Path, "/"); ok {
			nested[top] = true
		}
	}
	entries = slices.DeleteFunc(entries, func(e tagIndexEntry) bool { return nested[e.Path] })
	c.JSON(http.StatusOK, gin.H{"tags": entries})
}
