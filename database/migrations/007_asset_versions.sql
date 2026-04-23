BEGIN;

CREATE TABLE IF NOT EXISTS asset_versions (
    id                          bigserial PRIMARY KEY,
    asset_id                    bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    version_label               text NOT NULL,
    notes                       text,
    import_date                 timestamptz NOT NULL DEFAULT now(),
    original_package_object_key text,
    package_kind                text,
    created_by_user_id          bigint REFERENCES users(id),
    UNIQUE (asset_id, version_label)
);

CREATE INDEX IF NOT EXISTS asset_versions_asset_imported_idx
    ON asset_versions (asset_id, import_date DESC);

-- assets.current_version_id is declared in 006 without an FK so the two
-- migrations stay independent. Add it now that asset_versions exists.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'assets_current_version_fk'
    ) THEN
        ALTER TABLE assets
            ADD CONSTRAINT assets_current_version_fk
            FOREIGN KEY (current_version_id) REFERENCES asset_versions(id) ON DELETE SET NULL;
    END IF;
END
$$;

COMMIT;
