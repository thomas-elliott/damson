BEGIN;

ALTER TABLE library_settings
    ADD COLUMN IF NOT EXISTS preferred_store text NOT NULL DEFAULT 'unity';

COMMIT;
