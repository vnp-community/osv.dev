-- services/search-service/migrations/001_pgvector.up.sql
-- pgvector semantic search table for search-service.
-- ADDITIVE: no existing tables modified.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS cve_embeddings (
    cve_id     VARCHAR(30) PRIMARY KEY,
    embedding  vector(1536),
    model      VARCHAR(100) NOT NULL DEFAULT 'text-embedding-3-small',
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_embed_cosine
    ON cve_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
