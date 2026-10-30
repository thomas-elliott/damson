package api

import (
	"net/http"
	"slices"
	"testing"
)

func TestGetTagIndex_CardsCountsChildrenAndCovers(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedTaggedAsset(t, db, lib.ID, "knight", "3d/characters/humanoid")
	seedTaggedAsset(t, db, lib.ID, "orc", "3d/characters/humanoid", "3d/characters/creature")
	thumbed := seedTaggedAsset(t, db, lib.ID, "slime", "3d/characters/creature")
	thumbed.ThumbnailObjectKey = "thumb-key"
	if err := db.Save(thumbed).Error; err != nil {
		t.Fatal(err)
	}
	seedTaggedAsset(t, db, lib.ID, "robot", "3d/characters/robot")
	seedTaggedAsset(t, db, lib.ID, "tree", "3d/vegetation", "3d")
	seedTaggedAsset(t, db, lib.ID, "fave", "favourites")

	// Another library's tags stay out.
	other := seedLibrary(t, db, "Other")
	seedTaggedAsset(t, db, other.ID, "stranger", "3d/characters/humanoid")

	r := setupRouter(h, u)
	w := performGET(r, "/api/tags/index")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[struct {
		Tags []tagIndexEntry `json:"tags"`
	}](t, w)

	var paths []string
	for _, e := range body.Tags {
		paths = append(paths, e.Path)
	}
	// `3d` has deeper tags, so it shows through them; `favourites` has none.
	if want := []string{"3d/characters", "3d/vegetation", "favourites"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}

	chars := body.Tags[0]
	if chars.Count != 4 {
		t.Errorf("3d/characters count = %d, want 4 (the orc once)", chars.Count)
	}
	wantChildren := []tagIndexChild{{"creature", 2}, {"humanoid", 2}, {"robot", 1}}
	if !slices.Equal(chars.TopChildren, wantChildren) {
		t.Errorf("children = %+v, want %+v", chars.TopChildren, wantChildren)
	}
	if len(chars.Covers) != tagIndexCovers {
		t.Fatalf("covers = %+v, want %d", chars.Covers, tagIndexCovers)
	}
	if chars.Covers[0].Name != "slime" || chars.Covers[0].ThumbnailObjectKey != "thumb-key" {
		t.Errorf("first cover = %+v, want the asset with a thumbnail", chars.Covers[0])
	}
	if veg := body.Tags[1]; veg.Count != 1 || len(veg.TopChildren) != 0 || len(veg.Covers) != 1 {
		t.Errorf("3d/vegetation = %+v", veg)
	}
}
