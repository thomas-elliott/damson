BEGIN;

-- Store lookups the server runs in the background, one at a time, so a caller
-- with hundreds of assets to look up asks once and the server paces the store.
-- A batch groups the lookups one request queued, so the caller can follow
-- them; refresh marks a batch whose lookups may replace fields a person hasn't
-- changed, where every other lookup fills blanks only.
CREATE TABLE IF NOT EXISTS store_lookup_batches (
    id                 bigserial PRIMARY KEY,
    library_id         bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    created_by_user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    refresh            boolean NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_store_lookup_batches_library ON store_lookup_batches (library_id);

-- One asset's lookup. status is queued until the job has run, then done or
-- failed. result says what a done lookup did: changed or unchanged the asset,
-- or no_store_data when only the generic scraper answered. applied and
-- conflicts list the fields written and the fields kept against the store.
CREATE TABLE IF NOT EXISTS store_lookups (
    id         bigserial PRIMARY KEY,
    batch_id   bigint NOT NULL REFERENCES store_lookup_batches(id) ON DELETE CASCADE,
    library_id bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    asset_id   bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    status     text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'done', 'failed')),
    result     text NOT NULL DEFAULT '' CHECK (result IN ('', 'changed', 'unchanged', 'no_store_data')),
    applied    jsonb NOT NULL DEFAULT '[]'::jsonb,
    conflicts  jsonb NOT NULL DEFAULT '[]'::jsonb,
    error      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_store_lookups_batch ON store_lookups (batch_id);
CREATE INDEX IF NOT EXISTS idx_store_lookups_queued ON store_lookups (id) WHERE status = 'queued';

COMMIT;
