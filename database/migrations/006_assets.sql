BEGIN;

CREATE TABLE IF NOT EXISTS assets (
    id                     bigserial PRIMARY KEY,
    library_id             bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name                   text NOT NULL,
    description            text,
    source_url             text,
    source_metadata_json   jsonb,
    licence_id             bigint,
    current_version_id     bigint,
    thumbnail_object_key   text,
    thumbnail_content_type text,
    needs_review           boolean NOT NULL DEFAULT true,
    created_by_user_id     bigint REFERENCES users(id),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS assets_library_idx          ON assets (library_id);
CREATE INDEX IF NOT EXISTS assets_library_created_idx  ON assets (library_id, created_at DESC);
CREATE INDEX IF NOT EXISTS assets_library_review_idx   ON assets (library_id) WHERE needs_review;

COMMIT;
