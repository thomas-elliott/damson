BEGIN;

-- Store version becomes the version identity. version_label is now an
-- optional, editable label (blank = unversioned) rather than an
-- auto-incrementing v1/v2, so its uniqueness constraint is dropped and a
-- default of '' lets inserts omit it.
ALTER TABLE asset_versions ALTER COLUMN version_label SET DEFAULT '';

ALTER TABLE asset_versions
    DROP CONSTRAINT IF EXISTS asset_versions_asset_id_version_label_key;

-- Uniqueness moves onto the store version, scoped to non-blank values so any
-- number of unversioned rows (undetected uploads, subsets) can coexist while a
-- real store version can only appear once per asset.
CREATE UNIQUE INDEX IF NOT EXISTS idx_version_asset_store
    ON asset_versions (asset_id, store_version)
    WHERE store_version <> '';

COMMIT;
