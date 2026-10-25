package api

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/enrich"
	"github.com/thomas-elliott/damson/internal/metadata"
	"github.com/thomas-elliott/damson/internal/models"
)

// fakeMetadata is an injectable metadataService and enrich.LookupFetcher so tests don't do
// real outbound HTTP (which the SSRF guard blocks in any case).
type fakeMetadata struct {
	meta       *metadata.Metadata
	fetchErr   error
	img        []byte
	imgCT      string
	imgErr     error
	lastURL    string
	candidates []metadata.Candidate
	failed     []string
}

func (f *fakeMetadata) Fetch(_ context.Context, rawURL string) (*metadata.Metadata, error) {
	f.lastURL = rawURL
	return f.meta, f.fetchErr
}

func (f *fakeMetadata) FetchWaiting(ctx context.Context, rawURL string) (*metadata.Metadata, error) {
	return f.Fetch(ctx, rawURL)
}

func (f *fakeMetadata) FetchImage(_ context.Context, _ string) ([]byte, string, error) {
	return f.img, f.imgCT, f.imgErr
}

func (f *fakeMetadata) SearchStores(_ context.Context, _ string) ([]metadata.Candidate, []string) {
	return f.candidates, f.failed
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestMetadataFetch_FillsBlanks(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{
		meta: &metadata.Metadata{
			Store:       metadata.StoreSketchfab,
			Title:       "Low Poly Tree",
			Description: "A stylised tree.",
			LicenceHint: "CC-BY-4.0",
			CoverURL:    "https://cdn.example.com/cover.png",
			ListingURL:  "https://sketchfab.com/3d-models/low-poly-tree-abc123",
		},
		img:   tinyPNG(t),
		imgCT: "image/png",
	}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "tree")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch",
		map[string]any{"url": "https://sketchfab.com/3d-models/low-poly-tree-abc123"})
	requireStatus(t, w, http.StatusOK)

	resp := jsonResponse[metadataFetchResponse](t, w)
	// The store title differs from the filename-derived name, so a rename
	// surfaces as a conflict (names are never blank, so never fill silently).
	if len(resp.Conflicts) != 1 || resp.Conflicts[0].Field != "name" {
		t.Fatalf("expected a single name conflict, got %+v", resp.Conflicts)
	}
	assertContains(t, resp.Applied, "description", "licence", "thumbnail", "source_url")

	var refreshed models.Asset
	if err := db.First(&refreshed, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	// A no-op handler would leave these blank — these assertions fail against a stub.
	if refreshed.Description != "A stylised tree." {
		t.Errorf("description not filled: %q", refreshed.Description)
	}
	if refreshed.SourceURL == "" {
		t.Error("source_url not set from body")
	}
	if refreshed.ThumbnailObjectKey == "" {
		t.Error("thumbnail not set from cover")
	}
	if refreshed.LicenceID == nil {
		t.Fatal("licence not created")
	}
	if len(refreshed.SourceMetadataJSON) == 0 {
		t.Error("source metadata not persisted")
	}
	var lic models.Licence
	if err := db.First(&lic, *refreshed.LicenceID).Error; err != nil {
		t.Fatal(err)
	}
	if lic.DetectedType != "CC-BY-4.0" {
		t.Errorf("licence type: got %q", lic.DetectedType)
	}

	// source_metadata must surface in the detail view.
	dw := performGET(r, "/api/assets/1")
	requireStatus(t, dw, http.StatusOK)
	detail := jsonResponse[assetDetailView](t, dw)
	if detail.SourceMetadata == nil || detail.SourceMetadata.Store != metadata.StoreSketchfab {
		t.Errorf("source_metadata missing from detail view: %+v", detail.SourceMetadata)
	}

	// One audit entry covers everything the apply changed, licence and
	// thumbnail included.
	var entry models.AuditLog
	if err := db.Where("action_type = ?", "asset.metadata.apply").First(&entry).Error; err != nil {
		t.Fatalf("audit entry not written: %v", err)
	}
	diff := string(entry.DiffJSON)
	for _, want := range []string{"description", "licence", "thumbnail_object_key"} {
		if !strings.Contains(diff, want) {
			t.Errorf("audit diff missing %q: %s", want, diff)
		}
	}
}

