-- 005_create_alias_groups.up.sql
-- PostgreSQL alternative to Firestore for alias groups.
-- Pattern: parallel storage — Firestore remains default until switched via config.
--
-- AliasGroup mirrors the domain aggregate AliasGroup:
--   group_id        = AliasGroup.ID()
--   bug_ids         = AliasGroup.BugIDs()
--   canonical_id    = AliasGroup.CanonicalID()
--   detection_method = AliasGroup.DetectionMethod()
--
-- Two tables replicate the Firestore two-collection approach:
--   alias_groups        → the group document (alias-groups collection)
--   alias_group_members → the member index (alias-group-members collection)

CREATE TABLE IF NOT EXISTS alias_groups (
    group_id          TEXT        PRIMARY KEY,
    bug_ids           TEXT[]      NOT NULL DEFAULT '{}',
    canonical_id      TEXT        NOT NULL DEFAULT '',
    detection_method  TEXT        NOT NULL DEFAULT 'SOURCE_DECLARED',
    last_modified     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- GIN index for array containment queries (find group containing a given bug_id)
CREATE INDEX IF NOT EXISTS idx_alias_groups_bug_ids
    ON alias_groups USING GIN(bug_ids);

CREATE INDEX IF NOT EXISTS idx_alias_groups_canonical
    ON alias_groups(canonical_id);

CREATE INDEX IF NOT EXISTS idx_alias_groups_modified
    ON alias_groups(last_modified DESC);

-- alias_group_members is the denormalized member index.
-- Maps vuln_id → group_id for O(1) lookup of a group by member.
CREATE TABLE IF NOT EXISTS alias_group_members (
    vuln_id   TEXT PRIMARY KEY,
    group_id  TEXT NOT NULL REFERENCES alias_groups(group_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_alias_group_members_group
    ON alias_group_members(group_id);
