package imports

import (
	"context"
	"gorm.io/gorm"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/models"
)

type recordingEnqueuer struct{ jobs []Job }

func (r *recordingEnqueuer) Enqueue(_ *gorm.DB, j Job) error {
	r.jobs = append(r.jobs, j)
	return nil
}

// An import resumes against its own version. A second, keyless version of
// the same asset (a subset) must neither enqueue it twice nor mark it failed.
func TestResumeInFlight_ResumesEachImportOnceForItsOwnVersion(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedEnrichAsset(t, db, "Pack", false)

	imported := models.AssetVersion{AssetID: asset.ID, ImportDate: time.Now().Add(-time.Hour), PackageKind: "unitypackage", OriginalPackageObjectKey: "abc"}
	if err := db.Create(&imported).Error; err != nil {
		t.Fatal(err)
	}
	subset := models.AssetVersion{AssetID: asset.ID, ImportDate: time.Now(), PackageKind: "unitypackage"}
	if err := db.Create(&subset).Error; err != nil {
		t.Fatal(err)
	}
	imp := models.AssetImport{AssetID: asset.ID, AssetVersionID: &imported.ID, Stage: StageExtracting}
	if err := db.Create(&imp).Error; err != nil {
		t.Fatal(err)
	}

	enq := &recordingEnqueuer{}
	n, err := ResumeInFlight(context.Background(), db, enq)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if n != 1 || len(enq.jobs) != 1 || enq.jobs[0].ImportID != imp.ID {
		t.Fatalf("resumed %d (%v), want import %d once", n, enq.jobs, imp.ID)
	}
	var after models.AssetImport
	if err := db.First(&after, imp.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Stage == StageFailed {
		t.Fatalf("the resumed import was also marked failed: %q", after.Error)
	}

	// The pipeline then works on that version, not the newer subset.
	jc, err := loadJobContext(db, imp.ID)
	if err != nil {
		t.Fatalf("load job context: %v", err)
	}
	if jc.version.ID != imported.ID {
		t.Fatalf("pipeline loaded version %d, want the import's own %d", jc.version.ID, imported.ID)
	}
}
