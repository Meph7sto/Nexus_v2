-- Reconcile owner values written by the historical Nexus and Sub2API builds.
-- The statement is intentionally case-insensitive and idempotent.
UPDATE ops_error_logs
SET error_owner = 'platform'
WHERE LOWER(COALESCE(TRIM(error_owner), '')) IN ('nexus', 'sub2api');
