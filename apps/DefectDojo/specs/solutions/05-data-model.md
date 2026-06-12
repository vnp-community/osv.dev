# Data Model & Database Schema

## Thiết kế Database

Monolith sử dụng **một PostgreSQL database** với **schema namespacing** để phân tách dữ liệu của từng service (tái sử dụng migrations từ từng service).

### Schema Organization

```sql
-- Schema per service (logical isolation trong cùng DB)
CREATE SCHEMA auth;          -- auth-service tables
CREATE SCHEMA product;       -- product-service tables
CREATE SCHEMA finding;       -- finding-service tables
CREATE SCHEMA scan;          -- scan-service tables
CREATE SCHEMA vulnerability; -- vulnerability-service tables
CREATE SCHEMA notification;  -- notification-service tables
CREATE SCHEMA report;        -- report-service tables
CREATE SCHEMA integration;   -- integration-service tables
```

## Auth Schema

```sql
-- auth.users
CREATE TABLE auth.users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(150) UNIQUE NOT NULL,
    email           VARCHAR(254) UNIQUE NOT NULL,
    password_hash   VARCHAR(256) NOT NULL,
    first_name      VARCHAR(150),
    last_name       VARCHAR(150),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    is_superuser    BOOLEAN NOT NULL DEFAULT FALSE,
    last_login      TIMESTAMPTZ,
    date_joined     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    totp_secret     VARCHAR(64),
    totp_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- auth.api_keys  
CREATE TABLE auth.api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    key_hash        VARCHAR(256) NOT NULL UNIQUE,
    name            VARCHAR(100) NOT NULL,
    permissions     TEXT[] NOT NULL DEFAULT '{}',
    last_used_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- auth.sessions (Redis-backed, but table for audit)
CREATE TABLE auth.sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash      VARCHAR(256) NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked         BOOLEAN NOT NULL DEFAULT FALSE
);

-- auth.roles (RBAC)
CREATE TABLE auth.roles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(50) UNIQUE NOT NULL,  -- 'admin', 'maintainer', 'developer', 'viewer', 'api-importer'
    permissions     TEXT[] NOT NULL DEFAULT '{}'
);

-- auth.user_roles
CREATE TABLE auth.user_roles (
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role_id         UUID NOT NULL REFERENCES auth.roles(id) ON DELETE CASCADE,
    scope_type      VARCHAR(20),  -- 'global', 'product_type', 'product'
    scope_id        UUID,         -- product_type_id or product_id
    PRIMARY KEY (user_id, role_id, COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'::UUID))
);
```

## Product Schema

