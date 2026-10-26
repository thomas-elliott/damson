BEGIN;

-- Re-pinning an asset writes one action whether the new version is newer or
-- older, so project.pin.upgrade mislabelled every downgrade. Existing rows take
-- the neutral name the backend now writes.
UPDATE audit_log SET action_type = 'project.pin.change'
WHERE action_type = 'project.pin.upgrade';

COMMIT;
