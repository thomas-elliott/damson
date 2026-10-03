package api

import (
	"errors"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// TestReferenceAsset_SchemaInvariants proves the database itself refuses the
// two shapes the spec rules out, whichever code path writes the row: a path on
// a stored asset, a reference asset with neither path nor link, and a second
// asset on a path the library already records.
func TestReferenceAsset_SchemaInvariants(t *testing.T) {
	_, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	path := "/mnt/nfs/pack"

	stored := models.Asset{LibraryID: lib.ID, Name: "stored", LocationPath: &path}
	if err := db.Create(&stored).Error; err == nil {
		t.Fatal("a stored asset carrying a path was accepted")
	}
	bare := models.Asset{LibraryID: lib.ID, Name: "bare", IsReference: true}
	if err := db.Create(&bare).Error; err == nil {
		t.Fatal("a reference asset with neither path nor link was accepted")
	}
	linkOnly := models.Asset{LibraryID: lib.ID, Name: "link", IsReference: true, SourceURL: "https://example.com/p"}
	if err := db.Create(&linkOnly).Error; err != nil {
		t.Fatalf("a link-only reference asset was refused: %v", err)
	}
	first := models.Asset{LibraryID: lib.ID, Name: "first", IsReference: true, LocationPath: &path}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("a path-only reference asset was refused: %v", err)
	}
	dupe := models.Asset{LibraryID: lib.ID, Name: "dupe", IsReference: true, LocationPath: &path}
	if err := db.Create(&dupe).Error; err == nil {
		t.Fatal("a second asset on an already-recorded path was accepted")
	}
}

func recordReference(t *testing.T, r http.Handler, body map[string]any, want int) referenceRecordResponse {
	t.Helper()
	w := performRequest(r, http.MethodPost, "/api/assets/references", body)
	requireStatus(t, w, want)
	return jsonResponse[referenceRecordResponse](t, w)
}

func countAssets(t *testing.T, db *gorm.DB, libID uint) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&models.Asset{}).Where("library_id = ?", libID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRecordReference_PathOnlyIsNamedAfterLastSegmentAndSearchable(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	resp := recordReference(t, r, map[string]any{
		"location_path": "  /mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage  ",
	}, http.StatusCreated)
	if resp.Outcome != referenceCreated || resp.Name != "POLYGON_Fantasy_Kingdom" {
		t.Fatalf("got %+v, want created POLYGON_Fantasy_Kingdom", resp)
	}

	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if !a.IsReference || derefString(a.LocationPath) != "/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage" {
		t.Fatalf("stored row: is_reference=%v path=%q", a.IsReference, derefString(a.LocationPath))
	}
	if a.NeedsReview {
		t.Error("a hand-recorded reference asset was marked Needs review")
	}

	w := performGET(r, "/api/assets?q=Synty")
	requireStatus(t, w, http.StatusOK)
	list := jsonResponse[assetListResponse](t, w)
	if len(list.Assets) != 1 || list.Assets[0].ID != resp.AssetID {
		t.Fatalf("folder-word search: %+v", list.Assets)
	}
	_ = lib
}

func TestRecordReference_LinkOnlyTakesNameCoverAndDescriptionFromListing(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{
		meta: &metadata.Metadata{
			Store:       metadata.StoreUnity,
			Title:       "POLYGON Fantasy Kingdom",
			Description: "Low poly kingdom.",
			Publisher:   "Synty Studios",
			CoverURL:    "https://cdn.example.com/cover.png",
			Tags:        []string{"environment"},
		},
		img:   tinyPNG(t),
		imgCT: "image/png",
	}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	link := "https://assetstore.unity.com/packages/3d/polygon-fantasy-kingdom-1"
	resp := recordReference(t, r, map[string]any{"source_url": link}, http.StatusCreated)
	if resp.Name != "POLYGON Fantasy Kingdom" {
		t.Fatalf("name: got %q", resp.Name)
	}
	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.Description != "Low poly kingdom." || a.ThumbnailObjectKey == "" || derefString(a.Publisher) != "Synty Studios" {
		t.Fatalf("listing not applied: description=%q thumb=%q publisher=%q",
			a.Description, a.ThumbnailObjectKey, derefString(a.Publisher))
	}
	if a.SourceURL != link || a.LocationPath != nil || !a.IsReference {
		t.Fatalf("location: source_url=%q path=%v is_reference=%v", a.SourceURL, a.LocationPath, a.IsReference)
	}
}

func TestRecordReference_StoreTitleBeatsPathButPageTitleDoesNot(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreItch, Title: "Store Title"}}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	got := recordReference(t, r, map[string]any{
		"location_path": "/nfs/a/pack-one.zip", "source_url": "https://x.itch.io/one",
	}, http.StatusCreated)
	if got.Name != "Store Title" {
		t.Errorf("store title should name the asset, got %q", got.Name)
	}

	fake.meta = &metadata.Metadata{Store: metadata.StoreGeneric, Title: "Some Page"}
	got = recordReference(t, r, map[string]any{
		"location_path": "/nfs/a/pack-two.zip", "source_url": "https://example.com/two",
	}, http.StatusCreated)
	if got.Name != "pack-two" {
		t.Errorf("a page title should rank below the path, got %q", got.Name)
	}
}

