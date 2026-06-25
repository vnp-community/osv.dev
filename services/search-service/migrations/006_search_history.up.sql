-- Migration: 006_search_history.up.sql
-- TASK-HC-007: Persist user search history to PostgreSQL.
-- Replaces the empty list stub in GetSearchRecent handler.

CREATE TABLE IF NOT EXISTS search_history (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL,
    query        TEXT        NOT NULL,
    result_count INT         NOT NULL DEFAULT 0,
    search_type  VARCHAR(20) NOT NULL DEFAULT 'fulltext'
                 CHECK (search_type IN ('fulltext', 'semantic', 'cve_id', 'browse')),
    filters      JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_history_user
    ON search_history(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_search_history_created
    ON search_history(created_at);
