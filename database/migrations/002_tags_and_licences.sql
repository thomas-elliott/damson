BEGIN;

-- Tags are library-scoped with a materialised path for hierarchy queries
-- (`environment/trees/deciduous`). Descendant queries use `path LIKE '<p>/%'`.
CREATE TABLE IF NOT EXISTS tags (
    id          bigserial PRIMARY KEY,
    library_id  bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path        text   NOT NULL,
    parent_id   bigint REFERENCES tags(id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (library_id, path)
);

CREATE INDEX IF NOT EXISTS tags_library_parent_idx ON tags (library_id, parent_id);

CREATE TABLE IF NOT EXISTS asset_tags (
    asset_id   bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id     bigint NOT NULL REFERENCES tags(id)   ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, tag_id)
);

CREATE INDEX IF NOT EXISTS asset_tags_tag_idx ON asset_tags (tag_id);

CREATE TABLE IF NOT EXISTS licences (
    id                   bigserial PRIMARY KEY,
    asset_id             bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    detected_type        text,
    detection_confidence real,
    canonical_diff       text,
    original_text        text NOT NULL,
    attributes_json      jsonb NOT NULL DEFAULT '{}'::jsonb,
    reviewed_by_user_id  bigint REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at          timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS licences_asset_idx   ON licences (asset_id);
CREATE INDEX IF NOT EXISTS licences_type_idx    ON licences (detected_type);

-- assets.licence_id got its column in 001 but the FK was deferred until
-- licences existed.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'assets_licence_fk'
    ) THEN
        ALTER TABLE assets
            ADD CONSTRAINT assets_licence_fk
            FOREIGN KEY (licence_id) REFERENCES licences(id) ON DELETE SET NULL;
    END IF;
END
$$;

COMMIT;
