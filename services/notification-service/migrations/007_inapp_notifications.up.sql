-- services/notification-service/migrations/007_inapp_notifications.up.sql
-- In-App notification table for SSE-based real-time delivery.
-- ADDITIVE: existing notification tables are unchanged.

CREATE TABLE IF NOT EXISTS inapp_notifications (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL,
    event_type  VARCHAR(100) NOT NULL,
    title       VARCHAR(500) NOT NULL,
    body        TEXT,
    payload     JSONB,
    is_read     BOOLEAN     NOT NULL DEFAULT FALSE,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial index: fast unread query per user, ordered by newest first
CREATE INDEX IF NOT EXISTS idx_inapp_unread
    ON inapp_notifications(user_id, created_at DESC)
    WHERE is_read = FALSE;
