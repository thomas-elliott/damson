BEGIN;

-- When a library has retain_original_packages disabled, the uploaded archive is
-- deleted from object storage after a successful extraction. The object key is
-- retained on the row for forensics; this flag records that the bytes are gone
-- so has_original / re-extract / download degrade gracefully.
ALTER TABLE asset_versions
    ADD COLUMN IF NOT EXISTS original_package_purged boolean NOT NULL DEFAULT false;

COMMIT;
