BEGIN;

-- Richer storefront metadata as first-class asset columns. All nullable: a
-- store may expose any subset, and every column is hand-editable, so "unset"
-- has to be distinguishable from "zero".
--
-- Price and rating are deliberately point-in-time snapshots. There is no
-- background refresh, so they are only as fresh as the last user-triggered
-- fetch, whose timestamp lives in source_metadata_json's fetched_at.
ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS publisher text,
    ADD COLUMN IF NOT EXISTS publisher_url text,
    ADD COLUMN IF NOT EXISTS store text,
    ADD COLUMN IF NOT EXISTS store_product_id text,
    ADD COLUMN IF NOT EXISTS released_at timestamptz,
    ADD COLUMN IF NOT EXISTS store_updated_at timestamptz,
    ADD COLUMN IF NOT EXISTS latest_store_version text,
    ADD COLUMN IF NOT EXISTS engine text,
    ADD COLUMN IF NOT EXISTS min_engine_version text,
    ADD COLUMN IF NOT EXISTS price text,
    ADD COLUMN IF NOT EXISTS price_currency text,
    ADD COLUMN IF NOT EXISTS rating numeric(3, 2),
    ADD COLUMN IF NOT EXISTS rating_count integer;

-- Filter/sort support for the fields the library view exposes. Publisher and
-- store are equality filters; released_at sorts.
CREATE INDEX IF NOT EXISTS idx_assets_library_publisher
    ON assets (library_id, publisher)
    WHERE publisher IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_assets_library_store
    ON assets (library_id, store)
    WHERE store IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_assets_library_released
    ON assets (library_id, released_at DESC);

-- Publisher joins the search vector at weight C, alongside tag paths, so a
-- developer name is findable from the search box.
CREATE OR REPLACE FUNCTION assets_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('simple', coalesce(NEW.name, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.description, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(NEW.publisher, '')), 'C') ||
        setweight(
            to_tsvector('simple', coalesce((
                SELECT string_agg(t.path, ' ')
                FROM asset_tags at
                JOIN tags t ON t.id = at.tag_id
                WHERE at.asset_id = NEW.id
            ), '')),
            'C'
        );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- The trigger's UPDATE OF list must grow with the vector's inputs, or setting
-- publisher alone never recomputes the vector. This is the bug 007 had to fix
-- for tag paths; adding the column to the function body is only half the job.
DROP TRIGGER IF EXISTS assets_search_vector_trg ON assets;
CREATE TRIGGER assets_search_vector_trg
    BEFORE INSERT OR UPDATE OF name, description, publisher ON assets
    FOR EACH ROW EXECUTE FUNCTION assets_search_vector_update();

-- Recompute every vector so existing rows pick up the new weight-C segment.
-- `SET name = name` fires the BEFORE UPDATE OF name trigger; no-op on a fresh
-- install.
UPDATE assets SET name = name;

COMMIT;
