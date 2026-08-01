BEGIN;

-- Long-lived credentials for unattended clients. A token is pinned to one
-- library at creation, so a running sync is unaffected by its owner switching
-- library in the SPA.
--
-- token_hash is a plain SHA-256 hex digest, not bcrypt: the secret is 32
-- random bytes, so a work factor adds nothing, and a fast digest is what
-- allows the indexed lookup on every authenticated request. prefix is for
-- display only and is never enough to reconstruct the secret.
CREATE TABLE IF NOT EXISTS api_tokens (
    id           bigserial PRIMARY KEY,
    user_id      bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    library_id   bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    label        text NOT NULL DEFAULT '',
    token_hash   text NOT NULL,
    prefix       text NOT NULL DEFAULT '',
    scope        text NOT NULL DEFAULT 'write',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at   timestamptz,
    revoked_at   timestamptz,
    CONSTRAINT api_tokens_scope_check CHECK (scope IN ('read', 'write'))
);

CREATE UNIQUE INDEX IF NOT EXISTS api_tokens_hash_unique_idx
    ON api_tokens (token_hash);

CREATE INDEX IF NOT EXISTS api_tokens_user_idx
    ON api_tokens (user_id);

COMMIT;
