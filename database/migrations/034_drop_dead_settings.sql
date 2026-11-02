BEGIN;

-- Neither setting was ever read: content-addressed storage dedupes regardless,
-- and nothing refreshed the licence knowledge base.
ALTER TABLE library_settings DROP COLUMN IF EXISTS dedupe_enabled;
ALTER TABLE server_settings DROP COLUMN IF EXISTS licence_kb_auto_update;

COMMIT;
