BEGIN;

CREATE TABLE IF NOT EXISTS asset_imports (
    id          bigserial PRIMARY KEY,
    asset_id    bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    stage       text   NOT NULL,                  -- 'uploaded' | 'extracting' | 'detecting' | 'thumbnailing' | 'done' | 'failed'
    error       text,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS asset_imports_asset_idx ON asset_imports (asset_id);
CREATE INDEX IF NOT EXISTS asset_imports_stage_idx ON asset_imports (stage);

COMMIT;
