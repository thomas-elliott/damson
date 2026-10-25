package api

import (
	"fmt"
	"maps"
	"net/http"
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

// The grid, a project's pin list, the licence-status filter and the detail
// panel must all report the same status. Detail's rule is the authority: it
// reads the licence's attributes, not only its type.
func TestLicenceStatus_AgreesAcrossGridProjectFilterAndDetail(t *testing.T) {
	cases := []struct {
		name, spdx, attrs, want string
	}{
		{"custom licence barring commercial use", "LicenseRef-custom", `{"commercial_use": false}`, "err"},
		{"NC type whose commercial use is unknown", "CC-BY-NC-4.0", `{"commercial_use": null}`, "warn"},
		{"permissive type barring commercial use", "MIT", `{"commercial_use": false}`, "err"},
		{"permissive type", "MIT", `{}`, "ok"},
		{"undetected", "unknown", `{}`, "unk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, db := newTestHandler(t)
			u := seedUser(t, db, "librarian@example.com")
			lib := seedLibraryAndSwitch(t, h, u)
			r := setupRouter(h, u)

			asset := models.Asset{LibraryID: lib.ID, Name: "Pack"}
			if err := db.Create(&asset).Error; err != nil {
				t.Fatal(err)
			}
			l := models.Licence{AssetID: asset.ID, DetectedType: tc.spdx, OriginalText: "x", AttributesJSON: []byte(tc.attrs)}
			if err := db.Create(&l).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&asset).Update("licence_id", l.ID).Error; err != nil {
				t.Fatal(err)
			}
			proj := models.Project{LibraryID: lib.ID, Name: "Game"}
			if err := db.Create(&proj).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&models.ProjectAssetPin{ProjectID: proj.ID, AssetID: asset.ID}).Error; err != nil {
				t.Fatal(err)
			}

			w := performGET(r, fmt.Sprintf("/api/assets/%d", asset.ID))
			requireStatus(t, w, http.StatusOK)
			if lic := jsonResponse[assetDetailView](t, w).Licence; lic == nil || lic.Status != tc.want {
				t.Fatalf("detail licence %+v, want status %q", lic, tc.want)
			}

			w = performGET(r, "/api/assets")
			requireStatus(t, w, http.StatusOK)
			grid := jsonResponse[struct {
				Assets []assetGridRow `json:"assets"`
			}](t, w)
			if len(grid.Assets) != 1 || grid.Assets[0].LicenceStatus != tc.want {
				t.Fatalf("grid = %+v, want status %q", grid.Assets, tc.want)
			}

			w = performGET(r, fmt.Sprintf("/api/projects/%d", proj.ID))
			requireStatus(t, w, http.StatusOK)
			if pins := jsonResponse[projectDetailView](t, w).Pins; len(pins) != 1 || pins[0].LicenceStatus != tc.want {
				t.Fatalf("project pins = %+v, want status %q", pins, tc.want)
			}

			w = performGET(r, "/api/projects")
			requireStatus(t, w, http.StatusOK)
			list := jsonResponse[struct {
				Projects []projectView `json:"projects"`
			}](t, w)
			if len(list.Projects) != 1 {
				t.Fatalf("project list = %+v, want one project", list.Projects)
			}
			c := list.Projects[0].LicenceCounts
			got := map[string]int{"ok": c.OK, "warn": c.Warn, "err": c.Err, "unk": c.Unk}
			want := map[string]int{"ok": 0, "warn": 0, "err": 0, "unk": 0}
			want[tc.want] = 1
			if !maps.Equal(got, want) {
				t.Fatalf("project card licence counts = %v, want %v", got, want)
			}

			for _, status := range []string{"ok", "warn", "err", "unk"} {
				w = performGET(r, "/api/assets?licence_status="+status)
				requireStatus(t, w, http.StatusOK)
				n := len(jsonResponse[struct {
					Assets []assetGridRow `json:"assets"`
				}](t, w).Assets)
				if want := map[bool]int{true: 1, false: 0}[status == tc.want]; n != want {
					t.Errorf("filter licence_status=%s returned %d assets, want %d", status, n, want)
				}
			}
		})
	}
}
