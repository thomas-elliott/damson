package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
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

func TestRecordReference_PathOnlyIsNamedAfterLastSegment(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
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
	if second.MetadataFailed {
		t.Errorf("a store lookup that worked was reported as failed: %+v", second)
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

// barrierMetadata holds every store lookup until all expected callers have
// reached it, so each has passed the early "already in library" check first.
type barrierMetadata struct {
	fakeMetadata
	arrived sync.WaitGroup
}

func (b *barrierMetadata) Fetch(_ context.Context, _ string) (*metadata.Metadata, error) {
	b.arrived.Done()
	b.arrived.Wait()
	return b.meta, nil
}

func TestRecordReference_ConcurrentRecordsOfANewLinkCreateOneAsset(t *testing.T) {
	h, db := newTestHandler(t)
	const callers = 2
	fake := &barrierMetadata{fakeMetadata: fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreUnity, Title: "Pack"}}}
	fake.arrived.Add(callers)
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	codes := make([]int, callers)
	var done sync.WaitGroup
	for i := range callers {
		done.Add(1)
		go func() {
			defer done.Done()
			codes[i] = performRequest(r, http.MethodPost, "/api/assets/references",
				map[string]any{"source_url": "https://assetstore.unity.com/packages/p-1"}).Code
		}()
	}
	done.Wait()

	if n := countAssets(t, db, lib.ID); n != 1 {
		t.Fatalf("%d assets after concurrent records of one link (statuses %v)", n, codes)
	}
	if !slices.Contains(codes, http.StatusCreated) || !slices.Contains(codes, http.StatusOK) {
		t.Fatalf("want one created and one already-in-library, got %v", codes)
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
	if !resp.MetadataFailed {
		t.Errorf("a failed store lookup must be reported as metadata_failed: %+v", resp)
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

func assetTagPaths(t *testing.T, db *gorm.DB, assetID uint) []string {
	t.Helper()
	var paths []string
	err := db.Table("tags").Joins("JOIN asset_tags ON asset_tags.tag_id = tags.id").
		Where("asset_tags.asset_id = ?", assetID).Order("tags.path").Pluck("tags.path", &paths).Error
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// A scan records each pack with the publisher and folder tags read from its
// path. The given publisher beats the store's, and a rerun over a known path
// changes neither.
func TestRecordReference_PublisherAndTagsApplyOnCreateOnly(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreUnity, Title: "POLYGON Fantasy Kingdom", Publisher: "Synty Studios",
	}}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	path := "/mnt/nfs/3D/Environments/Synty/POLYGON_Fantasy_Kingdom.unitypackage"
	first := recordReference(t, r, map[string]any{
		"location_path": path, "source_url": "https://assetstore.unity.com/packages/k-1",
		"publisher": "Synty", "tags": []string{"3D", "Environments"}, "needs_review": true,
	}, http.StatusCreated)
	var a models.Asset
	if err := db.First(&a, first.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if derefString(a.Publisher) != "Synty" {
		t.Errorf("publisher: got %q, want the one read from the path", derefString(a.Publisher))
	}
	if got := assetTagPaths(t, db, a.ID); !slices.Equal(got, []string{"3d", "environments"}) {
		t.Errorf("tags: got %v", got)
	}

	recordReference(t, r, map[string]any{
		"location_path": path, "publisher": "Someone Else", "tags": []string{"Audio"},
	}, http.StatusOK)
	if err := db.First(&a, first.AssetID).Error; err != nil {
		t.Fatal(err)
	}
	if derefString(a.Publisher) != "Synty" {
		t.Errorf("a re-record changed the publisher to %q", derefString(a.Publisher))
	}
	if got := assetTagPaths(t, db, a.ID); !slices.Equal(got, []string{"3d", "environments"}) {
		t.Errorf("a re-record changed the tags to %v", got)
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
		"blank tag":    {"location_path": "/nfs/a", "tags": []string{"3D", " / "}},
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
		"/nfs/Packs/Pack v1.2":                                "Pack v1.2",
		"/nfs/Packs/Forest":                                   "Forest",
		`D:\Assets\Trees.zip`:                                 "Trees",
		"/":                                                   "",
	} {
		if got := nameFromLocationPath(in); got != want {
			t.Errorf("nameFromLocationPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func seedReference(t *testing.T, db *gorm.DB, libID uint, name, path, link string) *models.Asset {
	t.Helper()
	a := models.Asset{LibraryID: libID, Name: name, IsReference: true, SourceURL: link}
	if path != "" {
		a.LocationPath = &path
	}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("seed reference: %v", err)
	}
	return &a
}

// A scan of one folder must see every path recorded under it and nothing from
// a sibling folder, whose packs would otherwise be reported gone.
func TestAssetLocations_ListsRecordedPathsUnderAFolder(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	for _, p := range []string{"/mnt/nfs/3D/Synty/A_Pack", "/mnt/nfs/3D/B.zip", "/mnt/nfs/3Dx/C", "/mnt/nfs/Música/D"} {
		seedReference(t, db, lib.ID, p, p, "")
	}
	seedReference(t, db, lib.ID, "link only", "", "https://example.com/p")
	other := seedLibrary(t, db, "Other")
	seedReference(t, db, other.ID, "theirs", "/mnt/nfs/3D/Theirs", "")
	r := setupRouter(h, u)

	paths := func(query string) []string {
		t.Helper()
		w := performRequest(r, http.MethodGet, "/api/assets/locations"+query, nil)
		requireStatus(t, w, http.StatusOK)
		var out []string
		for _, l := range jsonResponse[struct {
			Locations []recordedLocation `json:"locations"`
		}](t, w).Locations {
			out = append(out, l.LocationPath)
		}
		return out
	}

	if got := paths("?under=/mnt/nfs/3D/"); !slices.Equal(got, []string{"/mnt/nfs/3D/B.zip", "/mnt/nfs/3D/Synty/A_Pack"}) {
		t.Errorf("under /mnt/nfs/3D: got %v", got)
	}
	if got := paths("?under=/mnt/nfs/M%C3%BAsica"); !slices.Equal(got, []string{"/mnt/nfs/Música/D"}) {
		t.Errorf("under a non-ASCII folder: got %v", got)
	}
	if got := paths("?under=/mnt/nfs/3_"); len(got) != 0 {
		t.Errorf("an underscore in under was read as a wildcard: %v", got)
	}
	if got := paths(""); len(got) != 4 {
		t.Errorf("no under: got %v, want this library's four paths", got)
	}
}

func TestReferenceAsset_ListAndDetailCarryTheLocation(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	ref := seedReference(t, db, lib.ID, "Kingdom", "/nfs/Kingdom", "https://example.com/k")
	stored := seedAsset(t, db, lib.ID, "Stored")
	r := setupRouter(h, u)

	w := performGET(r, "/api/assets")
	requireStatus(t, w, http.StatusOK)
	byID := map[uint]assetGridRow{}
	for _, row := range jsonResponse[assetListResponse](t, w).Assets {
		byID[row.ID] = row
	}
	if !byID[ref.ID].IsReference || byID[stored.ID].IsReference {
		t.Fatalf("grid is_reference: ref=%v stored=%v", byID[ref.ID].IsReference, byID[stored.ID].IsReference)
	}

	w = performGET(r, "/api/assets/"+itoa(ref.ID))
	requireStatus(t, w, http.StatusOK)
	d := jsonResponse[assetDetailView](t, w)
	if !d.IsReference || d.LocationPath != "/nfs/Kingdom" || d.SourceURL != "https://example.com/k" {
		t.Fatalf("detail: %+v", d)
	}
}

func TestReferenceAsset_PatchLocation(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	ref := seedReference(t, db, lib.ID, "Kingdom", "/nfs/Kingdom", "https://example.com/k")
	other := seedReference(t, db, lib.ID, "Forest", "/nfs/Forest", "")
	stored := seedAsset(t, db, lib.ID, "Stored")
	r := setupRouter(h, u)
	patch := func(id uint, body map[string]any) *httptest.ResponseRecorder {
		return performRequest(r, http.MethodPatch, "/api/assets/"+itoa(id), body)
	}

	w := patch(ref.ID, map[string]any{"location_path": "/nfs/Kingdom-v2/"})
	requireStatus(t, w, http.StatusOK)
	if got := jsonResponse[assetPatchResponse](t, w).LocationPath; got != "/nfs/Kingdom-v2" {
		t.Errorf("edited path: got %q", got)
	}

	w = patch(ref.ID, map[string]any{"location_path": "/nfs/Forest"})
	requireStatus(t, w, http.StatusConflict)
	if !strings.Contains(w.Body.String(), "Forest") {
		t.Errorf("refusal should name the other asset: %s", w.Body.String())
	}

	// Clearing the path is fine while the link remains; clearing both is not.
	requireStatus(t, patch(ref.ID, map[string]any{"location_path": ""}), http.StatusOK)
	requireStatus(t, patch(ref.ID, map[string]any{"source_url": ""}), http.StatusBadRequest)
	requireStatus(t, patch(other.ID, map[string]any{"location_path": "", "source_url": ""}), http.StatusBadRequest)

	var a models.Asset
	if err := db.First(&a, ref.ID).Error; err != nil {
		t.Fatal(err)
	}
	if a.LocationPath != nil || a.SourceURL != "https://example.com/k" {
		t.Fatalf("after edits: path=%v url=%q", a.LocationPath, a.SourceURL)
	}

	requireStatus(t, patch(stored.ID, map[string]any{"location_path": "/nfs/x"}), http.StatusBadRequest)
}

func TestReferenceAsset_RefusesFilesAndMerge(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	ref := seedReference(t, db, lib.ID, "Kingdom", "/nfs/Kingdom", "")
	stored := seedAsset(t, db, lib.ID, "Stored")
	r := setupRouter(h, u)

	body, ctype := buildMultipartWithFields(t, "file", "tex.png", []byte("png"),
		map[string]string{"asset_id": itoa(ref.ID)})
	requireStatus(t, postMultipart(r, "/api/imports", body, ctype), http.StatusConflict)

	w := performRequest(r, http.MethodPost, "/api/imports/uploads", map[string]any{
		"filename": "Kingdom.unitypackage", "size_bytes": 64,
		"sha256": hexSum([]byte("bytes")), "asset_id": ref.ID,
	})
	requireStatus(t, w, http.StatusConflict)

	var versions int64
	if err := db.Model(&models.AssetVersion{}).Where("asset_id = ?", ref.ID).Count(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if versions != 0 {
		t.Fatalf("a refused upload left %d versions on the reference asset", versions)
	}

	for _, pair := range [][2]uint{{ref.ID, stored.ID}, {stored.ID, ref.ID}} {
		w := performRequest(r, http.MethodPost, "/api/assets/"+itoa(pair[0])+"/merge",
			map[string]any{"source_asset_id": pair[1]})
		requireStatus(t, w, http.StatusConflict)
	}
}

func TestReferenceAsset_IsNeverSuggestedAsAnUploadMatch(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	seedReference(t, db, lib.ID, "grass texture", "/nfs/grass", "")
	r := setupRouter(h, u)

	body, ctype := buildMultipart(t, "file", "grass-texture.png", []byte("fake png bytes"))
	w := postMultipart(r, "/api/imports", body, ctype)
	requireStatus(t, w, http.StatusAccepted)
	if m := jsonResponse[importEnqueueResponse](t, w).SuggestedMatch; m != nil {
		t.Fatalf("a reference asset was offered as the version match: %+v", m)
	}
}

func TestReferenceAsset_TagsCollectsAndPinsWithoutAVersion(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	ref := seedReference(t, db, lib.ID, "Kingdom", "/nfs/Kingdom", "")
	proj := models.Project{LibraryID: lib.ID, Name: "Ashfall"}
	if err := db.Create(&proj).Error; err != nil {
		t.Fatal(err)
	}
	r := setupRouter(h, u)
	pinPath := "/api/projects/" + itoa(proj.ID) + "/pins"

	requireStatus(t, performRequest(r, http.MethodPost, pinPath,
		map[string]any{"asset_id": ref.ID, "version_id": 1}), http.StatusBadRequest)
	requireStatus(t, performRequest(r, http.MethodPost, pinPath, map[string]any{"asset_id": ref.ID}), http.StatusOK)
	requireStatus(t, performRequest(r, http.MethodPost, pinPath, map[string]any{"asset_id": ref.ID}), http.StatusOK)

	w := performGET(r, "/api/projects/"+itoa(proj.ID))
	requireStatus(t, w, http.StatusOK)
	pd := jsonResponse[projectDetailView](t, w)
	if len(pd.Pins) != 1 {
		t.Fatalf("want one pin, got %+v", pd.Pins)
	}
	pin := pd.Pins[0]
	if pin.AssetID != ref.ID || !pin.IsReference || pin.PinnedVersionID != nil || pin.HasUpdate {
		t.Fatalf("reference pin: %+v", pin)
	}

	w = performGET(r, "/api/assets/"+itoa(ref.ID))
	requireStatus(t, w, http.StatusOK)
	if d := jsonResponse[assetDetailView](t, w); len(d.Projects) != 1 || d.Projects[0].ProjectName != "Ashfall" {
		t.Fatalf("asset detail projects: %+v", d.Projects)
	}

	coll := models.Collection{LibraryID: lib.ID, Name: "Medieval"}
	if err := db.Create(&coll).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, performRequest(r, http.MethodPost, "/api/collections/"+itoa(coll.ID)+"/assets",
		map[string]any{"asset_id": ref.ID}), http.StatusNoContent)
	requireStatus(t, performRequest(r, http.MethodPost, "/api/assets/"+itoa(ref.ID)+"/tags",
		map[string]any{"path": "environment/castles"}), http.StatusCreated)

	w = performGET(r, "/api/assets/"+itoa(ref.ID))
	requireStatus(t, w, http.StatusOK)
	if d := jsonResponse[assetDetailView](t, w); len(d.Collections) != 1 || len(d.Tags) != 1 {
		t.Fatalf("tag/collection not held: collections=%+v tags=%+v", d.Collections, d.Tags)
	}
}

// TestAssetFilters_NotDownloaded proves Purchased, not downloaded is exactly
// the reference assets with no path: a stored asset, or a reference asset on a
// path (with or without a link), is downloaded somewhere.
func TestAssetFilters_NotDownloaded(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	pathOnly, pathAndLink := "/mnt/nfs/a", "/mnt/nfs/b"

	stored := models.Asset{LibraryID: lib.ID, Name: "stored", SourceURL: "https://example.com/stored"}
	onPath := models.Asset{LibraryID: lib.ID, Name: "on path", IsReference: true, LocationPath: &pathOnly}
	onPathLinked := models.Asset{LibraryID: lib.ID, Name: "on path, linked", IsReference: true,
		LocationPath: &pathAndLink, SourceURL: "https://example.com/b"}
	purchased := models.Asset{LibraryID: lib.ID, Name: "purchased", IsReference: true, SourceURL: "https://example.com/p"}
	for _, a := range []*models.Asset{&stored, &onPath, &onPathLinked, &purchased} {
		if err := db.Create(a).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := setupRouter(h, u)
	if got := listAssetIDs(t, r, "/api/assets?not_downloaded=true"); len(got) != 1 || got[0] != purchased.ID {
		t.Fatalf("not_downloaded = %v, want only the link-only reference asset %d", got, purchased.ID)
	}
	if got := listAssetIDs(t, r, "/api/assets"); len(got) != 4 {
		t.Fatalf("unfiltered list = %v, want all 4", got)
	}
}

// TestAssetLinks_ListsEveryAssetsStoreLink proves a purchase run sees every
// asset it could match, stored and reference, with its link and store product
// ID, and nothing from another library.
func TestAssetLinks_ListsEveryAssetsStoreLink(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	stored := seedAssetWithFields(t, db, lib.ID, "Toony Skies", map[string]any{
		"publisher": "Ludwig", "store": "unity", "store_product_id": "11020",
		"source_url": "https://assetstore.unity.com/packages/2d/textures-materials/sky/toony-skies-11020",
	})
	bare := seedAssetWithFields(t, db, lib.ID, "Bare", nil)
	other := seedLibrary(t, db, "Other")
	seedReference(t, db, other.ID, "theirs", "", "https://example.com/theirs")

	w := performGET(setupRouter(h, u), "/api/assets/links")
	requireStatus(t, w, http.StatusOK)
	got := jsonResponse[struct {
		Assets []assetLink `json:"assets"`
	}](t, w).Assets
	want := []assetLink{
		{AssetID: stored.ID, Name: "Toony Skies", Publisher: "Ludwig", Store: "unity", StoreProductID: "11020",
			SourceURL: "https://assetstore.unity.com/packages/2d/textures-materials/sky/toony-skies-11020"},
		{AssetID: bare.ID, Name: "Bare"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("links = %+v, want %+v", got, want)
	}
}
