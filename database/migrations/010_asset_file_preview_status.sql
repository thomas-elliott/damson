BEGIN;

-- First-class per-file preview state, so previewing can be observed (a
-- processing queue), requeued, and distinguished from "no preview yet". One of:
--   unsupported  no backend renderer for this file type (the common case)
--   queued       supported, awaiting a preview worker
--   processing   a worker is rendering it
--   done         preview_object_key is set
--   failed       the render errored (retryable via requeue)
-- Default 'unsupported' so existing and newly-inserted rows are inert until the
-- pipeline (or a requeue) explicitly enqueues them.
ALTER TABLE asset_files
    ADD COLUMN IF NOT EXISTS preview_status text NOT NULL DEFAULT 'unsupported';

-- Backfill: any row that already produced a derivative is terminally 'done'.
-- Historical rows that were supported-but-silently-failed read as 'unsupported'
-- until a requeue corrects them — acceptable and documented in the spec.
UPDATE asset_files
    SET preview_status = 'done'
    WHERE preview_object_key IS NOT NULL AND preview_object_key <> '';

COMMIT;
