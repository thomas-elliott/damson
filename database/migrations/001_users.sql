BEGIN;

CREATE TABLE IF NOT EXISTS users (
    id                  bigserial PRIMARY KEY,
    subject             text NOT NULL UNIQUE,
    email               text NOT NULL,
    display_name        text NOT NULL,
    role                text NOT NULL DEFAULT 'user',
    current_library_id  bigint,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMIT;
