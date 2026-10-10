BEGIN;

-- An import names the version it fills. Before this the pipeline and the
-- boot-time resume took the asset's newest version, so an asset with a second
-- version (a subset, say) resumed the wrong one, or resumed twice.
ALTER TABLE asset_imports
    ADD COLUMN IF NOT EXISTS asset_version_id bigint REFERENCES asset_versions(id) ON DELETE SET NULL;

-- Backfill with the version the pipeline picked until now: the newest by
-- import date. Only rows still unset, so a re-run changes nothing.
UPDATE asset_imports i
SET asset_version_id = (
    SELECT v.id FROM asset_versions v
    WHERE v.asset_id = i.asset_id
    ORDER BY v.import_date DESC, v.id ASC
    LIMIT 1
)
WHERE i.asset_version_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_asset_imports_version ON asset_imports (asset_version_id);

COMMIT;
