-- 013_siem_config.sql
-- SIEM configuration table (app-level — không có trong services gốc)

CREATE TABLE IF NOT EXISTS siem_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    host        VARCHAR(255) NOT NULL DEFAULT '',
    port        INTEGER NOT NULL DEFAULT 514,
    protocol    VARCHAR(10) NOT NULL DEFAULT 'udp' CHECK (protocol IN ('udp', 'tcp', 'tls')),
    facility    INTEGER NOT NULL DEFAULT 16,   -- local0 = 16
    severity    INTEGER NOT NULL DEFAULT 6,    -- informational
    app_name    VARCHAR(50) NOT NULL DEFAULT 'openvulnscan',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default config (chỉ 1 row)
INSERT INTO siem_configs (enabled, host, port, protocol)
VALUES (false, '', 514, 'udp')
ON CONFLICT DO NOTHING;
