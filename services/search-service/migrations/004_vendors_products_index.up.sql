-- Index for vendor/product GIN array filter
CREATE INDEX IF NOT EXISTS idx_cves_vendors  ON cves USING GIN(vendors);
CREATE INDEX IF NOT EXISTS idx_cves_products ON cves USING GIN(products);
