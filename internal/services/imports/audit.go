package imports

import (
	"context"
	"log/slog"
	"reflect"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/services/audit"
)

// unauditedAssetColumns change on every write or are too large to diff.
var unauditedAssetColumns = map[string]bool{
	"updated_at": true, "source_metadata_json": true, "search_vector": true,
}

// assetSnapshot reads an asset row as column → value, so a system write can
// be audited as the columns it changed.
func assetSnapshot(ctx context.Context, db *gorm.DB, assetID uint) (map[string]any, error) {
	row := map[string]any{}
	err := db.WithContext(ctx).Table("assets").Where("id = ?", assetID).Take(&row).Error
	return row, err
}

// auditSystemUpdate writes one asset.update row attributed to no user — the
// feed shows it as system — for the columns that differ between before and
// the asset now. It writes nothing when nothing changed. Like metadata apply,
// it runs after the writes it describes and only logs a failure, so auditing
// never fails an import.
func auditSystemUpdate(ctx context.Context, db *gorm.DB, libraryID, assetID uint, before map[string]any) {
	if before == nil {
		return
	}
	after, err := assetSnapshot(ctx, db, assetID)
	if err != nil {
		slog.Warn("imports: audit snapshot", "err", err, "asset_id", assetID)
		return
	}
	was, now := map[string]any{}, map[string]any{}
	for col, v := range after {
		if !unauditedAssetColumns[col] && !reflect.DeepEqual(before[col], v) {
			was[col], now[col] = before[col], v
		}
	}
	if len(now) == 0 {
		return
	}
	if err := audit.Write(db.WithContext(ctx), audit.Entry{
		LibraryID:  &libraryID,
		ActionType: "asset.update",
		TargetType: "asset",
		TargetID:   &assetID,
		Diff:       audit.Updated(was, now),
	}); err != nil {
		slog.Warn("imports: audit system update", "err", err, "asset_id", assetID)
	}
}
