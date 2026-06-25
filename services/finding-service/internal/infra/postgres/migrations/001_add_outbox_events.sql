CREATE TABLE IF NOT EXISTS outbox_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject      VARCHAR(200) NOT NULL,
    payload      JSONB NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','published','failed')),
    attempts     INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    last_error   TEXT
);

-- Partial index cho pending events (polling query)
CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox_events(created_at)
    WHERE status = 'pending' AND attempts < 10;
