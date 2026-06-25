-- 005_notification_rules.down.sql
-- Reverts migration 005_notification_rules.up.sql

DROP TRIGGER IF EXISTS notification_rules_updated_at ON notification_rules;
DROP FUNCTION IF EXISTS update_notification_rules_updated_at();
DROP TABLE IF EXISTS delivery_records;
DROP TABLE IF EXISTS inapp_alerts;
DROP TABLE IF EXISTS notification_rules;
