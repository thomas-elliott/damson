BEGIN;

-- Whether the store has deprecated the asset's listing: it can no longer be
-- bought, and the store's search index no longer carries it. Set or cleared by
-- any lookup that reads the listing page; a lookup that can't tell leaves it.
ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS store_deprecated boolean NOT NULL DEFAULT false;

COMMIT;
