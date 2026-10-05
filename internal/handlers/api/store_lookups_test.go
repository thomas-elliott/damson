package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// heldLookups queues lookups without running them, so a test can check what
// a request did before the server looks anything up.
type heldLookups struct{ ids []uint }

func (q *heldLookups) EnqueueStoreLookup(id uint) { q.ids = append(q.ids, id) }

// runAll runs every held lookup as the queue's worker would.
func (q *heldLookups) runAll(t *testing.T, h *Handler) {
	t.Helper()
	for _, id := range q.ids {
		if err := h.RunStoreLookup(context.Background(), id); err != nil {
			t.Fatalf("RunStoreLookup(%d): %v", id, err)
		}
	}
	q.ids = nil
}

func seedLinkedAsset(t *testing.T, db *gorm.DB, libID uint, name, link string, edit func(*models.Asset)) *models.Asset {
	t.Helper()
	a := models.Asset{LibraryID: libID, Name: name, SourceURL: link}
	if edit != nil {
		edit(&a)
	}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return &a
}

func strPtr(s string) *string { return &s }

func TestStoreLookups_QueueThenFillBlanksOnly(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	fake := &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreUnity, Title: "Store Title", Description: "From the store.", Publisher: "Unity Technologies",
	}}
	h.metadata, h.storeLookups = fake, q
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	blank := seedLinkedAsset(t, db, lib.ID, "Blank", "https://assetstore.unity.com/packages/slug/1", nil)
	edited := seedLinkedAsset(t, db, lib.ID, "Edited", "https://assetstore.unity.com/packages/slug/2", func(a *models.Asset) {
		a.Description = "My own words."
	})
	unlinked := seedAsset(t, db, lib.ID, "No link")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/store-lookups",
		map[string]any{"asset_ids": []uint{blank.ID, edited.ID, unlinked.ID}})
	requireStatus(t, w, http.StatusAccepted)
	queued := jsonResponse[storeLookupBatchResponse](t, w)
	if queued.Total != 2 || queued.Queued != 2 || queued.SkippedNoLink != 1 {
		t.Fatalf("got %+v, want 2 queued and 1 skipped for having no link", queued)
	}
	if fake.lastURL != "" {
		t.Fatal("queuing must not look the store up in the request")
	}

	q.runAll(t, h)
	var got models.Asset
	db.First(&got, blank.ID)
	if got.Description != "From the store." || derefString(got.Publisher) != "Unity Technologies" {
		t.Fatalf("blank asset: description %q publisher %v, want both filled", got.Description, got.Publisher)
	}
	if len(got.SourceMetadataJSON) == 0 {
		t.Fatal("a background lookup must save the source record")
	}
	var kept models.Asset
	db.First(&kept, edited.ID)
	if kept.Description != "My own words." || kept.Name != "Edited" {
		t.Fatalf("edited asset: name %q description %q, want both kept", kept.Name, kept.Description)
	}

	w = performRequest(r, http.MethodGet, "/api/store-lookups/batches/"+uintStr(queued.BatchID), nil)
	requireStatus(t, w, http.StatusOK)
	status := jsonResponse[storeLookupBatchResponse](t, w)
	if status.Done != 2 || status.Queued != 0 || status.Changed != 2 || status.Conflicted != 2 {
		t.Fatalf("batch = %+v, want 2 done, both changed, both with a name conflict", status)
	}
}

func TestStoreLookups_FiltersSelectTheAssets(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	h.metadata, h.storeLookups = &fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreUnity}}, q
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	generic := seedLinkedAsset(t, db, lib.ID, "Generic", "https://assetstore.unity.com/packages/slug/1", func(a *models.Asset) {
		a.Store = strPtr("generic")
	})
	seedLinkedAsset(t, db, lib.ID, "Unity", "https://assetstore.unity.com/packages/slug/2", func(a *models.Asset) {
		a.Store = strPtr("unity")
	})
	seedLinkedAsset(t, db, lib.ID, "Elsewhere", "https://example.com/pack", func(a *models.Asset) {
		a.Store = strPtr("generic")
	})
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/store-lookups?store=generic&source_host=assetstore.unity.com",
		map[string]any{"refresh": true})
	requireStatus(t, w, http.StatusAccepted)
	resp := jsonResponse[storeLookupBatchResponse](t, w)
	if resp.Total != 1 || !resp.Refresh {
		t.Fatalf("got %+v, want one refresh lookup", resp)
	}
	var lk models.StoreLookup
	db.First(&lk, q.ids[0])
	if lk.AssetID != generic.ID {
		t.Fatalf("queued asset %d, want the generic Unity-linked one %d", lk.AssetID, generic.ID)
	}

	// No asset IDs and no filter would be the whole library.
	w = performRequest(r, http.MethodPost, "/api/store-lookups?sort=name_asc", map[string]any{})
	requireStatus(t, w, http.StatusBadRequest)
}

