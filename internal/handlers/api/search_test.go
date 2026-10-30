package api

import (
	"net/http"
	"slices"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

func TestGetAssets_FiltersByTag(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	a1 := seedAsset(t, db, lib.ID, "with-tag")
	_ = seedAsset(t, db, lib.ID, "without-tag")

	tag := models.Tag{LibraryID: lib.ID, Path: "music"}
	_ = db.Create(&tag)
	_ = db.Create(&models.AssetTag{AssetID: a1.ID, TagID: tag.ID})

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?tag_id=1")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].ID != a1.ID {
		t.Fatalf("filter returned wrong rows: %+v", body.Assets)
	}
}

// seedTaggedAsset creates an asset carrying each of the tag paths, creating
// the tags as needed.
func seedTaggedAsset(t *testing.T, db *gorm.DB, libID uint, name string, paths ...string) *models.Asset {
	t.Helper()
	a := seedAsset(t, db, libID, name)
	for _, p := range paths {
		tag := models.Tag{LibraryID: libID, Path: p}
		if err := db.Where(models.Tag{LibraryID: libID, Path: p}).FirstOrCreate(&tag).Error; err != nil {
			t.Fatalf("seed tag %q: %v", p, err)
		}
		if err := db.Create(&models.AssetTag{AssetID: a.ID, TagID: tag.ID}).Error; err != nil {
			t.Fatalf("tag asset: %v", err)
		}
	}
	return a
}

func TestGetAssets_FiltersByTagPaths(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedTaggedAsset(t, db, lib.ID, "humanoid", "3d/characters/humanoid")
	seedTaggedAsset(t, db, lib.ID, "characters", "3d/characters")
	seedTaggedAsset(t, db, lib.ID, "sibling", "3d/charactersets")
	seedTaggedAsset(t, db, lib.ID, "tree", "3d/vegetation/trees")
	// Under both ticked tags: listed once.
	seedTaggedAsset(t, db, lib.ID, "both", "3d/characters/robot", "3d/vegetation")
	seedTaggedAsset(t, db, lib.ID, "wildcard", "2d/uixkit/icons")
	seedTaggedAsset(t, db, lib.ID, "underscore", "2d/ui_kit/icons")
	seedAsset(t, db, lib.ID, "untagged")

	r := setupRouter(h, u)
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"tag_path=3d/characters", []string{"both", "characters", "humanoid"}},
		{"tag_path=3d/characters&tag_path=3d/vegetation", []string{"both", "characters", "humanoid", "tree"}},
		{"tag_path=3d/characters/humanoid", []string{"humanoid"}},
		{"tag_path=2d/ui_kit", []string{"underscore"}},
		{"tag_path=3d/characters&tag_path=+3d/characters/+", []string{"both", "characters", "humanoid"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := performGET(r, "/api/assets?sort=name_asc&"+tc.query)
			requireStatus(t, w, http.StatusOK)
			body := jsonResponse[assetListResponse](t, w)
			var got []string
			for _, a := range body.Assets {
				got = append(got, a.Name)
			}
			if !slices.Equal(got, tc.want) || body.Total != int64(len(tc.want)) {
				t.Fatalf("got %v (total %d), want %v", got, body.Total, tc.want)
			}
		})
	}
}

func TestGetAssets_FiltersByQuery(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "fantasy-kingdom")
	seedAsset(t, db, lib.ID, "sci-fi-pack")

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?q=fantasy")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].Name != "fantasy-kingdom" {
		t.Fatalf("query didn't filter: %+v", body.Assets)
	}
}

func TestGetAssets_FiltersByPartialQuery(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "fantasy-kingdom")
	seedAsset(t, db, lib.ID, "sci-fi-pack")

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?q=fant")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].Name != "fantasy-kingdom" {
		t.Fatalf("partial query didn't filter: %+v", body.Assets)
	}
}

func TestGetAssets_ComposedFilters(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	a1 := seedAsset(t, db, lib.ID, "music-pack")
	_ = seedAsset(t, db, lib.ID, "untouched")

	tag := models.Tag{LibraryID: lib.ID, Path: "audio"}
	_ = db.Create(&tag)
	_ = db.Create(&models.AssetTag{AssetID: a1.ID, TagID: tag.ID})
	a1.NeedsReview = false
	_ = db.Save(a1)

	r := setupRouter(h, u)
	w := performGET(r, "/api/assets?tag_id=1&needs_review=false")
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[assetListResponse](t, w)
	if len(body.Assets) != 1 || body.Assets[0].ID != a1.ID {
		t.Fatalf("composed filter wrong: %+v", body.Assets)
	}
}
