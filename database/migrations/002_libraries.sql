BEGIN;

CREATE TABLE IF NOT EXISTS libraries (
    id          bigserial PRIMARY KEY,
    name        text NOT NULL,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS libraries_name_idx ON libraries (name);

-- users.current_library_id was declared without an FK in 001 so the two
-- migrations could be kept independent. Add the FK now that libraries exist.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'users_current_library_fk'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_current_library_fk
            FOREIGN KEY (current_library_id) REFERENCES libraries(id) ON DELETE SET NULL;
    END IF;
END
$$;

COMMIT;