func TestMetadataFetch_ConflictsNotApplied(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{meta: &metadata.Metadata{
		Store:       metadata.StoreGeneric,
		Description: "fetched description",
	}}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "x")
	asset.Description = "human-written description"
	asset.SourceURL = "https://example.com/x"
	if err := db.Save(asset).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil)
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[metadataFetchResponse](t, w)

	if len(resp.Conflicts) != 1 || resp.Conflicts[0].Field != "description" {
		t.Fatalf("expected a description conflict, got %+v", resp.Conflicts)
	}

	var refreshed models.Asset
	db.First(&refreshed, asset.ID)
	// The existing human value must NOT be clobbered on fill-blanks.
	if refreshed.Description != "human-written description" {
		t.Errorf("existing description was overwritten: %q", refreshed.Description)
	}
}

func TestMetadataApply_OverwritesConfirmed(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{meta: &metadata.Metadata{
		Store:       metadata.StoreGeneric,
		Description: "fetched description",
	}}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "x")
	asset.Description = "old"
	if err := db.Save(asset).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	// Fetch first to persist the source record.
	fw := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch",
		map[string]any{"url": "https://example.com/x"})
	requireStatus(t, fw, http.StatusOK)

	// Apply confirms the description overwrite.
	aw := performRequest(r, http.MethodPost, "/api/assets/1/metadata/apply",
		map[string]any{"fields": []string{"description"}})
	requireStatus(t, aw, http.StatusOK)

	var refreshed models.Asset
	db.First(&refreshed, asset.ID)
	if refreshed.Description != "fetched description" {
		t.Errorf("confirmed overwrite not applied: %q", refreshed.Description)
	}
}

func TestMetadataApply_RenamesFromTitleOnConfirm(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreUnity,
		Title: "Fantasy Pack Deluxe",
	}}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "fantasy pack")
	asset.SourceURL = "https://assetstore.unity.com/packages/x-1"
	if err := db.Save(asset).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	fw := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil)
	requireStatus(t, fw, http.StatusOK)
	fresp := jsonResponse[metadataFetchResponse](t, fw)
	if len(fresp.Conflicts) != 1 || fresp.Conflicts[0].Field != "name" {
		t.Fatalf("expected a name conflict, got %+v", fresp.Conflicts)
	}

	// Not yet confirmed — name unchanged.
	var mid models.Asset
	db.First(&mid, asset.ID)
	if mid.Name != "fantasy pack" {
		t.Fatalf("name changed before confirm: %q", mid.Name)
	}

	aw := performRequest(r, http.MethodPost, "/api/assets/1/metadata/apply",
		map[string]any{"fields": []string{"name"}})
	requireStatus(t, aw, http.StatusOK)

	var refreshed models.Asset
	db.First(&refreshed, asset.ID)
	if refreshed.Name != "Fantasy Pack Deluxe" {
		t.Errorf("confirmed rename not applied: %q", refreshed.Name)
	}
}

func TestMetadataFetch_StoresGalleryImages(t *testing.T) {
	h, db := newTestHandler(t)
	fake := &fakeMetadata{
		meta: &metadata.Metadata{
			Store:     metadata.StoreUnity,
			Title:     "asset", // equal to the asset name → no rename conflict
			ImageURLs: []string{"https://cdn.example.com/1.jpg", "https://cdn.example.com/2.jpg"},
		},
		img:   tinyPNG(t),
		imgCT: "image/png",
	}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")
	asset.SourceURL = "https://assetstore.unity.com/packages/x-1"
	if err := db.Save(asset).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil)
	requireStatus(t, w, http.StatusOK)

	var count int64
	db.Model(&models.AssetImage{}).Where("asset_id = ?", asset.ID).Count(&count)
	// A no-op would store nothing; this fails against it.
	if count != 2 {
		t.Fatalf("expected 2 gallery images, got %d", count)
	}

	// Re-fetch must skip already-stored source URLs.
	w2 := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil)
	requireStatus(t, w2, http.StatusOK)
	db.Model(&models.AssetImage{}).Where("asset_id = ?", asset.ID).Count(&count)
	if count != 2 {
		t.Fatalf("re-fetch duplicated gallery images: got %d", count)
	}

	// The detail view surfaces them and the thumb endpoint authorises the key.
	dw := performGET(r, "/api/assets/1")
	requireStatus(t, dw, http.StatusOK)
	detail := jsonResponse[assetDetailView](t, dw)
	if len(detail.Images) != 2 {
		t.Fatalf("detail view images: got %d", len(detail.Images))
	}
	tw := performGET(r, "/api/thumbs/"+detail.Images[0].ObjectKey)
	requireStatus(t, tw, http.StatusOK)
}

