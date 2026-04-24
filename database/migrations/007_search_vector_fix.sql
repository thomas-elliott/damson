BEGIN;

-- Corrects two bugs in 006 that left search_vector empty for assets imported
-- before M3:
--
-- 1. The asset_tags touch trigger updated `assets.updated_at`, but the
--    search_vector recompute trigger only fires on UPDATE OF name,
--    description. Tag changes therefore never recomputed the vector's
--    tag-weighted segment. Fix: update `name` on the asset row (idempotent
--    self-assignment) so the recompute fires.
--
-- 2. The initial backfill ran inside 006; if 006 had already been applied
--    before the `SET name = name` version landed (e.g. on a dev DB rolled
--    forward through the bug), existing rows have NULL/empty vectors.
--    Re-run the backfill here unconditionally — cheap for single-digit-
--    thousand rows, and idempotent (the trigger recomputes from source).

CREATE OR REPLACE FUNCTION asset_tags_touch_asset() RETURNS trigger AS $$
DECLARE
    target_asset bigint;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_asset := OLD.asset_id;
    ELSE
        target_asset := NEW.asset_id;
    END IF;
    UPDATE assets
        SET name = name
        WHERE id = target_asset;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Force recompute for every asset. `SET name = name` fires the
-- BEFORE UPDATE OF name trigger.
UPDATE assets SET name = name;

COMMIT;
