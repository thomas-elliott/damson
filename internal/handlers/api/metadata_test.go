package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// fakeMetadata is an injectable metadataService so endpoint tests don't do
// real outbound HTTP (which the SSRF guard blocks in any case).
type fakeMetadata struct {
	meta       *metadata.Metadata
	fetchErr   error
	img        []byte
	imgCT      string
	imgErr     error
	lastURL    string
	candidates []metadata.Candidate
	searchErr  error
}

func (f *fakeMetadata) Fetch(_ context.Context, rawURL string) (*metadata.Metadata, error) {
	f.lastURL = rawURL
	return f.meta, f.fetchErr
}

func (f *fakeMetadata) FetchImage(_ context.Context, _ string) ([]byte, string, error) {
	return f.img, f.imgCT, f.imgErr
}

func (f *fakeMetadata) Search(_ context.Context, _ string) ([]metadata.Candidate, error) {
	return f.candidates, f.searchErr
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
	if len(resp.Conflicts) != 0 {
		t.Fatalf("expected no conflicts on blank asset, got %+v", resp.Conflicts)
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
	}}
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")

	r := setupRouter(h, u)
	w := performGET(r, "/api/metadata/search?q=fantasy")
	requireStatus(t, w, http.StatusOK)
	resp := jsonResponse[metadataSearchResponse](t, w)
	if resp.Query != "fantasy" || len(resp.Candidates) != 2 {
		t.Fatalf("unexpected search response: %+v", resp)
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
