BEGIN;

-- The 2 GiB default predates streaming uploads and rejects the 20 GB archives
-- damson is meant to hold.
ALTER TABLE server_settings
    ALTER COLUMN max_upload_bytes SET DEFAULT 68719476736;

-- Carry over deployments still holding the old default verbatim: that value is
-- "nobody chose a cap", not a deliberate 2 GiB. Any other value was set on
-- purpose and is left alone.
UPDATE server_settings
   SET max_upload_bytes = 68719476736
 WHERE max_upload_bytes = 2147483648;

COMMIT;
