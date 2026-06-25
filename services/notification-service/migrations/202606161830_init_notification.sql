-- notification-service initial schema

-- Webhooks
CREATE TABLE IF NOT EXISTS webhooks (
    id          TEXT        PRIMARY KEY,
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL,
    events      TEXT[]      NOT NULL DEFAULT '{}',
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    owner_id    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhooks_owner  ON webhooks(owner_id) WHERE is_active;
CREATE INDEX IF NOT EXISTS idx_webhooks_events ON webhooks USING GIN(events);

-- Webhook deliveries (audit trail)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              TEXT        PRIMARY KEY,
    webhook_id      TEXT        NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    payload         TEXT        NOT NULL,
    status_code     INT         DEFAULT NULL,
    attempt         INT         NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','delivered','failed','retrying')),
    delivered_at    TIMESTAMPTZ DEFAULT NULL,
    next_retry_at   TIMESTAMPTZ DEFAULT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_deliveries_webhook
    ON webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deliveries_retry
    ON webhook_deliveries(next_retry_at)
    WHERE status = 'retrying';

-- Alert subscriptions
CREATE TABLE IF NOT EXISTS alert_subscriptions (
    id              TEXT        PRIMARY KEY,
    owner_id        TEXT        NOT NULL,
    type            TEXT        NOT NULL CHECK (type IN ('vendor','product','kev')),
    value           TEXT        NOT NULL,
    min_severity    TEXT        NOT NULL DEFAULT 'HIGH'
                    CHECK (min_severity IN ('CRITICAL','HIGH','MEDIUM','LOW')),
    min_epss        NUMERIC(8,6) DEFAULT NULL,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_owner
    ON alert_subscriptions(owner_id) WHERE is_active;
CREATE INDEX IF NOT EXISTS idx_subscriptions_vendor
    ON alert_subscriptions(type, lower(value))
    WHERE type IN ('vendor','product');
