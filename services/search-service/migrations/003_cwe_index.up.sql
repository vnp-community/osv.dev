-- Index for CWE array filter
CREATE INDEX IF NOT EXISTS idx_cves_cwe ON cves USING GIN(cwe);
