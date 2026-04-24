BEGIN;

CREATE TABLE IF NOT EXISTS projects (
    id                   bigserial PRIMARY KEY,
    library_id           bigint NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name                 text   NOT NULL,
    description          text,
    target_distribution  text   NOT NULL DEFAULT 'unspecified',
    glyph                text,
    glyph_color          text,
    created_by_user_id   bigint REFERENCES users(id) ON DELETE SET NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS projects_library_idx ON projects (library_id);
CREATE UNIQUE INDEX IF NOT EXISTS projects_library_name_idx
    ON projects (library_id, lower(name));

CREATE TABLE IF NOT EXISTS project_asset_pins (
    id                bigserial PRIMARY KEY,
    project_id        bigint NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    asset_id          bigint NOT NULL REFERENCES assets(id)   ON DELETE CASCADE,
    pinned_version_id bigint NOT NULL REFERENCES asset_versions(id) ON DELETE CASCADE,
    pinned_at         timestamptz NOT NULL DEFAULT now(),
    pinned_by_user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (project_id, asset_id)
);

CREATE INDEX IF NOT EXISTS project_asset_pins_asset_idx ON project_asset_pins (asset_id);

COMMIT;
