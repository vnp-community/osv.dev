-- migrations/006_kev_ransomware_extended.up.sql
-- CR-GCV-007: Extend kev_entries with ransomware and description fields
-- Also adds short_description, required_action from CISA KEV JSON 5.0

ALTER TABLE kev_entries
    ADD COLUMN IF NOT EXISTS short_description       TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS required_action         TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_known_ransomware     BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ransomware_campaign_use TEXT        NOT NULL DEFAULT '';

-- Index for ransomware filter (common query pattern)
CREATE INDEX IF NOT EXISTS idx_kev_ransomware ON kev_entries(is_known_ransomware)
    WHERE is_known_ransomware = TRUE;

-- Comment the new columns for documentation
COMMENT ON COLUMN kev_entries.short_description IS 'Brief summary of the vulnerability (from CISA KEV JSON)';
COMMENT ON COLUMN kev_entries.required_action IS 'CISA mandated remediation action';
COMMENT ON COLUMN kev_entries.is_known_ransomware IS 'TRUE if associated with known ransomware campaigns';
COMMENT ON COLUMN kev_entries.ransomware_campaign_use IS 'Ransomware campaign name or "Known"/"Unknown"';
