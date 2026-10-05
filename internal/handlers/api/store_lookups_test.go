package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

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
	// Both kept their names, but a fill-blanks lookup keeps every set value by
	// design, so that is not reported.
	if status.Done != 2 || status.Queued != 0 || status.Changed != 2 || status.Conflicted != 0 || len(status.Problems) != 0 {
		t.Fatalf("batch = %+v, want 2 done, both changed, and nothing reported as kept", status)
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

	// A filter that can't be read would otherwise be dropped, refreshing more
	// assets than were asked for.
	w = performRequest(r, http.MethodPost, "/api/store-lookups?store=unity&released_after=2024/01/01",
		map[string]any{"refresh": true})
	requireStatus(t, w, http.StatusBadRequest)
	for _, unreadable := range []string{"store_deprecated=yes", "updates_available=yes"} {
		w = performRequest(r, http.MethodPost, "/api/store-lookups?store=unity&"+unreadable,
			map[string]any{"refresh": true})
		requireStatus(t, w, http.StatusBadRequest)
	}
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

// sourceRecordJSON is a stored source record holding last as the previous
// lookup's answer.
func sourceRecordJSON(t *testing.T, last *metadata.Metadata) []byte {
	t.Helper()
	b, err := json.Marshal(metadata.SourceRecord{Store: last.Store, Matched: true, Metadata: last})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// lookUp queues one lookup of the asset and runs it, returning its batch.
func lookUp(t *testing.T, h *Handler, r http.Handler, q *heldLookups, assetID uint, refresh bool) storeLookupBatchResponse {
	t.Helper()
	w := performRequest(r, http.MethodPost, "/api/store-lookups", map[string]any{"asset_ids": []uint{assetID}, "refresh": refresh})
	requireStatus(t, w, http.StatusAccepted)
	batchID := jsonResponse[storeLookupBatchResponse](t, w).BatchID
	q.runAll(t, h)
	w = performRequest(r, http.MethodGet, "/api/store-lookups/batches/"+uintStr(batchID), nil)
	requireStatus(t, w, http.StatusOK)
	return jsonResponse[storeLookupBatchResponse](t, w)
}

func conflictFields(b storeLookupBatchResponse) []string {
	var out []string
	for _, p := range b.Problems {
		for _, c := range p.Conflicts {
			out = append(out, c.Field)
		}
	}
	return out
}

// The purchase run's case: an asset whose last lookup was the generic
// scraper's. A refresh replaces what that lookup wrote and fills blanks, but
// keeps what a person wrote, reporting it.
func TestStoreRefresh_ReplacesOnlyWhatAPersonHasNotChanged(t *testing.T) {
	released := time.Date(2015, 3, 16, 7, 40, 35, 0, time.UTC)
	last := &metadata.Metadata{Store: metadata.StoreGeneric, Title: "Page Title | Unity Asset Store", Description: "Short page text."}
	store := &metadata.Metadata{
		Store: metadata.StoreUnity, Title: "Standard Assets", Description: "The store's full description.",
		Publisher: "Unity Technologies", StoreID: "32351", ReleasedAt: &released,
	}
	seed := func(t *testing.T) (*Handler, *gorm.DB, http.Handler, *heldLookups, *models.Asset) {
		h, db := newTestHandler(t)
		q := &heldLookups{}
		h.metadata, h.storeLookups = &fakeMetadata{meta: store}, q
		u := seedUser(t, db, "t@example.com")
		lib := seedLibraryForUser(t, db, u, "Personal")
		a := seedLinkedAsset(t, db, lib.ID, "Plan Name", "https://assetstore.unity.com/packages/slug/32351", func(a *models.Asset) {
			a.Description = last.Description // as the last lookup wrote it
			a.Store = strPtr(metadata.StoreGeneric)
			a.Publisher = strPtr("Typed By Hand") // the last lookup gave none
			a.SourceMetadataJSON = sourceRecordJSON(t, last)
		})
		db.Model(a).Update("needs_review", false)
		return h, db, setupRouter(h, u), q, a
	}

	t.Run("refresh", func(t *testing.T) {
		h, db, r, q, a := seed(t)
		batch := lookUp(t, h, r, q, a.ID, true)

		var got models.Asset
		db.First(&got, a.ID)
		if got.Description != store.Description || derefString(got.Store) != metadata.StoreUnity {
			t.Fatalf("description %q store %v, want both replaced: the last lookup wrote them", got.Description, got.Store)
		}
		if got.ReleasedAt == nil || derefString(got.StoreProductID) != "32351" {
			t.Fatalf("released_at %v store_product_id %v, want blanks filled", got.ReleasedAt, got.StoreProductID)
		}
		if got.Name != "Plan Name" || derefString(got.Publisher) != "Typed By Hand" {
			t.Fatalf("name %q publisher %v, want both kept: a person wrote them", got.Name, got.Publisher)
		}
		assertContains(t, conflictFields(batch), "name", "publisher")
		if !got.NeedsReview || batch.Changed != 1 {
			t.Fatalf("needs_review %v, batch %+v: a refresh that changed fields marks the asset Needs review", got.NeedsReview, batch)
		}
	})

	t.Run("a lookup that is not a refresh", func(t *testing.T) {
		h, db, r, q, a := seed(t)
		lookUp(t, h, r, q, a.ID, false)

		var got models.Asset
		db.First(&got, a.ID)
		if got.Description != last.Description || derefString(got.Store) != metadata.StoreGeneric {
			t.Fatalf("description %q store %v: only an explicit refresh may replace a field with a value", got.Description, got.Store)
		}
		if got.ReleasedAt == nil {
			t.Fatal("blanks are still filled")
		}
	})
}

// With no source record to compare against, the audit log says what a person
// edited.
func TestStoreRefresh_NoSourceRecordUsesTheAuditLog(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	h.metadata, h.storeLookups = &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreUnity, Description: "The store's words.", Publisher: "Unity Technologies",
	}}, q
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	a := seedLinkedAsset(t, db, lib.ID, "Pack", "https://assetstore.unity.com/packages/slug/5", func(a *models.Asset) {
		a.Publisher = strPtr("Old Publisher")
	})
	r := setupRouter(h, u)
	requireStatus(t, performRequest(r, http.MethodPatch, "/api/assets/"+uintStr(a.ID),
		map[string]any{"description": "My words."}), http.StatusOK)

	batch := lookUp(t, h, r, q, a.ID, true)
	var got models.Asset
	db.First(&got, a.ID)
	if got.Description != "My words." {
		t.Fatalf("description %q, want the person's edit kept", got.Description)
	}
	if derefString(got.Publisher) != "Unity Technologies" {
		t.Fatalf("publisher %v, want it replaced: no person edited it", got.Publisher)
	}
	assertContains(t, conflictFields(batch), "description")
}

