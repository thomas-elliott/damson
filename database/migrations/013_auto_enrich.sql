BEGIN;

ALTER TABLE library_settings
    ADD COLUMN IF NOT EXISTS auto_enrich_enabled boolean NOT NULL DEFAULT true;

COMMIT;
