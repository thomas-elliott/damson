BEGIN;

CREATE TABLE IF NOT EXISTS server_settings (
    id                       int PRIMARY KEY DEFAULT 1,
    max_upload_bytes         bigint NOT NULL DEFAULT 2147483648,
    licence_kb_auto_update   boolean NOT NULL DEFAULT false,
    updated_at               timestamptz NOT NULL DEFAULT now(),
    updated_by_user_id       bigint REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT server_settings_singleton CHECK (id = 1)
);

INSERT INTO server_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS library_settings (
    library_id                 bigint PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
    theme_default              text    NOT NULL DEFAULT 'dark',
    grid_density_default       text    NOT NULL DEFAULT 'medium',
    retain_original_packages   boolean NOT NULL DEFAULT true,
    dedupe_enabled             boolean NOT NULL DEFAULT true,
    thumbnails_raster_enabled  boolean NOT NULL DEFAULT true,
    thumbnails_audio_enabled   boolean NOT NULL DEFAULT true,
    thumbnails_font_enabled    boolean NOT NULL DEFAULT true,
    thumbnails_unity_preview   boolean NOT NULL DEFAULT true,
    audit_retention_days       int,
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    updated_by_user_id         bigint REFERENCES users(id) ON DELETE SET NULL
);

-- Backfill a library_settings row for every existing library. Going forward,
-- library creation handlers are responsible for inserting one alongside the
-- library row.
INSERT INTO library_settings (library_id)
SELECT id FROM libraries
ON CONFLICT DO NOTHING;

COMMIT;
