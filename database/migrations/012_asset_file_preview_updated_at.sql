BEGIN;

-- When a file's preview status last changed. Gives the processing queue a real
-- per-version finish time (MAX across the version's files) so completed rollups
-- can be retained and ordered in a "finished" section, rather than vanishing the
-- instant they leave the in-flight set. Nullable: pre-existing rows have no
-- recorded transition and read as NULL until the next queue/render touches them.
ALTER TABLE asset_files
    ADD COLUMN IF NOT EXISTS preview_updated_at timestamptz;

-- Backfill terminal rows so historical previews sort sensibly rather than all
-- collapsing to NULL — best-effort, using the row's creation time.
UPDATE asset_files
    SET preview_updated_at = created_at
    WHERE preview_updated_at IS NULL
      AND preview_status IN ('done', 'failed');

COMMIT;