func TestMetadataFetch_AttachesTags(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreUnity,
		Title: "asset", // equal to the asset name → no rename conflict
		Tags:  []string{"Nature", "Low Poly", "nature"},
	}}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset")
	asset.SourceURL = "https://assetstore.unity.com/packages/x-1"
	if err := db.Save(asset).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil)
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[metadataFetchResponse](t, w)
	assertContains(t, resp.Applied, "tags")

	// "Nature"/"nature" collapse; both paths lowercase.
	var tagRows []models.Tag
	if err := db.Where("library_id = ?", lib.ID).Order("path").Find(&tagRows).Error; err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(tagRows))
	for i, tg := range tagRows {
		paths[i] = tg.Path
	}
	if strings.Join(paths, ",") != "low poly,nature" {
		t.Fatalf("tag paths: got %v", paths)
	}
	var linkCount int64
	db.Model(&models.AssetTag{}).Where("asset_id = ?", asset.ID).Count(&linkCount)
	if linkCount != 2 {
		t.Fatalf("expected 2 tag links, got %d", linkCount)
	}

	// Additive + idempotent: a second fetch neither duplicates links nor
	// re-reports "tags" applied (LinkTag audits/reports only new links).
	w2 := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil)
	requireStatus(t, w2, http.StatusOK)
	resp2 := jsonResponse[metadataFetchResponse](t, w2)
	for _, f := range resp2.Applied {
		if f == "tags" {
			t.Errorf("second fetch should not re-report tags applied: %v", resp2.Applied)
		}
	}
	db.Model(&models.AssetTag{}).Where("asset_id = ?", asset.ID).Count(&linkCount)
	if linkCount != 2 {
		t.Fatalf("links duplicated on re-fetch: %d", linkCount)
	}
}

func TestMetadataFetch_NoURL(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{}}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "x")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil)
	requireStatus(t, w, http.StatusBadRequest)
}

func TestMetadataFetch_ServiceUnavailable(t *testing.T) {
	h, db := newTestHandler(t) // no metadata service wired
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "x")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch",
		map[string]any{"url": "https://example.com/x"})
	requireStatus(t, w, http.StatusServiceUnavailable)
}

func TestMetadataSearch_Endpoint(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{candidates: []metadata.Candidate{
		{Title: "Fantasy Pack", ListingURL: "https://assetstore.unity.com/packages/x-1", Store: metadata.StoreUnity},
		{Title: "Rock", ListingURL: "https://sketchfab.com/models/abc", Store: metadata.StoreSketchfab},
	}, failed: []string{metadata.StoreItch}}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")

	r := setupRouter(h, u)
	w := performGET(r, "/api/metadata/search?q=fantasy")
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[metadataSearchResponse](t, w)
	if resp.Query != "fantasy" || len(resp.Candidates) != 2 {
		t.Fatalf("unexpected search response: %+v", resp)
	}
	if len(resp.FailedStores) != 1 || resp.FailedStores[0] != metadata.StoreItch {
		t.Fatalf("a store that failed should be named: %+v", resp.FailedStores)
	}
}

func TestMetadataSearch_RequiresQuery(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)
	w := performGET(r, "/api/metadata/search?q=")
	requireStatus(t, w, http.StatusBadRequest)
}

func TestMetadataImage_ProxiesImage(t *testing.T) {
	h, db := newTestHandler(t)
	png := tinyPNG(t)
	h.metadata = &fakeMetadata{img: png, imgCT: "image/png"}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")

	r := setupRouter(h, u)
	w := performGET(r, "/api/metadata/image?url=https://cdn.example.com/cover.png")
	requireStatus(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("content-type = %q, want image/png", ct)
	}
	if !bytes.Equal(w.Body.Bytes(), png) {
		t.Fatalf("body mismatch: got %d bytes, want %d", w.Body.Len(), len(png))
	}
}

func TestMetadataImage_RejectsNonImage(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{img: []byte("<html>gotcha</html>"), imgCT: "text/html"}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")

	r := setupRouter(h, u)
	w := performGET(r, "/api/metadata/image?url=https://evil.example.com/page")
	requireStatus(t, w, http.StatusBadGateway)
}

