-- +migrate Up
ALTER TABLE findings ADD COLUMN IF NOT EXISTS epss_score    NUMERIC(6,5);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS is_kev        BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS assigned_to   VARCHAR(255);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_ip      INET;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_hostname VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_findings_epss      ON findings(epss_score DESC) WHERE epss_score IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_findings_is_kev    ON findings(is_kev) WHERE is_kev = true;
CREATE INDEX IF NOT EXISTS idx_findings_assigned  ON findings(assigned_to) WHERE assigned_to IS NOT NULL;

-- +migrate Down
ALTER TABLE findings
    DROP COLUMN IF EXISTS epss_score,
    DROP COLUMN IF EXISTS is_kev,
    DROP COLUMN IF EXISTS assigned_to,
    DROP COLUMN IF EXISTS asset_ip,
    DROP COLUMN IF EXISTS asset_hostname;
