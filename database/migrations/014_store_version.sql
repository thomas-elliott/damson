BEGIN;

ALTER TABLE asset_versions
    ADD COLUMN IF NOT EXISTS store_version text NOT NULL DEFAULT '';

COMMIT;
