BEGIN;

CREATE TABLE IF NOT EXISTS audit_log (
    id           bigserial PRIMARY KEY,
    library_id   bigint REFERENCES libraries(id) ON DELETE CASCADE,
    user_id      bigint REFERENCES users(id),
    action_type  text NOT NULL,
    target_type  text NOT NULL,
    target_id    bigint,
    diff_json    jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_log_library_time_idx
    ON audit_log (library_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_library_target_idx
    ON audit_log (library_id, target_type, target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_user_idx
    ON audit_log (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_action_idx
    ON audit_log (action_type, created_at DESC);

COMMIT;
