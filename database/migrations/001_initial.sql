BEGIN;

CREATE TABLE IF NOT EXISTS libraries (
    id          bigserial PRIMARY KEY,
    name        text NOT NULL,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS libraries_name_idx ON libraries (name);

CREATE TABLE IF NOT EXISTS users (
    id                  bigserial PRIMARY KEY,
    subject             text NOT NULL UNIQUE,
    email               text NOT NULL,
    display_name        text NOT NULL,
    role                text NOT NULL DEFAULT 'user',
    current_library_id  bigint REFERENCES libraries(id) ON DELETE SET NULL,
    password_hash       text,
    tokens_valid_after  timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS users_email_idx ON users (lower(email));

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

CREATE TABLE IF NOT EXISTS asset_versions (
    id                          bigserial PRIMARY KEY,
    asset_id                    bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    version_label               text NOT NULL,
    notes                       text,
    import_date                 timestamptz NOT NULL DEFAULT now(),
    original_package_object_key text,
    package_kind                text,
    created_by_user_id          bigint REFERENCES users(id),
    UNIQUE (asset_id, version_label)
);

CREATE INDEX IF NOT EXISTS asset_versions_asset_imported_idx
    ON asset_versions (asset_id, import_date DESC);

-- assets.current_version_id gets its FK after asset_versions exists.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'assets_current_version_fk'
    ) THEN
        ALTER TABLE assets
            ADD CONSTRAINT assets_current_version_fk
            FOREIGN KEY (current_version_id) REFERENCES asset_versions(id) ON DELETE SET NULL;
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS asset_files (
    id               bigserial PRIMARY KEY,
    asset_version_id bigint NOT NULL REFERENCES asset_versions(id) ON DELETE CASCADE,
    relative_path    text NOT NULL,
    size_bytes       bigint NOT NULL,
    content_hash     text NOT NULL,
    object_key       text NOT NULL,
    detected_type    text,
    unity_guid       text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (asset_version_id, relative_path)
);

CREATE INDEX IF NOT EXISTS asset_files_version_idx ON asset_files (asset_version_id);
CREATE INDEX IF NOT EXISTS asset_files_hash_idx    ON asset_files (content_hash);

CREATE TABLE IF NOT EXISTS asset_imports (
    id          bigserial PRIMARY KEY,
    asset_id    bigint NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    stage       text   NOT NULL,
    error       text,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS asset_imports_asset_idx ON asset_imports (asset_id);
CREATE INDEX IF NOT EXISTS asset_imports_stage_idx ON asset_imports (stage);

COMMIT;
