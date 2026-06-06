-- Schedule service schema
SET search_path TO schedule;

CREATE TABLE IF NOT EXISTS scheduled_scans (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL,
    targets          JSONB NOT NULL DEFAULT '[]',
    scan_type        VARCHAR(20) NOT NULL DEFAULT 'discovery',
    cron_expr        VARCHAR(100),
    interval_minutes INT,
    status           VARCHAR(20) NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active','paused','expired')),
    next_run_at      TIMESTAMPTZ NOT NULL,
    last_run_at      TIMESTAMPTZ,
    options          JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT cron_or_interval CHECK (
        (cron_expr IS NOT NULL AND interval_minutes IS NULL) OR
        (cron_expr IS NULL AND interval_minutes IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_schedules_next_run
    ON schedule.scheduled_scans(next_run_at)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_schedules_user
    ON schedule.scheduled_scans(user_id);