func TestStoreLookups_ScopedToTheLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	h.metadata, h.storeLookups = &fakeMetadata{meta: &metadata.Metadata{}}, &heldLookups{}
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	other := seedUser(t, db, "o@example.com")
	otherLib := seedLibraryForUser(t, db, other, "Theirs")
	theirs := seedLinkedAsset(t, db, otherLib.ID, "Theirs", "https://example.com/x", nil)
	mine := seedLinkedAsset(t, db, lib.ID, "Mine", "https://example.com/y", nil)

	w := performRequest(setupRouter(h, other), http.MethodPost, "/api/store-lookups",
		map[string]any{"asset_ids": []uint{theirs.ID}})
	requireStatus(t, w, http.StatusAccepted)
	theirBatch := jsonResponse[storeLookupBatchResponse](t, w).BatchID

	r := setupRouter(h, u)
	requireStatus(t, performRequest(r, http.MethodPost, "/api/store-lookups",
		map[string]any{"asset_ids": []uint{mine.ID, theirs.ID}}), http.StatusNotFound)
	requireStatus(t, performRequest(r, http.MethodGet, "/api/store-lookups/batches/"+uintStr(theirBatch), nil), http.StatusNotFound)
	requireStatus(t, performRequest(r, http.MethodPost, "/api/store-lookups",
		map[string]any{"asset_ids": []uint{mine.ID}, "batch_id": theirBatch}), http.StatusNotFound)
}

func TestStoreLookups_ReportFailuresAndGenericFallbacks(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	fake := &fakeMetadata{}
	h.metadata, h.storeLookups = fake, q
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	broken := seedLinkedAsset(t, db, lib.ID, "Broken", "https://example.com/a", nil)
	scraped := seedLinkedAsset(t, db, lib.ID, "Scraped", "https://example.com/b", nil)
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/store-lookups", map[string]any{"asset_ids": []uint{broken.ID}})
	requireStatus(t, w, http.StatusAccepted)
	batchID := jsonResponse[storeLookupBatchResponse](t, w).BatchID
	fake.fetchErr = errors.New("unity: no product 1 in coveo, and its listing page could not be read")
	q.runAll(t, h)

	requireStatus(t, performRequest(r, http.MethodPost, "/api/store-lookups",
		map[string]any{"asset_ids": []uint{scraped.ID}, "batch_id": batchID}), http.StatusAccepted)
	fake.fetchErr = nil
	fake.meta = &metadata.Metadata{Store: metadata.StoreGeneric, StoreLookupFailed: true}
	q.runAll(t, h)

	w = performRequest(r, http.MethodGet, "/api/store-lookups/batches/"+uintStr(batchID), nil)
	requireStatus(t, w, http.StatusOK)
	status := jsonResponse[storeLookupBatchResponse](t, w)
	if status.Total != 2 || status.Failed != 1 || status.NoStoreData != 1 || len(status.Problems) != 2 {
		t.Fatalf("batch = %+v, want one failed and one no_store_data, both listed", status)
	}
	if status.Problems[0].Error == "" {
		t.Fatalf("a failed lookup must say why: %+v", status.Problems[0])
	}
}

// Recording with queue_lookup answers without a store lookup, joins the
// recording's batch, and the queued lookup saves the source record.
func TestRecordReference_QueuedLookup(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	fake := &fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreUnity, Title: "Toony Skies", Publisher: "Arkham"}}
	h.metadata, h.storeLookups = fake, q
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	first := recordReference(t, r, map[string]any{
		"source_url": "https://assetstore.unity.com/packages/slug/11020", "name": "Toony Skies", "queue_lookup": true,
	}, http.StatusCreated)
	if fake.lastURL != "" {
		t.Fatal("a queued record must not look the store up in the request")
	}
	if first.LookupBatchID == 0 || len(q.ids) != 1 {
		t.Fatalf("got batch %d and %d queued, want a new batch with one lookup", first.LookupBatchID, len(q.ids))
	}
	second := recordReference(t, r, map[string]any{
		"source_url": "https://assetstore.unity.com/packages/slug/11021", "name": "Other", "queue_lookup": true,
		"lookup_batch_id": first.LookupBatchID,
	}, http.StatusCreated)
	if second.LookupBatchID != first.LookupBatchID {
		t.Fatalf("second record joined batch %d, want %d", second.LookupBatchID, first.LookupBatchID)
	}

	// A record of a link already held queues nothing.
	again := recordReference(t, r, map[string]any{
		"source_url": "https://assetstore.unity.com/packages/slug/11020", "name": "Toony Skies", "queue_lookup": true,
		"lookup_batch_id": first.LookupBatchID,
	}, http.StatusOK)
	if again.Outcome != referenceExists || len(q.ids) != 2 {
		t.Fatalf("got %q with %d queued, want exists and nothing more queued", again.Outcome, len(q.ids))
	}

	q.runAll(t, h)
	var a models.Asset
	db.First(&a, first.AssetID)
	if len(a.SourceMetadataJSON) == 0 || derefString(a.Publisher) != "Arkham" {
		t.Fatalf("recorded asset: publisher %v, source record %d bytes; want both from the lookup", a.Publisher, len(a.SourceMetadataJSON))
	}

	// A batch that isn't the library's is refused before anything is recorded.
	w := performRequest(r, http.MethodPost, "/api/assets/references", map[string]any{
		"source_url": "https://assetstore.unity.com/packages/slug/9", "name": "Nine", "queue_lookup": true, "lookup_batch_id": 9999,
	})
	requireStatus(t, w, http.StatusNotFound)
	var n int64
	db.Model(&models.Asset{}).Where("name = ?", "Nine").Count(&n)
	if n != 0 {
		t.Fatal("a refused record must not create the asset")
	}
}
