-- Gateway/shared database settings table
CREATE TABLE IF NOT EXISTS platform_settings (
    key        VARCHAR(100) PRIMARY KEY,
    value_json JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255)
);

-- Initial seed
INSERT INTO platform_settings (key, value_json) VALUES
('general', '{"platform_name":"OSV Platform","max_scan_targets":100,"report_retention_days":90}'),
('notifications', '{"smtp_host":"","smtp_port":587,"smtp_from":"noreply@osv.local","smtp_password":""}'),
('ai', '{"ollama_enabled":true,"ollama_url":"http://ollama:11434","openai_enabled":false,"openai_model":"gpt-4","embedding_enabled":true,"embedding_dims":1536}'),
('security', '{"max_login_attempts":5,"session_timeout_minutes":15,"rate_limit_per_min":100}')
ON CONFLICT (key) DO NOTHING;
