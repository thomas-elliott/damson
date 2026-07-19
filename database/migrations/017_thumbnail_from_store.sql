BEGIN;

ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS thumbnail_from_store boolean NOT NULL DEFAULT false;

COMMIT;
