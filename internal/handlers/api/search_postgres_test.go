package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

// TestSearch_TsvectorMatchesNameAndDescription exercises the search branch
// of applyAssetFilters, `a.search_vector @@ to_tsquery('simple', ...)`. The
// search_vector column and its triggers only exist after the SQL migrations
// run.
func TestSearch_TsvectorMatchesNameAndDescription(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	// "fantasy-kingdom" matches "fant" via the prefix tsquery.
	a := seedAsset(t, db, lib.ID, "fantasy-kingdom")
	a.Description = "epic medieval pack"
	if err := db.Save(a).Error; err != nil {
		t.Fatalf("save asset: %v", err)
	}
	// "sci-fi-pack" must NOT match "fant".
	_ = seedAsset(t, db, lib.ID, "sci-fi-pack")

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?q=fant")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].Name != "fantasy-kingdom" {
		t.Fatalf("tsvector query returned wrong rows: %+v", body.Assets)
	}
}

// TestSearch_TsvectorMatchesTagPaths confirms the tag-aware (C-weighted)
// segment of search_vector. The asset_tags trigger added in 007 issues a
// `name = name` update on insert/delete so the search_vector recompute fires
// and incorporates the joined tag paths. The 'simple' parser keeps a path
// like "audio/loops" as a single token, so we query for a single-segment tag
// to assert the segment is being maintained at all.
func TestSearch_TsvectorMatchesTagPaths(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	tagged := seedAsset(t, db, lib.ID, "asset-one")
	_ = seedAsset(t, db, lib.ID, "asset-two")

	tag := models.Tag{LibraryID: lib.ID, Path: "loops"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if err := db.Create(&models.AssetTag{AssetID: tagged.ID, TagID: tag.ID}).Error; err != nil {
		t.Fatalf("seed asset_tag: %v", err)
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?q=loops")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].ID != tagged.ID {
		t.Fatalf("tag-path tsvector query returned wrong rows: %+v", body.Assets)
	}
}

// TestSearch_TsvectorMatchesLocationPathWords proves a word that appears only
// in a reference asset's folder path finds it. The simple parser would read the
// whole path as one file token, so this fails if the path is not split into
// words before it reaches the vector.
func TestSearch_TsvectorMatchesLocationPathWords(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	path := "/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage"
	ref := models.Asset{LibraryID: lib.ID, Name: "Kingdom", IsReference: true, LocationPath: &path}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatalf("seed reference asset: %v", err)
	}
	_ = seedAsset(t, db, lib.ID, "unrelated")

	r := setupRouter(h, u)
	for _, q := range []string{"synty", "nfs", "POLYGON"} {
		w := performGET(r, "/api/assets?q="+q)
		requireStatus(t, w, http.StatusOK)
		body := jsonResponse[assetListResponse](t, w)
		if len(body.Assets) != 1 || body.Assets[0].ID != ref.ID {
			t.Fatalf("q=%s: path-word query returned wrong rows: %+v", q, body.Assets)
		}
	}
}

// TestSearch_RelevanceRanksNameMatchesFirst proves sort=relevance lifts an
// exact name match above newer name-prefix matches, and a name-prefix match
// above a newer name match that is not a prefix. Without it the picker's
// first page, newest first, can bury an older asset named exactly what was
// typed.
func TestSearch_RelevanceRanksNameMatchesFirst(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")

	rock := seedAsset(t, db, lib.ID, "Rock")
	rocky := seedAsset(t, db, lib.ID, "Rocky Cliff")
	_ = seedAsset(t, db, lib.ID, "Big Rock")
	for i := 1; i <= 26; i++ {
		_ = seedAsset(t, db, lib.ID, fmt.Sprintf("Rocks Pack %d", i))
	}

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?q=rock&sort=relevance&limit=25")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[assetListResponse](t, w)
	if body.Total != 29 || len(body.Assets) != 25 {
		t.Fatalf("got %d of %d, want 25 of 29", len(body.Assets), body.Total)
	}
	if body.Assets[0].ID != rock.ID {
		t.Fatalf("first = %q, want the exact match Rock", body.Assets[0].Name)
	}

	w = performGET(r, "/api/assets?q=rocky&sort=relevance")
	requireStatus(t, w, http.StatusOK)
	body = jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].ID != rocky.ID {
		t.Fatalf("q=rocky returned %+v", body.Assets)
	}

	// "Rocky Cliff" starts with "rock"; "Big Rock" is newer but only contains it.
	w = performGET(r, "/api/assets?q=rock&sort=relevance&limit=100")
	requireStatus(t, w, http.StatusOK)
	body = jsonResponse[assetListResponse](t, w)
	pos := map[string]int{}
	for i, a := range body.Assets {
		pos[a.Name] = i
	}
	if pos["Rocky Cliff"] > pos["Big Rock"] {
		t.Fatalf("prefix match Rocky Cliff at %d after Big Rock at %d", pos["Rocky Cliff"], pos["Big Rock"])
	}

	// With no query, relevance falls back to newest first.
	w = performGET(r, "/api/assets?sort=relevance&limit=1")
	requireStatus(t, w, http.StatusOK)
	body = jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].Name != "Rocks Pack 26" {
		t.Fatalf("no-query relevance first = %+v, want the newest", body.Assets)
	}
}
