-- Migration 015: Fix missing created_by column + assigned_to type mismatch
-- Root cause of 500 error on GET /api/v1/findings?status[]=active
--
-- Bug 1: `created_by` column referenced in finding_repo.go List() SELECT but
--        was never created in any prior migration → "column does not exist" error.
--
-- Bug 2: `assigned_to` was added in 014 as UUID type, but the Go entity
--        has AssignedTo *string. The scan fails with type mismatch.
--        Fix: change to TEXT type to match the Go model.

BEGIN;

-- Fix Bug 1: add missing created_by column
ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS created_by TEXT DEFAULT '';

-- Fix Bug 2: change assigned_to from UUID → TEXT
-- (Go entity: AssignedTo *string, not a foreign key, just a display name)
-- Step 1: drop the UUID column added in 014
ALTER TABLE findings DROP COLUMN IF EXISTS assigned_to;
-- Step 2: re-add as TEXT
ALTER TABLE findings ADD COLUMN IF NOT EXISTS assigned_to TEXT DEFAULT '';

COMMIT;
