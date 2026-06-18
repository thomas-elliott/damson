BEGIN;

-- Per-file preview derivatives. A file may carry a backend-rendered preview
-- (a flattened/rasterised image) distinct from the per-asset thumbnail; the
-- key points into the same content-addressed thumb namespace as thumbnails.
ALTER TABLE asset_files
    ADD COLUMN IF NOT EXISTS preview_object_key   text,
    ADD COLUMN IF NOT EXISTS preview_content_type text;

-- Gate for the previewing pipeline stage, mirroring the thumbnails_* toggles.
ALTER TABLE library_settings
    ADD COLUMN IF NOT EXISTS previews_enabled boolean NOT NULL DEFAULT true;

COMMIT;