// A refresh that finds nothing new leaves the asset as it was, Needs review
// included.
func TestStoreRefresh_NoChangeLeavesNeedsReview(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	// The store sends a rating finer than the column keeps.
	same := &metadata.Metadata{Store: metadata.StoreUnity, Title: "Pack", Publisher: "P", Rating: 4.8456,
		CoverURL: "https://cdn.example.com/c.png"}
	h.metadata, h.storeLookups = &fakeMetadata{meta: same, img: tinyPNG(t), imgCT: "image/png"}, q
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	a := seedLinkedAsset(t, db, lib.ID, "Pack", "https://assetstore.unity.com/packages/slug/6", func(a *models.Asset) {
		a.Store, a.Publisher = strPtr(metadata.StoreUnity), strPtr("P")
		rating := 4.85
		a.Rating = &rating
		a.ThumbnailObjectKey, a.ThumbnailFromStore = "thumbs/aa/cover", true
		a.SourceMetadataJSON = sourceRecordJSON(t, same)
	})
	db.Model(a).Update("needs_review", false)
	r := setupRouter(h, u)

	batch := lookUp(t, h, r, q, a.ID, true)
	var got models.Asset
	db.First(&got, a.ID)
	if got.NeedsReview || batch.Unchanged != 1 || got.ThumbnailObjectKey != "thumbs/aa/cover" {
		t.Fatalf("needs_review %v thumbnail %q batch %+v, want the asset untouched", got.NeedsReview, got.ThumbnailObjectKey, batch)
	}
}

