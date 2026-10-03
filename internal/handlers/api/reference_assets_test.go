package api

import (
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
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