func TestRecordReference_SamePathTwiceUpdatesAndKeepsEditedName(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{fetchErr: errors.New("unreachable")}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	first := recordReference(t, r, map[string]any{"location_path": "/nfs/Synty/Kingdom/"}, http.StatusCreated)
	w := performRequest(r, http.MethodPatch, "/api/assets/"+itoa(first.AssetID),
		map[string]any{"name": "My Kingdom", "description": "kept"})
	requireStatus(t, w, http.StatusOK)

	fake.fetchErr = nil
	fake.meta = &metadata.Metadata{Store: metadata.StoreUnity, Title: "Listing", Description: "listing text", Publisher: "Synty"}
	link := "https://assetstore.unity.com/packages/k-1"
	second := recordReference(t, r, map[string]any{
		"location_path": "/nfs/Synty/Kingdom", "source_url": link, "needs_review": true,
	}, http.StatusOK)

	if second.Outcome != referenceUpdated || second.AssetID != first.AssetID {
		t.Fatalf("second record: %+v, want updated asset %d", second, first.AssetID)
	}
	if n := countAssets(t, db, lib.ID); n != 1 {
		t.Fatalf("recording the same path twice left %d assets", n)
	}
	var a models.Asset
	if err := db.First(&a, first.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.Name != "My Kingdom" || a.Description != "kept" {
		t.Errorf("edited fields overwritten: name=%q description=%q", a.Name, a.Description)
	}
	if a.SourceURL != link || derefString(a.Publisher) != "Synty" || !a.NeedsReview {
		t.Errorf("re-record did not set link / fill blank publisher / set review: url=%q publisher=%q review=%v",
			a.SourceURL, derefString(a.Publisher), a.NeedsReview)
	}
}

func TestRecordReference_KnownStoreLinkCreatesNothing(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreUnity, Title: "X"}}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	stored := seedAsset(t, db, lib.ID, "Imported Pack")
	link := "https://assetstore.unity.com/packages/p-9"
	if err := db.Model(stored).Update("source_url", link).Error; err != nil {
		t.Fatal(err)
	}
	r := setupRouter(h, u)

	resp := recordReference(t, r, map[string]any{"source_url": " " + link + " "}, http.StatusOK)
	if resp.Outcome != referenceExists || resp.AssetID != stored.ID || resp.Name != "Imported Pack" {
		t.Fatalf("got %+v, want exists → asset %d", resp, stored.ID)
	}
	if n := countAssets(t, db, lib.ID); n != 1 {
		t.Fatalf("a known store link created an asset (%d in library)", n)
	}
	if fake.lastURL != "" {
		t.Error("an already-known link still spent a store lookup")
	}
}

func TestRecordReference_UnreadableLinkIsStillRecorded(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{fetchErr: errors.New("parse failed")}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)
	link := "https://assetstore.unity.com/packages/unreadable-3"

	w := performRequest(r, http.MethodPost, "/api/assets/references", map[string]any{"source_url": link})
	requireStatus(t, w, http.StatusUnprocessableEntity)
	if n := countAssets(t, db, lib.ID); n != 0 {
		t.Fatalf("a nameless record created %d assets", n)
	}

	resp := recordReference(t, r, map[string]any{"source_url": link, "name": "Hand Named"}, http.StatusCreated)
	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if a.SourceURL != link || a.Name != "Hand Named" {
		t.Fatalf("got source_url=%q name=%q", a.SourceURL, a.Name)
	}
}

func TestRecordReference_NeedsReviewOnlyWhenAsked(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	resp := recordReference(t, r, map[string]any{"location_path": "/nfs/scan/a", "needs_review": true}, http.StatusCreated)
	var a models.Asset
	if err := db.First(&a, resp.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if !a.NeedsReview {
		t.Error("needs_review=true was not recorded")
	}
}

func TestRecordReference_RejectsEmptyAndBadInput(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	for name, body := range map[string]map[string]any{
		"neither":      {"location_path": "  "},
		"bad scheme":   {"source_url": "file:///etc/passwd"},
		"control char": {"location_path": "/nfs/a\nb"},
	} {
		w := performRequest(r, http.MethodPost, "/api/assets/references", body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", name, w.Code)
		}
	}
}

func TestNameFromLocationPath(t *testing.T) {
	for in, want := range map[string]string{
		"/mnt/nfs/Synty/POLYGON_Fantasy_Kingdom.unitypackage": "POLYGON_Fantasy_Kingdom",
		"/nfs/Packs/Pack v1.2":                                 "Pack v1.2",
		"/nfs/Packs/Forest":                                    "Forest",
		`D:\Assets\Trees.zip`:                                  "Trees",
		"/":                                                    "",
	} {
		if got := nameFromLocationPath(in); got != want {
			t.Errorf("nameFromLocationPath(%q) = %q, want %q", in, got, want)
		}
	}
}

