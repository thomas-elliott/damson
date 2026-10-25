package ingest

import (
	"fmt"
	"testing"

	"github.com/thomas-elliott/damson/internal/extract"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// Postgres' extended protocol caps one statement at 65535 bind parameters.
// asset_files binds a dozen-odd columns per row, so a single multi-row INSERT
// fails somewhere north of five thousand files — well inside the size of a real
// asset pack. The write must be batched.
func TestWriteAssetFilesHandlesPackagesBeyondTheBindParameterLimit(t *testing.T) {
	db := testdb.NewPostgres(t)
	if err := db.Create(&models.Library{ID: 1, Name: "lib"}).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}
	asset := models.Asset{LibraryID: 1, Name: "big pack"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	version := models.AssetVersion{AssetID: asset.ID}
	if err := db.Create(&version).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}

	const fileCount = 6000
	extracted := &extract.UnityResult{Files: make([]extract.UnityFile, 0, fileCount)}
	for i := 0; i < fileCount; i++ {
		extracted.Files = append(extracted.Files, extract.UnityFile{
			RelativePath: fmt.Sprintf("Assets/Pack/file_%05d.png", i),
			SizeBytes:    int64(i + 1),
			ContentHash:  fmt.Sprintf("%064x", i),
			ObjectKey:    fmt.Sprintf("%064x", i),
			GUID:         fmt.Sprintf("%032x", i),
		})
	}

	n, total, err := WriteAssetFiles(db, version.ID, "", 0, "", "", extracted)
	if err != nil {
		t.Fatalf("WriteAssetFiles(%d files): %v", fileCount, err)
	}
	if n != fileCount {
		t.Fatalf("wrote %d rows, want %d", n, fileCount)
	}
	if want := int64(fileCount) * int64(fileCount+1) / 2; total != want {
		t.Fatalf("total size = %d, want %d", total, want)
	}

	var persisted int64
	if err := db.Model(&models.AssetFile{}).
		Where("asset_version_id = ?", version.ID).Count(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted != fileCount {
		t.Fatalf("persisted %d rows, want %d", persisted, fileCount)
	}
}
