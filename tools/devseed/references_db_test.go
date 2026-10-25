package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/handlers/api"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// oneUser signs every request in as the same user.
type oneUser struct{ user *models.User }

func (a oneUser) Authenticate(context.Context, string) (*models.User, error) { return a.user, nil }

// seededLibrary is a fresh library holding the generated references, and the
// real API in front of it.
func seededLibrary(t *testing.T) (*gorm.DB, uint, *gin.Engine) {
	t.Helper()
	db := testdb.NewPostgres(t)
	lib := models.Library{Name: "Dev"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	user := models.User{Email: "dev@example.com", DisplayName: "Dev", Role: "user", CurrentLibraryID: &lib.ID}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.LibraryMembership{LibraryID: lib.ID, UserID: user.ID, Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	authenticate := auth.Middleware(oneUser{&user}, nil)
	api.AddRoutes(r.Group("/api"), func(c *gin.Context) {
		c.Request.Header.Set("Authorization", "Bearer test")
		authenticate(c)
	}, api.New(api.Deps{DB: db, AuthMode: config.AuthModeOIDC}))
	return db, lib.ID, r
}

func getJSON(t *testing.T, r http.Handler, path string, out any) {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
}

type listing struct {
	Assets []struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"assets"`
	Total int `json:"total"`
}

func TestWriteReferences_SecondRunAddsNothingAndNothingIsAudited(t *testing.T) {
	db, libID, _ := seededLibrary(t)
	refs := references()

	created, skipped, err := writeReferences(db, libID)
	if err != nil {
		t.Fatal(err)
	}
	if created != len(refs) || skipped != 0 {
		t.Fatalf("first run created %d and skipped %d, want %d and 0", created, skipped, len(refs))
	}
	created, skipped, err = writeReferences(db, libID)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 || skipped != len(refs) {
		t.Fatalf("second run created %d and skipped %d, want 0 and %d", created, skipped, len(refs))
	}

	var assets, licences, links, audits int64
	db.Model(&models.Asset{}).Where("library_id = ?", libID).Count(&assets)
	db.Model(&models.Licence{}).Count(&licences)
	db.Model(&models.AssetTag{}).Count(&links)
	db.Table("audit_log").Count(&audits)
	if want := int64(len(refs)); assets != want || licences != want || links != want {
		t.Errorf("%d assets, %d licences, %d tag links; want %d of each", assets, licences, links, want)
	}
	if audits != 0 {
		t.Errorf("seeding wrote %d audit entries, want none", audits)
	}
}

func TestWriteReferences_SkipsALinkAlreadyRecorded(t *testing.T) {
	db, libID, _ := seededLibrary(t)
	var linkOnly reference
	for _, r := range references() {
		if r.Path == "" {
			linkOnly = r
			break
		}
	}
	held := models.Asset{LibraryID: libID, Name: "Recorded by hand", SourceURL: linkOnly.Link, IsReference: true}
	if err := db.Create(&held).Error; err != nil {
		t.Fatal(err)
	}
	_, skipped, err := writeReferences(db, libID)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 {
		t.Errorf("skipped %d, want the one whose link was recorded", skipped)
	}
}

// The library's own filters and search narrow the seeded set.
func TestWriteReferences_FiltersAndSearch(t *testing.T) {
	db, libID, r := seededLibrary(t)
	if _, _, err := writeReferences(db, libID); err != nil {
		t.Fatal(err)
	}
	var all listing
	getJSON(t, r, "/api/assets?limit=1", &all)
	if all.Total != len(references()) {
		t.Fatalf("library holds %d assets, want %d", all.Total, len(references()))
	}

	first := references()[0]
	for _, q := range []string{
		"needs_review=true",
		"publisher=" + url.QueryEscape(first.Publisher),
		"store=" + first.Store,
		"licence_status=ok", "licence_status=warn", "licence_status=err", "licence_status=unk",
		"released_after=" + releasedAfterProbe.Format("2006-01-02"),
		"not_downloaded=true",
	} {
		var got listing
		getJSON(t, r, "/api/assets?limit=1&"+q, &got)
		if got.Total == 0 || got.Total >= all.Total {
			t.Errorf("%s matches %d of %d", q, got.Total, all.Total)
		}
	}

	var found listing
	getJSON(t, r, "/api/assets?limit=200&q="+url.QueryEscape(nearDuplicateStem), &found)
	names := map[string]bool{}
	for _, a := range found.Assets {
		names[a.Name] = true
	}
	for _, v := range []string{"v1.2", "v1.3"} {
		if !names[nearDuplicateStem+" "+v] {
			t.Errorf("searching %q did not find its %s", nearDuplicateStem, v)
		}
	}
}

// The panel shows a seeded asset's path or link with its publisher, store,
// licence and tag.
func TestWriteReferences_PanelFields(t *testing.T) {
	db, libID, r := seededLibrary(t)
	if _, _, err := writeReferences(db, libID); err != nil {
		t.Fatal(err)
	}
	for kind, where := range map[string]string{
		"path-only":     "location_path IS NOT NULL AND source_url = ''",
		"link-only":     "location_path IS NULL",
		"path and link": "location_path IS NOT NULL AND source_url <> ''",
	} {
		var asset models.Asset
		if err := db.Where("library_id = ? AND "+where, libID).First(&asset).Error; err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		var detail struct {
			LocationPath string `json:"location_path"`
			SourceURL    string `json:"source_url"`
			Publisher    string `json:"publisher"`
			Store        string `json:"store"`
			Licence      *struct {
				DetectedType string `json:"detected_type"`
			} `json:"licence"`
			Tags []struct {
				Path string `json:"path"`
			} `json:"tags"`
		}
		getJSON(t, r, fmt.Sprintf("/api/assets/%d", asset.ID), &detail)
		wantPath, wantLink := kind != "link-only", kind != "path-only"
		if (detail.LocationPath != "") != wantPath || (detail.SourceURL != "") != wantLink {
			t.Errorf("%s shows path %q and link %q", kind, detail.LocationPath, detail.SourceURL)
		}
		if detail.Publisher == "" || detail.Store == "" || detail.Licence == nil || len(detail.Tags) != 1 {
			t.Errorf("%s panel is missing publisher, store, licence or its tag: %+v", kind, detail)
		}
	}
}
