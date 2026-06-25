-- Migration: 005_search_taxonomy.up.sql
-- Creates CWE, CAPEC, and CPE vendor/product tables for search-service taxonomy endpoints.
-- Tables are seeded with minimal placeholder data; full data import happens via data-service sync.

-- CWE Weaknesses table
CREATE TABLE IF NOT EXISTS cwe_weaknesses (
    id          VARCHAR(20) PRIMARY KEY,    -- e.g. CWE-79
    name        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    abstraction VARCHAR(50),               -- Class | Base | Variant | Compound
    status      VARCHAR(50),               -- Draft | Stable | Deprecated
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cwe_name ON cwe_weaknesses(lower(name));

-- CAPEC Attack Patterns table
CREATE TABLE IF NOT EXISTS capec_patterns (
    id          VARCHAR(20) PRIMARY KEY,    -- e.g. CAPEC-1
    name        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    severity    VARCHAR(20),
    cwe_ids     TEXT[] NOT NULL DEFAULT '{}',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_capec_name   ON capec_patterns(lower(name));
CREATE INDEX IF NOT EXISTS idx_capec_cweids ON capec_patterns USING GIN(cwe_ids);

-- CPE Dictionary table (vendor/product lookup from NVD CPE data)
-- Populated by NVD data-service sync; empty at first, grows over time.
CREATE TABLE IF NOT EXISTS cpe_dict (
    id          BIGSERIAL PRIMARY KEY,
    vendor      TEXT NOT NULL,
    product     TEXT NOT NULL,
    version     TEXT,
    cpe_uri     TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (vendor, product, version)
);

CREATE INDEX IF NOT EXISTS idx_cpe_vendor  ON cpe_dict(lower(vendor));
CREATE INDEX IF NOT EXISTS idx_cpe_product ON cpe_dict(lower(product));

-- Seed a minimal set of well-known CWE entries for immediate usability
INSERT INTO cwe_weaknesses (id, name, description, abstraction, status) VALUES
    ('CWE-79',  'Cross-site Scripting (XSS)', 'Improper Neutralization of Input During Web Page Generation', 'Base', 'Stable'),
    ('CWE-89',  'SQL Injection', 'Improper Neutralization of Special Elements used in an SQL Command', 'Base', 'Stable'),
    ('CWE-22',  'Path Traversal', 'Improper Limitation of a Pathname to a Restricted Directory', 'Base', 'Stable'),
    ('CWE-78',  'OS Command Injection', 'Improper Neutralization of Special Elements used in an OS Command', 'Base', 'Stable'),
    ('CWE-352', 'Cross-Site Request Forgery (CSRF)', 'The web application does not, or can not, sufficiently verify whether a well-formed, valid, consistent request was intentionally provided by the user.', 'Class', 'Stable'),
    ('CWE-434', 'Unrestricted Upload of File with Dangerous Type', 'Allowing upload of dangerous file types', 'Base', 'Stable'),
    ('CWE-502', 'Deserialization of Untrusted Data', 'Deserializing untrusted data without verification', 'Base', 'Stable'),
    ('CWE-798', 'Use of Hard-coded Credentials', 'Hard-coded credentials in source code', 'Base', 'Stable'),
    ('CWE-306', 'Missing Authentication for Critical Function', 'No authentication check on critical function', 'Base', 'Stable'),
    ('CWE-287', 'Improper Authentication', 'Authentication mechanism improperly implemented', 'Class', 'Stable'),
    ('CWE-200', 'Exposure of Sensitive Information to an Unauthorized Actor', 'Information exposure to unauthorized users', 'Class', 'Stable'),
    ('CWE-400', 'Uncontrolled Resource Consumption', 'Denial of service through resource exhaustion', 'Class', 'Stable'),
    ('CWE-611', 'Improper Restriction of XML External Entity Reference', 'XXE injection vulnerability', 'Base', 'Stable'),
    ('CWE-918', 'Server-Side Request Forgery (SSRF)', 'Server-side forgery of requests to internal resources', 'Base', 'Stable'),
    ('CWE-94',  'Code Injection', 'Improper Control of Generation of Code', 'Class', 'Stable')
ON CONFLICT (id) DO NOTHING;

-- Seed minimal CPE vendor data from known KEV entries
-- Vendors extracted from kev_entries.vendor_project
INSERT INTO cpe_dict (vendor, product)
SELECT DISTINCT
    lower(regexp_replace(vendor_project, '[^a-zA-Z0-9_\-]', '_', 'g')) as vendor,
    lower(regexp_replace(product, '[^a-zA-Z0-9_\-]', '_', 'g')) as product
FROM kev_entries
WHERE vendor_project != '' AND product != ''
ON CONFLICT (vendor, product, version) DO NOTHING;
