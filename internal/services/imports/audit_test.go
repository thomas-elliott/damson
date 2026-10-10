package imports

import (
	"context"
	"encoding/json"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/services/metadata"
)

// systemUpdates returns the asset.update audit rows for assetID that name no
// user, which the feed shows as "system".
func systemUpdates(t *testing.T, db *gorm.DB, assetID uint) []models.AuditLog {
	t.Helper()
	var rows []models.AuditLog
	if err := db.Where("action_type = ? AND target_id = ? AND user_id IS NULL", "asset.update", assetID).Find(&rows).Error; err != nil {
		t.Fatalf("load audit rows: %v", err)
	}
	return rows
}

func TestEnrichAsset_AuditsItsChangesAsSystem(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedEnrichAsset(t, db, "Fantasy Pack", true)
	fake := &fakeEnricher{
		cands: []metadata.Candidate{{Title: "Fantasy Pack", ListingURL: "https://x/1", Store: "unity"}},
		meta:  &metadata.Metadata{Store: "unity", Title: "Fantasy Pack", Description: "From the store.", ListingURL: "https://x/1"},
	}

	enrichAsset(context.Background(), Deps{DB: db, Metadata: fake}, jobContext{asset: *asset, libraryID: 1})

	rows := systemUpdates(t, db, asset.ID)
	if len(rows) != 1 {
		t.Fatalf("got %d system asset.update rows, want 1", len(rows))
	}
	var diff struct {
		Before map[string]any `json:"before"`
		After  map[string]any `json:"after"`
	}
	if err := json.Unmarshal(rows[0].DiffJSON, &diff); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if diff.After["description"] != "From the store." || diff.Before["description"] != "" {
		t.Fatalf("diff = %s, want the description change", rows[0].DiffJSON)
	}
	if rows[0].LibraryID == nil || *rows[0].LibraryID != 1 {
		t.Fatalf("audit row library = %v, want 1", rows[0].LibraryID)
	}
}

func TestApplySourceHints_AuditsThePublisherAsSystem(t *testing.T) {
	db := enrichTestDB(t)
	asset := seedSourceAsset(t, db, 1, "RPG Builder")
	version := models.AssetVersion{
		AssetID:            asset.ID,
		SourceKind:         ptr(SourceKindUnityCache),
		SourceRelativePath: ptr("Blink/Editor ExtensionsGame ToolkitsRPG Toolkits/RPG Builder.unitypackage"),
	}
	if err := db.Create(&version).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}

	ApplySourceHints(context.Background(), Deps{DB: db}, jobContext{asset: *asset, version: version, libraryID: 1})

	rows := systemUpdates(t, db, asset.ID)
	if len(rows) != 1 {
		t.Fatalf("got %d system asset.update rows, want 1", len(rows))
	}
	var diff struct {
		After map[string]any `json:"after"`
	}
	if err := json.Unmarshal(rows[0].DiffJSON, &diff); err != nil || diff.After["publisher"] != "Blink" {
		t.Fatalf("diff = %s (err %v), want publisher Blink", rows[0].DiffJSON, err)
	}
}
