BEGIN;

-- Full-text search. Postgres-only: SQLite tests use the `search_q` LIKE
-- fallback inside handlers, so the tsvector path here is never exercised by
-- unit tests. Real-Postgres integration is the test tier that covers this
-- (see docs/technical-design.md §22 decision 5).

ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS search_vector tsvector;

CREATE INDEX IF NOT EXISTS assets_search_vector_idx
    ON assets USING gin (search_vector);

-- Recompute search_vector from (name, description) + aggregated tag paths.
-- Tag changes also trigger via asset_tags; see below.
CREATE OR REPLACE FUNCTION assets_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('simple', coalesce(NEW.name, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.description, '')), 'B') ||
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

DROP TRIGGER IF EXISTS assets_search_vector_trg ON assets;
CREATE TRIGGER assets_search_vector_trg
    BEFORE INSERT OR UPDATE OF name, description ON assets
    FOR EACH ROW EXECUTE FUNCTION assets_search_vector_update();

-- asset_tags changes need to force an update on the asset row so the
-- trigger recomputes search_vector. We do this by bumping assets.updated_at.
CREATE OR REPLACE FUNCTION asset_tags_touch_asset() RETURNS trigger AS $$
DECLARE
    target_asset bigint;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_asset := OLD.asset_id;
    ELSE
        target_asset := NEW.asset_id;
    END IF;
    UPDATE assets
        SET updated_at = now()
        WHERE id = target_asset;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS asset_tags_touch_asset_trg ON asset_tags;
CREATE TRIGGER asset_tags_touch_asset_trg
    AFTER INSERT OR DELETE ON asset_tags
    FOR EACH ROW EXECUTE FUNCTION asset_tags_touch_asset();

-- Backfill search_vector for existing rows. `SET name = name` triggers the
-- BEFORE UPDATE OF name trigger, which recomputes the vector. No-op on a
-- fresh install.
UPDATE assets SET name = name WHERE search_vector IS NULL;

COMMIT;
