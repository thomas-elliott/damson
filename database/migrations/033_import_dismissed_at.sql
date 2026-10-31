BEGIN;

-- A failed import someone has dismissed from the triage queue. The row stays
-- for the audit trail and the import history; the queue skips it.
ALTER TABLE asset_imports ADD COLUMN IF NOT EXISTS dismissed_at timestamptz;

COMMIT;
