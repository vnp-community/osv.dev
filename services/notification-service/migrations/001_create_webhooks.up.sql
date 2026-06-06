-- Notification service: webhooks table
CREATE TABLE IF NOT EXISTS webhooks (
    id          TEXT        PRIMARY KEY DEFAULT gen_random_uuid()::text,
    owner_id    TEXT        NOT NULL,
    url         TEXT        NOT NULL,
    events      TEXT[]      NOT NULL DEFAULT '{}',
    secret      TEXT        NOT NULL DEFAULT '',
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_owner  ON webhooks(owner_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(active) WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS idx_webhooks_events ON webhooks USING GIN(events) WHERE active = TRUE;
