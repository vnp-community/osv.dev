-- services/ai-service/migrations/002_vector_store.sql
-- pgvector extension + cve_embeddings table for semantic search.
-- ADDITIVE: 001_epss_scores.sql is unchanged.
-- Requires: pg_vector extension installed (PostgreSQL 14+).

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS cve_embeddings (
    cve_id      VARCHAR(30)      PRIMARY KEY,
    embedding   vector(1536),             -- OpenAI text-embedding-3-small / Vertex text-embedding-004
    model       VARCHAR(100)     NOT NULL DEFAULT 'text-embedding-3-small',
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

-- IVFFlat index for approximate cosine similarity search (fast for >1M rows)
CREATE INDEX IF NOT EXISTS idx_cve_embed_cosine
    ON cve_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