func TestMetadataImage_RequiresURL(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")

	r := setupRouter(h, u)
	w := performGET(r, "/api/metadata/image")
	requireStatus(t, w, http.StatusBadRequest)
}

func assertContains(t *testing.T, got []string, want ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("expected applied to contain %q; got %v", w, got)
		}
	}
}

func TestMetadataFetch_FillsAndConflictsOnStorefrontFields(t *testing.T) {
	released := time.Date(2020, 2, 11, 9, 47, 13, 0, time.UTC)
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{
		Store:        metadata.StoreUnity,
		Title:        "tree",
		Publisher:    "JustCreate",
		PublisherURL: "https://justcreate3d.studio/",
		StoreID:      "162153",
		Engine:       "unity",
		ReleasedAt:   &released,
		Rating:       4.85,
		RatingCount:  126,
		CategoryPath: "3d/environments/landscapes",
		ListingURL:   "https://assetstore.unity.com/packages/package/162153",
	}}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "tree")

	// A hand-set publisher must not be silently replaced.
	handEdited := "Hand Edited Studio"
	if err := db.Model(asset).Update("publisher", &handEdited).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch",
		map[string]any{"url": "https://assetstore.unity.com/packages/package/162153"})
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[metadataFetchResponse](t, w)

	assertContains(t, resp.Applied, "store", "store_product_id", "engine", "released_at", "rating", "rating_count", "tags")
	var publisherConflict *enrich.Conflict
	for i := range resp.Conflicts {
		if resp.Conflicts[i].Field == "publisher" {
			publisherConflict = &resp.Conflicts[i]
		}
	}
	if publisherConflict == nil {
		t.Fatalf("no publisher conflict reported: %+v", resp.Conflicts)
	}
	if publisherConflict.Current != handEdited || publisherConflict.Proposed != "JustCreate" {
		t.Fatalf("publisher conflict = %+v", *publisherConflict)
	}

	var refreshed models.Asset
	if err := db.First(&refreshed, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Publisher == nil || *refreshed.Publisher != handEdited {
		t.Fatalf("publisher = %v, want the hand-edited value kept", refreshed.Publisher)
	}
	if refreshed.Store == nil || *refreshed.Store != metadata.StoreUnity {
		t.Fatalf("store = %v", refreshed.Store)
	}
	if refreshed.Rating == nil || *refreshed.Rating != 4.85 {
		t.Fatalf("rating = %v", refreshed.Rating)
	}
	if refreshed.ReleasedAt == nil || !refreshed.ReleasedAt.Equal(released) {
		t.Fatalf("released_at = %v", refreshed.ReleasedAt)
	}

	// The category arrives as one hierarchical tag path, not three flat tags.
	var paths []string
	if err := db.Table("tags t").
		Joins("JOIN asset_tags at ON at.tag_id = t.id").
		Where("at.asset_id = ?", asset.ID).
		Pluck("t.path", &paths).Error; err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range paths {
		if p == "3d/environments/landscapes" {
			found = true
		}
		if strings.Contains(p, ">") {
			t.Errorf("tag path %q carries category arrow separators", p)
		}
	}
	if !found {
		t.Fatalf("category path not attached as a tag: %v", paths)
	}
}

// Confirming a conflicted storefront field overwrites it and audits the change.
func TestMetadataApply_ConfirmsStorefrontField(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{
		Store:      metadata.StoreUnity,
		Title:      "tree",
		Publisher:  "JustCreate",
		ListingURL: "https://assetstore.unity.com/packages/package/162153",
	}}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "tree")
	handEdited := "Hand Edited Studio"
	if err := db.Model(asset).Update("publisher", &handEdited).Error; err != nil {
		t.Fatal(err)
	}

	r := setupRouter(h, u)
	requireStatus(t, performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch",
		map[string]any{"url": "https://assetstore.unity.com/packages/package/162153"}), http.StatusOK)

	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/apply",
		map[string]any{"fields": []string{"publisher"}})
	requireStatus(t, w, http.StatusOK)

	var refreshed models.Asset
	if err := db.First(&refreshed, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Publisher == nil || *refreshed.Publisher != "JustCreate" {
		t.Fatalf("publisher = %v, want the confirmed overwrite", refreshed.Publisher)
	}
	var entry models.AuditLog
	if err := db.Where("action_type = ?", "asset.metadata.apply").
		Order("id DESC").First(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entry.DiffJSON), handEdited) {
		t.Fatalf("audit diff does not record the previous publisher: %s", entry.DiffJSON)
	}
}

