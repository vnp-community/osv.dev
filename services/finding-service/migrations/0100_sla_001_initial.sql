-- SLA service initial schema
BEGIN;

CREATE TABLE sla_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID UNIQUE,  -- NULL = global default
    critical_days INTEGER NOT NULL DEFAULT 7,
    high_days INTEGER NOT NULL DEFAULT 30,
    medium_days INTEGER NOT NULL DEFAULT 90,
    low_days INTEGER NOT NULL DEFAULT 180,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert global default
INSERT INTO sla_configurations (id, product_id, critical_days, high_days, medium_days, low_days)
VALUES (gen_random_uuid(), NULL, 7, 30, 90, 180);

COMMIT;
