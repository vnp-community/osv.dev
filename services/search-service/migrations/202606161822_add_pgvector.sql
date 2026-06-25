-- Enable pgvector extension (requires PostgreSQL 12+)
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE cves ADD COLUMN IF NOT EXISTS embedding vector(1536);

-- IVFFlat index: approximate nearest neighbor, cosine similarity
CREATE INDEX IF NOT EXISTS idx_cves_embedding
    ON cves USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
