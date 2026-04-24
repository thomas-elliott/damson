package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

const mitLicenceText = `MIT License

Copyright (c) 2024 Example

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

func TestPutLicence_DetectsMITAndPersists(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "asset-one")

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPut, "/api/assets/1/licence",
		map[string]any{
			"original_text": mitLicenceText,
			"use_detected":  true,
		})
	requireStatus(t, w, http.StatusOK)

	body := jsonResponse[licenceView](t, w)
	if body.DetectedType != "MIT" {
		t.Fatalf("expected MIT detection, got %q", body.DetectedType)
	}
	if body.Attributes.CommercialUse == nil || !*body.Attributes.CommercialUse {
		t.Fatalf("MIT should allow commercial use: %+v", body.Attributes)
	}

	var asset models.Asset
	_ = db.First(&asset, 1)
	if asset.LicenceID == nil {
		t.Fatalf("licence_id not linked on asset")
	}
}

func TestPostDetectLicence_RunsWithoutPersisting(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	seedLibraryForUser(t, db, u, "Personal")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/licences/detect",
		map[string]any{"text": mitLicenceText})
	requireStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), "MIT") {
		t.Fatalf("expected MIT in response body: %s", w.Body.String())
	}
}

func TestPutLicence_RejectsBlankText(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	seedAsset(t, db, lib.ID, "asset-one")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPut, "/api/assets/1/licence",
		map[string]any{"original_text": "   "})
	requireStatus(t, w, http.StatusBadRequest)
}

func TestPostLicenceReview_SetsReviewedFields(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "t@example.com")
	lib := seedLibraryForUser(t, db, u, "Personal")
	asset := seedAsset(t, db, lib.ID, "asset-one")
	l := models.Licence{AssetID: asset.ID, DetectedType: "MIT", OriginalText: "x", AttributesJSON: []byte("{}")}
	_ = db.Create(&l)
	asset.LicenceID = &l.ID
	_ = db.Save(asset)

	r := setupRouter(h, u)
	w := performRequest(r, http.MethodPost, "/api/assets/1/licence/review", nil)
	requireStatus(t, w, http.StatusNoContent)

	var refreshed models.Licence
	_ = db.First(&refreshed, l.ID)
	if refreshed.ReviewedAt == nil || refreshed.ReviewedByUserID == nil {
		t.Fatalf("reviewed fields not set: %+v", refreshed)
	}
}
