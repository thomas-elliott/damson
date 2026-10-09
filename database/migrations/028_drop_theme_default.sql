BEGIN;

-- The library default theme was never read by the SPA, which follows the
-- user's own choice or the OS. Dropped rather than wired up.
ALTER TABLE library_settings DROP COLUMN IF EXISTS theme_default;

COMMIT;
