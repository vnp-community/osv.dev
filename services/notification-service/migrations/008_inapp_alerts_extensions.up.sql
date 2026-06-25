-- 008_inapp_alerts_extensions.up.sql
ALTER TABLE inapp_alerts
ADD COLUMN IF NOT EXISTS type VARCHAR(100),
ADD COLUMN IF NOT EXISTS message TEXT,
ADD COLUMN IF NOT EXISTS severity VARCHAR(50),
ADD COLUMN IF NOT EXISTS entity_type VARCHAR(100),
ADD COLUMN IF NOT EXISTS entity_id VARCHAR(100),
ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;

-- Migrate old columns if needed
UPDATE inapp_alerts SET type = event_type WHERE type IS NULL;
UPDATE inapp_alerts SET message = description WHERE message IS NULL;
