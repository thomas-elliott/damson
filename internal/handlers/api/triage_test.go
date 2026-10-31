package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
)

func seedTriageAsset(t *testing.T, h *Handler, a models.Asset) models.Asset {
	t.Helper()
	needsReview := a.NeedsReview
	if err := h.db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	// needs_review defaults to true, so a false is dropped on insert.
	if !needsReview {
		if err := h.db.Model(&a).Update("needs_review", false).Error; err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func seedImport(t *testing.T, h *Handler, assetID uint, stage string) models.AssetImport {
	t.Helper()
	imp := models.AssetImport{AssetID: assetID, Stage: stage, UpdatedAt: time.Now().UTC()}
	if err := h.db.Create(&imp).Error; err != nil {
		t.Fatal(err)
	}
	return imp
}

func triageAssetIDs(rows []triageRow) map[uint]triageRow {
	out := make(map[uint]triageRow, len(rows))
	for _, r := range rows {
		out[r.AssetID] = r
	}
	return out
}

func TestGetTriage_ListsEveryNeedsReviewAsset(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	other := seedLibrary(t, db, "Other")

	// A reference recorded by path has no import, and still needs review.
	refPath := "/mnt/nfs/ref"
	ref := seedTriageAsset(t, h, models.Asset{LibraryID: lib.ID, Name: "ref", NeedsReview: true, IsReference: true, LocationPath: &refPath})
	imported := seedTriageAsset(t, h, models.Asset{
		LibraryID: lib.ID, Name: "imported", NeedsReview: true,
		SourceMetadataJSON: []byte(`{"matched":true}`),
	})
	seedImport(t, h, imported.ID, "done")
	// Unmatched metadata (search candidates only) is not a match.
	unmatched := seedTriageAsset(t, h, models.Asset{
		LibraryID: lib.ID, Name: "unmatched", NeedsReview: true,
		SourceMetadataJSON: []byte(`{"matched":false,"candidates":[]}`),
	})
	// In flight, and already reviewed: in the queue for its import.
	inFlight := seedTriageAsset(t, h, models.Asset{LibraryID: lib.ID, Name: "in-flight", NeedsReview: false})
	seedImport(t, h, inFlight.ID, "extracting")
	// Reviewed with a finished import: not in the queue.
	done := seedTriageAsset(t, h, models.Asset{LibraryID: lib.ID, Name: "done", NeedsReview: false})
	seedImport(t, h, done.ID, "done")
	// A retry superseded the failure: only the latest import counts.
	retried := seedTriageAsset(t, h, models.Asset{LibraryID: lib.ID, Name: "retried", NeedsReview: false})
	seedImport(t, h, retried.ID, "failed")
	seedImport(t, h, retried.ID, "done")
	// Another library's needs-review asset never shows.
	seedTriageAsset(t, h, models.Asset{LibraryID: other.ID, Name: "elsewhere", NeedsReview: true})

	w := performGET(setupRouter(h, u), "/api/triage")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[triageResponse](t, w)
	got := triageAssetIDs(body.Assets)
	if body.Total != 4 || len(got) != 4 {
		t.Fatalf("expected ref, imported, unmatched and in-flight, got total %d: %+v", body.Total, body.Assets)
	}
	for _, id := range []uint{ref.ID, imported.ID, unmatched.ID, inFlight.ID} {
		if _, ok := got[id]; !ok {
			t.Fatalf("asset %d missing from the queue: %+v", id, body.Assets)
		}
	}
	if body.Assets[0].AssetID != inFlight.ID {
		t.Fatalf("an open import sorts first, got %+v", body.Assets[0])
	}
	if got[inFlight.ID].Import == nil || got[inFlight.ID].Import.Stage != "extracting" {
		t.Fatalf("in-flight row carries its import: %+v", got[inFlight.ID])
	}
	if got[imported.ID].Import != nil {
		t.Fatalf("a done import is not shown on the row: %+v", got[imported.ID].Import)
	}
	if !got[imported.ID].Matched || got[unmatched.ID].Matched || got[ref.ID].Matched {
		t.Fatalf("matched follows source metadata's matched flag: %+v", body.Assets)
	}
}

func TestPostImportDismiss_TakesTheFailureOutOfTheQueue(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	r := setupRouter(h, u)

	asset := seedTriageAsset(t, h, models.Asset{LibraryID: lib.ID, Name: "broken", NeedsReview: true})
	failed := seedImport(t, h, asset.ID, "failed")

	w := performGET(r, "/api/triage")
	requireStatus(t, w, http.StatusOK)
	if body := jsonResponse[triageResponse](t, w); body.Total != 1 || body.Assets[0].Import == nil {
		t.Fatalf("expected the failed import in the queue, got %+v", body)
	}

	w = performRequest(r, http.MethodPost, "/api/imports/"+itoa(failed.ID)+"/dismiss", nil)
	requireStatus(t, w, http.StatusNoContent)

	w = performGET(r, "/api/triage")
	requireStatus(t, w, http.StatusOK)
	body := jsonResponse[triageResponse](t, w)
	if body.Total != 0 {
		t.Fatalf("expected an empty queue after dismissing, got %+v", body.Assets)
	}
	if len(body.Reviewed) != 1 || body.Reviewed[0].AssetID != asset.ID {
		t.Fatalf("expected the dismissed asset in the reviewed tail, got %+v", body.Reviewed)
	}

	var audits int64
	db.Model(&models.AuditLog{}).Where("action_type = ? AND target_id = ?", "asset.import.dismiss", asset.ID).Count(&audits)
	if audits != 1 {
		t.Fatalf("expected one dismiss audit row, got %d", audits)
	}

	// Dismissing again is a no-op.
	w = performRequest(r, http.MethodPost, "/api/imports/"+itoa(failed.ID)+"/dismiss", nil)
	requireStatus(t, w, http.StatusNoContent)
}

func TestPostImportDismiss_RefusesUnfailedAndOtherLibraries(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "librarian@example.com")
	lib := seedLibraryAndSwitch(t, h, u)
	other := seedLibrary(t, db, "Other")
	r := setupRouter(h, u)

	mine := seedTriageAsset(t, h, models.Asset{LibraryID: lib.ID, Name: "mine", NeedsReview: true})
	done := seedImport(t, h, mine.ID, "done")
	w := performRequest(r, http.MethodPost, "/api/imports/"+itoa(done.ID)+"/dismiss", nil)
	requireStatus(t, w, http.StatusConflict)

	theirs := seedTriageAsset(t, h, models.Asset{LibraryID: other.ID, Name: "theirs", NeedsReview: true})
	failed := seedImport(t, h, theirs.ID, "failed")
	w = performRequest(r, http.MethodPost, "/api/imports/"+itoa(failed.ID)+"/dismiss", nil)
	requireStatus(t, w, http.StatusNotFound)
}