// The deprecated flag is the store's state, so a plain Fetch metadata sets it
// and a later one that finds the listing live clears it, even though neither
// may overwrite a field a person could have written.
func TestMetadataFetch_SetsAndClearsStoreDeprecated(t *testing.T) {
	h, db := newTestHandler(t)
	deprecated := true
	fake := &fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreUnity, StoreDeprecated: &deprecated}}
	h.metadata = fake
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "pack")
	r := setupRouter(h, u)
	url := map[string]any{"url": "https://assetstore.unity.com/packages/slug/32351"}

	requireStatus(t, performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", url), http.StatusOK)
	w := performRequest(r, http.MethodGet, "/api/assets/1", nil)
	requireStatus(t, w, http.StatusOK)
	if got := jsonResponse[map[string]any](t, w)["store_deprecated"]; got != true {
		t.Fatalf("store_deprecated = %v after a deprecated lookup, want true", got)
	}

	live := false
	fake.meta = &metadata.Metadata{Store: metadata.StoreUnity, StoreDeprecated: &live}
	requireStatus(t, performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil), http.StatusOK)
	var stored models.Asset
	if err := db.First(&stored, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.StoreDeprecated {
		t.Fatal("a lookup that found the listing live must clear store_deprecated")
	}

	// A lookup that couldn't tell (no listing page read) leaves the flag alone.
	if err := db.Model(&stored).Update("store_deprecated", true).Error; err != nil {
		t.Fatal(err)
	}
	fake.meta = &metadata.Metadata{Store: metadata.StoreUnity}
	requireStatus(t, performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch", nil), http.StatusOK)
	if err := db.First(&stored, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.StoreDeprecated {
		t.Fatal("a lookup with no deprecated state must leave the flag as it was")
	}
}

func TestMetadataFetch_ReportsGenericFallback(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreGeneric, StoreLookupFailed: true}}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "pack")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch",
		map[string]any{"url": "https://assetstore.unity.com/packages/slug/32351"})
	requireStatus(t, w, http.StatusOK)
	if !jsonResponse[metadataFetchResponse](t, w).StoreLookupFailed {
		t.Fatal("a generic fallback must come back as store_lookup_failed")
	}
}

// A generic answer is not the store's details, so the record a store lookup
// left stays; otherwise the next refresh reads the scraper's values as a
// person's and keeps them.
func TestMetadataFetch_GenericFallbackKeepsTheStoreRecord(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreGeneric, Title: "Page Title", StoreLookupFailed: true,
	}}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "pack")
	if err := enrich.SaveSourceRecord(db, asset, metadata.SourceRecord{
		Store: metadata.StoreUnity, Matched: true,
		Metadata: &metadata.Metadata{Store: metadata.StoreUnity, Title: "pack"},
	}); err != nil {
		t.Fatal(err)
	}
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, fmt.Sprintf("/api/assets/%d/metadata/fetch", asset.ID),
		map[string]any{"url": "https://assetstore.unity.com/packages/slug/32351"})
	requireStatus(t, w, http.StatusOK)
	var stored models.Asset
	if err := db.First(&stored, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	rec := enrich.SourceRecordOf(&stored)
	if rec == nil || rec.Store != metadata.StoreUnity || rec.Metadata.Title != "pack" {
		t.Fatalf("record = %+v, want the Unity record kept", rec)
	}
}

// A store that asked for a pause is not a bad link: the person is told when
// to try again.
func TestMetadataFetch_StorePauseIs503WithRetryAfter(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata = &fakeMetadata{fetchErr: &metadata.RateLimitedError{
		Host: "assetstore.unity.com", Until: time.Now().Add(30 * time.Second),
	}}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "pack")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/assets/1/metadata/fetch",
		map[string]any{"url": "https://assetstore.unity.com/packages/slug/32351"})
	requireStatus(t, w, http.StatusServiceUnavailable)
	if got := w.Header().Get("Retry-After"); got != "30" {
		t.Fatalf("Retry-After = %q, want 30", got)
	}
}
