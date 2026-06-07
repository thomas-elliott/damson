BEGIN;

-- External IdP identities (issuer + subject) keyed to a damson user. One user
-- may carry several — the same person via Google and via Apple — reconciled
-- onto one account by verified email at login time. Local-auth users have no
-- rows here; they authenticate by email + password.
CREATE TABLE IF NOT EXISTS user_identities (
    id          bigserial PRIMARY KEY,
    user_id     bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer      text NOT NULL,
    subject     text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);

CREATE INDEX IF NOT EXISTS user_identities_user_idx
    ON user_identities (user_id);

-- Email becomes the canonical cross-IdP join key. Make it unique among
-- non-blank values: partial so multiple freshly-created OIDC users can sit at
-- '' (needs_profile) until they set one. Replaces the old non-unique lookup
-- index — the WHERE email <> '' index still serves lower(email) lookups.
DROP INDEX IF EXISTS users_email_idx;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_idx
    ON users (lower(email)) WHERE email <> '';

-- subject moves to user_identities. No data backfill: an existing OIDC user is
-- re-adopted on next login (issuer+subject miss → verified-email match → links
-- a fresh identity to the same row), and local users never used it for lookup.
ALTER TABLE users DROP COLUMN IF EXISTS subject;

COMMIT;
