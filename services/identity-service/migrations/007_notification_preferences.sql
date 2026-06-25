-- Migration: 007_notification_preferences.sql
-- TASK-V6-001: Bảng notification preferences cho profile notifications/settings endpoint.
-- Handler: adapter/handler/http/profile_handler.go → GetNotifSettings / UpdateNotifSettings
-- Repo:    adapter/repository/postgres/notif_pref_repo.go

SET search_path TO auth;

CREATE TABLE IF NOT EXISTS notification_preferences (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  VARCHAR(100) NOT NULL,   -- e.g. "finding.created", "scan.completed"
    label       VARCHAR(255) NOT NULL,
    description TEXT,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_notif_pref_user_event UNIQUE (user_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_notif_pref_user_id ON notification_preferences(user_id);

-- Default preferences seeded for known event types.
-- The INSERT is skipped if user already has preferences (ON CONFLICT DO NOTHING).
-- Actual seeding per-user happens lazily in application code.
COMMENT ON TABLE notification_preferences IS
    'Per-user notification opt-in/out preferences for each event_type.
     GET /api/v1/profile/notifications/settings reads this table.
     PUT /api/v1/profile/notifications/settings writes to this table.';
