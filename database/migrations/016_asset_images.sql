BEGIN;

CREATE TABLE IF NOT EXISTS asset_images (
    id           bigserial PRIMARY KEY,
    asset_id     bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    source_url   text NOT NULL,
    object_key   text NOT NULL,
    content_type text NOT NULL DEFAULT '',
    sort_order   int  NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_asset_image_source UNIQUE (asset_id, source_url)
);

CREATE INDEX IF NOT EXISTS idx_asset_images_asset ON asset_images(asset_id);

COMMIT;
