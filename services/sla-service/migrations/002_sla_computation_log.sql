-- Migration 002: SLA computation log + finding SLA assignments

BEGIN;

-- ── SLA Computation Log ───────────────────────────────────────────────────────
-- Records every time an SLA expiration date is computed or updated.
-- Used for audit trail and debugging.

CREATE TABLE IF NOT EXISTS sla_computation_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL,
    product_id UUID NOT NULL,
    sla_configuration_id UUID NOT NULL REFERENCES sla_configurations(id),
    severity VARCHAR(20) NOT NULL,
    sla_days INT NOT NULL,
    found_date DATE NOT NULL,
    computed_expiry DATE NOT NULL,
    previous_expiry DATE,           -- NULL if first computation
    trigger_event VARCHAR(50) NOT NULL DEFAULT 'import', -- import|sla_config_change|manual
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sla_log_finding ON sla_computation_log(finding_id, computed_at DESC);
CREATE INDEX IF NOT EXISTS idx_sla_log_product ON sla_computation_log(product_id, computed_at DESC);

-- ── Finding SLA assignments ───────────────────────────────────────────────────
-- Tracks current SLA expiry date for each finding.
-- Synced to finding-service via gRPC BatchUpdateSLADates.

CREATE TABLE IF NOT EXISTS finding_sla_assignments (
    finding_id UUID PRIMARY KEY,
    product_id UUID NOT NULL,
    severity VARCHAR(20) NOT NULL,
    sla_configuration_id UUID NOT NULL REFERENCES sla_configurations(id),
    found_date DATE NOT NULL,
    expiration_date DATE NOT NULL,
    is_breached BOOLEAN NOT NULL DEFAULT FALSE,
    breach_notified_at TIMESTAMPTZ,  -- when breach notification was sent
    last_computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fsa_product ON finding_sla_assignments(product_id);
CREATE INDEX IF NOT EXISTS idx_fsa_expiry ON finding_sla_assignments(expiration_date) WHERE is_breached = FALSE;

COMMIT;
