BEGIN;

CREATE TABLE IF NOT EXISTS asset_files (
    id               bigserial PRIMARY KEY,
    asset_version_id bigint NOT NULL REFERENCES asset_versions(id) ON DELETE CASCADE,
    relative_path    text NOT NULL,
    size_bytes       bigint NOT NULL,
    content_hash     text NOT NULL,
    object_key       text NOT NULL,
    detected_type    text,
    unity_guid       text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (asset_version_id, relative_path)
);

CREATE INDEX IF NOT EXISTS asset_files_version_idx ON asset_files (asset_version_id);
CREATE INDEX IF NOT EXISTS asset_files_hash_idx    ON asset_files (content_hash);

COMMIT;
