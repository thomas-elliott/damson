BEGIN;

-- A reference recorded with its store lookup queued is named from its path or
-- link until the lookup runs. The lookup replaces that name with the store's
-- title, as an immediate lookup would have, unless someone renamed it since.
ALTER TABLE store_lookups ADD COLUMN IF NOT EXISTS provisional_name text NOT NULL DEFAULT '';

COMMIT;