```sql
-- product.product_types
CREATE TABLE product.product_types (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) UNIQUE NOT NULL,
    description         TEXT,
    critical_product    BOOLEAN NOT NULL DEFAULT FALSE,
    key_product         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- product.products
CREATE TABLE product.products (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_type_id             UUID NOT NULL REFERENCES product.product_types(id),
    name                        VARCHAR(255) NOT NULL,
    description                 TEXT,
    prod_numeric_grade          INT DEFAULT 0,
    business_criticality        VARCHAR(20),  -- 'very high', 'high', 'medium', 'low', 'very low'
    platform                    VARCHAR(20),  -- 'web', 'mobile', 'desktop', 'iot'
    lifecycle                   VARCHAR(20),  -- 'construction', 'production', 'retirement'
    origin                      VARCHAR(20),  -- 'internal', 'acquired', 'contractor'
    external_audience           BOOLEAN NOT NULL DEFAULT FALSE,
    internet_accessible         BOOLEAN NOT NULL DEFAULT FALSE,
    enable_full_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    enable_simple_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    tags                        TEXT[] NOT NULL DEFAULT '{}',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- product.engagements
CREATE TABLE product.engagements (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id          UUID NOT NULL REFERENCES product.products(id) ON DELETE CASCADE,
    name                VARCHAR(300) NOT NULL,
    description         TEXT,
    target_start        DATE NOT NULL,
    target_end          DATE NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'In Progress',
    engagement_type     VARCHAR(20) NOT NULL DEFAULT 'Interactive',
    lead_id             UUID REFERENCES auth.users(id),
    version             VARCHAR(100),
    build_id            VARCHAR(100),
    commit_hash         VARCHAR(100),
    branch_tag          VARCHAR(500),
    source_code_management_uri TEXT,
    deduplication_on_engagement BOOLEAN NOT NULL DEFAULT TRUE,
    tags                TEXT[] NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at           TIMESTAMPTZ
);

-- product.tests
CREATE TABLE product.tests (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id       UUID NOT NULL REFERENCES product.engagements(id) ON DELETE CASCADE,
    test_type_id        UUID NOT NULL,
    title               VARCHAR(255),
    description         TEXT,
    target_start        TIMESTAMPTZ,
    target_end          TIMESTAMPTZ,
    estimated_time      INTERVAL,
    actual_time         INTERVAL,
    lead_id             UUID REFERENCES auth.users(id),
    environment_id      UUID,
    version             VARCHAR(100),
    branch_tag          VARCHAR(500),
    build_id            VARCHAR(100),
    commit_hash         VARCHAR(100),
    tags                TEXT[] NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Finding Schema

```sql
-- finding.findings (matches finding-service domain entity)
CREATE TABLE finding.findings (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title                   VARCHAR(511) NOT NULL,
    description             TEXT,
    mitigation              TEXT,
    impact                  TEXT,
    references              TEXT,
    severity                VARCHAR(10) NOT NULL DEFAULT 'Info',
    numerical_severity      INT NOT NULL DEFAULT 0,
    cve                     VARCHAR(50),
    cwe                     INT,
    vuln_id_from_tool       VARCHAR(500),
    cvss_v3                 VARCHAR(117),
    cvss_v3_score           DECIMAL(3,1),
    cvss_v4                 VARCHAR(117),
    cvss_v4_score           DECIMAL(3,1),
    
    -- Status flags
    active                  BOOLEAN NOT NULL DEFAULT TRUE,
    verified                BOOLEAN NOT NULL DEFAULT FALSE,
    false_positive          BOOLEAN NOT NULL DEFAULT FALSE,
    duplicate               BOOLEAN NOT NULL DEFAULT FALSE,
    out_of_scope            BOOLEAN NOT NULL DEFAULT FALSE,
    is_mitigated            BOOLEAN NOT NULL DEFAULT FALSE,
    risk_accepted           BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Timestamps
    date                    DATE NOT NULL,
    mitigated_at            TIMESTAMPTZ,
    mitigated_by_id         UUID REFERENCES auth.users(id),
    last_reviewed           TIMESTAMPTZ,
    last_status_update      TIMESTAMPTZ,
    sla_expiration_date     DATE,
    
    -- Context
    test_id                 UUID NOT NULL REFERENCES product.tests(id) ON DELETE CASCADE,
    engagement_id           UUID NOT NULL REFERENCES product.engagements(id),
    product_id              UUID NOT NULL REFERENCES product.products(id),
    duplicate_finding_id    UUID REFERENCES finding.findings(id),
    finding_group_id        UUID,
    
    -- Location
    component_name          VARCHAR(200),
    component_version       VARCHAR(200),
    service                 VARCHAR(200),
    file_path               VARCHAR(4000),
    line_number             INT,
    
    -- Dedup
    hash_code               VARCHAR(64),
    
    -- Tags
    tags                    TEXT[] NOT NULL DEFAULT '{}',
    inherited_tags          TEXT[] NOT NULL DEFAULT '{}',
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_findings_product     ON finding.findings(product_id);
CREATE INDEX idx_findings_engagement  ON finding.findings(engagement_id);
CREATE INDEX idx_findings_test        ON finding.findings(test_id);
CREATE INDEX idx_findings_hash        ON finding.findings(hash_code);
CREATE INDEX idx_findings_severity    ON finding.findings(severity);
CREATE INDEX idx_findings_active      ON finding.findings(active) WHERE active = TRUE;
CREATE INDEX idx_findings_sla         ON finding.findings(sla_expiration_date) WHERE sla_expiration_date IS NOT NULL;

-- finding.finding_groups
CREATE TABLE finding.finding_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    test_id     UUID NOT NULL REFERENCES product.tests(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- finding.endpoints
CREATE TABLE finding.endpoints (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID NOT NULL REFERENCES product.products(id),
    host        VARCHAR(500) NOT NULL,
    port        INT,
    path        VARCHAR(2000),
    protocol    VARCHAR(20),
    tags        TEXT[] NOT NULL DEFAULT '{}'
);

-- finding.finding_endpoints (many-to-many)
CREATE TABLE finding.finding_endpoints (
    finding_id  UUID NOT NULL REFERENCES finding.findings(id) ON DELETE CASCADE,
    endpoint_id UUID NOT NULL REFERENCES finding.endpoints(id) ON DELETE CASCADE,
    PRIMARY KEY (finding_id, endpoint_id)
);

-- finding.risk_acceptances
CREATE TABLE finding.risk_acceptances (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(100) NOT NULL,
    recommendation      TEXT,
    recommendation_details TEXT,
    decision            VARCHAR(20),
    decision_details    TEXT,
    accepted_by_id      UUID REFERENCES auth.users(id),
    owner_id            UUID NOT NULL REFERENCES auth.users(id),
    expiration_date     DATE,
    reactivate_expired  BOOLEAN NOT NULL DEFAULT TRUE,
    restart_sla         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- finding.risk_acceptance_findings
CREATE TABLE finding.risk_acceptance_findings (
    risk_acceptance_id  UUID NOT NULL REFERENCES finding.risk_acceptances(id) ON DELETE CASCADE,
    finding_id          UUID NOT NULL REFERENCES finding.findings(id) ON DELETE CASCADE,
    PRIMARY KEY (risk_acceptance_id, finding_id)
);

-- finding.sla_configurations
CREATE TABLE finding.sla_configurations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(100) NOT NULL,
    description         TEXT,
    critical_days       INT NOT NULL DEFAULT 7,
    high_days           INT NOT NULL DEFAULT 30,
    medium_days         INT NOT NULL DEFAULT 90,
    low_days            INT NOT NULL DEFAULT 120,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- finding.notes
CREATE TABLE finding.notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES finding.findings(id) ON DELETE CASCADE,
    author_id   UUID NOT NULL REFERENCES auth.users(id),
    entry       TEXT NOT NULL,
    note_type   VARCHAR(50),
    edited      BOOLEAN NOT NULL DEFAULT FALSE,
    private     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Notification Schema

```sql
-- notification.alert_rules
CREATE TABLE notification.alert_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    event_type      VARCHAR(50) NOT NULL,  -- 'finding.created', 'sla.breach', etc.
    severity_filter TEXT[],               -- filter on severity
    product_id      UUID,                 -- NULL = all products
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- notification.subscriptions
CREATE TABLE notification.subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID NOT NULL REFERENCES notification.alert_rules(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id),
    channel         VARCHAR(20) NOT NULL,  -- 'email', 'slack', 'webhook', 'teams'
    destination     TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- notification.alerts (user inbox)
CREATE TABLE notification.alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES auth.users(id),
    title           VARCHAR(250) NOT NULL,
    description     TEXT,
    url             VARCHAR(2000),
    icon            VARCHAR(100),
    source          VARCHAR(50),
    read            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Integration Schema

```sql
-- integration.jira_instances
CREATE TABLE integration.jira_instances (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url                 VARCHAR(2000) NOT NULL,
    username            VARCHAR(200) NOT NULL,
    password_encrypted  BYTEA NOT NULL,
    default_issue_type  VARCHAR(100) NOT NULL DEFAULT 'Bug',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- integration.jira_projects
CREATE TABLE integration.jira_projects (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jira_instance_id    UUID NOT NULL REFERENCES integration.jira_instances(id),
    product_id          UUID NOT NULL REFERENCES product.products(id),
    jira_project_key    VARCHAR(20) NOT NULL,
    issue_template      TEXT,
    push_all_issues     BOOLEAN NOT NULL DEFAULT FALSE,
    push_notes          BOOLEAN NOT NULL DEFAULT FALSE,
    send_sla_notifications BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- integration.jira_issues
CREATE TABLE integration.jira_issues (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jira_id         VARCHAR(200) NOT NULL,
    jira_key        VARCHAR(200) NOT NULL UNIQUE,
    jira_creation   TIMESTAMPTZ,
    jira_change     TIMESTAMPTZ,
    finding_id      UUID REFERENCES finding.findings(id) ON DELETE SET NULL,
    jira_project_id UUID REFERENCES integration.jira_projects(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Vulnerability Schema

```sql
-- vulnerability.vulnerabilities (CVE/OSV database mirror)
CREATE TABLE vulnerability.vulnerabilities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vuln_id         VARCHAR(50) UNIQUE NOT NULL,  -- CVE-2024-xxxx or GHSA-xxxx
    summary         TEXT,
    detail          TEXT,
    severity        VARCHAR(10),
    cvss_v3         VARCHAR(117),
    cvss_v3_score   DECIMAL(3,1),
    cvss_v4         VARCHAR(117),
    cvss_v4_score   DECIMAL(3,1),
    cwe_ids         INT[],
    affected_pkgs   JSONB,  -- [{ecosystem, name, ranges}]
    references      JSONB,
    source          VARCHAR(20),  -- 'nvd', 'osv', 'ghsa'
    published_at    TIMESTAMPTZ,
    modified_at     TIMESTAMPTZ,
    ingested_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vuln_cvss_score ON vulnerability.vulnerabilities(cvss_v3_score DESC);
CREATE INDEX idx_vuln_published  ON vulnerability.vulnerabilities(published_at DESC);
CREATE INDEX idx_vuln_affected   ON vulnerability.vulnerabilities USING gin(affected_pkgs);
```

## Migration Strategy

```
Chạy migrations theo thứ tự dependency:
1. auth schema (no deps)
2. product schema (depends: auth.users)
3. finding schema (depends: product.tests, product.engagements, auth.users)
4. notification schema (depends: auth.users)
5. integration schema (depends: product.products, finding.findings)
6. vulnerability schema (no deps)
7. report schema (no deps — report results are generated, not stored long-term)
```

### Migration Runner

```go
// apps/DefectDojo/internal/migration/runner.go

package migration

import (
    "context"
    "fmt"
    
    "github.com/jackc/pgx/v5/pgxpool"
)

type MigrationSet struct {
    Schema    string
    MigrDir   string  // path to migrations dir
}

var migrationOrder = []MigrationSet{
    {Schema: "auth",          MigrDir: "../../services/auth-service/migrations"},
    {Schema: "product",       MigrDir: "../../services/product-service/migrations"},
    {Schema: "finding",       MigrDir: "../../services/finding-service/migrations"},
    {Schema: "scan",          MigrDir: "../../services/scan-service/migrations"},
    {Schema: "notification",  MigrDir: "../../services/notification-service/migrations"},
    {Schema: "report",        MigrDir: "../../services/report-service/migrations"},
    {Schema: "integration",   MigrDir: "../../services/integration-service/migrations"},
    {Schema: "vulnerability",  MigrDir: "../../services/vulnerability-service/migrations"},
}

func RunAll(ctx context.Context, pool *pgxpool.Pool) error {
    for _, set := range migrationOrder {
        if err := RunSchema(ctx, pool, set); err != nil {
            return fmt.Errorf("migration %s: %w", set.Schema, err)
        }
    }
    return nil
}
```
