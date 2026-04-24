BEGIN;

CREATE TABLE IF NOT EXISTS collections (
    id                 bigserial PRIMARY KEY,
    library_id         bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name               text   NOT NULL,
    description        text,
    created_by_user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS collections_library_idx ON collections (library_id);
CREATE UNIQUE INDEX IF NOT EXISTS collections_library_name_idx
    ON collections (library_id, lower(name));

CREATE TABLE IF NOT EXISTS collection_assets (
    collection_id    bigint NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    asset_id         bigint NOT NULL REFERENCES assets(id)      ON DELETE CASCADE,
    added_at         timestamptz NOT NULL DEFAULT now(),
    added_by_user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (collection_id, asset_id)
);

CREATE INDEX IF NOT EXISTS collection_assets_asset_idx ON collection_assets (asset_id);

COMMIT;
