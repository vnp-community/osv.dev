-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS cpe_dictionary (
    cpe_uri    TEXT        PRIMARY KEY,
    vendor     TEXT        NOT NULL DEFAULT '',
    product    TEXT        NOT NULL DEFAULT '',
    version    TEXT        NOT NULL DEFAULT '',
    title      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cpe_vendor  ON cpe_dictionary(vendor);
CREATE INDEX IF NOT EXISTS idx_cpe_product ON cpe_dictionary(product);

-- Webhooks table for notification service
CREATE TABLE IF NOT EXISTS webhooks (
    id          BIGSERIAL   PRIMARY KEY,
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL DEFAULT '',
    events      TEXT[]      NOT NULL DEFAULT '{}',  -- e.g. {"cve.new", "kev.updated"}
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_enabled ON webhooks(enabled) WHERE enabled = TRUE;

-- Notification delivery log
CREATE TABLE IF NOT EXISTS notification_log (
    id          BIGSERIAL   PRIMARY KEY,
    webhook_id  BIGINT      NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event       TEXT        NOT NULL,
    payload     JSONB       NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('PENDING', 'SENT', 'FAILED')),
    attempts    INT         NOT NULL DEFAULT 0,
    sent_at     TIMESTAMPTZ,
    error_msg   TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notif_log_webhook  ON notification_log(webhook_id);
CREATE INDEX IF NOT EXISTS idx_notif_log_status   ON notification_log(status);
CREATE INDEX IF NOT EXISTS idx_notif_log_created  ON notification_log(created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_notif_log_created;
DROP INDEX IF EXISTS idx_notif_log_status;
DROP INDEX IF EXISTS idx_notif_log_webhook;
DROP TABLE IF EXISTS notification_log;
DROP INDEX IF EXISTS idx_webhooks_enabled;
DROP TABLE IF EXISTS webhooks;
DROP INDEX IF EXISTS idx_cpe_product;
DROP INDEX IF EXISTS idx_cpe_vendor;
DROP TABLE IF EXISTS cpe_dictionary;
-- +goose StatementEnd
