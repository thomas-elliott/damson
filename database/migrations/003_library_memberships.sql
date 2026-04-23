BEGIN;

CREATE TABLE IF NOT EXISTS library_memberships (
    id          bigserial PRIMARY KEY,
    library_id  bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id     bigint NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
    role        text   NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (library_id, user_id)
);

CREATE INDEX IF NOT EXISTS library_memberships_user_idx
    ON library_memberships (user_id);

COMMIT;