func TestListAssets_CarriesStoreFields(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedLinkedAsset(t, db, lib.ID, "Standard Assets", "https://assetstore.unity.com/packages/slug/32351", func(a *models.Asset) {
		a.Store, a.StoreDeprecated = strPtr(metadata.StoreUnity), true
	})
	w := performRequest(setupRouter(h, u), http.MethodGet, "/api/assets", nil)
	requireStatus(t, w, http.StatusOK)
	rows := jsonResponse[assetListResponse](t, w).Assets
	if len(rows) != 1 || rows[0].Store != "unity" || !rows[0].StoreDeprecated ||
		rows[0].SourceURL != "https://assetstore.unity.com/packages/slug/32351" {
		t.Fatalf("list rows = %+v, want store, source_url and store_deprecated", rows)
	}
}

// A refresh during a store outage gets only the generic scraper's answer. It
// must not replace the store's details with it, nor the store's source record.
func TestStoreRefresh_GenericFallbackOnlyFillsBlanks(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	h.metadata, h.storeLookups = &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreGeneric, Title: "Page Title", Description: "Page text.", Publisher: "From The Page",
		StoreLookupFailed: true,
	}}, q
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	last := &metadata.Metadata{Store: metadata.StoreUnity, Title: "Pack", Description: "The store's words."}
	a := seedLinkedAsset(t, db, lib.ID, "Pack", "https://assetstore.unity.com/packages/slug/7", func(a *models.Asset) {
		a.Store, a.Description, a.SourceMetadataJSON = strPtr(metadata.StoreUnity), last.Description, sourceRecordJSON(t, last)
	})
	db.Model(a).Update("needs_review", false)

	batch := lookUp(t, h, setupRouter(h, u), q, a.ID, true)
	var got models.Asset
	db.First(&got, a.ID)
	if derefString(got.Store) != metadata.StoreUnity || got.Name != "Pack" || got.Description != last.Description {
		t.Fatalf("store %v name %q description %q, want the store's details kept", got.Store, got.Name, got.Description)
	}
	if derefString(got.Publisher) != "From The Page" {
		t.Fatalf("publisher %v, want the blank filled", got.Publisher)
	}
	var kept metadata.SourceRecord
	if err := json.Unmarshal(got.SourceMetadataJSON, &kept); err != nil || kept.Store != metadata.StoreUnity {
		t.Fatalf("source record store %q (%v), want the store's record kept, not the scraper's", kept.Store, err)
	}
	if batch.NoStoreData != 1 {
		t.Fatalf("batch %+v, want the lookup counted as no_store_data", batch)
	}

	// The blank the scraper filled was a lookup's, not a person's, so the
	// store's next answer replaces it.
	h.metadata = &fakeMetadata{meta: &metadata.Metadata{Store: metadata.StoreUnity, Title: "Pack", Publisher: "The Publisher"}}
	lookUp(t, h, setupRouter(h, u), q, a.ID, true)
	db.First(&got, a.ID)
	if derefString(got.Publisher) != "The Publisher" {
		t.Fatalf("publisher %q, want the scraper's fill replaced by the store's", derefString(got.Publisher))
	}
}

func TestStoreRefresh_ReplacesWhatAnEarlierLookupKept(t *testing.T) {
	h, db := newTestHandler(t)
	q := &heldLookups{}
	h.metadata, h.storeLookups = &fakeMetadata{meta: &metadata.Metadata{
		Store: metadata.StoreUnity, Title: "Pack", Publisher: "New Listing's Publisher",
	}}, q
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	// The publisher is the one an earlier lookup of the old listing gave.
	last := &metadata.Metadata{Store: metadata.StoreUnity, Title: "Pack", Publisher: "Old Listing's Publisher"}
	a := seedLinkedAsset(t, db, lib.ID, "Pack", "https://assetstore.unity.com/packages/slug/7", func(a *models.Asset) {
		a.Publisher, a.SourceMetadataJSON = strPtr(last.Publisher), sourceRecordJSON(t, last)
	})

	// A fill-blanks lookup keeps the publisher; a refresh then replaces it,
	// because no person wrote it.
	lookUp(t, h, setupRouter(h, u), q, a.ID, false)
	batch := lookUp(t, h, setupRouter(h, u), q, a.ID, true)
	var got models.Asset
	db.First(&got, a.ID)
	if derefString(got.Publisher) != "New Listing's Publisher" {
		t.Fatalf("publisher %q (batch %+v), want the earlier lookup's value replaced", derefString(got.Publisher), batch)
	}
}
