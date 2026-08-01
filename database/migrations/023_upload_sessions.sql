BEGIN;

-- One resumable upload in flight. The client declares the SHA-256 up front, so
-- the target key is known before the first byte and the parts are written
-- straight into `originals/<hh>/<hash>` — there is no staging key and no
-- copy-on-complete.
--
-- hash_state is the marshalled crypto/sha256 digest, carried across requests
-- because chunks may land on different pods. It is what lets the server verify
-- the declared hash without a second full read of the object, at the cost of
-- making chunks strictly sequential.
--
-- No asset/version/import rows exist while a session is open: an abandoned
-- upload should cost an aborted multipart and nothing else, and a half-created
-- asset would otherwise be visible in the library for the life of the session.
CREATE TABLE IF NOT EXISTS upload_sessions (
    id                   bigserial PRIMARY KEY,
    library_id           bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id              bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status               text NOT NULL DEFAULT 'open',

    filename             text NOT NULL,
    content_type         text NOT NULL DEFAULT '',
    size_bytes           bigint NOT NULL,
    sha256               text NOT NULL,
    chunk_size           bigint NOT NULL,

    object_prefix        text NOT NULL,
    backend_upload_id    text NOT NULL DEFAULT '',
    parts_json           jsonb NOT NULL DEFAULT '[]'::jsonb,
    hash_state           bytea,
    received_bytes       bigint NOT NULL DEFAULT 0,
    next_chunk           integer NOT NULL DEFAULT 0,

    target_asset_id      bigint REFERENCES assets(id) ON DELETE CASCADE,
    store_version        text NOT NULL DEFAULT '',
    source_kind          text NOT NULL DEFAULT '',
    source_relative_path text NOT NULL DEFAULT '',

    asset_id             bigint,
    import_id            bigint,

    expires_at           timestamptz NOT NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT upload_sessions_status_check
        CHECK (status IN ('open', 'completed', 'aborted', 'expired'))
);

CREATE INDEX IF NOT EXISTS upload_sessions_library_idx
    ON upload_sessions (library_id, status, created_at DESC);

-- The janitor's only query: open sessions past their expiry.
CREATE INDEX IF NOT EXISTS upload_sessions_expiry_idx
    ON upload_sessions (expires_at)
    WHERE status = 'open';

COMMIT;
