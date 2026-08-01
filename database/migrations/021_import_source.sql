BEGIN;

-- Where a version's bytes came from, when the client knew something damson
-- cannot recover from the archive itself. Nullable throughout: every version
-- imported before this migration, and every hand-dragged upload after it, has
-- no source and must stay distinguishable from one whose source is blank.
--
-- source_relative_path is the path within that source (for the Unity Asset
-- Store cache, <Publisher>/<CategoryPath>/<Name>.unitypackage). Together with
-- source_size_bytes it is what makes a re-sync answer "already imported" or
-- "updated" without hashing gigabytes.
ALTER TABLE asset_versions
    ADD COLUMN IF NOT EXISTS source_kind text,
    ADD COLUMN IF NOT EXISTS source_relative_path text,
    ADD COLUMN IF NOT EXISTS source_size_bytes bigint;

-- Preflight looks up a whole manifest at once, joining through assets for the
-- library scope. Partial, because only cache-imported versions carry a path.
CREATE INDEX IF NOT EXISTS idx_versions_source_path
    ON asset_versions (source_kind, source_relative_path)
    WHERE source_relative_path IS NOT NULL;

COMMIT;
