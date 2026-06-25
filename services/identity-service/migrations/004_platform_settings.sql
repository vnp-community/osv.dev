-- Migration: 004_platform_settings.sql
-- TASK-HC-009: Platform settings table for admin configuration.
-- Replaces hardcoded defaults in gateway-service AdminSettings handler.

CREATE TABLE IF NOT EXISTS platform_settings (
    key         VARCHAR(128)  PRIMARY KEY,
    value       JSONB         NOT NULL DEFAULT '{}',
    section     VARCHAR(64)   NOT NULL DEFAULT 'general',
    description TEXT,
    updated_by  UUID,
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Seed default settings (idempotent via ON CONFLICT DO NOTHING)
INSERT INTO platform_settings (key, value, section, description) VALUES
  ('general.platform_name',   '"OpenVulnScan"',          'general',  'Platform display name'),
  ('general.organization',    '""',                       'general',  'Organization name'),
  ('general.support_email',   '""',                       'general',  'Support email address'),
  ('general.timezone',        '"UTC"',                    'general',  'Default timezone'),
  ('general.date_format',     '"YYYY-MM-DD"',             'general',  'Date display format'),
  ('general.session_timeout', '30',                       'general',  'Session timeout in minutes'),
  ('smtp.enabled',            'false',                    'smtp',     'Enable SMTP email sending'),
  ('smtp.host',               '""',                       'smtp',     'SMTP host'),
  ('smtp.port',               '587',                      'smtp',     'SMTP port'),
  ('smtp.username',           '""',                       'smtp',     'SMTP username'),
  ('smtp.from_email',         '""',                       'smtp',     'From email address'),
  ('smtp.tls',                'true',                     'smtp',     'Enable TLS'),
  ('security.mfa_required',         'false',              'security', 'Require MFA for all users'),
  ('security.password_policy',      '"medium"',           'security', 'Password strength policy'),
  ('security.max_login_attempts',   '5',                  'security', 'Max failed login attempts'),
  ('security.lockout_duration_min', '30',                 'security', 'Account lockout duration (min)'),
  ('security.api_key_expiry_days',  '90',                 'security', 'API key expiry in days'),
  ('security.jwt_expiry_min',       '15',                 'security', 'JWT token expiry in minutes'),
  ('ai.enabled',         'false',                         'ai',       'Enable AI features'),
  ('ai.provider',        '"ollama"',                      'ai',       'AI provider (ollama/openai)'),
  ('ai.model',           '"qwen2.5:1.5b"',               'ai',       'LLM model name'),
  ('ai.endpoint',        '"http://ollama:11434"',         'ai',       'AI provider endpoint URL'),
  ('ai.auto_triage',     'false',                         'ai',       'Enable automatic finding triage'),
  ('ai.auto_enrichment', 'false',                         'ai',       'Enable automatic CVE enrichment')
ON CONFLICT (key) DO NOTHING;
