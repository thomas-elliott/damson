BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash       text,
    ADD COLUMN IF NOT EXISTS tokens_valid_after  timestamptz;

-- Local-auth users are looked up by email at login time; OIDC users by
-- subject. Both are already indexed in practice (subject is UNIQUE from
-- 001), but email has no index yet and login is the hot path for local mode.
CREATE INDEX IF NOT EXISTS users_email_idx ON users (lower(email));

COMMIT;
