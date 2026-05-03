package audit

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

func newPurgeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.LibrarySettings{},
		&models.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestPurgeOnce_DeletesOnlyOldLibraryRows is the happy-path: rows older than
// retention go, fresh rows stay, libraries without retention are untouched,
// and server-level rows (library_id IS NULL) are never affected.
func TestPurgeOnce_DeletesOnlyOldLibraryRows(t *testing.T) {
	db := newPurgeDB(t)

	days7 := 7
	if err := db.Create(&models.LibrarySettings{LibraryID: 1, AuditRetentionDays: &days7}).Error; err != nil {
		t.Fatalf("settings 1: %v", err)
	}
	if err := db.Create(&models.LibrarySettings{LibraryID: 2}).Error; err != nil {
		t.Fatalf("settings 2: %v", err)
	}

	now := time.Now().UTC()
	lib1 := uint(1)
	lib2 := uint(2)
	rows := []models.AuditLog{
		{LibraryID: &lib1, ActionType: "x", TargetType: "y", CreatedAt: now.Add(-30 * 24 * time.Hour)},  // delete
		{LibraryID: &lib1, ActionType: "x", TargetType: "y", CreatedAt: now.Add(-1 * time.Hour)},        // keep
		{LibraryID: &lib2, ActionType: "x", TargetType: "y", CreatedAt: now.Add(-365 * 24 * time.Hour)}, // keep (no retention)
		{LibraryID: nil, ActionType: "x", TargetType: "y", CreatedAt: now.Add(-365 * 24 * time.Hour)},   // keep (server-level)
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create row %d: %v", i, err)
		}
	}

	deleted, err := PurgeOnce(context.Background(), db, now)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	var total int64
	if err := db.Model(&models.AuditLog{}).Count(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 rows remaining, got %d", total)
	}

	var oldExists int64
	if err := db.Model(&models.AuditLog{}).Where("id = ?", rows[0].ID).Count(&oldExists).Error; err != nil {
		t.Fatalf("count old: %v", err)
	}
	if oldExists != 0 {
		t.Fatalf("expected old library-1 row to be deleted")
	}
}

// TestPurgeOnce_BoundaryAtCutoff exercises the strict-less-than boundary.
// A row exactly at the cutoff stays (created_at < cutoff is false); one
// nanosecond older is deleted.
func TestPurgeOnce_BoundaryAtCutoff(t *testing.T) {
	db := newPurgeDB(t)

	days := 1
	if err := db.Create(&models.LibrarySettings{LibraryID: 1, AuditRetentionDays: &days}).Error; err != nil {
		t.Fatalf("settings: %v", err)
	}

	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour)
	lib := uint(1)
	atCutoff := models.AuditLog{LibraryID: &lib, ActionType: "x", TargetType: "y", CreatedAt: cutoff}
	justBefore := models.AuditLog{LibraryID: &lib, ActionType: "x", TargetType: "y", CreatedAt: cutoff.Add(-time.Nanosecond)}
	for _, r := range []*models.AuditLog{&atCutoff, &justBefore} {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	if _, err := PurgeOnce(context.Background(), db, now); err != nil {
		t.Fatalf("purge: %v", err)
	}

	var stillThere int64
	if err := db.Model(&models.AuditLog{}).Where("id = ?", atCutoff.ID).Count(&stillThere).Error; err != nil {
		t.Fatalf("count atCutoff: %v", err)
	}
	if stillThere != 1 {
		t.Fatalf("expected row exactly at cutoff to survive")
	}
	if err := db.Model(&models.AuditLog{}).Where("id = ?", justBefore.ID).Count(&stillThere).Error; err != nil {
		t.Fatalf("count justBefore: %v", err)
	}
	if stillThere != 0 {
		t.Fatalf("expected row before cutoff to be deleted")
	}
}

// TestPurgeOnce_ZeroDaysIgnored confirms a non-positive retention value is
// treated as "no retention" — guards against an operator setting 0 and
// accidentally truncating the entire log.
func TestPurgeOnce_ZeroDaysIgnored(t *testing.T) {
	db := newPurgeDB(t)

	zero := 0
	if err := db.Create(&models.LibrarySettings{LibraryID: 1, AuditRetentionDays: &zero}).Error; err != nil {
		t.Fatalf("settings: %v", err)
	}
	lib := uint(1)
	if err := db.Create(&models.AuditLog{
		LibraryID: &lib, ActionType: "x", TargetType: "y",
		CreatedAt: time.Now().UTC().Add(-365 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	deleted, err := PurgeOnce(context.Background(), db, time.Now().UTC())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted, got %d", deleted)
	}
}

// TestPurger_RunStopsOnContextCancel checks Run returns when its context is
// cancelled. Uses a tiny interval and short window so the test is quick.
func TestPurger_RunStopsOnContextCancel(t *testing.T) {
	db := newPurgeDB(t)
	p := &Purger{DB: db, Interval: 10 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Run did not return after ctx cancel")
	}
}

// TestPurger_RunDisabledOnNonPositiveInterval ensures a misconfigured
// interval makes Run return immediately rather than spinning.
func TestPurger_RunDisabledOnNonPositiveInterval(t *testing.T) {
	p := &Purger{Interval: 0}
	done := make(chan struct{})
	go func() {
		p.Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Run with zero interval did not return immediately")
	}
}
