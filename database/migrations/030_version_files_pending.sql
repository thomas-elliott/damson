BEGIN;

-- A version whose import has not yet committed its files. The current-version
-- choice passes over it. The import that creates the version sets it, and the
-- transaction that writes the files (finalize, re-extract) clears it.
ALTER TABLE asset_versions
    ADD COLUMN IF NOT EXISTS files_pending boolean NOT NULL DEFAULT false;

-- Backfill from each version's latest import: still in flight, or failed with
-- no files written. An import that failed after finalize has its files.
UPDATE asset_versions v
SET files_pending = true
FROM asset_imports i
WHERE i.asset_version_id = v.id
  AND NOT EXISTS (SELECT 1 FROM asset_imports j WHERE j.asset_version_id = i.asset_version_id AND j.id > i.id)
  AND (i.stage NOT IN ('enriching', 'done', 'failed')
       OR (i.stage = 'failed' AND NOT EXISTS (SELECT 1 FROM asset_files f WHERE f.asset_version_id = v.id)));

COMMIT;
