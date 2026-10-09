BEGIN;

-- Migrations 001, 002 and 024 add four constraints only when pg_constraint has
-- none of that name, in any schema. A database holding two schemas (a test
-- database whose public schema the e2e server migrated, beside per-test
-- schemas) therefore gave every schema after the first none of them. This
-- adds each one the current schema lacks; where all four exist, as in
-- production, it changes nothing.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conname = 'assets_current_version_fk' AND connamespace = current_schema()::regnamespace) THEN
        ALTER TABLE assets ADD CONSTRAINT assets_current_version_fk
            FOREIGN KEY (current_version_id) REFERENCES asset_versions(id) ON DELETE SET NULL;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conname = 'assets_licence_fk' AND connamespace = current_schema()::regnamespace) THEN
        ALTER TABLE assets ADD CONSTRAINT assets_licence_fk
            FOREIGN KEY (licence_id) REFERENCES licences(id) ON DELETE SET NULL;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conname = 'assets_location_path_reference_only' AND connamespace = current_schema()::regnamespace) THEN
        ALTER TABLE assets ADD CONSTRAINT assets_location_path_reference_only
            CHECK (location_path IS NULL OR is_reference);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conname = 'assets_reference_has_location' AND connamespace = current_schema()::regnamespace) THEN
        ALTER TABLE assets ADD CONSTRAINT assets_reference_has_location
            CHECK (NOT is_reference OR location_path IS NOT NULL OR coalesce(source_url, '') <> '');
    END IF;
END
$$;

COMMIT;
