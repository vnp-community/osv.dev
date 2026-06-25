-- Migration: 005_rbac_roles_metadata.sql
-- TASK-HC-010: Role metadata and permission categories from PostgreSQL.
-- Replaces static maps in GetRBACMatrix handler.

-- Roles metadata table (display name, color, description per role)
CREATE TABLE IF NOT EXISTS rbac_roles (
    id           SERIAL       PRIMARY KEY,
    name         VARCHAR(64)  NOT NULL UNIQUE,
    display_name VARCHAR(128) NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    color        VARCHAR(32)  NOT NULL DEFAULT '#6B7280',
    is_system    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Permission categories for the UI matrix
CREATE TABLE IF NOT EXISTS rbac_permission_categories (
    id         SERIAL       PRIMARY KEY,
    category   VARCHAR(64)  NOT NULL UNIQUE,
    sort_order INT          NOT NULL DEFAULT 0
);

-- Permissions within each category
CREATE TABLE IF NOT EXISTS rbac_category_permissions (
    category_id INT         NOT NULL REFERENCES rbac_permission_categories(id) ON DELETE CASCADE,
    permission  VARCHAR(64) NOT NULL,
    PRIMARY KEY (category_id, permission)
);

-- Seed roles (idempotent)
INSERT INTO rbac_roles (name, display_name, description, color) VALUES
  ('admin',    'Administrator',    'Full system access',     '#8B5CF6'),
  ('user',     'Security Analyst', 'Standard user access',   '#3B82F6'),
  ('readonly', 'Read-Only Viewer', 'View-only access',       '#6B7280'),
  ('agent',    'Scan Agent',       'Automated scanner',      '#10B981')
ON CONFLICT (name) DO UPDATE
  SET display_name = EXCLUDED.display_name,
      description  = EXCLUDED.description,
      color        = EXCLUDED.color;

-- Seed permission categories
INSERT INTO rbac_permission_categories (category, sort_order) VALUES
  ('Dashboard',      1),
  ('Scanning',       2),
  ('Findings',       3),
  ('Reports',        4),
  ('AI Center',      5),
  ('Administration', 6),
  ('Agent',          7)
ON CONFLICT (category) DO NOTHING;

-- Seed category permissions
INSERT INTO rbac_category_permissions (category_id, permission)
SELECT id, perm FROM rbac_permission_categories,
  (VALUES
    ('Dashboard',      'scan:read'),
    ('Dashboard',      'finding:read'),
    ('Scanning',       'scan:create'),
    ('Scanning',       'scan:read'),
    ('Scanning',       'scan:delete'),
    ('Findings',       'finding:write'),
    ('Findings',       'finding:read'),
    ('Reports',        'report:download'),
    ('AI Center',      'finding:write'),
    ('Administration', 'user:manage'),
    ('Administration', 'system:configure'),
    ('Agent',          'agent:report')
  ) AS t(cat, perm)
WHERE rbac_permission_categories.category = t.cat
ON CONFLICT DO NOTHING;
