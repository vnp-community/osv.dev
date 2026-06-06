-- Migration 001: finding-management initial schema

BEGIN;

CREATE TABLE findings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(511) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    mitigation TEXT,
    impact TEXT,
    references TEXT,
    severity VARCHAR(20) NOT NULL DEFAULT 'Info',
    numerical_severity SMALLINT NOT NULL DEFAULT 0,
    cve VARCHAR(50),
    cwe INTEGER,
    vuln_id_from_tool VARCHAR(500),
    cvss_v3 VARCHAR(200),
    cvss_v3_score NUMERIC(4,1),
    cvss_v4 VARCHAR(200),
    cvss_v4_score NUMERIC(4,1),

    -- Status flags
    active BOOLEAN NOT NULL DEFAULT TRUE,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    false_positive BOOLEAN NOT NULL DEFAULT FALSE,
    duplicate BOOLEAN NOT NULL DEFAULT FALSE,
    out_of_scope BOOLEAN NOT NULL DEFAULT FALSE,
    is_mitigated BOOLEAN NOT NULL DEFAULT FALSE,
    risk_accepted BOOLEAN NOT NULL DEFAULT FALSE,

    -- Timestamps
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    mitigated_at TIMESTAMPTZ,
    mitigated_by_id UUID,
    last_reviewed TIMESTAMPTZ,
    last_status_update TIMESTAMPTZ,
    sla_expiration_date DATE,

    -- Context
    test_id UUID NOT NULL,
    engagement_id UUID NOT NULL,
    product_id UUID NOT NULL,
    duplicate_finding_id UUID REFERENCES findings(id),
    finding_group_id UUID,

    -- Location
    component_name VARCHAR(200),
    component_version VARCHAR(100),
    service VARCHAR(200),
    file_path TEXT,
    line_number INTEGER,

    -- Deduplication
    hash_code VARCHAR(64),

    -- Tags
    tags TEXT[] NOT NULL DEFAULT '{}',
    inherited_tags TEXT[] NOT NULL DEFAULT '{}',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Critical indexes for performance
CREATE INDEX idx_findings_hash_code ON findings(hash_code) WHERE hash_code IS NOT NULL;
CREATE INDEX idx_findings_test_active ON findings(test_id, active) WHERE active = TRUE;
CREATE INDEX idx_findings_sla_expiration ON findings(sla_expiration_date) WHERE active = TRUE;
CREATE INDEX idx_findings_product_severity ON findings(product_id, severity, active);
CREATE INDEX idx_findings_engagement ON findings(engagement_id);
CREATE INDEX idx_findings_product_active ON findings(product_id, active);

CREATE TABLE finding_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    author_id UUID NOT NULL,
    note TEXT NOT NULL,
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notes_finding ON finding_notes(finding_id);

CREATE TABLE finding_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    jira_issue TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_groups_test ON finding_groups(test_id);

CREATE TABLE endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    host VARCHAR(500) NOT NULL,
    path TEXT,
    protocol VARCHAR(10),
    port INTEGER,
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_endpoints_product ON endpoints(product_id);
CREATE UNIQUE INDEX idx_endpoints_unique ON endpoints(product_id, host, protocol, port);

CREATE TABLE finding_endpoints (
    finding_id UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    PRIMARY KEY (finding_id, endpoint_id)
);

COMMIT;
