BEGIN;

-- A reference asset is recorded by where it lives — a path on the NFS store
-- and/or a store link (source_url) — and holds no versions or files. Damson
-- never reads the path; it is a label to see, copy and search.
ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS is_reference boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS location_path text;

-- Only reference assets carry a path, and a reference asset always keeps at
-- least one of its path and its store link.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'assets_location_path_reference_only') THEN
        ALTER TABLE assets ADD CONSTRAINT assets_location_path_reference_only
            CHECK (location_path IS NULL OR is_reference);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'assets_reference_has_location') THEN
        ALTER TABLE assets ADD CONSTRAINT assets_reference_has_location
            CHECK (NOT is_reference OR location_path IS NOT NULL OR coalesce(source_url, '') <> '');
    END IF;
END
$$;

-- A path is unique within a library: recording it again updates that asset.
CREATE UNIQUE INDEX IF NOT EXISTS idx_assets_library_location_path
    ON assets (library_id, location_path)
    WHERE location_path IS NOT NULL;

-- The path's folder and file names join the search vector at weight D, below
-- name and description. The simple parser reads `/a/b/c.ext` as one file
-- token, so every run of non-alphanumerics is turned into a space first.
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
        ) ||
        setweight(
            to_tsvector('simple', regexp_replace(coalesce(NEW.location_path, ''), '[^[:alnum:]]+', ' ', 'g')),
            'D'
        );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- The UPDATE OF list grows with the vector's inputs (see 019).
DROP TRIGGER IF EXISTS assets_search_vector_trg ON assets;
CREATE TRIGGER assets_search_vector_trg
    BEFORE INSERT OR UPDATE OF name, description, publisher, location_path ON assets
    FOR EACH ROW EXECUTE FUNCTION assets_search_vector_update();

-- A reference asset has no versions, so its project pins carry none.
ALTER TABLE project_asset_pins ALTER COLUMN pinned_version_id DROP NOT NULL;

COMMIT;
