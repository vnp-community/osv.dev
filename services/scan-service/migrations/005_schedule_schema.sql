CREATE SCHEMA IF NOT EXISTS schedule;

CREATE TABLE IF NOT EXISTS schedule.scheduled_scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    name VARCHAR(255),
    targets JSONB NOT NULL DEFAULT '[]'::jsonb,
    frequency VARCHAR(50),
    schedule_time VARCHAR(100),
    status VARCHAR(50) DEFAULT 'active',
    enabled BOOLEAN DEFAULT true,
    scan_type VARCHAR(100),
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
