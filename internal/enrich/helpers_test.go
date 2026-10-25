package enrich

import (
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/testdb"
)

func enrichTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.NewPostgres(t)
}

// seedEnrichAsset seeds library 1, its settings with auto-enrichment as
// given, and an asset in it.
func seedEnrichAsset(t *testing.T, db *gorm.DB, name string, enrich bool) *models.Asset {
	t.Helper()
	if err := db.Create(&models.Library{ID: 1, Name: "lib"}).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	if err := db.Create(&models.LibrarySettings{LibraryID: 1, AutoEnrichEnabled: enrich}).Error; err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	// GORM omits a zero-value bool on Create, letting the column default:true
	// apply — force the intended value so a false toggle really is off.
	if err := db.Model(&models.LibrarySettings{}).Where("library_id = ?", 1).
		Update("auto_enrich_enabled", enrich).Error; err != nil {
		t.Fatalf("force enrich toggle: %v", err)
	}
	a := models.Asset{LibraryID: 1, Name: name}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return &a
}
