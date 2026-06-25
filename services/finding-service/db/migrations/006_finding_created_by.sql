-- +migrate Up
ALTER TABLE findings ADD COLUMN IF NOT EXISTS created_by VARCHAR(255);

-- +migrate Down
ALTER TABLE findings DROP COLUMN IF EXISTS created_by;
